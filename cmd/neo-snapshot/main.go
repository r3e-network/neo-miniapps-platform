package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"archive/tar"
	"compress/gzip"
)

// Config controls snapshot generation for a given block height.
type Config struct {
	RPCURL     string
	Height     int64
	OutputDir  string
	Network    string
	Contracts  []string
}

func main() {
	var cfg Config
	flag.StringVar(&cfg.RPCURL, "rpc", envDefault("NEO_RPC_URL", "http://localhost:10332"), "NEO RPC endpoint")
	flag.Int64Var(&cfg.Height, "height", 0, "block height to snapshot (required)")
	flag.StringVar(&cfg.OutputDir, "out", envDefault("NEO_SNAPSHOT_OUT", "./snapshots"), "output directory for KV bundle + manifest")
	flag.StringVar(&cfg.Network, "network", envDefault("NEO_NETWORK", "mainnet"), "network label (mainnet|testnet)")
	contracts := flag.String("contracts", envDefault("NEO_CONTRACTS", ""), "comma-separated contract hashes to include (optional; empty means skip kv bundle)")
	flag.Parse()

	if cfg.Height <= 0 {
		log.Fatal("height must be > 0")
	}
	if strings.TrimSpace(*contracts) != "" {
		cfg.Contracts = strings.Split(*contracts, ",")
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	manifestPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("block-%d.json", cfg.Height))
	kvPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("block-%d-kv.tar.gz", cfg.Height))

	stateRoot, err := fetchStateRoot(context.Background(), cfg.RPCURL, cfg.Height)
	if err != nil {
		log.Printf("warning: failed to fetch state root (stub placeholder used): %v", err)
		stateRoot = "TODO"
	}

	if len(cfg.Contracts) > 0 {
		if err := writeKVBundle(context.Background(), cfg.RPCURL, cfg.Contracts, kvPath); err != nil {
			log.Printf("warning: failed to build KV bundle: %v", err)
		}
	}

	manifest := fmt.Sprintf(`{"network":"%s","height":%d,"state_root":"%s","kv_path":"%s","generated_at":"%s"}`, cfg.Network, cfg.Height, stateRoot, kvPath, time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		log.Fatalf("write manifest: %v", err)
	}
	log.Printf("snapshot stub written: manifest=%s kv=%s", manifestPath, kvPath)
}

func envDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type stateRoot struct {
	Hash string `json:"hash"`
}

// stateEntry represents a contract storage key/value.
type stateEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func fetchStateRoot(ctx context.Context, rpcURL string, height int64) (string, error) {
	body, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: "getstateroot", Params: []interface{}{height}, ID: 1})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", err
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var sr stateRoot
	if err := json.Unmarshal(rpcResp.Result, &sr); err != nil {
		return "", err
	}
	return sr.Hash, nil
}

func fetchContractStorage(ctx context.Context, rpcURL, contract string) ([]stateEntry, error) {
	var entries []stateEntry
	if err := postRPC(ctx, rpcURL, "getcontractstorage", []interface{}{contract}, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func postRPC(ctx context.Context, rpcURL, method string, params []interface{}, out interface{}) error {
	body, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params, ID: 1})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if out != nil && rpcResp.Result != nil {
		return json.Unmarshal(rpcResp.Result, out)
	}
	return nil
}

func writeKVBundle(ctx context.Context, rpcURL string, contracts []string, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, c := range contracts {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		entries, err := fetchContractStorage(ctx, rpcURL, c)
		if err != nil {
			return fmt.Errorf("fetch contract storage %s: %w", c, err)
		}
		payload, _ := json.Marshal(entries)
		hdr := &tar.Header{
			Name: fmt.Sprintf("%s.json", c),
			Mode: 0o644,
			Size: int64(len(payload)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write(payload); err != nil {
			return err
		}
	}
	return nil
}
