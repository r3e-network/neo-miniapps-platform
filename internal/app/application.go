package app

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/metrics"
	"github.com/R3E-Network/service_layer/internal/app/services/accounts"
	automationsvc "github.com/R3E-Network/service_layer/internal/app/services/automation"
	ccipsvc "github.com/R3E-Network/service_layer/internal/app/services/ccip"
	confsvc "github.com/R3E-Network/service_layer/internal/app/services/confidential"
	cresvc "github.com/R3E-Network/service_layer/internal/app/services/cre"
	datafeedsvc "github.com/R3E-Network/service_layer/internal/app/services/datafeeds"
	datalinksvc "github.com/R3E-Network/service_layer/internal/app/services/datalink"
	datastreamsvc "github.com/R3E-Network/service_layer/internal/app/services/datastreams"
	dtasvc "github.com/R3E-Network/service_layer/internal/app/services/dta"
	"github.com/R3E-Network/service_layer/internal/app/services/functions"
	gasbanksvc "github.com/R3E-Network/service_layer/internal/app/services/gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/internal/app/services/oracle"
	pricefeedsvc "github.com/R3E-Network/service_layer/internal/app/services/pricefeed"
	randomsvc "github.com/R3E-Network/service_layer/internal/app/services/random"
	"github.com/R3E-Network/service_layer/internal/app/services/secrets"
	"github.com/R3E-Network/service_layer/internal/app/services/triggers"
	vrfsvc "github.com/R3E-Network/service_layer/internal/app/services/vrf"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	"github.com/R3E-Network/service_layer/internal/app/system"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Stores encapsulates persistence dependencies. Nil stores default to the
// in-memory implementation.
type Stores struct {
	Accounts         storage.AccountStore
	Functions        storage.FunctionStore
	Triggers         storage.TriggerStore
	GasBank          storage.GasBankStore
	Automation       storage.AutomationStore
	PriceFeeds       storage.PriceFeedStore
	DataFeeds        storage.DataFeedStore
	DataStreams      storage.DataStreamStore
	DataLink         storage.DataLinkStore
	DTA              storage.DTAStore
	Confidential     storage.ConfidentialStore
	Oracle           storage.OracleStore
	Secrets          storage.SecretStore
	CRE              storage.CREStore
	CCIP             storage.CCIPStore
	VRF              storage.VRFStore
	WorkspaceWallets storage.WorkspaceWalletStore
}

func (s *Stores) applyDefaults(mem *memory.Store) {
	if s == nil || mem == nil {
		return
	}
	if s.Accounts == nil {
		s.Accounts = mem
	}
	if s.Functions == nil {
		s.Functions = mem
	}
	if s.Triggers == nil {
		s.Triggers = mem
	}
	if s.GasBank == nil {
		s.GasBank = mem
	}
	if s.Automation == nil {
		s.Automation = mem
	}
	if s.PriceFeeds == nil {
		s.PriceFeeds = mem
	}
	if s.DataFeeds == nil {
		s.DataFeeds = mem
	}
	if s.DataStreams == nil {
		s.DataStreams = mem
	}
	if s.DataLink == nil {
		s.DataLink = mem
	}
	if s.DTA == nil {
		s.DTA = mem
	}
	if s.Confidential == nil {
		s.Confidential = mem
	}
	if s.Oracle == nil {
		s.Oracle = mem
	}
	if s.Secrets == nil {
		s.Secrets = mem
	}
	if s.CRE == nil {
		s.CRE = mem
	}
	if s.CCIP == nil {
		s.CCIP = mem
	}
	if s.VRF == nil {
		s.VRF = mem
	}
	if s.WorkspaceWallets == nil {
		s.WorkspaceWallets = mem
	}
}

// RuntimeConfig captures environment-dependent wiring that was previously
// sourced directly from OS variables. It allows callers to supply explicit
// configuration when embedding the application or running tests.
type RuntimeConfig struct {
	TEEMode             string
	RandomSigningKey    string
	CREHTTPRunner       bool
	PriceFeedFetchURL   string
	PriceFeedFetchKey   string
	GasBankResolverURL  string
	GasBankResolverKey  string
	GasBankPollInterval string
	GasBankMaxAttempts  int
	OracleTTLSeconds    int
	OracleMaxAttempts   int
	OracleBackoff       string
	OracleDLQEnabled    bool
	OracleRunnerTokens  string
	DataFeedMinSigners  int
	DataFeedAggregation string
}

// Option customises the application runtime.
type Option func(*builderConfig)

// Environment exposes a simple lookup mechanism which callers can implement to
// inject custom environment sources (for example when testing).
type Environment interface {
	Lookup(key string) string
}

type builderConfig struct {
	httpClient     *http.Client
	environment    Environment
	runtime        RuntimeConfig
	runtimeDefined bool
}

type resolvedBuilder struct {
	httpClient *http.Client
	runtime    runtimeSettings
}

type runtimeSettings struct {
	teeMode             string
	randomSigningKey    string
	creHTTPRunner       bool
	priceFeedFetchURL   string
	priceFeedFetchKey   string
	gasBankResolverURL  string
	gasBankResolverKey  string
	gasBankPollInterval time.Duration
	gasBankMaxAttempts  int
	oracleTTL           time.Duration
	oracleMaxAttempts   int
	oracleBackoff       time.Duration
	oracleDLQ           bool
	oracleRunnerTokens  []string
	dataFeedMinSigners  int
	dataFeedAggregation string
}

// WithRuntimeConfig overrides the runtime configuration used when wiring
// services. When omitted, environment variables are consulted.
func WithRuntimeConfig(cfg RuntimeConfig) Option {
	return func(b *builderConfig) {
		b.runtime = cfg
		b.runtimeDefined = true
	}
}

// WithHTTPClient injects a shared HTTP client used by background services. A
// nil client falls back to the default 10-second timeout client.
func WithHTTPClient(client *http.Client) Option {
	return func(b *builderConfig) {
		b.httpClient = client
	}
}

// WithEnvironment provides a custom environment lookup used when no explicit
// runtime configuration was supplied. Passing nil retains the default.
func WithEnvironment(env Environment) Option {
	return func(b *builderConfig) {
		if env != nil {
			b.environment = env
		}
	}
}

// Application ties domain services together and manages their lifecycle.
type Application struct {
	manager *system.Manager
	log     *logger.Logger

	Accounts           *accounts.Service
	Functions          *functions.Service
	Triggers           *triggers.Service
	GasBank            *gasbanksvc.Service
	Automation         *automationsvc.Service
	PriceFeeds         *pricefeedsvc.Service
	DataFeeds          *datafeedsvc.Service
	DataStreams        *datastreamsvc.Service
	DataLink           *datalinksvc.Service
	DTA                *dtasvc.Service
	Confidential       *confsvc.Service
	Oracle             *oraclesvc.Service
	Secrets            *secrets.Service
	Random             *randomsvc.Service
	CRE                *cresvc.Service
	CCIP               *ccipsvc.Service
	VRF                *vrfsvc.Service
	OracleRunnerTokens []string
	WorkspaceWallets   storage.WorkspaceWalletStore

	descriptors []core.Descriptor
}

// New builds a fully initialised application with the provided stores.
func New(stores Stores, log *logger.Logger, opts ...Option) (*Application, error) {
	options := resolveBuilderOptions(opts...)
	if log == nil {
		log = logger.NewDefault("app")
	}

	mem := memory.New()
	stores.applyDefaults(mem)

	manager := system.NewManager()

	acctService := accounts.New(stores.Accounts, log)
	funcService := functions.New(stores.Accounts, stores.Functions, log)
	secretsService := secrets.New(stores.Accounts, stores.Secrets, log)
	var executor functions.FunctionExecutor
	switch options.runtime.teeMode {
	case "mock", "disabled", "off":
		log.Warn("TEE mode set to mock; using mock TEE executor")
		executor = functions.NewMockTEEExecutor()
	default:
		executor = functions.NewTEEExecutor(secretsService)
	}
	funcService.AttachExecutor(executor)
	funcService.AttachSecretResolver(secretsService)
	trigService := triggers.New(stores.Accounts, stores.Functions, stores.Triggers, log)
	gasService := gasbanksvc.New(stores.Accounts, stores.GasBank, log)
	automationService := automationsvc.New(stores.Accounts, stores.Functions, stores.Automation, log)
	priceFeedService := pricefeedsvc.New(stores.Accounts, stores.PriceFeeds, log)
	priceFeedService.WithObservationHooks(metrics.PriceFeedSubmissionHooks())
	dataFeedService := datafeedsvc.New(stores.Accounts, stores.DataFeeds, log)
	dataFeedService.WithAggregationConfig(options.runtime.dataFeedMinSigners, options.runtime.dataFeedAggregation)
	dataFeedService.WithObservationHooks(metrics.DataFeedUpdateHooks())
	dataFeedService.WithWorkspaceWallets(stores.WorkspaceWallets)
	dataStreamService := datastreamsvc.New(stores.Accounts, stores.DataStreams, log)
	dataStreamService.WithObservationHooks(metrics.DatastreamFrameHooks())
	dataLinkService := datalinksvc.New(stores.Accounts, stores.DataLink, log)
	dataLinkService.WithWorkspaceWallets(stores.WorkspaceWallets)
	dataLinkService.WithDispatcherHooks(metrics.DataLinkDispatchHooks())
	dtaService := dtasvc.New(stores.Accounts, stores.DTA, log)
	dtaService.WithWorkspaceWallets(stores.WorkspaceWallets)
	dtaService.WithObservationHooks(metrics.DTAOrderHooks())
	confService := confsvc.New(stores.Accounts, stores.Confidential, log)
	confService.WithSealedKeyHooks(metrics.ConfidentialSealedKeyHooks())
	confService.WithAttestationHooks(metrics.ConfidentialAttestationHooks())
	oracleService := oraclesvc.New(stores.Accounts, stores.Oracle, log)
	creService := cresvc.New(stores.Accounts, stores.CRE, log)
	ccipService := ccipsvc.New(stores.Accounts, stores.CCIP, log)
	ccipService.WithWorkspaceWallets(stores.WorkspaceWallets)
	ccipService.WithDispatcherHooks(metrics.CCIPDispatchHooks())
	vrfService := vrfsvc.New(stores.Accounts, stores.VRF, log)
	vrfService.WithWorkspaceWallets(stores.WorkspaceWallets)
	vrfService.WithDispatcherHooks(metrics.VRFDispatchHooks())

	var randomOpts []randomsvc.Option
	if key := options.runtime.randomSigningKey; key != "" {
		if decoded, err := decodeSigningKey(key); err != nil {
			log.WithError(err).Warn("configure random signing key")
		} else {
			randomOpts = append(randomOpts, randomsvc.WithSigningKey(decoded))
		}
	}
	randomService := randomsvc.New(stores.Accounts, log, randomOpts...)

	httpClient := options.httpClient

	funcService.AttachDependencies(trigService, automationService, priceFeedService, oracleService, gasService)

	if options.runtime.creHTTPRunner {
		creService.WithRunner(cresvc.NewHTTPRunner(httpClient, log))
	}

	for _, name := range []string{"accounts", "functions", "triggers"} {
		if err := manager.Register(system.NoopService{ServiceName: name}); err != nil {
			return nil, fmt.Errorf("register %s service: %w", name, err)
		}
	}

	autoRunner := automationsvc.NewScheduler(automationService, log)
	autoRunner.WithDispatcher(automationsvc.NewFunctionDispatcher(automationsvc.FunctionRunnerFunc(func(ctx context.Context, functionID string, payload map[string]any) (function.Execution, error) {
		return funcService.Execute(ctx, functionID, payload)
	}), automationService, log))
	priceRunner := pricefeedsvc.NewRefresher(priceFeedService, log)
	priceRunner.WithObservationHooks(metrics.PriceFeedRefreshHooks())
	if endpoint := options.runtime.priceFeedFetchURL; endpoint != "" {
		fetcher, err := pricefeedsvc.NewHTTPFetcher(httpClient, endpoint, options.runtime.priceFeedFetchKey, log)
		if err != nil {
			log.WithError(err).Warn("configure price feed fetcher")
		} else {
			priceRunner.WithFetcher(fetcher)
		}
	} else {
		log.Warn("price feed fetch URL not configured; price feed refresher disabled")
	}

	oracleRunner := oraclesvc.NewDispatcher(oracleService, log)
	oracleRunner.WithResolver(oraclesvc.NewHTTPResolver(oracleService, httpClient, log))
	oracleRunner.WithRetryPolicy(options.runtime.oracleMaxAttempts, options.runtime.oracleBackoff, options.runtime.oracleTTL)
	oracleRunner.EnableDeadLetter(options.runtime.oracleDLQ)

	var settlement system.Service
	if endpoint := options.runtime.gasBankResolverURL; endpoint != "" {
		resolver, err := gasbanksvc.NewHTTPWithdrawalResolver(httpClient, endpoint, options.runtime.gasBankResolverKey, log)
		if err != nil {
			log.WithError(err).Warn("configure gas bank resolver")
		} else {
			poller := gasbanksvc.NewSettlementPoller(stores.GasBank, gasService, resolver, log)
			poller.WithObservationHooks(metrics.GasBankSettlementHooks())
			poller.WithRetryPolicy(options.runtime.gasBankMaxAttempts, options.runtime.gasBankPollInterval)
			settlement = poller
		}
	} else {
		log.Warn("gas bank resolver URL not configured; gas bank settlement disabled")
	}

	services := []system.Service{autoRunner, priceRunner, oracleRunner}
	if settlement != nil {
		services = append(services, settlement)
	}

	for _, svc := range services {
		if err := manager.Register(svc); err != nil {
			return nil, fmt.Errorf("register %s: %w", svc.Name(), err)
		}
	}

	descriptors := manager.Descriptors()

	return &Application{
		manager:            manager,
		log:                log,
		Accounts:           acctService,
		Functions:          funcService,
		Triggers:           trigService,
		GasBank:            gasService,
		Automation:         automationService,
		PriceFeeds:         priceFeedService,
		DataFeeds:          dataFeedService,
		DataStreams:        dataStreamService,
		DataLink:           dataLinkService,
		Oracle:             oracleService,
		Secrets:            secretsService,
		Random:             randomService,
		CRE:                creService,
		CCIP:               ccipService,
		VRF:                vrfService,
		DTA:                dtaService,
		Confidential:       confService,
		OracleRunnerTokens: options.runtime.oracleRunnerTokens,
		WorkspaceWallets:   stores.WorkspaceWallets,
		descriptors:        descriptors,
	}, nil
}

// Attach registers an additional lifecycle-managed service. Call before Start.
func (a *Application) Attach(service system.Service) error {
	return a.manager.Register(service)
}

// Start begins all registered services.
func (a *Application) Start(ctx context.Context) error {
	return a.manager.Start(ctx)
}

// Stop stops all services.
func (a *Application) Stop(ctx context.Context) error {
	return a.manager.Stop(ctx)
}

// Descriptors returns advertised service descriptors for orchestration/CLI
// introspection. It is safe to call even if some services are nil.
func (a *Application) Descriptors() []core.Descriptor {
	out := make([]core.Descriptor, len(a.descriptors))
	copy(out, a.descriptors)
	return out
}

func resolveBuilderOptions(opts ...Option) resolvedBuilder {
	cfg := builderConfig{environment: osEnvironment{}}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.environment == nil {
		cfg.environment = osEnvironment{}
	}
	if cfg.httpClient == nil {
		cfg.httpClient = defaultHTTPClient()
	}
	runtimeCfg := cfg.runtime
	if !cfg.runtimeDefined {
		runtimeCfg = runtimeConfigFromEnv(cfg.environment)
	}
	return resolvedBuilder{
		httpClient: cfg.httpClient,
		runtime:    normalizeRuntimeConfig(runtimeCfg),
	}
}

func runtimeConfigFromEnv(env Environment) RuntimeConfig {
	if env == nil {
		env = osEnvironment{}
	}
	maxAttempts := 0
	if parsed, ok := parseInt(env.Lookup("GASBANK_MAX_ATTEMPTS")); ok {
		maxAttempts = parsed
	}
	oracleAttempts := 0
	if parsed, ok := parseInt(env.Lookup("ORACLE_MAX_ATTEMPTS")); ok {
		oracleAttempts = parsed
	}
	return RuntimeConfig{
		TEEMode:             env.Lookup("TEE_MODE"),
		RandomSigningKey:    env.Lookup("RANDOM_SIGNING_KEY"),
		PriceFeedFetchURL:   env.Lookup("PRICEFEED_FETCH_URL"),
		PriceFeedFetchKey:   env.Lookup("PRICEFEED_FETCH_KEY"),
		GasBankResolverURL:  env.Lookup("GASBANK_RESOLVER_URL"),
		GasBankResolverKey:  env.Lookup("GASBANK_RESOLVER_KEY"),
		GasBankPollInterval: env.Lookup("GASBANK_POLL_INTERVAL"),
		GasBankMaxAttempts:  maxAttempts,
		CREHTTPRunner:       parseBool(env.Lookup("CRE_HTTP_RUNNER")),
		OracleTTLSeconds:    parseIntOrZero(env.Lookup("ORACLE_TTL_SECONDS")),
		OracleMaxAttempts:   oracleAttempts,
		OracleBackoff:       env.Lookup("ORACLE_BACKOFF"),
		OracleDLQEnabled:    parseBool(env.Lookup("ORACLE_DLQ_ENABLED")),
		OracleRunnerTokens:  env.Lookup("ORACLE_RUNNER_TOKENS"),
		DataFeedMinSigners:  parseIntOrZero(env.Lookup("DATAFEEDS_MIN_SIGNERS")),
		DataFeedAggregation: env.Lookup("DATAFEEDS_AGGREGATION"),
	}
}

func normalizeRuntimeConfig(cfg RuntimeConfig) runtimeSettings {
	pollInterval := 15 * time.Second
	if trimmed := strings.TrimSpace(cfg.GasBankPollInterval); trimmed != "" {
		if parsed, err := time.ParseDuration(trimmed); err == nil && parsed > 0 {
			pollInterval = parsed
		}
	}
	maxAttempts := cfg.GasBankMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	oracleTTL := cfg.OracleTTLSeconds
	if oracleTTL < 0 {
		oracleTTL = 0
	}
	oracleAttempts := cfg.OracleMaxAttempts
	if oracleAttempts <= 0 {
		oracleAttempts = 3
	}
	oracleBackoff := 10 * time.Second
	if trimmed := strings.TrimSpace(cfg.OracleBackoff); trimmed != "" {
		if parsed, err := time.ParseDuration(trimmed); err == nil && parsed > 0 {
			oracleBackoff = parsed
		}
	}
	agg := strings.ToLower(strings.TrimSpace(cfg.DataFeedAggregation))
	if agg == "" {
		agg = "median"
	}
	minSigners := cfg.DataFeedMinSigners
	if minSigners < 0 {
		minSigners = 0
	}
	runnerTokens := parseTokens(cfg.OracleRunnerTokens)
	return runtimeSettings{
		teeMode:             strings.ToLower(strings.TrimSpace(cfg.TEEMode)),
		randomSigningKey:    strings.TrimSpace(cfg.RandomSigningKey),
		priceFeedFetchURL:   strings.TrimSpace(cfg.PriceFeedFetchURL),
		priceFeedFetchKey:   strings.TrimSpace(cfg.PriceFeedFetchKey),
		gasBankResolverURL:  strings.TrimSpace(cfg.GasBankResolverURL),
		gasBankResolverKey:  strings.TrimSpace(cfg.GasBankResolverKey),
		creHTTPRunner:       cfg.CREHTTPRunner,
		gasBankPollInterval: pollInterval,
		gasBankMaxAttempts:  maxAttempts,
		oracleTTL:           time.Duration(oracleTTL) * time.Second,
		oracleMaxAttempts:   oracleAttempts,
		oracleBackoff:       oracleBackoff,
		oracleDLQ:           cfg.OracleDLQEnabled,
		oracleRunnerTokens:  runnerTokens,
		dataFeedMinSigners:  minSigners,
		dataFeedAggregation: agg,
	}
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseInt(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseIntOrZero(value string) int {
	if parsed, ok := parseInt(value); ok {
		return parsed
	}
	return 0
}

func parseTokens(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	seen := make(map[string]struct{}, len(parts))
	var result []string
	for _, p := range parts {
		token := strings.TrimSpace(p)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

type osEnvironment struct{}

func (osEnvironment) Lookup(key string) string {
	return os.Getenv(key)
}

func decodeSigningKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("signing key is empty")
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		if len(decoded) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("expected %d-byte key, got %d", ed25519.PrivateKeySize, len(decoded))
		}
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(value); err == nil {
		if len(decoded) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("expected %d-byte key, got %d", ed25519.PrivateKeySize, len(decoded))
		}
		return decoded, nil
	}
	return nil, fmt.Errorf("invalid signing key encoding; provide base64 or hex encoded ed25519 key")
}
