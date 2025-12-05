package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// RealtimeClient handles Supabase Realtime subscriptions.
// Note: Full WebSocket support requires additional implementation.
// This provides the subscription configuration and channel management.
type RealtimeClient struct {
	client *Client
	mu     sync.RWMutex

	// Active subscriptions
	subscriptions map[string]*Subscription
}

// Subscription represents an active realtime subscription.
type Subscription struct {
	ID      string
	Config  SubscriptionConfig
	Handler RealtimeHandler
	stopCh  chan struct{}
	active  bool
}

// RealtimeHandler handles realtime events.
type RealtimeHandler func(event RealtimeEvent)

// Channel creates a new channel for subscriptions.
func (r *RealtimeClient) Channel(name string) *Channel {
	return &Channel{
		client: r,
		name:   name,
		events: make([]SubscriptionConfig, 0),
	}
}

// Channel represents a realtime channel.
type Channel struct {
	client  *RealtimeClient
	name    string
	events  []SubscriptionConfig
	handler RealtimeHandler
}

// On subscribes to a specific event type on a table.
func (c *Channel) On(event RealtimeEventType, schema, table string, handler RealtimeHandler) *Channel {
	c.events = append(c.events, SubscriptionConfig{
		Schema: schema,
		Table:  table,
		Event:  event,
	})
	c.handler = handler
	return c
}

// OnPostgresChanges subscribes to postgres changes.
func (c *Channel) OnPostgresChanges(config SubscriptionConfig, handler RealtimeHandler) *Channel {
	c.events = append(c.events, config)
	c.handler = handler
	return c
}

// Subscribe activates the channel subscription.
// Note: This is a simplified implementation. Full WebSocket support
// would require a persistent connection to the Supabase Realtime server.
func (c *Channel) Subscribe() (*Subscription, error) {
	c.client.mu.Lock()
	defer c.client.mu.Unlock()

	if c.client.subscriptions == nil {
		c.client.subscriptions = make(map[string]*Subscription)
	}

	sub := &Subscription{
		ID:      c.name,
		Config:  c.events[0], // Primary config
		Handler: c.handler,
		stopCh:  make(chan struct{}),
		active:  true,
	}

	c.client.subscriptions[c.name] = sub

	return sub, nil
}

// Unsubscribe removes a subscription.
func (r *RealtimeClient) Unsubscribe(subscriptionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	sub, exists := r.subscriptions[subscriptionID]
	if !exists {
		return fmt.Errorf("subscription not found: %s", subscriptionID)
	}

	sub.active = false
	close(sub.stopCh)
	delete(r.subscriptions, subscriptionID)

	return nil
}

// UnsubscribeAll removes all subscriptions.
func (r *RealtimeClient) UnsubscribeAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, sub := range r.subscriptions {
		sub.active = false
		close(sub.stopCh)
		delete(r.subscriptions, id)
	}
}

// =============================================================================
// Broadcast (HTTP-based)
// =============================================================================

// Broadcast sends a message to a channel via HTTP.
// This is useful for server-to-client broadcasts without WebSocket.
func (r *RealtimeClient) Broadcast(ctx context.Context, channel, event string, payload interface{}) error {
	urlStr := r.client.baseURL + "/realtime/v1/api/broadcast"

	req := map[string]interface{}{
		"channel": channel,
		"event":   event,
		"payload": payload,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := r.client.requestWithServiceKey(ctx, "POST", urlStr, body, nil)
	if err != nil {
		return err
	}

	if statusCode >= 400 {
		return parseError(respBody, statusCode)
	}

	return nil
}

// =============================================================================
// Polling-based Change Detection (Alternative to WebSocket)
// =============================================================================

// PollChanges polls for changes on a table.
// This is a fallback for environments where WebSocket is not available.
type ChangePoller struct {
	client   *RealtimeClient
	table    string
	schema   string
	interval time.Duration
	lastID   interface{}
	handler  RealtimeHandler
	stopCh   chan struct{}
	running  bool
	mu       sync.Mutex
	// Optional user access token for RLS
	accessToken string
}

// NewChangePoller creates a new change poller.
func (r *RealtimeClient) NewChangePoller(schema, table string, interval time.Duration) *ChangePoller {
	return &ChangePoller{
		client:   r,
		table:    table,
		schema:   schema,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// OnChange sets the handler for changes.
func (p *ChangePoller) OnChange(handler RealtimeHandler) *ChangePoller {
	p.handler = handler
	return p
}

// WithToken configures the poller to use a user access token (RLS).
func (p *ChangePoller) WithToken(token string) *ChangePoller {
	p.accessToken = token
	return p
}

// Start begins polling for changes.
func (p *ChangePoller) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("poller already running")
	}
	p.running = true
	p.mu.Unlock()

	go p.poll(ctx)
	return nil
}

// Stop stops the poller.
func (p *ChangePoller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		close(p.stopCh)
		p.running = false
	}
}

// poll is the internal polling loop.
func (p *ChangePoller) poll(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.checkForChanges(ctx)
		}
	}
}

// checkForChanges checks for new records.
func (p *ChangePoller) checkForChanges(ctx context.Context) {
	// Build query to get recent changes
	query := p.client.client.Database().From(p.table).Select("*").Order("created_at", OrderDesc).Limit(10)

	if p.accessToken != "" {
		query = query.WithToken(p.accessToken)
	}

	if p.lastID != nil {
		query = query.Gt("id", p.lastID)
	}

	data, err := query.Execute(ctx)
	if err != nil {
		return // Silently ignore errors in polling
	}

	var records []map[string]interface{}
	if err := json.Unmarshal(data, &records); err != nil {
		return
	}

	// Process new records
	for i := len(records) - 1; i >= 0; i-- {
		record := records[i]
		if p.handler != nil {
			p.handler(RealtimeEvent{
				Type:      string(EventInsert),
				Table:     p.table,
				Schema:    p.schema,
				Record:    record,
				Timestamp: time.Now(),
			})
		}

		// Update last ID
		if id, ok := record["id"]; ok {
			p.lastID = id
		}
	}
}

// =============================================================================
// Presence (Simplified)
// =============================================================================

// PresenceState represents presence state for a user.
type PresenceState struct {
	UserID   string                 `json:"user_id"`
	OnlineAt time.Time              `json:"online_at"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PresenceChannel manages presence for a channel.
type PresenceChannel struct {
	client  *RealtimeClient
	channel string
	states  map[string]PresenceState
	mu      sync.RWMutex
}

// Presence creates a presence channel.
func (r *RealtimeClient) Presence(channel string) *PresenceChannel {
	return &PresenceChannel{
		client:  r,
		channel: channel,
		states:  make(map[string]PresenceState),
	}
}

// Track tracks a user's presence.
func (p *PresenceChannel) Track(ctx context.Context, userID string, metadata map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.states[userID] = PresenceState{
		UserID:   userID,
		OnlineAt: time.Now(),
		Metadata: metadata,
	}

	// In a full implementation, this would send to the Realtime server
	return nil
}

// Untrack removes a user's presence.
func (p *PresenceChannel) Untrack(ctx context.Context, userID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.states, userID)
	return nil
}

// List returns all presence states.
func (p *PresenceChannel) List() []PresenceState {
	p.mu.RLock()
	defer p.mu.RUnlock()

	states := make([]PresenceState, 0, len(p.states))
	for _, state := range p.states {
		states = append(states, state)
	}
	return states
}
