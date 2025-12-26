// Package neorequests provides on-chain service request dispatch.
package neorequests

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/chain"
	"github.com/R3E-Network/service_layer/infrastructure/database"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
	"github.com/R3E-Network/service_layer/infrastructure/runtime"
	commonservice "github.com/R3E-Network/service_layer/infrastructure/service"
	txproxytypes "github.com/R3E-Network/service_layer/infrastructure/txproxy/types"
	neorequestsupabase "github.com/R3E-Network/service_layer/services/requests/supabase"
)

const (
	ServiceID   = "neorequests"
	ServiceName = "NeoRequests Service"
	Version     = "1.0.0"

	// Neo notifications are capped at 1024 bytes. Keep a safe default
	// to avoid callback failures when ServiceLayerGateway emits events.
	defaultMaxResultBytes = 800
	defaultMaxErrorLen    = 256
)

// Config holds NeoRequests service configuration.
type Config struct {
	Marble *marble.Marble
	DB     database.RepositoryInterface

	RequestsRepo  neorequestsupabase.RepositoryInterface
	EventListener *chain.EventListener
	TxProxy       txproxytypes.Invoker
	ChainClient   *chain.Client

	ServiceGatewayHash string
	AppRegistryHash    string
	NeoVRFURL          string
	NeoOracleURL       string
	NeoComputeURL      string

	HTTPClient     *http.Client
	ChainID        string
	MaxResultBytes int
	MaxErrorLen    int
	RNGResultMode  string
	TxWait         bool

	EnforceAppRegistry      bool
	AppRegistryCacheSeconds int
}

// Service implements the NeoRequests service.
type Service struct {
	*commonservice.BaseService

	repo               neorequestsupabase.RepositoryInterface
	eventListener      *chain.EventListener
	txProxy            txproxytypes.Invoker
	serviceGatewayHash string
	appRegistryHash    string
	appRegistry        *chain.AppRegistryContract
	chainClient        *chain.Client
	enforceAppRegistry bool
	appRegistryCache   map[string]appRegistryCacheEntry
	appRegistryMu      sync.RWMutex
	appRegistryTTL     time.Duration

	httpClient  *http.Client
	vrfURL      string
	oracleURL   string
	computeURL  string
	chainID     string
	txWait      bool
	maxResult   int
	maxErrorLen int
	rngMode     string

	requestIndex sync.Map
}

// New creates a new NeoRequests service.
func New(cfg Config) (*Service, error) { //nolint:gocritic // cfg is read once at startup.
	if cfg.Marble == nil {
		return nil, fmt.Errorf("neorequests: marble is required")
	}

	strict := runtime.StrictIdentityMode() || cfg.Marble.IsEnclave()

	if strict {
		if cfg.EventListener == nil {
			return nil, fmt.Errorf("neorequests: event listener is required in strict/enclave mode")
		}
		if cfg.TxProxy == nil {
			return nil, fmt.Errorf("neorequests: txproxy is required in strict/enclave mode")
		}
	}

	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	repo := cfg.RequestsRepo
	if repo == nil {
		if r, ok := cfg.DB.(*database.Repository); ok {
			repo = neorequestsupabase.NewRepository(r)
		}
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = cfg.Marble.HTTPClient()
		if httpClient == nil {
			httpClient = &http.Client{Timeout: 30 * time.Second}
		}
	}

	serviceGatewayHash := normalizeContractHash(cfg.ServiceGatewayHash)
	if serviceGatewayHash == "" {
		serviceGatewayHash = normalizeContractHash(os.Getenv("CONTRACT_SERVICEGATEWAY_HASH"))
	}
	if serviceGatewayHash == "" {
		serviceGatewayHash = normalizeContractHash(os.Getenv("CONTRACT_SERVICE_GATEWAY_HASH"))
	}
	if serviceGatewayHash == "" {
		if secret, ok := cfg.Marble.Secret("CONTRACT_SERVICEGATEWAY_HASH"); ok && len(secret) > 0 {
			serviceGatewayHash = normalizeContractHash(string(secret))
		}
	}
	if serviceGatewayHash == "" {
		if secret, ok := cfg.Marble.Secret("CONTRACT_SERVICE_GATEWAY_HASH"); ok && len(secret) > 0 {
			serviceGatewayHash = normalizeContractHash(string(secret))
		}
	}
	if strict && serviceGatewayHash == "" {
		return nil, fmt.Errorf("neorequests: ServiceLayerGateway hash required in strict/enclave mode")
	}

	appRegistryHash := normalizeContractHash(cfg.AppRegistryHash)
	if appRegistryHash == "" {
		appRegistryHash = normalizeContractHash(os.Getenv("CONTRACT_APPREGISTRY_HASH"))
	}
	if appRegistryHash == "" {
		appRegistryHash = normalizeContractHash(os.Getenv("CONTRACT_APP_REGISTRY_HASH"))
	}
	if appRegistryHash == "" {
		if secret, ok := cfg.Marble.Secret("CONTRACT_APPREGISTRY_HASH"); ok && len(secret) > 0 {
			appRegistryHash = normalizeContractHash(string(secret))
		}
	}
	if appRegistryHash == "" {
		if secret, ok := cfg.Marble.Secret("CONTRACT_APP_REGISTRY_HASH"); ok && len(secret) > 0 {
			appRegistryHash = normalizeContractHash(string(secret))
		}
	}

	maxResult := cfg.MaxResultBytes
	if maxResult <= 0 {
		if parsed, ok := parseEnvInt("NEOREQUESTS_MAX_RESULT_BYTES"); ok && parsed > 0 {
			maxResult = parsed
		} else {
			maxResult = defaultMaxResultBytes
		}
	}

	maxErrorLen := cfg.MaxErrorLen
	if maxErrorLen <= 0 {
		if parsed, ok := parseEnvInt("NEOREQUESTS_MAX_ERROR_LEN"); ok && parsed > 0 {
			maxErrorLen = parsed
		} else {
			maxErrorLen = defaultMaxErrorLen
		}
	}

	rngMode := strings.ToLower(strings.TrimSpace(cfg.RNGResultMode))
	if rngMode == "" {
		rngMode = strings.ToLower(strings.TrimSpace(os.Getenv("NEOREQUESTS_RNG_RESULT_MODE")))
	}
	if rngMode == "" {
		rngMode = "raw"
	}
	if rngMode != "raw" && rngMode != "json" {
		rngMode = "raw"
	}

	chainID := strings.TrimSpace(cfg.ChainID)
	if chainID == "" {
		chainID = resolveChainID()
	}

	txWait := cfg.TxWait
	if raw := strings.TrimSpace(os.Getenv("NEOREQUESTS_TX_WAIT")); raw != "" {
		txWait = strings.EqualFold(raw, "true") || raw == "1"
	}

	enforceAppRegistry := cfg.EnforceAppRegistry
	if raw := strings.TrimSpace(os.Getenv("NEOREQUESTS_ENFORCE_APPREGISTRY")); raw != "" {
		enforceAppRegistry = parseEnvBool(raw)
	}
	if !enforceAppRegistry && appRegistryHash != "" && cfg.ChainClient != nil {
		enforceAppRegistry = true
	}

	cacheSeconds := cfg.AppRegistryCacheSeconds
	if cacheSeconds <= 0 {
		if parsed, ok := parseEnvInt("NEOREQUESTS_APPREGISTRY_CACHE_SECONDS"); ok && parsed >= 0 {
			cacheSeconds = parsed
		}
	}
	if cacheSeconds <= 0 {
		cacheSeconds = 60
	}

	s := &Service{
		BaseService:        base,
		repo:               repo,
		eventListener:      cfg.EventListener,
		txProxy:            cfg.TxProxy,
		serviceGatewayHash: serviceGatewayHash,
		appRegistryHash:    appRegistryHash,
		chainClient:        cfg.ChainClient,
		enforceAppRegistry: enforceAppRegistry,
		appRegistryCache:   map[string]appRegistryCacheEntry{},
		appRegistryTTL:     time.Duration(cacheSeconds) * time.Second,
		httpClient:         httpClient,
		vrfURL:             strings.TrimSpace(cfg.NeoVRFURL),
		oracleURL:          strings.TrimSpace(cfg.NeoOracleURL),
		computeURL:         strings.TrimSpace(cfg.NeoComputeURL),
		chainID:            chainID,
		txWait:             txWait,
		maxResult:          maxResult,
		maxErrorLen:        maxErrorLen,
		rngMode:            rngMode,
	}

	if s.enforceAppRegistry {
		if s.appRegistryHash == "" {
			if strict {
				return nil, fmt.Errorf("neorequests: AppRegistry hash required when enforcement enabled")
			}
			s.Logger().WithContext(context.Background()).Warn("AppRegistry enforcement enabled but hash missing; disabling enforcement")
			s.enforceAppRegistry = false
		}
		if s.chainClient == nil {
			if strict {
				return nil, fmt.Errorf("neorequests: chain client required when AppRegistry enforcement enabled")
			}
			s.Logger().WithContext(context.Background()).Warn("AppRegistry enforcement enabled but chain client missing; disabling enforcement")
			s.enforceAppRegistry = false
		}
	}
	if s.enforceAppRegistry && s.chainClient != nil && s.appRegistryHash != "" {
		s.appRegistry = chain.NewAppRegistryContract(s.chainClient, s.appRegistryHash)
	}

	if s.vrfURL == "" {
		s.vrfURL = strings.TrimSpace(os.Getenv("NEOVRF_URL"))
	}
	if s.oracleURL == "" {
		s.oracleURL = strings.TrimSpace(os.Getenv("NEOORACLE_URL"))
	}
	if s.computeURL == "" {
		s.computeURL = strings.TrimSpace(os.Getenv("NEOCOMPUTE_URL"))
	}

	base.RegisterStandardRoutes()
	s.registerHandlers()

	return s, nil
}

func (s *Service) registerHandlers() {
	if s.eventListener == nil || s.serviceGatewayHash == "" {
		return
	}

	s.eventListener.On("ServiceRequested", func(event *chain.ContractEvent) error {
		return s.handleServiceRequested(context.Background(), event)
	})
	s.eventListener.On("ServiceFulfilled", func(event *chain.ContractEvent) error {
		return s.handleServiceFulfilled(context.Background(), event)
	})
	s.eventListener.On("Notification", func(event *chain.ContractEvent) error {
		return s.handleNotificationEvent(context.Background(), event)
	})

	s.BaseService.AddWorker(s.runEventListener)
}

func (s *Service) runEventListener(ctx context.Context) {
	if s.eventListener == nil {
		return
	}

	if err := s.eventListener.Start(ctx); err != nil {
		s.Logger().WithContext(ctx).WithError(err).Warn("failed to start event listener")
	}
}

func parseEnvInt(key string) (int, bool) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func parseEnvBool(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func resolveChainID() string {
	raw := strings.TrimSpace(os.Getenv("NEO_NETWORK_MAGIC"))
	if raw == "" {
		return "neo-n3"
	}
	if _, err := strconv.ParseUint(raw, 10, 32); err != nil {
		return "neo-n3"
	}
	return fmt.Sprintf("neo-n3:%s", raw)
}

func normalizeContractHash(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "0x")
	value = strings.TrimPrefix(value, "0X")
	value = strings.ToLower(value)
	if len(value) != 40 {
		return ""
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return ""
		}
	}
	return value
}
