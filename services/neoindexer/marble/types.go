// Package neoindexer provides the unified chain event indexing service.
package neoindexer

import (
	"context"
	"encoding/json"
	"time"
)

// =============================================================================
// Interfaces
// =============================================================================

// JetStreamPublisher defines the interface for publishing events to JetStream.
type JetStreamPublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

// =============================================================================
// Event Types
// =============================================================================

// ChainEvent represents a standardized chain event.
type ChainEvent struct {
	ChainID         string          `json:"chain_id"`
	TxHash          string          `json:"tx_hash"`
	LogIndex        int             `json:"log_index"`
	BlockHeight     int64           `json:"block_height"`
	BlockHash       string          `json:"block_hash"`
	ContractAddress string          `json:"contract_address"`
	EventName       string          `json:"event_name"`
	Payload         json.RawMessage `json:"payload"`
	Timestamp       time.Time       `json:"timestamp"`
	Confirmations   int             `json:"confirmations"`
}

// ProcessedEvent represents a processed event record for idempotency.
type ProcessedEvent struct {
	ID              int64           `json:"id"`
	ChainID         string          `json:"chain_id"`
	TxHash          string          `json:"tx_hash"`
	LogIndex        int             `json:"log_index"`
	BlockHeight     int64           `json:"block_height"`
	BlockHash       string          `json:"block_hash"`
	ContractAddress string          `json:"contract_address"`
	EventName       string          `json:"event_name"`
	Payload         json.RawMessage `json:"payload"`
	Confirmations   int             `json:"confirmations"`
	ProcessedAt     time.Time       `json:"processed_at"`
}

// =============================================================================
// RPC Types
// =============================================================================

// RPCEndpoint represents a NEO N3 RPC endpoint configuration.
type RPCEndpoint struct {
	URL      string `json:"url"`
	Priority int    `json:"priority"`
	Healthy  bool   `json:"healthy"`
	Latency  int64  `json:"latency_ms"`
}

// IndexerProgress tracks the indexer's progress.
type IndexerProgress struct {
	LastProcessedBlock int64     `json:"last_processed_block"`
	LastBlockHash      string    `json:"last_block_hash"`
	LastProcessedAt    time.Time `json:"last_processed_at"`
}

// =============================================================================
// JetStream Topics
// =============================================================================

const (
	// StreamName is the JetStream stream name for chain events.
	StreamName = "NEO_EVENTS"

	// TopicPrefix is the prefix for all chain event topics.
	TopicPrefix = "neo.events."

	// Event topics
	TopicOracleRequested  = TopicPrefix + "oracle.requested"
	TopicRandRequested    = TopicPrefix + "rand.requested"
	TopicFeedsUpdated     = TopicPrefix + "feeds.updated"
	TopicComputeRequested = TopicPrefix + "compute.requested"
	TopicVaultRequested   = TopicPrefix + "vault.requested"
	TopicGasDeposited     = TopicPrefix + "gas.deposited"
	TopicFlowTriggered    = TopicPrefix + "flow.triggered"
)

// =============================================================================
// Configuration
// =============================================================================

// Config holds NeoIndexer service configuration.
type Config struct {
	// ChainID is the NEO N3 chain identifier (e.g., "neo3-mainnet", "neo3-testnet").
	ChainID string `json:"chain_id"`

	// RPCEndpoints is the list of NEO N3 RPC endpoints.
	RPCEndpoints []RPCEndpoint `json:"rpc_endpoints"`

	// ConfirmationDepth is the number of blocks to wait before considering an event final.
	ConfirmationDepth int `json:"confirmation_depth"`

	// PollInterval is the interval between block polls.
	PollInterval time.Duration `json:"poll_interval"`

	// BatchSize is the maximum number of blocks to process in a single batch.
	BatchSize int `json:"batch_size"`

	// ContractAddresses is the list of contract addresses to monitor.
	ContractAddresses []string `json:"contract_addresses"`

	// NATSUrl is the NATS server URL.
	NATSUrl string `json:"nats_url"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		ChainID:           "neo3-mainnet",
		ConfirmationDepth: 3,
		PollInterval:      time.Second,
		BatchSize:         100,
	}
}
