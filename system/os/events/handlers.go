// Package events provides standard event handlers for service layer contracts.
package events

import (
	"context"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Standard contract event names aligned with C# contracts
const (
	// OracleHub events
	EventOracleRequested = "OracleRequested"
	EventOracleFulfilled = "OracleFulfilled"

	// RandomnessHub events
	EventRandomnessRequested = "RandomnessRequested"
	EventRandomnessFulfilled = "RandomnessFulfilled"

	// DataFeedHub events
	EventFeedUpdated   = "FeedUpdated"
	EventFeedRequested = "FeedRequested"

	// AutomationScheduler events
	EventJobScheduled = "JobScheduled"
	EventJobExecuted  = "JobExecuted"
	EventJobCancelled = "JobCancelled"

	// AccountManager events
	EventAccountCreated = "AccountCreated"
	EventWalletLinked   = "WalletLinked"

	// ServiceRegistry events
	EventServiceRegistered = "ServiceRegistered"
	EventServiceUpdated    = "ServiceUpdated"
	EventServicePaused     = "ServicePaused"

	// GasBank events
	EventDeposited      = "Deposited"
	EventWithdrawn      = "Withdrawn"
	EventFeeCollected   = "FeeCollected"
	EventFeeRefunded    = "FeeRefunded"
	EventBalanceUpdated = "BalanceUpdated"

	// Manager events
	EventModuleUpgraded = "ModuleUpgraded"
	EventRoleGranted    = "RoleGranted"
	EventRoleRevoked    = "RoleRevoked"
	EventModulePaused   = "ModulePaused"
)

// OracleEventHandler processes Oracle contract events.
type OracleEventHandler struct {
	log           *logger.Logger
	contracts     []string
	onRequested   func(ctx context.Context, requestID, serviceID string, fee int64) error
	onFulfilled   func(ctx context.Context, requestID, resultHash string) error
}

// OracleHandlerConfig configures the oracle event handler.
type OracleHandlerConfig struct {
	Logger      *logger.Logger
	Contracts   []string // OracleHub contract hashes
	OnRequested func(ctx context.Context, requestID, serviceID string, fee int64) error
	OnFulfilled func(ctx context.Context, requestID, resultHash string) error
}

// NewOracleEventHandler creates a new oracle event handler.
func NewOracleEventHandler(cfg OracleHandlerConfig) *OracleEventHandler {
	if cfg.Logger == nil {
		cfg.Logger = logger.NewDefault("oracle-events")
	}
	return &OracleEventHandler{
		log:         cfg.Logger,
		contracts:   cfg.Contracts,
		onRequested: cfg.OnRequested,
		onFulfilled: cfg.OnFulfilled,
	}
}

func (h *OracleEventHandler) SupportedEvents() []string {
	return []string{EventOracleRequested, EventOracleFulfilled}
}

func (h *OracleEventHandler) SupportedContracts() []string {
	return h.contracts
}

func (h *OracleEventHandler) HandleEvent(ctx context.Context, event *ContractEvent) error {
	switch event.EventName {
	case EventOracleRequested:
		return h.handleRequested(ctx, event)
	case EventOracleFulfilled:
		return h.handleFulfilled(ctx, event)
	default:
		return nil
	}
}

func (h *OracleEventHandler) handleRequested(ctx context.Context, event *ContractEvent) error {
	requestID, _ := event.State["id"].(string)
	serviceID, _ := event.State["service_id"].(string)
	fee, _ := event.State["fee"].(float64)

	h.log.WithField("request_id", requestID).
		WithField("service_id", serviceID).
		WithField("fee", fee).
		Info("oracle request received")

	if h.onRequested != nil {
		return h.onRequested(ctx, requestID, serviceID, int64(fee))
	}
	return nil
}

func (h *OracleEventHandler) handleFulfilled(ctx context.Context, event *ContractEvent) error {
	requestID, _ := event.State["id"].(string)
	resultHash, _ := event.State["result_hash"].(string)

	h.log.WithField("request_id", requestID).
		WithField("result_hash", resultHash).
		Info("oracle request fulfilled")

	if h.onFulfilled != nil {
		return h.onFulfilled(ctx, requestID, resultHash)
	}
	return nil
}

// VRFEventHandler processes VRF/Randomness contract events.
type VRFEventHandler struct {
	log         *logger.Logger
	contracts   []string
	onRequested func(ctx context.Context, requestID, serviceID, seedHash string) error
	onFulfilled func(ctx context.Context, requestID, output string) error
}

// VRFHandlerConfig configures the VRF event handler.
type VRFHandlerConfig struct {
	Logger      *logger.Logger
	Contracts   []string
	OnRequested func(ctx context.Context, requestID, serviceID, seedHash string) error
	OnFulfilled func(ctx context.Context, requestID, output string) error
}

// NewVRFEventHandler creates a new VRF event handler.
func NewVRFEventHandler(cfg VRFHandlerConfig) *VRFEventHandler {
	if cfg.Logger == nil {
		cfg.Logger = logger.NewDefault("vrf-events")
	}
	return &VRFEventHandler{
		log:         cfg.Logger,
		contracts:   cfg.Contracts,
		onRequested: cfg.OnRequested,
		onFulfilled: cfg.OnFulfilled,
	}
}

func (h *VRFEventHandler) SupportedEvents() []string {
	return []string{EventRandomnessRequested, EventRandomnessFulfilled}
}

func (h *VRFEventHandler) SupportedContracts() []string {
	return h.contracts
}

func (h *VRFEventHandler) HandleEvent(ctx context.Context, event *ContractEvent) error {
	switch event.EventName {
	case EventRandomnessRequested:
		return h.handleRequested(ctx, event)
	case EventRandomnessFulfilled:
		return h.handleFulfilled(ctx, event)
	default:
		return nil
	}
}

func (h *VRFEventHandler) handleRequested(ctx context.Context, event *ContractEvent) error {
	requestID, _ := event.State["id"].(string)
	serviceID, _ := event.State["service_id"].(string)
	seedHash, _ := event.State["seed_hash"].(string)

	h.log.WithField("request_id", requestID).
		WithField("service_id", serviceID).
		Info("VRF request received")

	if h.onRequested != nil {
		return h.onRequested(ctx, requestID, serviceID, seedHash)
	}
	return nil
}

func (h *VRFEventHandler) handleFulfilled(ctx context.Context, event *ContractEvent) error {
	requestID, _ := event.State["id"].(string)
	output, _ := event.State["output"].(string)

	h.log.WithField("request_id", requestID).Info("VRF request fulfilled")

	if h.onFulfilled != nil {
		return h.onFulfilled(ctx, requestID, output)
	}
	return nil
}

// GasBankEventHandler processes GasBank contract events.
type GasBankEventHandler struct {
	log          *logger.Logger
	contracts    []string
	onDeposited  func(ctx context.Context, accountID string, amount int64, depositID string) error
	onWithdrawn  func(ctx context.Context, accountID string, amount int64, withdrawID string) error
	onFeeCollected func(ctx context.Context, accountID string, amount int64, serviceID, requestID string) error
}

// GasBankHandlerConfig configures the GasBank event handler.
type GasBankHandlerConfig struct {
	Logger       *logger.Logger
	Contracts    []string
	OnDeposited  func(ctx context.Context, accountID string, amount int64, depositID string) error
	OnWithdrawn  func(ctx context.Context, accountID string, amount int64, withdrawID string) error
	OnFeeCollected func(ctx context.Context, accountID string, amount int64, serviceID, requestID string) error
}

// NewGasBankEventHandler creates a new GasBank event handler.
func NewGasBankEventHandler(cfg GasBankHandlerConfig) *GasBankEventHandler {
	if cfg.Logger == nil {
		cfg.Logger = logger.NewDefault("gasbank-events")
	}
	return &GasBankEventHandler{
		log:          cfg.Logger,
		contracts:    cfg.Contracts,
		onDeposited:  cfg.OnDeposited,
		onWithdrawn:  cfg.OnWithdrawn,
		onFeeCollected: cfg.OnFeeCollected,
	}
}

func (h *GasBankEventHandler) SupportedEvents() []string {
	return []string{EventDeposited, EventWithdrawn, EventFeeCollected, EventFeeRefunded, EventBalanceUpdated}
}

func (h *GasBankEventHandler) SupportedContracts() []string {
	return h.contracts
}

func (h *GasBankEventHandler) HandleEvent(ctx context.Context, event *ContractEvent) error {
	switch event.EventName {
	case EventDeposited:
		return h.handleDeposited(ctx, event)
	case EventWithdrawn:
		return h.handleWithdrawn(ctx, event)
	case EventFeeCollected:
		return h.handleFeeCollected(ctx, event)
	default:
		return nil
	}
}

func (h *GasBankEventHandler) handleDeposited(ctx context.Context, event *ContractEvent) error {
	accountID, _ := event.State["account_id"].(string)
	amount, _ := event.State["amount"].(float64)
	depositID, _ := event.State["deposit_id"].(string)

	h.log.WithField("account_id", accountID).
		WithField("amount", amount).
		Info("deposit received")

	if h.onDeposited != nil {
		return h.onDeposited(ctx, accountID, int64(amount), depositID)
	}
	return nil
}

func (h *GasBankEventHandler) handleWithdrawn(ctx context.Context, event *ContractEvent) error {
	accountID, _ := event.State["account_id"].(string)
	amount, _ := event.State["amount"].(float64)
	withdrawID, _ := event.State["withdraw_id"].(string)

	h.log.WithField("account_id", accountID).
		WithField("amount", amount).
		Info("withdrawal processed")

	if h.onWithdrawn != nil {
		return h.onWithdrawn(ctx, accountID, int64(amount), withdrawID)
	}
	return nil
}

func (h *GasBankEventHandler) handleFeeCollected(ctx context.Context, event *ContractEvent) error {
	accountID, _ := event.State["account_id"].(string)
	amount, _ := event.State["amount"].(float64)
	serviceID, _ := event.State["service_id"].(string)
	requestID, _ := event.State["request_id"].(string)

	h.log.WithField("account_id", accountID).
		WithField("amount", amount).
		WithField("service_id", serviceID).
		WithField("request_id", requestID).
		Info("fee collected")

	if h.onFeeCollected != nil {
		return h.onFeeCollected(ctx, accountID, int64(amount), serviceID, requestID)
	}
	return nil
}

// GenericEventHandler handles any event with a callback.
type GenericEventHandler struct {
	id        string
	events    []string
	contracts []string
	callback  func(ctx context.Context, event *ContractEvent) error
}

// NewGenericEventHandler creates a generic event handler.
func NewGenericEventHandler(id string, events, contracts []string, callback func(ctx context.Context, event *ContractEvent) error) *GenericEventHandler {
	return &GenericEventHandler{
		id:        id,
		events:    events,
		contracts: contracts,
		callback:  callback,
	}
}

func (h *GenericEventHandler) SupportedEvents() []string {
	return h.events
}

func (h *GenericEventHandler) SupportedContracts() []string {
	return h.contracts
}

func (h *GenericEventHandler) HandleEvent(ctx context.Context, event *ContractEvent) error {
	if h.callback != nil {
		return h.callback(ctx, event)
	}
	return nil
}

// EventNameFromContract maps contract types to their event names.
func EventNameFromContract(contractType string) []string {
	switch strings.ToLower(contractType) {
	case "oraclehub", "oracle":
		return []string{EventOracleRequested, EventOracleFulfilled}
	case "randomnesshub", "vrf":
		return []string{EventRandomnessRequested, EventRandomnessFulfilled}
	case "datafeedhub", "datafeeds":
		return []string{EventFeedUpdated, EventFeedRequested}
	case "automationscheduler", "automation":
		return []string{EventJobScheduled, EventJobExecuted, EventJobCancelled}
	case "accountmanager", "accounts":
		return []string{EventAccountCreated, EventWalletLinked}
	case "serviceregistry", "registry":
		return []string{EventServiceRegistered, EventServiceUpdated, EventServicePaused}
	case "gasbank":
		return []string{EventDeposited, EventWithdrawn, EventFeeCollected, EventFeeRefunded, EventBalanceUpdated}
	case "manager":
		return []string{EventModuleUpgraded, EventRoleGranted, EventRoleRevoked, EventModulePaused}
	default:
		return nil
	}
}
