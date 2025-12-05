// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
)

// eventsAPIImpl implements EventsAPI.
type eventsAPIImpl struct {
	ctx       *ServiceContext
	serviceID string
	bus       *EventBus
}

func newEventsAPI(ctx *ServiceContext, serviceID string) *eventsAPIImpl {
	return &eventsAPIImpl{
		ctx:       ctx,
		serviceID: serviceID,
		bus:       GetEventBus(),
	}
}

func (e *eventsAPIImpl) Publish(ctx context.Context, topic string, data any) error {
	if err := e.ctx.RequireCapability(CapEvents); err != nil {
		return err
	}
	return e.bus.Publish(ctx, topic, e.serviceID, data)
}

func (e *eventsAPIImpl) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	if err := e.ctx.RequireCapability(CapEvents); err != nil {
		return nil, err
	}
	return e.bus.Subscribe(topic, e.serviceID, handler)
}
