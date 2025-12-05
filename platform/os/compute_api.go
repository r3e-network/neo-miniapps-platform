// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"

	"github.com/R3E-Network/service_layer/tee/types"
)

// computeAPIImpl implements ComputeAPI.
type computeAPIImpl struct {
	ctx       *ServiceContext
	compute   types.ConfidentialCompute
	serviceID string
}

func newComputeAPI(ctx *ServiceContext, compute types.ConfidentialCompute, serviceID string) *computeAPIImpl {
	return &computeAPIImpl{
		ctx:       ctx,
		compute:   compute,
		serviceID: serviceID,
	}
}

func (c *computeAPIImpl) Execute(ctx context.Context, req ComputeRequest) (*ComputeResult, error) {
	if err := c.ctx.RequireCapability(CapCompute); err != nil {
		return nil, err
	}

	teeReq := types.ComputeRequest{
		ServiceID:  c.serviceID,
		Script:     req.Script,
		EntryPoint: req.EntryPoint,
		Input:      req.Input,
		Timeout:    req.Timeout,
	}

	result, err := c.compute.Execute(ctx, teeReq)
	if err != nil {
		return nil, err
	}

	return &ComputeResult{
		Success: result.Status == types.ComputeStatusSucceeded,
		Output:  result.Output,
		Logs:    result.Logs,
		Error:   result.Error,
	}, nil
}

func (c *computeAPIImpl) ExecuteWithSecrets(ctx context.Context, req ComputeRequest, secretNames []string) (*ComputeResult, error) {
	if err := c.ctx.RequireCapability(CapCompute); err != nil {
		return nil, err
	}

	teeReq := types.ComputeRequest{
		ServiceID:  c.serviceID,
		Script:     req.Script,
		EntryPoint: req.EntryPoint,
		Input:      req.Input,
		Timeout:    req.Timeout,
	}

	refs := make([]types.SecretRef, len(secretNames))
	for i, name := range secretNames {
		refs[i] = types.SecretRef{Namespace: c.serviceID, Name: name, Alias: name}
	}

	result, err := c.compute.ExecuteWithSecrets(ctx, teeReq, refs)
	if err != nil {
		return nil, err
	}

	return &ComputeResult{
		Success: result.Status == types.ComputeStatusSucceeded,
		Output:  result.Output,
		Logs:    result.Logs,
		Error:   result.Error,
	}, nil
}
