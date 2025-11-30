package app

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/applications/system"
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
	mixersvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.mixer"
	oraclesvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.secrets"
	vrfsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.vrf"
	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/pkg/metrics"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Stores encapsulates persistence dependencies.
type Stores struct {
	// Database is the database connection for service-local stores.
	// Each service package creates its own typed store using this connection.
	Database *sql.DB
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
	if stores.Database == nil {
		return fmt.Errorf("database connection required")
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
	Mixer              *mixersvc.Service
	OracleRunnerTokens []string
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

	db := stores.Database

	// Create all service-local stores using the database connection
	acctStore := accounts.NewPostgresStore(db)
	acctService := accounts.New(acctStore, log)

	// Use acctService as AccountChecker for other stores
	funcStore := functions.NewPostgresStore(db, acctService)
	secretStore := secrets.NewPostgresStore(db, acctService)
	gasStore := gasbanksvc.NewPostgresStore(db, acctService)
	dsStore := datastreamsvc.NewPostgresStore(db, acctService)
	dfStore := datafeedsvc.NewPostgresStore(db, acctService)
	dlStore := datalinksvc.NewPostgresStore(db, acctService)
	dtaStore := dtasvc.NewPostgresStore(db, acctService)
	confStore := confsvc.NewPostgresStore(db, acctService)
	oracleStore := oraclesvc.NewPostgresStore(db, acctService)
	creStore := cresvc.NewPostgresStore(db, acctService)
	ccipStore := ccipsvc.NewPostgresStore(db, acctService)
	vrfStore := vrfsvc.NewPostgresStore(db, acctService)
	autoStore := automationsvc.NewPostgresStore(db, acctService)

	// Create services with their local stores
	funcService := functions.New(acctService, funcStore, log)
	secretsService := secrets.New(acctService, secretStore, log)
	gasService := gasbanksvc.New(acctService, gasStore, log)
	dataStreamService := datastreamsvc.New(acctService, dsStore, log)
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
	dataStreamService.WithObservationHooks(metrics.DatastreamFrameHooks())
	automationService := automationsvc.New(acctService, autoStore, log)
	dataFeedService := datafeedsvc.New(acctService, dfStore, log)
	dataFeedService.WithAggregationConfig(options.runtime.dataFeedMinSigners, options.runtime.dataFeedAggregation)
	dataFeedService.WithObservationHooks(metrics.DataFeedUpdateHooks())
	dataLinkService := datalinksvc.New(acctService, dlStore, log)
	dataLinkService.WithDispatcherHooks(metrics.DataLinkDispatchHooks())
	dtaService := dtasvc.New(acctService, dtaStore, log)
	dtaService.WithObservationHooks(metrics.DTAOrderHooks())
	confService := confsvc.New(acctService, confStore, log)
	confService.WithSealedKeyHooks(metrics.ConfidentialSealedKeyHooks())
	confService.WithAttestationHooks(metrics.ConfidentialAttestationHooks())
	oracleService := oraclesvc.New(acctService, oracleStore, log)
	creService := cresvc.New(acctService, creStore, log)
	ccipService := ccipsvc.New(acctService, ccipStore, log)
	ccipService.WithDispatcherHooks(metrics.CCIPDispatchHooks())
	vrfService := vrfsvc.New(acctService, vrfStore, log)
	vrfService.WithDispatcherHooks(metrics.VRFDispatchHooks())

	httpClient := options.httpClient

	// Wire action processor to decouple Functions from direct service dependencies
	actionProcessor := newServiceActionProcessor(automationService, dataFeedService, dataStreamService, dataLinkService, oracleService, gasService, vrfService)
	funcService.AttachActionProcessor(actionProcessor)

	if options.runtime.creHTTPRunner {
		creService.WithRunner(cresvc.NewHTTPRunner(httpClient, log))
	}

	autoRunner := automationsvc.NewScheduler(automationService, log)
	autoRunner.WithDispatcher(automationsvc.NewFunctionDispatcher(automationsvc.FunctionRunnerFunc(func(ctx context.Context, functionID string, payload map[string]any) (automationsvc.FunctionExecution, error) {
		exec, err := funcService.Execute(ctx, functionID, payload)
		if err != nil {
			return automationsvc.FunctionExecution{}, err
		}
		// Convert functions.Execution to automationsvc.FunctionExecution
		return automationsvc.FunctionExecution{
			ID:          exec.ID,
			AccountID:   exec.AccountID,
			FunctionID:  exec.FunctionID,
			Input:       exec.Input,
			Output:      exec.Output,
			Logs:        exec.Logs,
			Error:       exec.Error,
			Status:      string(exec.Status),
			StartedAt:   exec.StartedAt,
			CompletedAt: exec.CompletedAt,
			Duration:    exec.Duration,
		}, nil
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
			poller := gasbanksvc.NewSettlementPoller(gasStore, gasService, resolver, log)
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

// serviceActionProcessor implements functions.ActionProcessor by routing
// devpack actions to the appropriate service implementations.
type serviceActionProcessor struct {
	automation  *automationsvc.Service
	dataFeeds   *datafeedsvc.Service
	dataStreams *datastreamsvc.Service
	dataLink    *datalinksvc.Service
	oracle      *oraclesvc.Service
	gasBank     *gasbanksvc.Service
	vrf         *vrfsvc.Service
}

// newServiceActionProcessor creates a new service action processor.
func newServiceActionProcessor(
	automation *automationsvc.Service,
	dataFeeds *datafeedsvc.Service,
	dataStreams *datastreamsvc.Service,
	dataLink *datalinksvc.Service,
	oracle *oraclesvc.Service,
	gasBank *gasbanksvc.Service,
	vrf *vrfsvc.Service,
) *serviceActionProcessor {
	return &serviceActionProcessor{
		automation:  automation,
		dataFeeds:   dataFeeds,
		dataStreams: dataStreams,
		dataLink:    dataLink,
		oracle:      oracle,
		gasBank:     gasBank,
		vrf:         vrf,
	}
}

// SupportsAction returns true if this processor can handle the given action type.
func (p *serviceActionProcessor) SupportsAction(actionType string) bool {
	switch actionType {
	case functions.ActionTypeGasBankEnsureAccount,
		functions.ActionTypeGasBankWithdraw,
		functions.ActionTypeGasBankBalance,
		functions.ActionTypeGasBankListTx,
		functions.ActionTypeOracleCreateRequest,
		functions.ActionTypeDataFeedSubmit,
		functions.ActionTypeDatastreamPublish,
		functions.ActionTypeDatalinkDeliver,
		functions.ActionTypeAutomationSchedule:
		return true
	}
	return false
}

// ProcessAction handles a single devpack action by routing to the appropriate service.
func (p *serviceActionProcessor) ProcessAction(ctx context.Context, accountID string, actionType string, params map[string]any) (map[string]any, error) {
	switch actionType {
	case functions.ActionTypeGasBankEnsureAccount:
		return p.handleGasBankEnsure(ctx, accountID, params)
	case functions.ActionTypeGasBankWithdraw:
		return p.handleGasBankWithdraw(ctx, accountID, params)
	case functions.ActionTypeGasBankBalance:
		return p.handleGasBankBalance(ctx, accountID, params)
	case functions.ActionTypeGasBankListTx:
		return p.handleGasBankListTx(ctx, accountID, params)
	case functions.ActionTypeOracleCreateRequest:
		return p.handleOracleCreateRequest(ctx, accountID, params)
	case functions.ActionTypeDataFeedSubmit:
		return p.handleDataFeedSubmit(ctx, accountID, params)
	case functions.ActionTypeDatastreamPublish:
		return p.handleDatastreamPublish(ctx, accountID, params)
	case functions.ActionTypeDatalinkDeliver:
		return p.handleDatalinkDeliver(ctx, accountID, params)
	case functions.ActionTypeAutomationSchedule:
		return p.handleAutomationSchedule(ctx, accountID, params)
	default:
		return nil, fmt.Errorf("unsupported action type: %s", actionType)
	}
}

func (p *serviceActionProcessor) handleGasBankEnsure(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	if p.gasBank == nil {
		return nil, fmt.Errorf("gasbank service not configured")
	}
	wallet, _ := params["wallet"].(string)
	acct, err := p.gasBank.EnsureAccount(ctx, accountID, wallet)
	if err != nil {
		return nil, err
	}
	return map[string]any{"account": toMap(acct)}, nil
}

func (p *serviceActionProcessor) handleGasBankWithdraw(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	if p.gasBank == nil {
		return nil, fmt.Errorf("gasbank service not configured")
	}
	gasAccountID, _ := params["gasAccountId"].(string)
	wallet, _ := params["wallet"].(string)
	if gasAccountID == "" && wallet == "" {
		return nil, fmt.Errorf("gasAccountId or wallet required")
	}
	if gasAccountID == "" {
		ensured, err := p.gasBank.EnsureAccount(ctx, accountID, wallet)
		if err != nil {
			return nil, err
		}
		gasAccountID = ensured.ID
	}
	amount, _ := params["amount"].(float64)
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	toAddress, _ := params["to"].(string)
	updated, tx, err := p.gasBank.Withdraw(ctx, accountID, gasAccountID, amount, toAddress)
	if err != nil {
		return nil, err
	}
	return map[string]any{"account": toMap(updated), "transaction": toMap(tx)}, nil
}

func (p *serviceActionProcessor) handleGasBankBalance(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	if p.gasBank == nil {
		return nil, fmt.Errorf("gasbank service not configured")
	}
	gasAccountID, _ := params["gasAccountId"].(string)
	wallet, _ := params["wallet"].(string)
	if gasAccountID == "" && wallet == "" {
		return nil, fmt.Errorf("gasAccountId or wallet required")
	}
	var acct gasbanksvc.GasBankAccount
	var err error
	if gasAccountID != "" {
		acct, err = p.gasBank.GetAccount(ctx, gasAccountID)
	} else {
		acct, err = p.gasBank.EnsureAccount(ctx, accountID, wallet)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"account": toMap(acct)}, nil
}

func (p *serviceActionProcessor) handleGasBankListTx(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	if p.gasBank == nil {
		return nil, fmt.Errorf("gasbank service not configured")
	}
	gasAccountID, _ := params["gasAccountId"].(string)
	wallet, _ := params["wallet"].(string)
	if gasAccountID == "" && wallet == "" {
		return nil, fmt.Errorf("gasAccountId or wallet required")
	}
	var acct gasbanksvc.GasBankAccount
	var err error
	if gasAccountID != "" {
		acct, err = p.gasBank.GetAccount(ctx, gasAccountID)
	} else {
		acct, err = p.gasBank.EnsureAccount(ctx, accountID, wallet)
	}
	if err != nil {
		return nil, err
	}
	status, _ := params["status"].(string)
	txType, _ := params["type"].(string)
	limit := core.DefaultListLimit
	if l, ok := params["limit"].(int); ok && l > 0 {
		limit = l
	}
	txs, err := p.gasBank.ListTransactionsFiltered(ctx, acct.ID, txType, status, limit)
	if err != nil {
		return nil, err
	}
	serialized := make([]map[string]any, len(txs))
	for i, tx := range txs {
		serialized[i] = toMap(tx)
	}
	return map[string]any{"transactions": serialized}, nil
}

func (p *serviceActionProcessor) handleOracleCreateRequest(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	if p.oracle == nil {
		return nil, fmt.Errorf("oracle service not configured")
	}
	dataSourceID, _ := params["dataSourceId"].(string)
	if dataSourceID == "" {
		return nil, fmt.Errorf("dataSourceId required")
	}
	payload, _ := params["payload"].(string)
	req, err := p.oracle.CreateRequest(ctx, accountID, dataSourceID, payload)
	if err != nil {
		return nil, err
	}
	return map[string]any{"request": toMap(req)}, nil
}

func (p *serviceActionProcessor) handleDataFeedSubmit(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	if p.dataFeeds == nil {
		return nil, fmt.Errorf("datafeeds service not configured")
	}
	feedID, _ := params["feedId"].(string)
	if feedID == "" {
		feedID, _ = params["feed_id"].(string)
	}
	if feedID == "" {
		return nil, fmt.Errorf("feedId required")
	}
	roundID, _ := params["roundId"].(int64)
	if roundID == 0 {
		if r, ok := params["round_id"].(int64); ok {
			roundID = r
		}
	}
	price, _ := params["price"].(string)
	ts := time.Now().UTC()
	signer, _ := params["signer"].(string)
	signature, _ := params["signature"].(string)
	meta := make(map[string]string)
	if m, ok := params["metadata"].(map[string]string); ok {
		meta = m
	}
	update, err := p.dataFeeds.SubmitUpdate(ctx, accountID, feedID, roundID, price, ts, signer, signature, meta)
	if err != nil {
		return nil, err
	}
	return map[string]any{"update": toMap(update)}, nil
}

func (p *serviceActionProcessor) handleDatastreamPublish(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	if p.dataStreams == nil {
		return nil, fmt.Errorf("datastreams service not configured")
	}
	streamID, _ := params["streamId"].(string)
	if streamID == "" {
		streamID, _ = params["stream_id"].(string)
	}
	if streamID == "" {
		return nil, fmt.Errorf("streamId required")
	}
	seq, _ := params["sequence"].(int64)
	if seq == 0 {
		if s, ok := params["seq"].(int64); ok {
			seq = s
		}
	}
	payload, _ := params["payload"].(map[string]any)
	latency, _ := params["latencyMs"].(int)
	status := datastreamsvc.FrameStatus(params["status"].(string))
	meta := make(map[string]string)
	if m, ok := params["metadata"].(map[string]string); ok {
		meta = m
	}
	frame, err := p.dataStreams.CreateFrame(ctx, accountID, streamID, seq, payload, latency, status, meta)
	if err != nil {
		return nil, err
	}
	return map[string]any{"frame": toMap(frame)}, nil
}

func (p *serviceActionProcessor) handleDatalinkDeliver(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	if p.dataLink == nil {
		return nil, fmt.Errorf("datalink service not configured")
	}
	channelID, _ := params["channelId"].(string)
	if channelID == "" {
		channelID, _ = params["channel_id"].(string)
	}
	if channelID == "" {
		return nil, fmt.Errorf("channelId required")
	}
	payload, _ := params["payload"].(map[string]any)
	meta := make(map[string]string)
	if m, ok := params["metadata"].(map[string]string); ok {
		meta = m
	}
	delivery, err := p.dataLink.CreateDelivery(ctx, accountID, channelID, payload, meta)
	if err != nil {
		return nil, err
	}
	return map[string]any{"delivery": toMap(delivery)}, nil
}

func (p *serviceActionProcessor) handleAutomationSchedule(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	if p.automation == nil {
		return nil, fmt.Errorf("automation service not configured")
	}
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	schedule, _ := params["schedule"].(string)
	if schedule == "" {
		return nil, fmt.Errorf("schedule required")
	}
	description, _ := params["description"].(string)
	functionID, _ := params["functionId"].(string)
	job, err := p.automation.CreateJob(ctx, accountID, functionID, name, schedule, description)
	if err != nil {
		return nil, err
	}
	return map[string]any{"job": toMap(job)}, nil
}

// toMap converts a struct to map[string]any using JSON marshaling.
func toMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return map[string]any{"error": err.Error()}
	}
	return result
}
