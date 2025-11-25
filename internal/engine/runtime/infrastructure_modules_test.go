package runtime

import (
	"testing"

	"github.com/R3E-Network/service_layer/internal/config"
	engine "github.com/R3E-Network/service_layer/internal/engine"
	"strings"
)

func TestRegisterInfrastructureModulesSetsMetadata(t *testing.T) {
	cfg := config.New()
	cfg.Runtime.Neo.Enabled = true
	cfg.Runtime.Neo.RPCURL = "http://neo-rpc"
	cfg.Runtime.Neo.IndexerURL = "http://neo-indexer"
	cfg.Runtime.Neo.Network = "testnet"

	cfg.Runtime.Chains.Enabled = true
	cfg.Runtime.Chains.Endpoints = map[string]string{"eth": "http://eth", "btc": "http://btc"}
	cfg.Runtime.Chains.RequireTenant = true
	cfg.Runtime.Chains.PerTenantPerMinute = 5
	cfg.Runtime.Chains.PerTokenPerMinute = 7
	cfg.Runtime.Chains.Burst = 3

	cfg.Runtime.DataSources.Enabled = true
	cfg.Runtime.DataSources.Sources = map[string]string{"oracle": "https://oracle", "feeds": "https://feeds"}

	cfg.Runtime.Contracts.Enabled = true
	cfg.Runtime.Contracts.Network = "neox"

	cfg.Runtime.ServiceBank.Enabled = true
	cfg.Runtime.ServiceBank.Limits = map[string]float64{"svc-functions": 10.5}

	cfg.Runtime.Crypto.Enabled = true
	cfg.Runtime.Crypto.Endpoint = "http://crypto"
	cfg.Runtime.Crypto.Capabilities = []string{"zkp", "fhe"}

	cfg.Runtime.RocketMQ.Enabled = true
	cfg.Runtime.RocketMQ.NameServers = []string{"127.0.0.1:9876"}
	cfg.Runtime.RocketMQ.TopicPrefix = "sl"
	cfg.Runtime.RocketMQ.ConsumerGroup = "sl-group"
	cfg.Runtime.RocketMQ.Namespace = "dev"
	cfg.Runtime.RocketMQ.MaxReconsume = 16
	cfg.Runtime.RocketMQ.ConsumeBatch = 2
	cfg.Runtime.RocketMQ.ConsumeFrom = "first"

	eng := engine.New()
	if err := registerInfrastructureModules(eng, cfg); err != nil {
		t.Fatalf("register infra modules: %v", err)
	}

	infos := eng.ModulesInfo()

	assertModule := func(name string) engine.ModuleInfo {
		for _, info := range infos {
			if info.Name == name {
				return info
			}
		}
		t.Fatalf("module %s not registered", name)
		return engine.ModuleInfo{}
	}
	containsAll := func(have []string, want ...string) bool {
	outer:
		for _, w := range want {
			for _, h := range have {
				if h == w {
					continue outer
				}
			}
			return false
		}
		return true
	}

	rpc := assertModule("svc-chain-rpc")
	if rpc.Layer != "infra" {
		t.Fatalf("expected chain rpc layer infra, got %q", rpc.Layer)
	}
	if !containsAll(rpc.Capabilities, "chain-rpc", "chain:btc", "chain:eth") {
		t.Fatalf("rpc capabilities missing, got %+v", rpc.Capabilities)
	}
	if rpc.Quotas["per_tenant_per_minute"] != "5" || rpc.Quotas["per_token_per_minute"] != "7" || rpc.Quotas["burst"] != "3" {
		t.Fatalf("unexpected rpc quotas: %+v", rpc.Quotas)
	}
	if len(rpc.Notes) == 0 {
		t.Fatalf("expected rpc module notes when tenant required")
	}

	neoNode := assertModule("svc-neo-node")
	if neoNode.Layer != "infra" || !containsAll(neoNode.Capabilities, "neo-ledger", "neo-rpc", "network:testnet") {
		t.Fatalf("unexpected neo node metadata: %+v", neoNode)
	}

	neoIndexer := assertModule("svc-neo-indexer")
	if neoIndexer.Layer != "infra" || !containsAll(neoIndexer.Capabilities, "neo-indexer", "neo-rpc", "network:testnet") {
		t.Fatalf("unexpected neo indexer metadata: %+v", neoIndexer)
	}

	ds := assertModule("svc-data-sources")
	if ds.Layer != "infra" || !containsAll(ds.Capabilities, "data-sources", "source:feeds", "source:oracle") {
		t.Fatalf("unexpected data source metadata: %+v", ds)
	}

	contracts := assertModule("svc-contracts")
	if contracts.Layer != "infra" || !containsAll(contracts.Capabilities, "contracts", "network:neox") {
		t.Fatalf("unexpected contracts metadata: %+v", contracts)
	}

	bank := assertModule("svc-service-bank")
	if bank.Layer != "infra" || !containsAll(bank.Capabilities, "service-bank", "gasbank-control") {
		t.Fatalf("unexpected service bank metadata: %+v", bank)
	}
	if bank.Quotas["svc-functions"] != "10.5" {
		t.Fatalf("unexpected service bank quotas: %+v", bank.Quotas)
	}

	crypto := assertModule("svc-crypto")
	if crypto.Layer != "infra" || !containsAll(crypto.Capabilities, "zkp", "fhe") {
		t.Fatalf("unexpected crypto metadata: %+v", crypto)
	}

	rocket := assertModule("svc-rocketmq")
	if rocket.Layer != "infra" || !containsAll(rocket.Capabilities, "rocketmq", "events") {
		t.Fatalf("unexpected rocketmq metadata: %+v", rocket)
	}
	foundNote := false
	for _, n := range rocket.Notes {
		if strings.Contains(strings.ToLower(n), "nameservers") {
			foundNote = true
			break
		}
	}
	if !foundNote {
		t.Fatalf("expected rocketmq nameservers note, got %+v", rocket.Notes)
	}
	if !containsAll(rocket.Notes, "consumer group: sl-group") {
		t.Fatalf("expected consumer group note, got %+v", rocket.Notes)
	}
	if !containsAll(rocket.Notes, "max reconsume: 16") {
		t.Fatalf("expected max reconsume note, got %+v", rocket.Notes)
	}
	if !containsAll(rocket.Notes, "consume batch: 2") {
		t.Fatalf("expected consume batch note, got %+v", rocket.Notes)
	}
	if !containsAll(rocket.Notes, "consume from: first") {
		t.Fatalf("expected consume from note, got %+v", rocket.Notes)
	}
}
