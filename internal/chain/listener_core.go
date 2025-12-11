// Package chain provides event listening for Neo N3 contracts.
package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// EventListener listens for contract events on Neo N3.
type EventListener struct {
	mu             sync.RWMutex
	client         *Client
	contractHashes map[string]bool // Multiple contracts to monitor
	handlers       map[string][]EventHandler
	pollInterval   time.Duration
	lastBlock      uint64
	running        bool
	stopCh         chan struct{}
}

// EventHandler is a callback for contract events.
type EventHandler func(event *ContractEvent) error

// ContractEvent represents a contract event.
type ContractEvent struct {
	TxHash     string
	BlockIndex uint64
	Contract   string
	EventName  string
	State      []StackItem
	Timestamp  time.Time
}

// ListenerConfig holds event listener configuration.
type ListenerConfig struct {
	Client       *Client
	Contracts    ContractAddresses // All contract addresses to monitor
	PollInterval time.Duration
	StartBlock   uint64
}

// NewEventListener creates a new event listener.
func NewEventListener(cfg ListenerConfig) *EventListener {
	interval := cfg.PollInterval
	if interval == 0 {
		interval = 5 * time.Second
	}

	// Build contract hash map for filtering
	contractHashes := make(map[string]bool)
	if cfg.Contracts.Gateway != "" {
		contractHashes[cfg.Contracts.Gateway] = true
	}
	if cfg.Contracts.VRF != "" {
		contractHashes[cfg.Contracts.VRF] = true
	}
	if cfg.Contracts.NeoVault != "" {
		contractHashes[cfg.Contracts.NeoVault] = true
	}
	if cfg.Contracts.NeoFeeds != "" {
		contractHashes[cfg.Contracts.NeoFeeds] = true
	}
	if cfg.Contracts.NeoFlow != "" {
		contractHashes[cfg.Contracts.NeoFlow] = true
	}

	return &EventListener{
		client:         cfg.Client,
		contractHashes: contractHashes,
		handlers:       make(map[string][]EventHandler),
		pollInterval:   interval,
		lastBlock:      cfg.StartBlock,
		stopCh:         make(chan struct{}),
	}
}

// On registers an event handler.
func (l *EventListener) On(eventName string, handler EventHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handlers[eventName] = append(l.handlers[eventName], handler)
}

// Start starts the event listener.
func (l *EventListener) Start(ctx context.Context) error {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return fmt.Errorf("listener already running")
	}
	l.running = true
	l.mu.Unlock()

	go l.poll(ctx)
	return nil
}

// Stop stops the event listener.
func (l *EventListener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return
	}

	l.running = false
	close(l.stopCh)
}

// poll continuously polls for new blocks and events.
func (l *EventListener) poll(ctx context.Context) {
	ticker := time.NewTicker(l.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.processNewBlocks(ctx)
		}
	}
}

// processNewBlocks processes new blocks for events.
func (l *EventListener) processNewBlocks(ctx context.Context) {
	// Get current block height
	currentBlock, err := l.client.GetBlockCount(ctx)
	if err != nil {
		return
	}

	l.mu.RLock()
	lastBlock := l.lastBlock
	l.mu.RUnlock()

	// Process new blocks
	for blockIndex := lastBlock + 1; blockIndex < currentBlock; blockIndex++ {
		block, err := l.client.GetBlock(ctx, blockIndex)
		if err != nil {
			continue
		}

		// Process each transaction in the block
		for _, tx := range block.Tx {
			l.processTransaction(ctx, tx.Hash, blockIndex)
		}

		l.mu.Lock()
		l.lastBlock = blockIndex
		l.mu.Unlock()
	}
}

// processTransaction processes a transaction for events.
func (l *EventListener) processTransaction(ctx context.Context, txHash string, blockIndex uint64) {
	log, err := l.client.GetApplicationLog(ctx, txHash)
	if err != nil {
		return
	}

	for _, exec := range log.Executions {
		if exec.VMState != "HALT" {
			continue
		}

		for _, notif := range exec.Notifications {
			// Filter by contract (if we have contracts configured)
			if len(l.contractHashes) > 0 && !l.contractHashes[notif.Contract] {
				continue
			}

			event := &ContractEvent{
				TxHash:     txHash,
				BlockIndex: blockIndex,
				Contract:   notif.Contract,
				EventName:  notif.EventName,
				Timestamp:  time.Now(),
			}

			// Parse state array
			if notif.State.Type == "Array" {
				var items []StackItem
				json.Unmarshal(notif.State.Value, &items)
				event.State = items
			}

			// Call handlers
			l.mu.RLock()
			handlers := l.handlers[notif.EventName]
			l.mu.RUnlock()

			for _, handler := range handlers {
				go handler(event)
			}
		}
	}
}
