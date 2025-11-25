package config

// RuntimeConfig configures integrations that were previously only exposed via
// environment variables (TEE mode, price feed fetchers, etc.).
type RuntimeConfig struct {
	TEE         TEEConfig         `json:"tee"`
	Random      RandomConfig      `json:"random"`
	PriceFeed   PriceFeedConfig   `json:"pricefeed"`
	GasBank     GasBankConfig     `json:"gasbank"`
	ServiceBank ServiceBankConfig `json:"service_bank" mapstructure:"service_bank"`
	CRE         CREConfig         `json:"cre"`
	Oracle      OracleConfig      `json:"oracle"`
	DataFeeds   DataFeedDefaults  `json:"datafeeds"`
	JAM         JAMConfig         `json:"jam"`
	Neo         NeoEngineConfig   `json:"neo"`
	Chains      ChainRPCConfig    `json:"chains"`
	DataSources DataSourceConfig  `json:"data_sources" mapstructure:"data_sources"`
	Contracts   ContractConfig    `json:"contracts"`
	Crypto      CryptoConfig      `json:"crypto"`
	RocketMQ    RocketMQConfig    `json:"rocketmq" mapstructure:"rocketmq"`
	SlowMS      int               `json:"slow_module_threshold_ms" env:"MODULE_SLOW_MS"`
	// BusPermissions allow overriding event/data/compute fan-out per module (name -> permissions).
	BusPermissions map[string]BusPermission `json:"bus_permissions" mapstructure:"bus_permissions"`
	// ModuleDeps declares dependencies between modules (name -> list of required module names).
	ModuleDeps map[string][]string `json:"module_deps" mapstructure:"module_deps"`
	// UnknownModulesStrict controls whether unknown module names in config cause startup failures (default true).
	UnknownModulesStrict bool `json:"unknown_modules_strict" mapstructure:"unknown_modules_strict" env:"UNKNOWN_MODULES_STRICT"`
	// RequireAPIsStrict controls whether missing required API surfaces cause startup failure (default false).
	RequireAPIsStrict bool `json:"require_apis_strict" mapstructure:"require_apis_strict" env:"REQUIRE_APIS_STRICT"`
}

// BusPermission controls which bus fan-outs a module participates in.
type BusPermission struct {
	Events  *bool `json:"events" mapstructure:"events"`
	Data    *bool `json:"data" mapstructure:"data"`
	Compute *bool `json:"compute" mapstructure:"compute"`
}

type TEEConfig struct {
	Mode string `json:"mode" env:"TEE_MODE"`
}

type RandomConfig struct {
	SigningKey string `json:"signing_key" env:"RANDOM_SIGNING_KEY"`
}

type PriceFeedConfig struct {
	FetchURL string `json:"fetch_url" env:"PRICEFEED_FETCH_URL"`
	FetchKey string `json:"fetch_key" env:"PRICEFEED_FETCH_KEY"`
}

type GasBankConfig struct {
	ResolverURL  string `json:"resolver_url" env:"GASBANK_RESOLVER_URL"`
	ResolverKey  string `json:"resolver_key" env:"GASBANK_RESOLVER_KEY"`
	PollInterval string `json:"poll_interval" env:"GASBANK_POLL_INTERVAL"`
	MaxAttempts  int    `json:"max_attempts" env:"GASBANK_MAX_ATTEMPTS"`
}

// ServiceBankConfig governs the service-layer-owned GAS bank.
type ServiceBankConfig struct {
	Enabled bool               `json:"enabled" env:"SERVICE_BANK_ENABLED"`
	Limits  map[string]float64 `json:"limits" mapstructure:"limits"` // module/service -> quota (optional)
}

type CREConfig struct {
	HTTPRunner bool `json:"http_runner" env:"CRE_HTTP_RUNNER"`
}

// OracleConfig tunes request lifecycle handling.
type OracleConfig struct {
	TTLSeconds   int    `json:"ttl_seconds" env:"ORACLE_TTL_SECONDS"`
	MaxAttempts  int    `json:"max_attempts" env:"ORACLE_MAX_ATTEMPTS"`
	Backoff      string `json:"backoff" env:"ORACLE_BACKOFF"` // duration string
	DLQEnabled   bool   `json:"dlq_enabled" env:"ORACLE_DLQ_ENABLED"`
	RunnerTokens string `json:"runner_tokens" env:"ORACLE_RUNNER_TOKENS"` // comma separated
}

// DataFeedDefaults controls aggregation/threshold defaults.
type DataFeedDefaults struct {
	MinSigners  int    `json:"min_signers" env:"DATAFEEDS_MIN_SIGNERS"`
	Aggregation string `json:"aggregation" env:"DATAFEEDS_AGGREGATION"` // e.g. "median"
}

// NeoEngineConfig configures the embedded Neo node/indexer surfaces.
type NeoEngineConfig struct {
	Enabled    bool   `json:"enabled" env:"NEO_ENABLED"`
	RPCURL     string `json:"rpc_url" env:"NEO_RPC_URL"`
	Network    string `json:"network" env:"NEO_NETWORK"`
	IndexerURL string `json:"indexer_url" env:"NEO_INDEXER_URL"`
}

// ChainRPCConfig registers arbitrary chain RPC endpoints (btc/eth/neox/etc.).
type ChainRPCConfig struct {
	Enabled bool `json:"enabled" env:"CHAIN_RPC_ENABLED"`
	// RequireTenant enforces X-Tenant-ID on /system/rpc calls for multi-tenant safety.
	RequireTenant bool `json:"require_tenant" mapstructure:"require_tenant" env:"CHAIN_RPC_REQUIRE_TENANT"`
	// PerTenantPerMinute caps RPC fan-out per tenant. Zero disables the tenant limiter.
	PerTenantPerMinute int `json:"per_tenant_per_minute" mapstructure:"per_tenant_per_minute" env:"CHAIN_RPC_PER_TENANT_PER_MIN"`
	// PerTokenPerMinute caps RPC fan-out per auth token/user. Zero disables the token limiter.
	PerTokenPerMinute int `json:"per_token_per_minute" mapstructure:"per_token_per_minute" env:"CHAIN_RPC_PER_TOKEN_PER_MIN"`
	// Burst controls short-term bursts allowed by the limiter (defaults to 3 when unset).
	Burst int `json:"burst" mapstructure:"burst" env:"CHAIN_RPC_BURST"`
	// AllowedMethods optionally whitelists JSON-RPC methods per chain.
	AllowedMethods map[string][]string `json:"allowed_methods" mapstructure:"allowed_methods"`
	Endpoints      map[string]string   `json:"endpoints" mapstructure:"endpoints"`
}

// DataSourceConfig represents upstream data source adapters (feeds, oracles, triggers).
type DataSourceConfig struct {
	Enabled bool              `json:"enabled" env:"DATA_SOURCES_ENABLED"`
	Sources map[string]string `json:"sources" mapstructure:"sources"`
}

// ContractConfig configures smart contract deployment/invocation defaults.
type ContractConfig struct {
	Enabled bool   `json:"enabled" env:"CONTRACTS_ENABLED"`
	Network string `json:"network" env:"CONTRACTS_NETWORK"`
}

// CryptoConfig enables the crypto engine (ZKP/FHE/MPC helpers).
type CryptoConfig struct {
	Enabled      bool     `json:"enabled" env:"CRYPTO_ENABLED"`
	Endpoint     string   `json:"endpoint" env:"CRYPTO_ENDPOINT"`
	Capabilities []string `json:"capabilities" mapstructure:"capabilities" env:"CRYPTO_CAPABILITIES"`
}

// RocketMQConfig controls the RocketMQ-backed event bus.
type RocketMQConfig struct {
	Enabled       bool     `json:"enabled" env:"ROCKETMQ_ENABLED"`
	NameServers   []string `json:"name_servers" mapstructure:"name_servers" env:"ROCKETMQ_NAME_SERVERS"`
	AccessKey     string   `json:"access_key" env:"ROCKETMQ_ACCESS_KEY"`
	SecretKey     string   `json:"secret_key" env:"ROCKETMQ_SECRET_KEY"`
	TopicPrefix   string   `json:"topic_prefix" mapstructure:"topic_prefix" env:"ROCKETMQ_TOPIC_PREFIX"`
	ConsumerGroup string   `json:"consumer_group" mapstructure:"consumer_group" env:"ROCKETMQ_CONSUMER_GROUP"`
	Namespace     string   `json:"namespace" env:"ROCKETMQ_NAMESPACE"`
	MaxReconsume  int      `json:"max_reconsume_times" mapstructure:"max_reconsume_times" env:"ROCKETMQ_MAX_RECONSUME_TIMES"`
	ConsumeBatch  int      `json:"consume_batch" mapstructure:"consume_batch" env:"ROCKETMQ_CONSUME_BATCH"`
	ConsumeFrom   string   `json:"consume_from" mapstructure:"consume_from" env:"ROCKETMQ_CONSUME_FROM"` // latest|first
}

// JAMConfig controls the experimental JAM HTTP API.
type JAMConfig struct {
	Enabled             bool     `json:"enabled" env:"JAM_ENABLED"`
	Store               string   `json:"store" env:"JAM_STORE"`   // memory (default) or postgres
	PGDSN               string   `json:"pg_dsn" env:"JAM_PG_DSN"` // optional; falls back to DATABASE_DSN
	AuthRequired        bool     `json:"auth_required" env:"JAM_AUTH_REQUIRED"`
	AllowedTokens       []string `json:"allowed_tokens" env:"JAM_ALLOWED_TOKENS"`
	RateLimitPerMinute  int      `json:"rate_limit_per_minute" env:"JAM_RATE_LIMIT_PER_MINUTE"`
	MaxPreimageBytes    int64    `json:"max_preimage_bytes" env:"JAM_MAX_PREIMAGE_BYTES"`
	MaxPendingPackages  int      `json:"max_pending_packages" env:"JAM_MAX_PENDING_PACKAGES"`
	LegacyListResponse  bool     `json:"legacy_list_response" env:"JAM_LEGACY_LIST_RESPONSE"`
	AccumulatorsEnabled bool     `json:"accumulators_enabled" env:"JAM_ACCUMULATORS_ENABLED"`
	AccumulatorHash     string   `json:"accumulator_hash" env:"JAM_ACCUMULATOR_HASH"`
}
