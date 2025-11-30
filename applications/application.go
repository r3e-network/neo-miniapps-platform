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

	"github.com/R3E-Network/service_layer/pkg/metrics"
	"github.com/R3E-Network/service_layer/pkg/storage"
	"github.com/R3E-Network/service_layer/applications/system"
	"github.com/R3E-Network/service_layer/domain/function"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.accounts"
	automationsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.automation"
	ccipsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.ccip"
	confsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.confidential"
	cresvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.cre"
	datafeedsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datafeeds"
	datalinksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datalink"
	datastreamsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datastreams"
	dtasvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.dta"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.functions"
	gasbanksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.gasbank"
	oraclesvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.secrets"
	vrfsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.vrf"
	"github.com/R3E-Network/service_layer/pkg/logger"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Stores encapsulates persistence dependencies. All stores must be provided.
type Stores struct {
	Accounts         storage.AccountStore
	Functions        storage.FunctionStore
	GasBank          storage.GasBankStore
	Automation       storage.AutomationStore
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

// RuntimeConfig captures environment-dependent wiring that was previously
// sourced directly from OS variables. It allows callers to supply explicit
// configuration when embedding the application or running tests.
type RuntimeConfig struct {
	TEEMode                string
	CREHTTPRunner          bool
	GasBankResolverURL     string
	GasBankResolverKey     string
	GasBankPollInterval    string
	GasBankMaxAttempts     int
	OracleTTLSeconds       int
	OracleMaxAttempts      int
	OracleBackoff          string
	OracleDLQEnabled       bool
	OracleRunnerTokens     string
	DataFeedMinSigners     int
	DataFeedAggregation    string
	JAMEnabled             bool
	JAMStore               string
	JAMPGDSN               string
	JAMAuthRequired        bool
	JAMAllowedTokens       []string
	JAMRateLimitPerMin     int
	JAMMaxPreimageBytes    int64
	JAMMaxPendingPkgs      int
	JAMLegacyList          bool
	JAMAccumulatorsEnabled bool
	JAMAccumulatorHash     string
	BusMaxBytes            int64
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
	managerEnabled bool
}

type resolvedBuilder struct {
	httpClient *http.Client
	runtime    runtimeSettings
	manager    bool
}

type runtimeSettings struct {
	teeMode             string
	creHTTPRunner       bool
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
	busMaxBytes         int64
}

func validateStores(stores Stores) error {
	missing := map[string]bool{}
	if stores.Accounts == nil {
		missing["accounts"] = true
	}
	if stores.Functions == nil {
		missing["functions"] = true
	}
	if stores.GasBank == nil {
		missing["gasbank"] = true
	}
	if stores.Automation == nil {
		missing["automation"] = true
	}
	if stores.DataFeeds == nil {
		missing["datafeeds"] = true
	}
	if stores.DataStreams == nil {
		missing["datastreams"] = true
	}
	if stores.DataLink == nil {
		missing["datalink"] = true
	}
	if stores.DTA == nil {
		missing["dta"] = true
	}
	if stores.Confidential == nil {
		missing["confidential"] = true
	}
	if stores.Oracle == nil {
		missing["oracle"] = true
	}
	if stores.Secrets == nil {
		missing["secrets"] = true
	}
	if stores.CRE == nil {
		missing["cre"] = true
	}
	if stores.CCIP == nil {
		missing["ccip"] = true
	}
	if stores.VRF == nil {
		missing["vrf"] = true
	}
	if stores.WorkspaceWallets == nil {
		missing["workspace_wallets"] = true
	}
	if len(missing) > 0 {
		keys := make([]string, 0, len(missing))
		for k := range missing {
			keys = append(keys, k)
		}
		return fmt.Errorf("missing required stores: %s", strings.Join(keys, ", "))
	}
	return nil
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

// WithManagerEnabled toggles construction of the legacy manager (defaults to true).
// Disable this when the engine owns lifecycle to avoid double start/stop graphs.
func WithManagerEnabled(enabled bool) Option {
	return func(b *builderConfig) {
		b.managerEnabled = enabled
	}
}

// Application ties domain services together and manages their lifecycle.
type Application struct {
	manager *system.Manager
	log     *logger.Logger

	Accounts           *accounts.Service
	Functions          *functions.Service
	GasBank            *gasbanksvc.Service
	Automation         *automationsvc.Service
	DataFeeds          *datafeedsvc.Service
	DataStreams        *datastreamsvc.Service
	DataLink           *datalinksvc.Service
	DTA                *dtasvc.Service
	Confidential       *confsvc.Service
	Oracle             *oraclesvc.Service
	Secrets            *secrets.Service
	CRE                *cresvc.Service
	CCIP               *ccipsvc.Service
	VRF                *vrfsvc.Service
	OracleRunnerTokens []string
	WorkspaceWallets   storage.WorkspaceWalletStore
	AutomationRunner   *automationsvc.Scheduler
	OracleRunner       *oraclesvc.Dispatcher
	GasBankSettlement  system.Service

	descriptors []core.Descriptor
}

// New builds a fully initialised application with the provided stores.
func New(stores Stores, log *logger.Logger, opts ...Option) (*Application, error) {
	options := resolveBuilderOptions(opts...)
	if log == nil {
		log = logger.NewDefault("app")
	}

	if err := validateStores(stores); err != nil {
		return nil, err
	}

	var manager *system.Manager
	if options.manager {
		manager = system.NewManager()
	}

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
	gasService := gasbanksvc.New(stores.Accounts, stores.GasBank, log)
	automationService := automationsvc.New(stores.Accounts, stores.Functions, stores.Automation, log)
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
	oracleService := oraclesvc.New(stores.Accounts, oraclesvc.NewStoreAdapter(stores.Oracle), log)
	creService := cresvc.New(stores.Accounts, stores.CRE, log)
	ccipService := ccipsvc.New(stores.Accounts, stores.CCIP, log)
	ccipService.WithWorkspaceWallets(stores.WorkspaceWallets)
	ccipService.WithDispatcherHooks(metrics.CCIPDispatchHooks())
	vrfService := vrfsvc.New(stores.Accounts, stores.VRF, log)
	vrfService.WithWorkspaceWallets(stores.WorkspaceWallets)
	vrfService.WithDispatcherHooks(metrics.VRFDispatchHooks())

	httpClient := options.httpClient

	funcService.AttachDependencies(automationService, dataFeedService, dataStreamService, dataLinkService, oracleService, gasService, vrfService)

	if options.runtime.creHTTPRunner {
		creService.WithRunner(cresvc.NewHTTPRunner(httpClient, log))
	}

	autoRunner := automationsvc.NewScheduler(automationService, log)
	autoRunner.WithDispatcher(automationsvc.NewFunctionDispatcher(automationsvc.FunctionRunnerFunc(func(ctx context.Context, functionID string, payload map[string]any) (function.Execution, error) {
		return funcService.Execute(ctx, functionID, payload)
	}), automationService, log))

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

	services := []system.Service{autoRunner, oracleRunner}
	if settlement != nil {
		services = append(services, settlement)
	}

	// Register services with the legacy manager only when engine is disabled.
	if manager != nil {
		for _, svc := range []system.Service{
			acctService,
			funcService,
			secretsService,
			gasService,
			automationService,
			dataFeedService,
			dataStreamService,
			dataLinkService,
			dtaService,
			confService,
			oracleService,
			creService,
			ccipService,
			vrfService,
		} {
			if err := manager.Register(svc); err != nil {
				return nil, fmt.Errorf("register %s service: %w", svc.Name(), err)
			}
		}
		for _, svc := range services {
			if err := manager.Register(svc); err != nil {
				return nil, fmt.Errorf("register %s: %w", svc.Name(), err)
			}
		}
	}

	var descrProviders []system.DescriptorProvider
	descrProviders = appendDescriptorProviders(descrProviders,
		acctService, funcService, gasService, automationService,
		dataFeedService, dataStreamService, dataLinkService,
		dtaService, confService, oracleService, secretsService,
		creService, ccipService, vrfService, autoRunner, oracleRunner,
	)
	if settlement != nil {
		if p, ok := settlement.(system.DescriptorProvider); ok {
			descrProviders = append(descrProviders, p)
		}
	}
	descriptors := system.CollectDescriptors(descrProviders)

	return &Application{
		manager:            manager,
		log:                log,
		Accounts:           acctService,
		Functions:          funcService,
		GasBank:            gasService,
		Automation:         automationService,
		DataFeeds:          dataFeedService,
		DataStreams:        dataStreamService,
		DataLink:           dataLinkService,
		Oracle:             oracleService,
		Secrets:            secretsService,
		CRE:                creService,
		CCIP:               ccipService,
		VRF:                vrfService,
		DTA:                dtaService,
		Confidential:       confService,
		OracleRunnerTokens: options.runtime.oracleRunnerTokens,
		WorkspaceWallets:   stores.WorkspaceWallets,
		AutomationRunner:   autoRunner,
		OracleRunner:       oracleRunner,
		GasBankSettlement:  settlement,
		descriptors:        descriptors,
	}, nil
}

// Attach registers an additional lifecycle-managed service. Call before Start.
func (a *Application) Attach(service system.Service) error {
	if a.manager == nil {
		return nil
	}
	return a.manager.Register(service)
}

// Start begins all registered services.
func (a *Application) Start(ctx context.Context) error {
	// Only start the legacy manager when the engine is not enabled.
	if a.manager == nil {
		return nil
	}
	return a.manager.Start(ctx)
}

// Stop stops all services.
func (a *Application) Stop(ctx context.Context) error {
	// Only stop the legacy manager when the engine is not enabled.
	if a.manager == nil {
		return nil
	}
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
	cfg := builderConfig{environment: osEnvironment{}, managerEnabled: true}
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
		manager:    cfg.managerEnabled,
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
	var busMax int64
	if parsed, ok := parseInt64(env.Lookup("BUS_MAX_BYTES")); ok {
		busMax = parsed
	}
	return RuntimeConfig{
		TEEMode:             env.Lookup("TEE_MODE"),
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
		BusMaxBytes:         busMax,
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
	busMax := cfg.BusMaxBytes
	if busMax <= 0 {
		busMax = 1 << 20 // 1 MiB default
	}
	return runtimeSettings{
		teeMode:             strings.ToLower(strings.TrimSpace(cfg.TEEMode)),
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
		busMaxBytes:         busMax,
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

func parseInt64(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
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

func appendDescriptorProviders(dst []system.DescriptorProvider, providers ...any) []system.DescriptorProvider {
	for _, p := range providers {
		if p == nil {
			continue
		}
		if d, ok := p.(system.DescriptorProvider); ok {
			dst = append(dst, d)
		}
	}
	return dst
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
