package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/config"
	engine "github.com/R3E-Network/service_layer/internal/engine"
)

// Infrastructure-oriented modules that behave like OS services.

type neoNodeModule struct {
	name    string
	domain  string
	rpcURL  string
	network string
}

func (m neoNodeModule) Name() string              { return m.name }
func (m neoNodeModule) Domain() string            { return m.domain }
func (m neoNodeModule) LedgerInfo() string        { return m.network }
func (neoNodeModule) Start(context.Context) error { return nil }
func (neoNodeModule) Stop(context.Context) error  { return nil }
func (m neoNodeModule) Ready(ctx context.Context) error {
	if strings.TrimSpace(m.rpcURL) == "" {
		return fmt.Errorf("neo rpc url not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := 3 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getblockcount",
		"params":  []any{},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.rpcURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build neo rpc request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("neo rpc health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("neo rpc status %d", resp.StatusCode)
	}
	return nil
}
func (m neoNodeModule) APIs() []engine.APIDescriptor {
	return []engine.APIDescriptor{
		{Name: "neo-ledger", Surface: engine.APISurfaceLedger, Summary: "Neo full node (neo-go)", Stability: "stable"},
		{Name: "neo-rpc", Surface: engine.APISurfaceRPC, Summary: "Neo RPC for service engine", Stability: "beta"},
	}
}

type neoIndexerModule struct {
	name   string
	domain string
	url    string
}

func (m neoIndexerModule) Name() string              { return m.name }
func (m neoIndexerModule) Domain() string            { return m.domain }
func (m neoIndexerModule) IndexerInfo() string       { return m.url }
func (neoIndexerModule) Start(context.Context) error { return nil }
func (neoIndexerModule) Stop(context.Context) error  { return nil }
func (m neoIndexerModule) Ready(ctx context.Context) error {
	if strings.TrimSpace(m.url) == "" {
		return fmt.Errorf("neo indexer url not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	if err != nil {
		return fmt.Errorf("build indexer probe: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("indexer health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("indexer health: status %d", resp.StatusCode)
	}
	return nil
}
func (m neoIndexerModule) APIs() []engine.APIDescriptor {
	return []engine.APIDescriptor{
		{Name: "neo-indexer", Surface: engine.APISurfaceIndexer, Summary: "Neo indexer (neo-go, testnet)", Stability: "beta"},
		{Name: "neo-rpc", Surface: engine.APISurfaceRPC, Summary: "Indexer RPC for snapshots/blocks", Stability: "beta"},
	}
}

type chainRPCModule struct {
	name      string
	domain    string
	endpoints map[string]string
}

func (m chainRPCModule) Name() string   { return m.name }
func (m chainRPCModule) Domain() string { return m.domain }
func (chainRPCModule) RPCInfo() string  { return "multi-chain-rpc" }
func (m chainRPCModule) RPCEndpoints() map[string]string {
	return m.endpoints
}
func (chainRPCModule) Start(context.Context) error { return nil }
func (chainRPCModule) Stop(context.Context) error  { return nil }
func (m chainRPCModule) Ready(ctx context.Context) error {
	if len(m.endpoints) == 0 {
		return fmt.Errorf("no chain rpc endpoints configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	client := &http.Client{Timeout: 2 * time.Second}
	for name, url := range m.endpoints {
		url = strings.TrimSpace(url)
		if url == "" {
			return fmt.Errorf("rpc endpoint %q empty", name)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("rpc %s request: %w", name, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("rpc %s health: %w", name, err)
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("rpc %s status %d", name, resp.StatusCode)
		}
	}
	return nil
}
func (m chainRPCModule) APIs() []engine.APIDescriptor {
	return []engine.APIDescriptor{
		{Name: "chain-rpc", Surface: engine.APISurfaceRPC, Summary: "Multi-chain RPC fanout (btc/eth/neox/etc.)", Stability: "beta"},
	}
}

type dataSourceModule struct {
	name    string
	domain  string
	sources map[string]string
}

func (m dataSourceModule) Name() string              { return m.name }
func (m dataSourceModule) Domain() string            { return m.domain }
func (m dataSourceModule) DataSourcesInfo() string   { return fmt.Sprintf("%d sources", len(m.sources)) }
func (dataSourceModule) Start(context.Context) error { return nil }
func (dataSourceModule) Stop(context.Context) error  { return nil }
func (m dataSourceModule) Ready(ctx context.Context) error {
	if len(m.sources) == 0 {
		return fmt.Errorf("no data sources configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	client := &http.Client{Timeout: 2 * time.Second}
	for name, url := range m.sources {
		url = strings.TrimSpace(url)
		if url == "" {
			return fmt.Errorf("data source %q empty url", name)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("data source %s request: %w", name, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("data source %s health: %w", name, err)
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("data source %s status %d", name, resp.StatusCode)
		}
	}
	return nil
}
func (m dataSourceModule) APIs() []engine.APIDescriptor {
	return []engine.APIDescriptor{
		{Name: "data-sources", Surface: engine.APISurfaceDataSource, Summary: "Shared data source hub for feeds/triggers", Stability: "beta"},
	}
}

type contractsModule struct {
	name    string
	domain  string
	network string
}

func (m contractsModule) Name() string              { return m.name }
func (m contractsModule) Domain() string            { return m.domain }
func (m contractsModule) ContractsNetwork() string  { return m.network }
func (contractsModule) Start(context.Context) error { return nil }
func (contractsModule) Stop(context.Context) error  { return nil }
func (m contractsModule) Ready(context.Context) error {
	if strings.TrimSpace(m.network) == "" {
		return fmt.Errorf("contract network not configured")
	}
	return nil
}
func (m contractsModule) APIs() []engine.APIDescriptor {
	return []engine.APIDescriptor{
		{Name: "contracts", Surface: engine.APISurfaceContracts, Summary: "Deploy/invoke service-layer contracts", Stability: "beta"},
	}
}

type serviceBankModule struct {
	name    string
	domain  string
	network string
	limits  map[string]float64
}

func (m serviceBankModule) Name() string   { return m.name }
func (m serviceBankModule) Domain() string { return m.domain }
func (m serviceBankModule) ServiceBankInfo() string {
	if len(m.limits) == 0 {
		return m.network
	}
	return fmt.Sprintf("%s limits", m.network)
}
func (serviceBankModule) Start(context.Context) error { return nil }
func (serviceBankModule) Stop(context.Context) error  { return nil }
func (m serviceBankModule) Ready(context.Context) error {
	if strings.TrimSpace(m.network) == "" {
		return fmt.Errorf("service bank network not configured")
	}
	if len(m.limits) == 0 {
		return fmt.Errorf("service bank limits not configured")
	}
	return nil
}
func (m serviceBankModule) APIs() []engine.APIDescriptor {
	return []engine.APIDescriptor{
		{Name: "gasbank-ops", Surface: engine.APISurfaceGasBank, Summary: "Service-owned GAS metering and control", Stability: "beta"},
	}
}

type cryptoModule struct {
	name         string
	domain       string
	endpoint     string
	capabilities []string
}

func (m cryptoModule) Name() string              { return m.name }
func (m cryptoModule) Domain() string            { return m.domain }
func (m cryptoModule) CryptoInfo() string        { return strings.Join(m.capabilities, ",") }
func (cryptoModule) Start(context.Context) error { return nil }
func (cryptoModule) Stop(context.Context) error  { return nil }
func (m cryptoModule) Ready(ctx context.Context) error {
	if len(m.capabilities) == 0 {
		return fmt.Errorf("crypto capabilities not configured")
	}
	if strings.TrimSpace(m.endpoint) == "" {
		return fmt.Errorf("crypto endpoint not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.endpoint, nil)
	if err != nil {
		return fmt.Errorf("crypto health request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("crypto health: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("crypto health: status %d", resp.StatusCode)
	}
	return nil
}
func (m cryptoModule) APIs() []engine.APIDescriptor {
	return []engine.APIDescriptor{
		{Name: "crypto-kernel", Surface: engine.APISurfaceCrypto, Summary: "Advanced crypto (ZKP/FHE/MPC)", Stability: "alpha"},
	}
}

type moduleMeta struct {
	layer        string
	capabilities []string
	requires     []engine.APISurface
	quotas       map[string]string
	notes        []string
}

func applyModuleMeta(eng *engine.Engine, name string, meta moduleMeta) {
	if eng == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if meta.layer != "" {
		eng.SetModuleLayer(name, meta.layer)
	}
	if len(meta.capabilities) > 0 {
		eng.SetModuleCapabilities(name, meta.capabilities...)
	}
	if len(meta.requires) > 0 {
		eng.SetModuleRequiredAPIs(name, meta.requires...)
	}
	if len(meta.quotas) > 0 {
		eng.SetModuleQuotas(name, meta.quotas)
	}
	for _, note := range meta.notes {
		if n := strings.TrimSpace(note); n != "" {
			eng.AddModuleNote(name, n)
		}
	}
}

// registerInfrastructureModules attaches infrastructure/ledger modules when enabled.
func registerInfrastructureModules(eng *engine.Engine, cfg *config.Config) error {
	if eng == nil || cfg == nil {
		return nil
	}
	if cfg.Runtime.Neo.Enabled {
		node := neoNodeModule{
			name:    "svc-neo-node",
			domain:  "neo",
			rpcURL:  cfg.Runtime.Neo.RPCURL,
			network: cfg.Runtime.Neo.Network,
		}
		if err := eng.Register(node); err != nil {
			return fmt.Errorf("register neo node: %w", err)
		}
		netCap := strings.TrimSpace(cfg.Runtime.Neo.Network)
		nodeCaps := []string{"neo-ledger", "neo-rpc"}
		if netCap != "" {
			nodeCaps = append(nodeCaps, "network:"+strings.ToLower(netCap))
		}
		applyModuleMeta(eng, node.name, moduleMeta{
			layer:        "infra",
			capabilities: nodeCaps,
		})

		indexer := neoIndexerModule{
			name:   "svc-neo-indexer",
			domain: "neo",
			url:    cfg.Runtime.Neo.IndexerURL,
		}
		if err := eng.Register(indexer); err != nil {
			return fmt.Errorf("register neo indexer: %w", err)
		}
		indexerCaps := []string{"neo-indexer", "neo-rpc"}
		if netCap != "" {
			indexerCaps = append(indexerCaps, "network:"+strings.ToLower(netCap))
		}
		applyModuleMeta(eng, indexer.name, moduleMeta{
			layer:        "infra",
			capabilities: indexerCaps,
		})
	}
	if cfg.Runtime.Chains.Enabled {
		rpc := chainRPCModule{
			name:      "svc-chain-rpc",
			domain:    "chains",
			endpoints: cfg.Runtime.Chains.Endpoints,
		}
		if err := eng.Register(rpc); err != nil {
			return fmt.Errorf("register chain rpc: %w", err)
		}
		var chainCaps []string
		for name := range cfg.Runtime.Chains.Endpoints {
			name = strings.TrimSpace(strings.ToLower(name))
			if name != "" {
				chainCaps = append(chainCaps, "chain:"+name)
			}
		}
		sort.Strings(chainCaps)
		caps := append([]string{"chain-rpc"}, chainCaps...)
		quotas := map[string]string{}
		if cfg.Runtime.Chains.PerTenantPerMinute > 0 {
			quotas["per_tenant_per_minute"] = strconv.Itoa(cfg.Runtime.Chains.PerTenantPerMinute)
		}
		if cfg.Runtime.Chains.PerTokenPerMinute > 0 {
			quotas["per_token_per_minute"] = strconv.Itoa(cfg.Runtime.Chains.PerTokenPerMinute)
		}
		if cfg.Runtime.Chains.Burst > 0 {
			quotas["burst"] = strconv.Itoa(cfg.Runtime.Chains.Burst)
		}
		var notes []string
		if cfg.Runtime.Chains.RequireTenant {
			notes = append(notes, "tenant required for rpc")
		}
		applyModuleMeta(eng, rpc.name, moduleMeta{
			layer:        "infra",
			capabilities: caps,
			quotas:       quotas,
			notes:        notes,
		})
	}
	if cfg.Runtime.DataSources.Enabled {
		ds := dataSourceModule{
			name:    "svc-data-sources",
			domain:  "data-sources",
			sources: cfg.Runtime.DataSources.Sources,
		}
		if err := eng.Register(ds); err != nil {
			return fmt.Errorf("register data sources: %w", err)
		}
		var caps []string
		for name := range cfg.Runtime.DataSources.Sources {
			name = strings.TrimSpace(strings.ToLower(name))
			if name != "" {
				caps = append(caps, "source:"+name)
			}
		}
		sort.Strings(caps)
		caps = append([]string{"data-sources"}, caps...)
		applyModuleMeta(eng, ds.name, moduleMeta{
			layer:        "infra",
			capabilities: caps,
		})
	}
	if cfg.Runtime.Contracts.Enabled {
		contracts := contractsModule{
			name:    "svc-contracts",
			domain:  "contracts",
			network: cfg.Runtime.Contracts.Network,
		}
		if err := eng.Register(contracts); err != nil {
			return fmt.Errorf("register contracts: %w", err)
		}
		netCap := strings.TrimSpace(cfg.Runtime.Contracts.Network)
		caps := []string{"contracts"}
		if netCap != "" {
			caps = append(caps, "network:"+strings.ToLower(netCap))
		}
		applyModuleMeta(eng, contracts.name, moduleMeta{
			layer:        "infra",
			capabilities: caps,
		})
	}
	if cfg.Runtime.ServiceBank.Enabled {
		bank := serviceBankModule{
			name:    "svc-service-bank",
			domain:  "gasbank",
			network: cfg.Runtime.Neo.Network,
			limits:  cfg.Runtime.ServiceBank.Limits,
		}
		if err := eng.Register(bank); err != nil {
			return fmt.Errorf("register service bank: %w", err)
		}
		quotas := map[string]string{}
		var quotaKeys []string
		for k := range cfg.Runtime.ServiceBank.Limits {
			quotaKeys = append(quotaKeys, k)
		}
		sort.Strings(quotaKeys)
		for _, k := range quotaKeys {
			quotas[k] = strconv.FormatFloat(cfg.Runtime.ServiceBank.Limits[k], 'f', -1, 64)
		}
		applyModuleMeta(eng, bank.name, moduleMeta{
			layer:        "infra",
			capabilities: []string{"service-bank", "gasbank-control"},
			quotas:       quotas,
		})
	}
	if cfg.Runtime.Crypto.Enabled {
		crypto := cryptoModule{
			name:         "svc-crypto",
			domain:       "crypto",
			endpoint:     cfg.Runtime.Crypto.Endpoint,
			capabilities: cfg.Runtime.Crypto.Capabilities,
		}
		if err := eng.Register(crypto); err != nil {
			return fmt.Errorf("register crypto: %w", err)
		}
		var caps []string
		for _, cap := range cfg.Runtime.Crypto.Capabilities {
			if cap = strings.TrimSpace(cap); cap != "" {
				caps = append(caps, cap)
			}
		}
		if len(caps) == 0 {
			caps = []string{"crypto-kernel"}
		}
		applyModuleMeta(eng, crypto.name, moduleMeta{
			layer:        "infra",
			capabilities: caps,
		})
	}
	if cfg.Runtime.RocketMQ.Enabled {
		mod := newRocketMQModule(cfg.Runtime.RocketMQ)
		if err := eng.Register(mod); err != nil {
			return fmt.Errorf("register rocketmq: %w", err)
		}
		var notes []string
		if len(cfg.Runtime.RocketMQ.NameServers) > 0 {
			notes = append(notes, "nameservers: "+strings.Join(cfg.Runtime.RocketMQ.NameServers, ","))
		}
		if prefix := strings.TrimSpace(cfg.Runtime.RocketMQ.TopicPrefix); prefix != "" {
			notes = append(notes, "topic prefix: "+prefix)
		}
		if group := strings.TrimSpace(cfg.Runtime.RocketMQ.ConsumerGroup); group != "" {
			notes = append(notes, "consumer group: "+group)
		}
		if ns := strings.TrimSpace(cfg.Runtime.RocketMQ.Namespace); ns != "" {
			notes = append(notes, "namespace: "+ns)
		}
		if cfg.Runtime.RocketMQ.MaxReconsume > 0 {
			notes = append(notes, fmt.Sprintf("max reconsume: %d", cfg.Runtime.RocketMQ.MaxReconsume))
		}
		if cfg.Runtime.RocketMQ.ConsumeBatch > 0 {
			notes = append(notes, fmt.Sprintf("consume batch: %d", cfg.Runtime.RocketMQ.ConsumeBatch))
		}
		if cf := strings.TrimSpace(cfg.Runtime.RocketMQ.ConsumeFrom); cf != "" {
			notes = append(notes, "consume from: "+cf)
		}
		applyModuleMeta(eng, mod.Name(), moduleMeta{
			layer:        "infra",
			capabilities: []string{"rocketmq", "events"},
			notes:        notes,
		})
	}
	return nil
}
