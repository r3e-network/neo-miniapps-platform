package ops

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"
)

func repoRoot(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve current file path via runtime.Caller")
	}

	// test/ops/... -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

func decodeYAMLDocuments(t *testing.T, path string) []any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(false)

	var docs []any
	for {
		var doc any
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if doc == nil {
			continue
		}
		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		t.Fatalf("no YAML documents found in %s", path)
	}

	return docs
}

func asStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		converted := make(map[string]any, len(typed))
		for k, v := range typed {
			key, ok := k.(string)
			if !ok {
				return nil, false
			}
			converted[key] = v
		}
		return converted, true
	default:
		return nil, false
	}
}

func nestedMap(root map[string]any, path ...string) (map[string]any, bool) {
	current := root
	for _, key := range path {
		nextRaw, ok := current[key]
		if !ok {
			return nil, false
		}
		next, ok := asStringMap(nextRaw)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func nestedString(root map[string]any, path ...string) (string, bool) {
	if len(path) == 0 {
		return "", false
	}

	parent, ok := nestedMap(root, path[:len(path)-1]...)
	if !ok {
		return "", false
	}
	value, ok := parent[path[len(path)-1]]
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	return str, ok
}

func nestedStringSlice(root map[string]any, path ...string) ([]string, bool) {
	if len(path) == 0 {
		return nil, false
	}

	parent, ok := nestedMap(root, path[:len(path)-1]...)
	if !ok {
		return nil, false
	}
	value, ok := parent[path[len(path)-1]]
	if !ok {
		return nil, false
	}

	rawSlice, ok := value.([]any)
	if !ok {
		// yaml sometimes decodes into []interface{} (alias of []any), but be defensive.
		rawIface, ok := value.([]interface{})
		if !ok {
			return nil, false
		}
		rawSlice = make([]any, 0, len(rawIface))
		for _, v := range rawIface {
			rawSlice = append(rawSlice, v)
		}
	}

	out := make([]string, 0, len(rawSlice))
	for _, item := range rawSlice {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func TestCoordinatorManifestsParseAsYAML(t *testing.T) {
	root := repoRoot(t)

	files := []string{
		filepath.Join(root, "k8s/marblerun/coordinator.yaml"),
		filepath.Join(root, "k8s/marblerun/overlays/simulation/coordinator.yaml"),
		filepath.Join(root, "k8s/monitoring/prometheus/servicemonitor.yaml"),
	}

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			_ = decodeYAMLDocuments(t, file)
		})
	}
}

func TestCoordinatorDeploymentUsesRecreateStrategy(t *testing.T) {
	root := repoRoot(t)

	files := []string{
		filepath.Join(root, "k8s/marblerun/coordinator.yaml"),
		filepath.Join(root, "k8s/marblerun/overlays/simulation/coordinator.yaml"),
	}

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			docs := decodeYAMLDocuments(t, file)

			found := false
			for _, doc := range docs {
				m, ok := asStringMap(doc)
				if !ok {
					continue
				}
				kind, _ := m["kind"].(string)
				if kind != "Deployment" {
					continue
				}

				name, ok := nestedString(m, "metadata", "name")
				if !ok || name != "coordinator" {
					continue
				}
				namespace, ok := nestedString(m, "metadata", "namespace")
				if !ok || namespace != "marblerun" {
					continue
				}

				found = true
				strategyType, ok := nestedString(m, "spec", "strategy", "type")
				if !ok {
					t.Fatalf("deployment/coordinator missing spec.strategy.type in %s", file)
				}
				if strategyType != "Recreate" {
					t.Fatalf("deployment/coordinator spec.strategy.type = %q, want %q in %s", strategyType, "Recreate", file)
				}
			}

			if !found {
				t.Fatalf("deployment/coordinator not found in %s", file)
			}
		})
	}
}

func TestServiceMonitorTargetsMarblerunNamespace(t *testing.T) {
	root := repoRoot(t)
	file := filepath.Join(root, "k8s/monitoring/prometheus/servicemonitor.yaml")

	docs := decodeYAMLDocuments(t, file)

	found := false
	for _, doc := range docs {
		m, ok := asStringMap(doc)
		if !ok {
			continue
		}
		kind, _ := m["kind"].(string)
		if kind != "ServiceMonitor" {
			continue
		}

		name, ok := nestedString(m, "metadata", "name")
		if !ok || name != "marblerun-coordinator" {
			continue
		}

		found = true
		matchNames, ok := nestedStringSlice(m, "spec", "namespaceSelector", "matchNames")
		if !ok {
			t.Fatalf("servicemonitor/%s missing spec.namespaceSelector.matchNames", name)
		}

		hasMarblerun := false
		for _, n := range matchNames {
			if n == "marblerun" {
				hasMarblerun = true
				break
			}
		}
		if !hasMarblerun {
			t.Fatalf("servicemonitor/%s spec.namespaceSelector.matchNames = %v, want it to contain %q", name, matchNames, "marblerun")
		}
	}

	if !found {
		t.Fatalf("servicemonitor/marblerun-coordinator not found in %s", file)
	}
}

func TestCoordinatorBackupRestoreScriptsHaveValidBashSyntax(t *testing.T) {
	root := repoRoot(t)

	scripts := []string{
		filepath.Join(root, "scripts/coordinator_backup.sh"),
		filepath.Join(root, "scripts/coordinator_restore.sh"),
	}

	for _, script := range scripts {
		t.Run(filepath.Base(script), func(t *testing.T) {
			cmd := exec.Command("bash", "-n", script)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("bash -n %s failed: %v\n%s", script, err, string(out))
			}
		})
	}
}

