package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	rocketmq "github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"

	"github.com/R3E-Network/service_layer/internal/config"
)

// rocketmqModule adapts RocketMQ as an EventEngine backend for the service OS.
type rocketmqModule struct {
	name    string
	domain  string
	cfg     config.RocketMQConfig
	prefix  string
	mu      sync.Mutex
	prod    rocketmq.Producer
	cons    rocketmq.PushConsumer
	started bool
	consUp  bool
}

func (m *rocketmqModule) Name() string   { return m.name }
func (m *rocketmqModule) Domain() string { return m.domain }

func (m *rocketmqModule) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return nil
	}
	prod, err := rocketmq.NewProducer(
		producer.WithNameServer(m.cfg.NameServers),
		producer.WithCredentials(primitive.Credentials{
			AccessKey: m.cfg.AccessKey,
			SecretKey: m.cfg.SecretKey,
		}),
		producer.WithNamespace(strings.TrimSpace(m.cfg.Namespace)),
		producer.WithRetry(2),
	)
	if err != nil {
		return fmt.Errorf("create rocketmq producer: %w", err)
	}
	if err := prod.Start(); err != nil {
		return fmt.Errorf("start rocketmq producer: %w", err)
	}
	m.prod = prod
	m.started = true
	return nil
}

func (m *rocketmqModule) Stop(ctx context.Context) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.prod != nil {
		_ = m.prod.Shutdown()
		m.prod = nil
	}
	if m.cons != nil {
		_ = m.cons.Shutdown()
		m.cons = nil
	}
	m.started = false
	m.consUp = false
	return nil
}

func (m *rocketmqModule) Ready(ctx context.Context) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return fmt.Errorf("rocketmq producer not started")
	}
	if len(m.cfg.NameServers) == 0 {
		return fmt.Errorf("no rocketmq name servers configured")
	}
	return nil
}

// Publish sends an event payload to RocketMQ.
func (m *rocketmqModule) Publish(ctx context.Context, event string, payload any) error {
	event = strings.TrimSpace(event)
	if event == "" {
		return fmt.Errorf("event required")
	}
	topic := m.topicFor(event)

	body, err := json.Marshal(map[string]any{
		"event":   event,
		"payload": payload,
		"sent_at": time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}
	m.mu.Lock()
	prod := m.prod
	m.mu.Unlock()
	if prod == nil {
		return fmt.Errorf("rocketmq producer not ready")
	}

	msg := &primitive.Message{
		Topic: topic,
		Body:  body,
	}
	msg.WithProperty("event", event)
	msg.WithProperty("domain", m.domain)
	if m.cfg.Namespace != "" {
		msg.WithProperty("namespace", m.cfg.Namespace)
	}

	if _, err := prod.SendSync(ctx, msg); err != nil {
		return fmt.Errorf("rocketmq send: %w", err)
	}
	return nil
}

// Subscribe registers a handler for an event topic via RocketMQ.
func (m *rocketmqModule) Subscribe(ctx context.Context, event string, handler func(context.Context, any) error) error {
	if handler == nil {
		return fmt.Errorf("handler required")
	}
	event = strings.TrimSpace(event)
	if event == "" {
		return fmt.Errorf("event required")
	}
	topic := m.topicFor(event)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cons == nil {
		opts := []consumer.Option{
			consumer.WithGroupName(m.consumerGroup()),
			consumer.WithNameServer(m.cfg.NameServers),
			consumer.WithCredentials(primitive.Credentials{
				AccessKey: m.cfg.AccessKey,
				SecretKey: m.cfg.SecretKey,
			}),
			consumer.WithNamespace(strings.TrimSpace(m.cfg.Namespace)),
		}
		if m.cfg.MaxReconsume > 0 {
			opts = append(opts, consumer.WithMaxReconsumeTimes(int32(m.cfg.MaxReconsume)))
		}
		if m.cfg.ConsumeBatch > 0 {
			opts = append(opts, consumer.WithConsumeMessageBatchMaxSize(m.cfg.ConsumeBatch))
		}
		switch strings.ToLower(strings.TrimSpace(m.cfg.ConsumeFrom)) {
		case "first":
			opts = append(opts, consumer.WithConsumeFromWhere(consumer.ConsumeFromFirstOffset))
		case "latest":
			opts = append(opts, consumer.WithConsumeFromWhere(consumer.ConsumeFromLastOffset))
		}
		cons, err := rocketmq.NewPushConsumer(opts...)
		if err != nil {
			return fmt.Errorf("create rocketmq consumer: %w", err)
		}
		m.cons = cons
	}

	if err := m.cons.Subscribe(topic, consumer.MessageSelector{}, func(c context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, msg := range msgs {
			var envelope map[string]any
			if err := json.Unmarshal(msg.Body, &envelope); err != nil {
				return consumer.ConsumeRetryLater, nil
			}
			payload := envelope["payload"]
			if payload == nil {
				payload = envelope
			}
			if err := handler(c, payload); err != nil {
				return consumer.ConsumeRetryLater, nil
			}
		}
		return consumer.ConsumeSuccess, nil
	}); err != nil {
		return fmt.Errorf("subscribe to topic %s: %w", topic, err)
	}

	// Start consumer lazily.
	if !m.consUp {
		if err := m.cons.Start(); err != nil {
			return fmt.Errorf("start rocketmq consumer: %w", err)
		}
		m.consUp = true
	}
	return nil
}

func (m *rocketmqModule) consumerGroup() string {
	if strings.TrimSpace(m.cfg.ConsumerGroup) != "" {
		return m.cfg.ConsumerGroup
	}
	return "service-layer"
}

func (m *rocketmqModule) topicFor(event string) string {
	event = sanitize(event)
	prefix := strings.TrimSpace(m.cfg.TopicPrefix)
	if prefix == "" {
		prefix = "sl"
	}
	if ns := strings.TrimSpace(m.cfg.Namespace); ns != "" {
		prefix = sanitize(ns) + "." + prefix
	}
	return fmt.Sprintf("%s.%s", prefix, event)
}

func sanitize(in string) string {
	in = strings.TrimSpace(strings.ToLower(in))
	in = strings.ReplaceAll(in, " ", "-")
	return in
}

func newRocketMQModule(cfg config.RocketMQConfig) *rocketmqModule {
	return &rocketmqModule{
		name:   "svc-rocketmq",
		domain: "event",
		cfg:    cfg,
		prefix: cfg.TopicPrefix,
	}
}
