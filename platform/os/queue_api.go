// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// queueAPIImpl implements QueueAPI.
type queueAPIImpl struct {
	ctx       *ServiceContext
	serviceID string
	mu        sync.RWMutex
	queues    map[string][]*QueueMessage
	nextID    int
}

func newQueueAPI(ctx *ServiceContext, serviceID string) *queueAPIImpl {
	return &queueAPIImpl{
		ctx:       ctx,
		serviceID: serviceID,
		queues:    make(map[string][]*QueueMessage),
	}
}

func (q *queueAPIImpl) Publish(ctx context.Context, queue string, message *QueueMessage) error {
	if err := q.ctx.RequireCapability(CapQueue); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	q.nextID++
	message.ID = fmt.Sprintf("%s-msg-%d", q.serviceID, q.nextID)
	message.Queue = queue
	message.PublishedAt = time.Now()

	q.queues[queue] = append(q.queues[queue], message)
	return nil
}

func (q *queueAPIImpl) Subscribe(ctx context.Context, queue string, handler func(*QueueMessage) error) (Subscription, error) {
	if err := q.ctx.RequireCapability(CapQueue); err != nil {
		return nil, err
	}
	return &queueSubscription{queue: queue, queueAPI: q}, nil
}

func (q *queueAPIImpl) Ack(ctx context.Context, messageID string) error {
	if err := q.ctx.RequireCapability(CapQueue); err != nil {
		return err
	}
	// TODO: Implement message acknowledgment
	return nil
}

func (q *queueAPIImpl) Nack(ctx context.Context, messageID string, requeue bool) error {
	if err := q.ctx.RequireCapability(CapQueue); err != nil {
		return err
	}
	// TODO: Implement negative acknowledgment
	return nil
}

func (q *queueAPIImpl) GetQueueInfo(ctx context.Context, queue string) (*QueueInfo, error) {
	if err := q.ctx.RequireCapability(CapQueue); err != nil {
		return nil, err
	}
	q.mu.RLock()
	defer q.mu.RUnlock()

	messages := q.queues[queue]
	return &QueueInfo{
		Name:         queue,
		MessageCount: int64(len(messages)),
	}, nil
}

type queueSubscription struct {
	queue    string
	queueAPI *queueAPIImpl
}

func (s *queueSubscription) Unsubscribe() error {
	return nil
}
