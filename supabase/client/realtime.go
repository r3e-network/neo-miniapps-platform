// Package client provides realtime subscription support for Supabase.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RealtimeClient handles Supabase Realtime subscriptions.
type RealtimeClient struct {
	mu       sync.RWMutex
	url      string
	apiKey   string
	conn     *websocket.Conn
	channels map[string]*Channel
	handlers map[string][]EventHandler
	done     chan struct{}
	ref      int
}

// EventHandler handles realtime events.
type EventHandler func(event *RealtimeEvent)

// RealtimeEvent represents a realtime event.
type RealtimeEvent struct {
	Type      string         `json:"type"`
	Event     string         `json:"event"`
	Topic     string         `json:"topic"`
	Payload   map[string]any `json:"payload"`
	Ref       string         `json:"ref"`
	JoinRef   string         `json:"join_ref,omitempty"`
}

// Channel represents a realtime channel.
type Channel struct {
	client  *RealtimeClient
	topic   string
	joined  bool
	joinRef string
}

// NewRealtimeClient creates a new realtime client.
func NewRealtimeClient(supabaseURL, apiKey string) *RealtimeClient {
	// Convert HTTP URL to WebSocket URL
	wsURL := supabaseURL
	if len(wsURL) > 5 && wsURL[:5] == "https" {
		wsURL = "wss" + wsURL[5:]
	} else if len(wsURL) > 4 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	}
	wsURL += "/realtime/v1/websocket?apikey=" + apiKey + "&vsn=1.0.0"

	return &RealtimeClient{
		url:      wsURL,
		apiKey:   apiKey,
		channels: make(map[string]*Channel),
		handlers: make(map[string][]EventHandler),
		done:     make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection.
func (r *RealtimeClient) Connect(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conn != nil {
		return nil // Already connected
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, r.url, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	r.conn = conn
	r.done = make(chan struct{})

	// Start message handler
	go r.handleMessages()

	// Start heartbeat
	go r.heartbeat()

	return nil
}

// Disconnect closes the WebSocket connection.
func (r *RealtimeClient) Disconnect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conn == nil {
		return nil
	}

	close(r.done)

	err := r.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	if err != nil {
		r.conn.Close()
		r.conn = nil
		return fmt.Errorf("close message: %w", err)
	}

	r.conn.Close()
	r.conn = nil
	return nil
}

// Channel returns or creates a channel.
func (r *RealtimeClient) Channel(topic string) *Channel {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ch, ok := r.channels[topic]; ok {
		return ch
	}

	ch := &Channel{
		client: r,
		topic:  topic,
	}
	r.channels[topic] = ch
	return ch
}

// Subscribe subscribes to a table.
func (c *Channel) Subscribe(ctx context.Context) error {
	c.client.mu.Lock()
	defer c.client.mu.Unlock()

	if c.joined {
		return nil
	}

	c.client.ref++
	ref := fmt.Sprintf("%d", c.client.ref)
	c.joinRef = ref

	msg := map[string]any{
		"topic":    c.topic,
		"event":    "phx_join",
		"payload":  map[string]any{},
		"ref":      ref,
		"join_ref": ref,
	}

	if err := c.client.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("send join: %w", err)
	}

	c.joined = true
	return nil
}

// Unsubscribe unsubscribes from the channel.
func (c *Channel) Unsubscribe(ctx context.Context) error {
	c.client.mu.Lock()
	defer c.client.mu.Unlock()

	if !c.joined {
		return nil
	}

	c.client.ref++
	ref := fmt.Sprintf("%d", c.client.ref)

	msg := map[string]any{
		"topic":    c.topic,
		"event":    "phx_leave",
		"payload":  map[string]any{},
		"ref":      ref,
		"join_ref": c.joinRef,
	}

	if err := c.client.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("send leave: %w", err)
	}

	c.joined = false
	delete(c.client.channels, c.topic)
	return nil
}

// On registers an event handler.
func (c *Channel) On(event string, handler EventHandler) *Channel {
	c.client.mu.Lock()
	defer c.client.mu.Unlock()

	key := c.topic + ":" + event
	c.client.handlers[key] = append(c.client.handlers[key], handler)
	return c
}

// OnInsert registers a handler for INSERT events.
func (c *Channel) OnInsert(handler EventHandler) *Channel {
	return c.On("INSERT", handler)
}

// OnUpdate registers a handler for UPDATE events.
func (c *Channel) OnUpdate(handler EventHandler) *Channel {
	return c.On("UPDATE", handler)
}

// OnDelete registers a handler for DELETE events.
func (c *Channel) OnDelete(handler EventHandler) *Channel {
	return c.On("DELETE", handler)
}

// OnAll registers a handler for all events.
func (c *Channel) OnAll(handler EventHandler) *Channel {
	c.On("INSERT", handler)
	c.On("UPDATE", handler)
	c.On("DELETE", handler)
	return c
}

func (r *RealtimeClient) handleMessages() {
	for {
		select {
		case <-r.done:
			return
		default:
		}

		r.mu.RLock()
		conn := r.conn
		r.mu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			// Connection closed
			return
		}

		var event RealtimeEvent
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}

		r.dispatchEvent(&event)
	}
}

func (r *RealtimeClient) dispatchEvent(event *RealtimeEvent) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get event type from payload if available
	eventType := event.Event
	if payload, ok := event.Payload["type"].(string); ok {
		eventType = payload
	}

	key := event.Topic + ":" + eventType
	handlers := r.handlers[key]

	for _, handler := range handlers {
		go handler(event)
	}
}

func (r *RealtimeClient) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.done:
			return
		case <-ticker.C:
			r.mu.Lock()
			if r.conn != nil {
				r.ref++
				msg := map[string]any{
					"topic":   "phoenix",
					"event":   "heartbeat",
					"payload": map[string]any{},
					"ref":     fmt.Sprintf("%d", r.ref),
				}
				r.conn.WriteJSON(msg)
			}
			r.mu.Unlock()
		}
	}
}

// =============================================================================
// Postgres Changes Subscription
// =============================================================================

// PostgresChangesConfig configures postgres changes subscription.
type PostgresChangesConfig struct {
	Event  string // INSERT, UPDATE, DELETE, *
	Schema string
	Table  string
	Filter string // Optional filter like "id=eq.1"
}

// SubscribeToPostgresChanges subscribes to postgres changes.
func (r *RealtimeClient) SubscribeToPostgresChanges(ctx context.Context, cfg PostgresChangesConfig, handler EventHandler) (*Channel, error) {
	if cfg.Schema == "" {
		cfg.Schema = "public"
	}
	if cfg.Event == "" {
		cfg.Event = "*"
	}

	topic := fmt.Sprintf("realtime:%s:%s", cfg.Schema, cfg.Table)
	if cfg.Filter != "" {
		topic += ":" + cfg.Filter
	}

	ch := r.Channel(topic)

	// Register handler based on event type
	switch cfg.Event {
	case "*":
		ch.OnAll(handler)
	case "INSERT":
		ch.OnInsert(handler)
	case "UPDATE":
		ch.OnUpdate(handler)
	case "DELETE":
		ch.OnDelete(handler)
	}

	if err := ch.Subscribe(ctx); err != nil {
		return nil, err
	}

	return ch, nil
}
