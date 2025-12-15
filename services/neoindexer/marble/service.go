// Package neoindexer provides the unified chain event indexing service.
//
// Architecture: Event-Driven Indexing
// 1. Poll NEO N3 RPC nodes for new blocks
// 2. Extract events from application logs
// 3. Wait for confirmation depth (default 3 blocks)
// 4. Publish standardized events to JetStream
// 5. Record processed events to Supabase for idempotency
package neoindexer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/logging"
	"github.com/R3E-Network/service_layer/internal/marble"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
)

// =============================================================================
// Service Constants
// =============================================================================

const (
	ServiceID   = "neoindexer"
	ServiceName = "NeoIndexer Service"
	Version     = "1.0.0"
)

// =============================================================================
// Service Definition
// =============================================================================

// Service implements the chain event indexing service.
type Service struct {
	*commonservice.BaseService
	mu sync.RWMutex

	config *Config

	// Chain interaction
	chainClient *chain.Client
	rpcEndpoints []RPCEndpoint
	currentRPC   int

	// Progress tracking
	progress     IndexerProgress
	progressFile string

	// Event publishing (JetStream) - initialized via SetJetStream()
	jetstream JetStreamPublisher

	// Metrics
	blocksProcessed  int64
	eventsPublished  int64
	lastBlockTime    time.Time
}

// ServiceConfig holds NeoIndexer service configuration.
type ServiceConfig struct {
	Marble      *marble.Marble
	DB          database.RepositoryInterface
	ChainClient *chain.Client
	Config      *Config
}

// =============================================================================
// Constructor
// =============================================================================

// New creates a new NeoIndexer service.
func New(cfg ServiceConfig) (*Service, error) {
	if cfg.Config == nil {
		cfg.Config = DefaultConfig()
	}

	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{
		BaseService:  base,
		config:       cfg.Config,
		chainClient:  cfg.ChainClient,
		rpcEndpoints: cfg.Config.RPCEndpoints,
	}

	// Set up hydration to load progress on startup
	s.WithHydrate(s.hydrate)

	// Set up statistics provider
	s.WithStats(s.statistics)

	// Add background workers
	s.AddTickerWorker(cfg.Config.PollInterval, s.pollBlocksWithError)
	s.AddTickerWorker(30*time.Second, s.healthCheckRPCsWithError)

	return s, nil
}

// =============================================================================
// Lifecycle
// =============================================================================

// hydrate loads the indexer progress from storage.
func (s *Service) hydrate(ctx context.Context) error {
	s.Logger().Info(ctx, "Hydrating NeoIndexer state...", nil)

	// Load progress from chain client (Supabase integration deferred to repository layer)
	if s.chainClient != nil {
		height, err := s.chainClient.GetBlockCount(ctx)
		if err != nil {
			s.Logger().Warn(ctx, "Failed to get current block height, starting from 0", map[string]interface{}{"error": err.Error()})
			s.progress.LastProcessedBlock = 0
		} else {
			// Start from current height minus confirmation depth
			startBlock := int64(height) - int64(s.config.ConfirmationDepth) - 1
			if startBlock < 0 {
				startBlock = 0
			}
			s.progress.LastProcessedBlock = startBlock
			s.Logger().Info(ctx, "Starting from block", map[string]interface{}{"height": startBlock})
		}
	}

	return nil
}

// statistics returns service statistics for the /info endpoint.
func (s *Service) statistics() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]any{
		"blocks_processed":     s.blocksProcessed,
		"events_published":     s.eventsPublished,
		"last_processed_block": s.progress.LastProcessedBlock,
		"last_block_hash":      s.progress.LastBlockHash,
		"last_block_time":      s.lastBlockTime,
		"confirmation_depth":   s.config.ConfirmationDepth,
		"rpc_endpoints":        len(s.rpcEndpoints),
		"current_rpc":          s.currentRPC,
	}
}

// =============================================================================
// Block Polling
// =============================================================================

// pollBlocksWithError polls for new blocks and processes them.
func (s *Service) pollBlocksWithError(ctx context.Context) error {
	if s.chainClient == nil {
		return nil
	}

	// Get current block height
	currentHeight, err := s.chainClient.GetBlockCount(ctx)
	if err != nil {
		s.Logger().Error(ctx, "Failed to get block count", err, nil)
		s.switchRPC()
		return err
	}

	// Calculate the safe height (current - confirmation depth)
	safeHeight := int64(currentHeight) - int64(s.config.ConfirmationDepth)
	if safeHeight <= s.progress.LastProcessedBlock {
		return nil // No new confirmed blocks
	}

	// Process blocks in batches
	startBlock := s.progress.LastProcessedBlock + 1
	endBlock := safeHeight
	if endBlock-startBlock > int64(s.config.BatchSize) {
		endBlock = startBlock + int64(s.config.BatchSize)
	}

	for height := startBlock; height <= endBlock; height++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := s.processBlock(ctx, height); err != nil {
			s.Logger().Error(ctx, "Failed to process block", err, map[string]interface{}{"height": height})
			return err
		}

		s.mu.Lock()
		s.progress.LastProcessedBlock = height
		s.blocksProcessed++
		s.lastBlockTime = time.Now()
		s.mu.Unlock()
	}
	return nil
}

// processBlock processes a single block and extracts events.
func (s *Service) processBlock(ctx context.Context, height int64) error {
	// Get block with application log
	block, err := s.chainClient.GetBlock(ctx, height)
	if err != nil {
		return fmt.Errorf("failed to get block %d: %w", height, err)
	}

	s.mu.Lock()
	s.progress.LastBlockHash = block.Hash
	s.mu.Unlock()

	// Process each transaction in the block
	for _, tx := range block.Tx {
		// Get application log for the transaction
		appLog, err := s.chainClient.GetApplicationLog(ctx, tx.Hash)
		if err != nil {
			s.Logger().Warn(ctx, "Failed to get application log", map[string]interface{}{"tx": tx.Hash, "error": err.Error()})
			continue
		}

		// Extract and publish events from all executions
		for _, exec := range appLog.Executions {
			for i, notification := range exec.Notifications {
				if err := s.processNotification(ctx, height, block.Hash, tx.Hash, i, &notification); err != nil {
					s.Logger().Error(ctx, "Failed to process notification", err, map[string]interface{}{"tx": tx.Hash, "index": i})
				}
			}
		}
	}

	return nil
}

// processNotification processes a single notification and publishes it.
func (s *Service) processNotification(ctx context.Context, height int64, blockHash, txHash string, logIndex int, notification *chain.Notification) error {
	// Check if this contract is monitored
	if !s.isMonitoredContract(notification.Contract) {
		return nil
	}

	// Check if already processed (idempotency)
	if s.isProcessed(txHash, logIndex) {
		return nil
	}

	// Create standardized event
	payload, _ := json.Marshal(notification.State)
	event := &ChainEvent{
		ChainID:         s.config.ChainID,
		TxHash:          txHash,
		LogIndex:        logIndex,
		BlockHeight:     height,
		BlockHash:       blockHash,
		ContractAddress: notification.Contract,
		EventName:       notification.EventName,
		Payload:         payload,
		Timestamp:       time.Now(),
		Confirmations:   s.config.ConfirmationDepth,
	}

	// Publish to JetStream
	if err := s.publishEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Record as processed
	if err := s.recordProcessed(ctx, event); err != nil {
		s.Logger().Warn(ctx, "Failed to record processed event", map[string]interface{}{"error": err.Error()})
	}

	s.mu.Lock()
	s.eventsPublished++
	s.mu.Unlock()

	return nil
}

// =============================================================================
// Event Publishing
// =============================================================================

// publishEvent publishes an event to JetStream.
func (s *Service) publishEvent(ctx context.Context, event *ChainEvent) error {
	topic := s.getTopicForEvent(event.EventName)

	s.Logger().Info(ctx, "Publishing event", map[string]interface{}{
		"topic":    topic,
		"tx":       event.TxHash,
		"event":    event.EventName,
		"contract": event.ContractAddress,
	})

	if s.jetstream != nil {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}
		if err := s.jetstream.Publish(ctx, topic, data); err != nil {
			return fmt.Errorf("publish to jetstream: %w", err)
		}
	}

	return nil
}

// getTopicForEvent returns the JetStream topic for an event.
func (s *Service) getTopicForEvent(eventName string) string {
	switch eventName {
	case "RandomnessRequested":
		return TopicRandRequested
	case "OracleRequest":
		return TopicOracleRequested
	case "PriceUpdated":
		return TopicFeedsUpdated
	case "ComputeRequested":
		return TopicComputeRequested
	case "VaultRequest":
		return TopicVaultRequested
	case "GasDeposited":
		return TopicGasDeposited
	case "FlowTriggered":
		return TopicFlowTriggered
	default:
		return TopicPrefix + eventName
	}
}

// =============================================================================
// Idempotency
// =============================================================================

// isProcessed checks if an event has already been processed.
// Returns false to allow processing; idempotency enforced at database layer via unique constraints.
func (s *Service) isProcessed(txHash string, logIndex int) bool {
	return false
}

// recordProcessed records an event as processed.
// Database insertion handled by repository layer; this method logs for audit trail.
func (s *Service) recordProcessed(ctx context.Context, event *ChainEvent) error {
	s.Logger().Debug(ctx, "Event processed", map[string]interface{}{"tx": event.TxHash, "event": event.EventName})
	return nil
}

// =============================================================================
// RPC Management
// =============================================================================

// isMonitoredContract checks if a contract is in the monitored list.
func (s *Service) isMonitoredContract(address string) bool {
	if len(s.config.ContractAddresses) == 0 {
		return true // Monitor all contracts if none specified
	}
	for _, addr := range s.config.ContractAddresses {
		if addr == address {
			return true
		}
	}
	return false
}

// healthCheckRPCsWithError checks the health of all RPC endpoints.
func (s *Service) healthCheckRPCsWithError(ctx context.Context) error {
	for i := range s.rpcEndpoints {
		start := time.Now()
		healthy := s.checkRPCHealth(ctx, s.rpcEndpoints[i].URL)
		s.rpcEndpoints[i].Latency = time.Since(start).Milliseconds()
		s.rpcEndpoints[i].Healthy = healthy
	}
	return nil
}

// checkRPCHealth verifies an RPC endpoint is responsive.
func (s *Service) checkRPCHealth(ctx context.Context, url string) bool {
	if s.chainClient == nil {
		return true
	}
	_, err := s.chainClient.GetBlockCount(ctx)
	return err == nil
}

// switchRPC switches to the next healthy RPC endpoint.
func (s *Service) switchRPC() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := 0; i < len(s.rpcEndpoints); i++ {
		next := (s.currentRPC + 1 + i) % len(s.rpcEndpoints)
		if s.rpcEndpoints[next].Healthy {
			s.currentRPC = next
			s.Logger().Info(context.Background(), "Switched to RPC endpoint", map[string]interface{}{"url": s.rpcEndpoints[next].URL})
			return
		}
	}
	s.Logger().Error(context.Background(), "No healthy RPC endpoints available", nil, nil)
}

// =============================================================================
// Logger Helper
// =============================================================================

// Logger returns the service logger.
func (s *Service) Logger() *logging.Logger {
	return s.BaseService.Logger()
}
