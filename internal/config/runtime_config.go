package config

// RuntimeConfig configures integrations that were previously only exposed via
// environment variables (TEE mode, price feed fetchers, etc.).
type RuntimeConfig struct {
	TEE       TEEConfig        `json:"tee"`
	Random    RandomConfig     `json:"random"`
	PriceFeed PriceFeedConfig  `json:"pricefeed"`
	GasBank   GasBankConfig    `json:"gasbank"`
	CRE       CREConfig        `json:"cre"`
	Oracle    OracleConfig     `json:"oracle"`
	DataFeeds DataFeedDefaults `json:"datafeeds"`
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

type CREConfig struct {
	HTTPRunner bool `json:"http_runner" env:"CRE_HTTP_RUNNER"`
}

// OracleConfig tunes request lifecycle handling.
type OracleConfig struct {
	TTLSeconds  int    `json:"ttl_seconds" env:"ORACLE_TTL_SECONDS"`
	MaxAttempts int    `json:"max_attempts" env:"ORACLE_MAX_ATTEMPTS"`
	Backoff     string `json:"backoff" env:"ORACLE_BACKOFF"` // duration string
	DLQEnabled  bool   `json:"dlq_enabled" env:"ORACLE_DLQ_ENABLED"`
	RunnerTokens string `json:"runner_tokens" env:"ORACLE_RUNNER_TOKENS"` // comma separated
}

// DataFeedDefaults controls aggregation/threshold defaults.
type DataFeedDefaults struct {
	MinSigners  int    `json:"min_signers" env:"DATAFEEDS_MIN_SIGNERS"`
	Aggregation string `json:"aggregation" env:"DATAFEEDS_AGGREGATION"` // e.g. "median"
}
