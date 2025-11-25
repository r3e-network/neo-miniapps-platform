package runtime

import (
	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/config"
)

// AppRuntimeConfig converts a config file runtime section into the
// application-level runtime configuration structure.
func AppRuntimeConfig(cfg *config.Config) app.RuntimeConfig {
	if cfg == nil {
		return app.RuntimeConfig{}
	}
	return app.RuntimeConfig{
		TEEMode:                cfg.Runtime.TEE.Mode,
		RandomSigningKey:       cfg.Runtime.Random.SigningKey,
		PriceFeedFetchURL:      cfg.Runtime.PriceFeed.FetchURL,
		PriceFeedFetchKey:      cfg.Runtime.PriceFeed.FetchKey,
		GasBankResolverURL:     cfg.Runtime.GasBank.ResolverURL,
		GasBankResolverKey:     cfg.Runtime.GasBank.ResolverKey,
		GasBankPollInterval:    cfg.Runtime.GasBank.PollInterval,
		GasBankMaxAttempts:     cfg.Runtime.GasBank.MaxAttempts,
		CREHTTPRunner:          cfg.Runtime.CRE.HTTPRunner,
		OracleTTLSeconds:       cfg.Runtime.Oracle.TTLSeconds,
		OracleMaxAttempts:      cfg.Runtime.Oracle.MaxAttempts,
		OracleBackoff:          cfg.Runtime.Oracle.Backoff,
		DataFeedMinSigners:     cfg.Runtime.DataFeeds.MinSigners,
		DataFeedAggregation:    cfg.Runtime.DataFeeds.Aggregation,
		JAMEnabled:             cfg.Runtime.JAM.Enabled,
		JAMStore:               cfg.Runtime.JAM.Store,
		JAMPGDSN:               cfg.Runtime.JAM.PGDSN,
		JAMAuthRequired:        cfg.Runtime.JAM.AuthRequired,
		JAMAllowedTokens:       cfg.Runtime.JAM.AllowedTokens,
		JAMRateLimitPerMin:     cfg.Runtime.JAM.RateLimitPerMinute,
		JAMMaxPreimageBytes:    cfg.Runtime.JAM.MaxPreimageBytes,
		JAMMaxPendingPkgs:      cfg.Runtime.JAM.MaxPendingPackages,
		JAMLegacyList:          cfg.Runtime.JAM.LegacyListResponse,
		JAMAccumulatorsEnabled: cfg.Runtime.JAM.AccumulatorsEnabled,
		JAMAccumulatorHash:     cfg.Runtime.JAM.AccumulatorHash,
	}
}
