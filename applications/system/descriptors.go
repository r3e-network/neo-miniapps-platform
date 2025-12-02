package system

import (
	"sort"
	"strings"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// CollectDescriptors extracts service descriptors, skipping nil entries, and
// sorts them for deterministic presentation (layer + name).
func CollectDescriptors(providers []DescriptorProvider) []core.Descriptor {
	var out []core.Descriptor
	for _, p := range providers {
		if p == nil {
			continue
		}
		out = append(out, normalizeDescriptor(p.Descriptor()))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Layer == out[j].Layer {
			return out[i].Name < out[j].Name
		}
		return out[i].Layer < out[j].Layer
	})
	return out
}

// normalizeDescriptor trims fields for consistency.
func normalizeDescriptor(d core.Descriptor) core.Descriptor {
	d.Name = strings.TrimSpace(d.Name)
	d.Domain = strings.TrimSpace(d.Domain)
	layer := strings.TrimSpace(string(d.Layer))
	if layer == "" {
		layer = string(core.LayerService)
	}
	d.Layer = core.Layer(layer)
	d.Capabilities = dedupeStrings(d.Capabilities)
	d.RequiresAPIs = dedupeStrings(d.RequiresAPIs)
	d.DependsOn = dedupeStrings(d.DependsOn)
	return d
}

// SortDescriptors sorts descriptors by layer then name for consistent presentation.
func SortDescriptors(descriptors []core.Descriptor) []core.Descriptor {
	sort.SliceStable(descriptors, func(i, j int) bool {
		if descriptors[i].Layer == descriptors[j].Layer {
			return descriptors[i].Name < descriptors[j].Name
		}
		return descriptors[i].Layer < descriptors[j].Layer
	})
	return descriptors
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return out
}
