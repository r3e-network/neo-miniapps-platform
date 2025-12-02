// Package events provides the bridge between neo-indexer and the event dispatcher.
// IndexerBridge monitors the neo_notifications table and dispatches events to handlers.
package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
)

// IndexerBridge connects the neo-indexer database to the event dispatcher.
// It polls the neo_notifications table for new events and dispatches them.
type IndexerBridge struct {
	db           *sql.DB
	dispatcher   *Dispatcher
	router       *RequestRouter
	log          *logger.Logger

	// Contract hash mappings
	contractTypes map[string]string // contract hash -> type (e.g., "oraclehub")

	// Polling configuration
	pollInterval  time.Duration
	batchSize     int
	lastProcessed int64

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// IndexerBridgeConfig configures the indexer bridge.
type IndexerBridgeConfig struct {
	DB            *sql.DB
	Dispatcher    *Dispatcher
	Router        *RequestRouter
	Logger        *logger.Logger
	PollInterval  time.Duration
	BatchSize     int
	ContractTypes map[string]string // contract hash -> type
}

// NewIndexerBridge creates a new indexer bridge.
func NewIndexerBridge(cfg IndexerBridgeConfig) *IndexerBridge {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.Logger == nil {
		cfg.Logger = logger.NewDefault("indexer-bridge")
	}
	if cfg.ContractTypes == nil {
		cfg.ContractTypes = make(map[string]string)
	}

	return &IndexerBridge{
		db:            cfg.DB,
		dispatcher:    cfg.Dispatcher,
		router:        cfg.Router,
		log:           cfg.Logger,
		contractTypes: cfg.ContractTypes,
		pollInterval:  cfg.PollInterval,
		batchSize:     cfg.BatchSize,
	}
}

// RegisterContract registers a contract hash with its type.
func (b *IndexerBridge) RegisterContract(hash, contractType string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.contractTypes[hash] = contractType
}

// Start begins polling for new notifications.
func (b *IndexerBridge) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return fmt.Errorf("bridge already running")
	}
	b.running = true
	b.stopCh = make(chan struct{})
	b.doneCh = make(chan struct{})
	b.mu.Unlock()

	// Load last processed ID
	if err := b.loadLastProcessed(ctx); err != nil {
		b.log.WithError(err).Warn("failed to load last processed ID, starting from 0")
	}

	go b.pollLoop(ctx)

	b.log.WithField("poll_interval", b.pollInterval).
		WithField("batch_size", b.batchSize).
		WithField("last_processed", b.lastProcessed).
		Info("indexer bridge started")

	return nil
}

// Stop halts polling.
func (b *IndexerBridge) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	b.running = false
	close(b.stopCh)
	b.mu.Unlock()

	<-b.doneCh
	b.log.Info("indexer bridge stopped")
}

// pollLoop continuously polls for new notifications.
func (b *IndexerBridge) pollLoop(ctx context.Context) {
	defer close(b.doneCh)

	ticker := time.NewTicker(b.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		case <-ticker.C:
			if err := b.processBatch(ctx); err != nil {
				b.log.WithError(err).Error("failed to process notification batch")
			}
		}
	}
}

// processBatch fetches and processes a batch of notifications.
func (b *IndexerBridge) processBatch(ctx context.Context) error {
	notifications, err := b.fetchNotifications(ctx)
	if err != nil {
		return fmt.Errorf("fetch notifications: %w", err)
	}

	if len(notifications) == 0 {
		return nil
	}

	var lastID int64
	for _, n := range notifications {
		event := b.notificationToEvent(n)
		if event != nil {
			// Dispatch to event handlers
			if b.dispatcher != nil {
				if err := b.dispatcher.Dispatch(event); err != nil {
					b.log.WithField("event", event.EventName).
						WithError(err).
						Warn("failed to dispatch event")
				}
			}

			// Route to request handlers if applicable
			b.routeToRequestHandler(ctx, event)
		}
		lastID = n.ID
	}

	// Update last processed
	if lastID > 0 {
		b.mu.Lock()
		b.lastProcessed = lastID
		b.mu.Unlock()

		if err := b.saveLastProcessed(ctx, lastID); err != nil {
			b.log.WithError(err).Warn("failed to save last processed ID")
		}
	}

	b.log.WithField("count", len(notifications)).
		WithField("last_id", lastID).
		Debug("processed notification batch")

	return nil
}

// DBNotification represents a row from neo_notifications.
type DBNotification struct {
	ID        int64
	TxHash    string
	Contract  string
	Event     string
	ExecIndex int
	State     json.RawMessage
	CreatedAt time.Time
}

// fetchNotifications fetches unprocessed notifications from the database.
func (b *IndexerBridge) fetchNotifications(ctx context.Context) ([]DBNotification, error) {
	query := `
		SELECT id, tx_hash, contract, event, exec_index, state, created_at
		FROM neo_notifications
		WHERE id > $1
		ORDER BY id ASC
		LIMIT $2
	`

	rows, err := b.db.QueryContext(ctx, query, b.lastProcessed, b.batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []DBNotification
	for rows.Next() {
		var n DBNotification
		if err := rows.Scan(&n.ID, &n.TxHash, &n.Contract, &n.Event, &n.ExecIndex, &n.State, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}

	return notifications, rows.Err()
}

// notificationToEvent converts a database notification to a ContractEvent.
func (b *IndexerBridge) notificationToEvent(n DBNotification) *ContractEvent {
	var state map[string]any
	if len(n.State) > 0 {
		// Try to parse as map
		if err := json.Unmarshal(n.State, &state); err != nil {
			// Try to parse as array and convert
			var arr []any
			if err := json.Unmarshal(n.State, &arr); err == nil {
				state = b.arrayToMap(n.Event, arr)
			} else {
				b.log.WithField("event", n.Event).
					WithError(err).
					Warn("failed to parse notification state")
				state = make(map[string]any)
			}
		}
	}

	return &ContractEvent{
		TxHash:    n.TxHash,
		Contract:  n.Contract,
		EventName: n.Event,
		State:     state,
		Timestamp: n.CreatedAt,
	}
}

// arrayToMap converts Neo event state array to a named map based on event type.
func (b *IndexerBridge) arrayToMap(eventName string, arr []any) map[string]any {
	result := make(map[string]any)

	switch eventName {
	case EventOracleRequested:
		if len(arr) >= 3 {
			result["id"] = arr[0]
			result["service_id"] = arr[1]
			result["fee"] = arr[2]
		}
	case EventOracleFulfilled:
		if len(arr) >= 2 {
			result["id"] = arr[0]
			result["result_hash"] = arr[1]
		}
	case EventRandomnessRequested:
		if len(arr) >= 2 {
			result["id"] = arr[0]
			result["service_id"] = arr[1]
		}
	case EventRandomnessFulfilled:
		if len(arr) >= 2 {
			result["id"] = arr[0]
			result["output"] = arr[1]
		}
	case EventDeposited:
		if len(arr) >= 3 {
			result["account_id"] = arr[0]
			result["amount"] = arr[1]
			result["deposit_id"] = arr[2]
		}
	case EventWithdrawn:
		if len(arr) >= 3 {
			result["account_id"] = arr[0]
			result["amount"] = arr[1]
			result["withdraw_id"] = arr[2]
		}
	case EventFeeCollected:
		if len(arr) >= 4 {
			result["account_id"] = arr[0]
			result["amount"] = arr[1]
			result["service_id"] = arr[2]
			result["request_id"] = arr[3]
		}
	case EventAccountCreated:
		if len(arr) >= 2 {
			result["id"] = arr[0]
			result["owner"] = arr[1]
		}
	case EventServiceRegistered:
		if len(arr) >= 3 {
			result["id"] = arr[0]
			result["owner"] = arr[1]
			result["capabilities"] = arr[2]
		}
	default:
		// Generic mapping
		for i, v := range arr {
			result[fmt.Sprintf("arg%d", i)] = v
		}
	}

	return result
}

// routeToRequestHandler routes events to the request router for processing.
func (b *IndexerBridge) routeToRequestHandler(ctx context.Context, event *ContractEvent) {
	if b.router == nil {
		return
	}

	// Determine service type from contract or event
	serviceType := b.getServiceType(event)
	if serviceType == "" {
		return
	}

	// Only process request events
	switch event.EventName {
	case EventOracleRequested:
		b.handleOracleRequest(ctx, event, serviceType)
	case EventRandomnessRequested:
		b.handleVRFRequest(ctx, event, serviceType)
	}
}

// getServiceType determines the service type from an event.
func (b *IndexerBridge) getServiceType(event *ContractEvent) ServiceType {
	b.mu.RLock()
	contractType, ok := b.contractTypes[event.Contract]
	b.mu.RUnlock()

	if ok {
		switch contractType {
		case "oraclehub", "oracle":
			return ServiceOracle
		case "randomnesshub", "vrf":
			return ServiceVRF
		case "datafeedhub", "datafeeds":
			return ServiceDataFeeds
		case "automationscheduler", "automation":
			return ServiceAutomation
		}
	}

	// Infer from event name
	switch event.EventName {
	case EventOracleRequested, EventOracleFulfilled:
		return ServiceOracle
	case EventRandomnessRequested, EventRandomnessFulfilled:
		return ServiceVRF
	case EventFeedUpdated, EventFeedRequested:
		return ServiceDataFeeds
	case EventJobScheduled, EventJobExecuted:
		return ServiceAutomation
	}

	return ""
}

// handleOracleRequest creates a request from an oracle event.
func (b *IndexerBridge) handleOracleRequest(ctx context.Context, event *ContractEvent, serviceType ServiceType) {
	externalID, _ := event.State["id"].(string)
	serviceID, _ := event.State["service_id"].(string)
	fee, _ := event.State["fee"].(float64)

	// Extract account ID from service ID or other means
	accountID := serviceID // Simplified; in practice, look up from contract

	req, err := b.router.CreateRequest(ctx, accountID, serviceType, event.State,
		WithExternalID(externalID),
		WithServiceID(serviceID),
		WithFee(int64(fee), ""),
		WithTxHash(event.TxHash),
	)
	if err != nil {
		b.log.WithError(err).Error("failed to create oracle request")
		return
	}

	if err := b.router.SubmitRequest(req); err != nil {
		b.log.WithError(err).Error("failed to submit oracle request")
	}
}

// handleVRFRequest creates a request from a VRF event.
func (b *IndexerBridge) handleVRFRequest(ctx context.Context, event *ContractEvent, serviceType ServiceType) {
	externalID, _ := event.State["id"].(string)
	serviceID, _ := event.State["service_id"].(string)

	accountID := serviceID

	req, err := b.router.CreateRequest(ctx, accountID, serviceType, event.State,
		WithExternalID(externalID),
		WithServiceID(serviceID),
		WithTxHash(event.TxHash),
	)
	if err != nil {
		b.log.WithError(err).Error("failed to create VRF request")
		return
	}

	if err := b.router.SubmitRequest(req); err != nil {
		b.log.WithError(err).Error("failed to submit VRF request")
	}
}

// loadLastProcessed loads the last processed notification ID from the database.
func (b *IndexerBridge) loadLastProcessed(ctx context.Context) error {
	var lastID int64
	err := b.db.QueryRowContext(ctx, `
		SELECT COALESCE(value::BIGINT, 0) FROM neo_meta WHERE key = 'event_bridge_last_id'
	`).Scan(&lastID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	b.lastProcessed = lastID
	return nil
}

// saveLastProcessed saves the last processed notification ID to the database.
func (b *IndexerBridge) saveLastProcessed(ctx context.Context, id int64) error {
	_, err := b.db.ExecContext(ctx, `
		INSERT INTO neo_meta (key, value, updated_at)
		VALUES ('event_bridge_last_id', $1, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, fmt.Sprintf("%d", id))
	return err
}

// Stats returns bridge statistics.
type BridgeStats struct {
	Running         bool  `json:"running"`
	LastProcessedID int64 `json:"last_processed_id"`
	ContractsCount  int   `json:"contracts_count"`
}

func (b *IndexerBridge) Stats() BridgeStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return BridgeStats{
		Running:         b.running,
		LastProcessedID: b.lastProcessed,
		ContractsCount:  len(b.contractTypes),
	}
}
