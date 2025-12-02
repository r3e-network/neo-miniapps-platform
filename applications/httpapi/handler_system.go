package httpapi

import (
	"net/http"
	"strings"

	"github.com/R3E-Network/service_layer/applications/system"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

func templateEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}

// systemDescriptors exposes registered service descriptors for introspection.
func (h *handler) systemDescriptors(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.descriptorsSnapshot())
}

// systemDescriptorsHTML renders descriptors in a minimal HTML table for browsers.
func (h *handler) systemDescriptorsHTML(w http.ResponseWriter, r *http.Request) {
	descr := h.descriptorsSnapshot()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<!doctype html><html><head><title>Service Descriptors</title><style>body{font-family:sans-serif;padding:16px;} table{border-collapse:collapse;width:100%;} th,td{border:1px solid #ddd;padding:8px;} th{text-align:left;background:#f5f5f5;}</style></head><body>"))
	_, _ = w.Write([]byte("<h2>Registered Services</h2><table><tr><th>Name</th><th>Domain</th><th>Layer</th><th>Capabilities</th><th>Requires APIs</th><th>Depends On</th></tr>"))
	for _, d := range descr {
		_, _ = w.Write([]byte("<tr><td>" + templateEscape(d.Name) + "</td><td>" + templateEscape(d.Domain) + "</td><td>" + templateEscape(string(d.Layer)) + "</td><td>" + templateEscape(strings.Join(d.Capabilities, ", ")) + "</td><td>" + templateEscape(strings.Join(d.RequiresAPIs, ", ")) + "</td><td>" + templateEscape(strings.Join(d.DependsOn, ", ")) + "</td></tr>"))
	}
	_, _ = w.Write([]byte("</table></body></html>"))
}

func (h *handler) descriptorsSnapshot() []core.Descriptor {
	if h == nil || h.services == nil {
		return nil
	}
	base := h.services.DescriptorSnapshot()
	seen := make(map[string]bool, len(base))
	providers := make([]system.DescriptorProvider, 0, len(base))
	for _, d := range base {
		seen[strings.ToLower(strings.TrimSpace(d.Name))] = true
		providers = append(providers, staticDescriptorProvider{desc: d})
	}
	if h.modulesFn != nil {
		for _, mod := range h.modulesFn() {
			name := strings.TrimSpace(mod.Name)
			if name == "" || seen[strings.ToLower(name)] {
				continue
			}
			providers = append(providers, staticDescriptorProvider{
				desc: core.Descriptor{
					Name:         name,
					Domain:       strings.TrimSpace(mod.Domain),
					Layer:        core.Layer(strings.TrimSpace(mod.Layer)),
					Capabilities: append([]string{}, mod.Capabilities...),
					RequiresAPIs: append([]string{}, mod.RequiresAPIs...),
					DependsOn:    append([]string{}, mod.DependsOn...),
				},
			})
			seen[strings.ToLower(name)] = true
		}
	}
	return system.CollectDescriptors(providers)
}

type staticDescriptorProvider struct{ desc core.Descriptor }

func (s staticDescriptorProvider) Descriptor() core.Descriptor { return s.desc }
