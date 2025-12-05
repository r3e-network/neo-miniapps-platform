// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"sync"
)

// contractAPIImpl implements ContractAPI.
type contractAPIImpl struct {
	ctx       *ServiceContext
	serviceID string
	addresses *ContractAddresses
	mu        sync.RWMutex
	requests  map[string]*ServiceRequest
	triggers  map[string]*AutomationTrigger
	handlers  []ServiceRequestHandler
}

func newContractAPI(ctx *ServiceContext, serviceID string) *contractAPIImpl {
	return &contractAPIImpl{
		ctx:       ctx,
		serviceID: serviceID,
		addresses: &ContractAddresses{},
		requests:  make(map[string]*ServiceRequest),
		triggers:  make(map[string]*AutomationTrigger),
	}
}

func (c *contractAPIImpl) SubscribeRequests(ctx context.Context, handler ServiceRequestHandler) (Subscription, error) {
	if err := c.ctx.RequireCapability(CapContract); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers = append(c.handlers, handler)
	return &contractSubscription{api: c, handler: handler}, nil
}

func (c *contractAPIImpl) SendCallback(ctx context.Context, response *ServiceResponse) error {
	if err := c.ctx.RequireCapability(CapContractWrite); err != nil {
		return err
	}
	// TODO: Implement actual Neo N3 contract call via client
	// This would use the contract client to call Gateway.Callback()
	c.ctx.logger.Info("sending callback", "request_id", response.RequestID, "success", response.Success)
	return nil
}

func (c *contractAPIImpl) GetRequest(ctx context.Context, requestID string) (*ServiceRequest, error) {
	if err := c.ctx.RequireCapability(CapContract); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	req, ok := c.requests[requestID]
	if !ok {
		return nil, NewOSError("NOT_FOUND", "request not found: "+requestID)
	}
	return req, nil
}

func (c *contractAPIImpl) GetPendingRequests(ctx context.Context) ([]*ServiceRequest, error) {
	if err := c.ctx.RequireCapability(CapContract); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	var pending []*ServiceRequest
	for _, req := range c.requests {
		pending = append(pending, req)
	}
	return pending, nil
}

func (c *contractAPIImpl) UpdatePrice(ctx context.Context, update *PriceUpdate) error {
	if err := c.ctx.RequireCapability(CapContractWrite); err != nil {
		return err
	}
	// TODO: Implement actual Neo N3 contract call via client
	// This would use the contract client to call DataFeeds.UpdatePrice()
	c.ctx.logger.Info("updating price", "feed_id", update.FeedID, "price", update.Price)
	return nil
}

func (c *contractAPIImpl) ExecuteTrigger(ctx context.Context, triggerID string) error {
	if err := c.ctx.RequireCapability(CapContractWrite); err != nil {
		return err
	}
	// TODO: Implement actual Neo N3 contract call via client
	// This would use the contract client to call Automation.ExecuteTrigger()
	c.ctx.logger.Info("executing trigger", "trigger_id", triggerID)
	return nil
}

func (c *contractAPIImpl) GetTrigger(ctx context.Context, triggerID string) (*AutomationTrigger, error) {
	if err := c.ctx.RequireCapability(CapContract); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	trigger, ok := c.triggers[triggerID]
	if !ok {
		return nil, NewOSError("NOT_FOUND", "trigger not found: "+triggerID)
	}
	return trigger, nil
}

func (c *contractAPIImpl) GetContractAddresses() *ContractAddresses {
	return c.addresses
}

// SetContractAddresses sets the contract addresses (called during initialization).
func (c *contractAPIImpl) SetContractAddresses(addresses *ContractAddresses) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addresses = addresses
}

// AddRequest adds a request to the pending list (called by event listener).
func (c *contractAPIImpl) AddRequest(req *ServiceRequest) {
	c.mu.Lock()
	c.requests[req.RequestID] = req
	handlers := c.handlers
	c.mu.Unlock()

	// Notify handlers
	for _, h := range handlers {
		go func(handler ServiceRequestHandler) {
			_ = handler(c.ctx.ctx, req)
		}(h)
	}
}

// RemoveRequest removes a request from the pending list.
func (c *contractAPIImpl) RemoveRequest(requestID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.requests, requestID)
}

// AddTrigger adds a trigger to the list.
func (c *contractAPIImpl) AddTrigger(trigger *AutomationTrigger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.triggers[trigger.TriggerID] = trigger
}

// RemoveTrigger removes a trigger from the list.
func (c *contractAPIImpl) RemoveTrigger(triggerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.triggers, triggerID)
}

type contractSubscription struct {
	api     *contractAPIImpl
	handler ServiceRequestHandler
}

func (s *contractSubscription) Unsubscribe() error {
	s.api.mu.Lock()
	defer s.api.mu.Unlock()
	// Remove handler from list
	for i, h := range s.api.handlers {
		// Compare function pointers (this is a simplification)
		if &h == &s.handler {
			s.api.handlers = append(s.api.handlers[:i], s.api.handlers[i+1:]...)
			break
		}
	}
	return nil
}
