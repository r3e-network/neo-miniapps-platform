package engine

import (
	"sort"
	"strings"
	"sync"
)

// MetadataManager manages module metadata (notes, capabilities, quotas, layers, etc).
type MetadataManager struct {
	mu          sync.RWMutex
	notes       map[string][]string
	caps        map[string][]string
	quotas      map[string]map[string]string
	reqAPIs     map[string][]APISurface
	layers      map[string]string
}

// NewMetadataManager creates a new metadata manager.
func NewMetadataManager() *MetadataManager {
	return &MetadataManager{
		notes:   make(map[string][]string),
		caps:    make(map[string][]string),
		quotas:  make(map[string]map[string]string),
		reqAPIs: make(map[string][]APISurface),
		layers:  make(map[string]string),
	}
}

// AddNote attaches a note to a module for observability.
func (m *MetadataManager) AddNote(name, note string) {
	if m == nil {
		return
	}
	name = trimSpace(name)
	note = trimSpace(note)
	if name == "" || note == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.notes[name] = append(m.notes[name], note)
}

// GetNotes returns all notes for a module.
func (m *MetadataManager) GetNotes(name string) []string {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.notes[name]...)
}

// SetCapabilities records declared capabilities for a module.
func (m *MetadataManager) SetCapabilities(name string, caps ...string) {
	if m == nil {
		return
	}
	name = trimSpace(name)
	if name == "" {
		return
	}

	var cleaned []string
	seen := make(map[string]bool)
	for _, c := range caps {
		c = trimSpace(c)
		if c == "" || seen[strings.ToLower(c)] {
			continue
		}
		seen[strings.ToLower(c)] = true
		cleaned = append(cleaned, c)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.caps[name] = cleaned
}

// GetCapabilities returns capabilities for a module.
func (m *MetadataManager) GetCapabilities(name string) []string {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.caps[name]...)
}

// SetQuotas records declared quotas for a module.
func (m *MetadataManager) SetQuotas(name string, quotas map[string]string) {
	if m == nil {
		return
	}
	name = trimSpace(name)
	if name == "" {
		return
	}

	clean := make(map[string]string)
	for k, v := range quotas {
		k = trimSpace(k)
		v = trimSpace(v)
		if k == "" || v == "" {
			continue
		}
		clean[k] = v
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if len(clean) == 0 {
		delete(m.quotas, name)
		return
	}
	m.quotas[name] = clean
}

// GetQuotas returns quotas for a module.
func (m *MetadataManager) GetQuotas(name string) map[string]string {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range m.quotas[name] {
		result[k] = v
	}
	return result
}

// SetRequiredAPIs records declared required API surfaces for a module.
func (m *MetadataManager) SetRequiredAPIs(name string, surfaces ...APISurface) {
	if m == nil {
		return
	}
	name = trimSpace(name)
	if name == "" {
		return
	}

	seen := make(map[string]bool)
	var cleaned []APISurface
	for _, s := range surfaces {
		key := trimSpace(strings.ToLower(string(s)))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, s)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if len(cleaned) == 0 {
		delete(m.reqAPIs, name)
		return
	}
	m.reqAPIs[name] = cleaned
}

// GetRequiredAPIs returns required APIs for a module.
func (m *MetadataManager) GetRequiredAPIs(name string) []APISurface {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]APISurface{}, m.reqAPIs[name]...)
}

// SetLayer records an optional layer hint for a module.
func (m *MetadataManager) SetLayer(name, layer string) {
	if m == nil {
		return
	}
	name = trimSpace(name)
	layer = trimSpace(layer)
	if name == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if layer == "" {
		delete(m.layers, name)
		return
	}
	m.layers[name] = layer
}

// GetLayer returns the layer for a module.
func (m *MetadataManager) GetLayer(name string) string {
	if m == nil {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.layers[name]
}

// ModuleInfo describes a registered module with its domain and inferred category.
type ModuleInfo struct {
	Name         string            `json:"name"`
	Domain       string            `json:"domain,omitempty"`
	Category     string            `json:"category,omitempty"`
	Layer        string            `json:"layer,omitempty"`
	Interfaces   []string          `json:"interfaces,omitempty"`
	APIs         []APIDescriptor   `json:"apis,omitempty"`
	Notes        []string          `json:"notes,omitempty"`
	Permissions  []string          `json:"permissions,omitempty"`
	DependsOn    []string          `json:"depends_on,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Quotas       map[string]string `json:"quotas,omitempty"`
	RequiresAPIs []APISurface      `json:"requires_apis,omitempty"`
}

// BuildModuleInfo constructs module info for a registered module.
func (m *MetadataManager) BuildModuleInfo(
	mod ServiceModule,
	perms BusPermissions,
	deps []string,
	availableAPIs map[string]bool,
) ModuleInfo {
	name := mod.Name()
	ifaces := enumerateInterfaces(mod)

	// Build permission strings
	var permStrings []string
	if perms.AllowEvents {
		if _, ok := mod.(EventEngine); ok {
			permStrings = append(permStrings, "events")
		}
	}
	if perms.AllowData {
		if _, ok := mod.(DataEngine); ok {
			permStrings = append(permStrings, "data")
		}
	}
	if perms.AllowCompute {
		if _, ok := mod.(ComputeEngine); ok {
			permStrings = append(permStrings, "compute")
		}
	}

	// Build APIs
	apis := standardAPIs(mod, perms)
	if described, ok := mod.(APIDescriber); ok {
		apis = mergeAPIs(apis, described.APIs())
	}

	// Get metadata
	m.mu.RLock()
	notes := append([]string{}, m.notes[name]...)
	caps := append([]string{}, m.caps[name]...)
	quotas := make(map[string]string, len(m.quotas[name]))
	for k, v := range m.quotas[name] {
		quotas[k] = v
	}
	reqAPIs := append([]APISurface{}, m.reqAPIs[name]...)
	layer := m.layers[name]
	m.mu.RUnlock()

	// Add infrastructure notes
	notes = m.addInfrastructureNotes(mod, notes)

	// Check for missing required APIs
	for _, req := range reqAPIs {
		surf := trimSpace(strings.ToLower(string(req)))
		if surf == "" {
			continue
		}
		if !availableAPIs[surf] {
			notes = append(notes, "requires api "+surf+" (missing)")
		}
	}

	if layer == "" {
		layer = "service"
	}

	return ModuleInfo{
		Name:         name,
		Domain:       mod.Domain(),
		Category:     classify(mod),
		Layer:        layer,
		Interfaces:   ifaces,
		APIs:         apis,
		Notes:        notes,
		Permissions:  permStrings,
		DependsOn:    deps,
		Capabilities: caps,
		Quotas:       quotas,
		RequiresAPIs: reqAPIs,
	}
}

// addInfrastructureNotes adds notes based on infrastructure interfaces.
func (m *MetadataManager) addInfrastructureNotes(mod ServiceModule, notes []string) []string {
	// RPC endpoints
	if rpcer, ok := mod.(interface{ RPCEndpoints() map[string]string }); ok {
		var chains []string
		for k := range rpcer.RPCEndpoints() {
			if k = trimSpace(strings.ToLower(k)); k != "" {
				chains = append(chains, k)
			}
		}
		if len(chains) > 0 {
			sort.Strings(chains)
			notes = append(notes, "rpc endpoints: "+strings.Join(chains, ","))
		}
	}

	// Ledger info
	if le, ok := mod.(LedgerEngine); ok {
		if info := trimSpace(le.LedgerInfo()); info != "" {
			notes = append(notes, "ledger: "+info)
		}
	}

	// Indexer info
	if ie, ok := mod.(IndexerEngine); ok {
		if info := trimSpace(ie.IndexerInfo()); info != "" {
			notes = append(notes, "indexer: "+info)
		}
	}

	// Data sources info
	if ds, ok := mod.(DataSourceEngine); ok {
		if info := trimSpace(ds.DataSourcesInfo()); info != "" {
			notes = append(notes, "data sources: "+info)
		}
	}

	// Contracts info
	if ce, ok := mod.(ContractsEngine); ok {
		if info := trimSpace(ce.ContractsNetwork()); info != "" {
			notes = append(notes, "contracts: "+info)
		}
	}

	// Service bank info
	if sb, ok := mod.(ServiceBankEngine); ok {
		if info := trimSpace(sb.ServiceBankInfo()); info != "" {
			notes = append(notes, "service bank: "+info)
		}
	}

	// Crypto info
	if ce, ok := mod.(CryptoEngine); ok {
		if info := trimSpace(ce.CryptoInfo()); info != "" {
			notes = append(notes, "crypto: "+info)
		}
	}

	return notes
}

// RemoveModule removes all metadata for a module.
func (m *MetadataManager) RemoveModule(name string) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.notes, name)
	delete(m.caps, name)
	delete(m.quotas, name)
	delete(m.reqAPIs, name)
	delete(m.layers, name)
}

// Clear removes all metadata.
func (m *MetadataManager) Clear() {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.notes = make(map[string][]string)
	m.caps = make(map[string][]string)
	m.quotas = make(map[string]map[string]string)
	m.reqAPIs = make(map[string][]APISurface)
	m.layers = make(map[string]string)
}
