package framework

import (
	"context"

	engine "github.com/R3E-Network/service_layer/system/core"
	service "github.com/R3E-Network/service_layer/system/framework/core"
)

// EngineBus wraps engine fan-out so services can depend on BusClient without
// importing the engine directly.
type EngineBus struct {
	eng *engine.Engine
}

// NewEngineBus creates a BusClient backed by the engine. It is safe to pass a nil engine
// (methods will return an error).
func NewEngineBus(eng *engine.Engine) *EngineBus {
	return &EngineBus{eng: eng}
}

// PublishEvent fan-outs an event to all registered EventEngines.
func (b *EngineBus) PublishEvent(ctx context.Context, event string, payload any) error {
	if b == nil || b.eng == nil {
		return service.ErrBusUnavailable
	}
	return b.eng.PublishEvent(ctx, event, payload)
}

// PushData fan-outs a payload to all registered DataEngines.
func (b *EngineBus) PushData(ctx context.Context, topic string, payload any) error {
	if b == nil || b.eng == nil {
		return service.ErrBusUnavailable
	}
	return b.eng.PushData(ctx, topic, payload)
}

// InvokeCompute invokes all ComputeEngines and returns results.
func (b *EngineBus) InvokeCompute(ctx context.Context, payload any) ([]ComputeResult, error) {
	if b == nil || b.eng == nil {
		return nil, service.ErrBusUnavailable
	}
	results, err := b.eng.InvokeComputeAll(ctx, payload)
	out := make([]ComputeResult, 0, len(results))
	for _, r := range results {
		out = append(out, ComputeResult{Module: r.Module, Result: r.Result, Err: r.Err})
	}
	return out, err
}
