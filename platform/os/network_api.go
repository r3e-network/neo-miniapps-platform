// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/R3E-Network/service_layer/tee/types"
)

// networkAPIImpl implements NetworkAPI.
type networkAPIImpl struct {
	ctx          *ServiceContext
	network      types.SecureNetwork
	namespace    string
	allowedHosts []string
}

func newNetworkAPI(ctx *ServiceContext, network types.SecureNetwork, namespace string, allowedHosts []string) *networkAPIImpl {
	return &networkAPIImpl{
		ctx:          ctx,
		network:      network,
		namespace:    namespace,
		allowedHosts: allowedHosts,
	}
}

func (n *networkAPIImpl) Fetch(ctx context.Context, req HTTPRequest) (*HTTPResponse, error) {
	if err := n.ctx.RequireCapability(CapNetwork); err != nil {
		return nil, err
	}

	if err := n.ensureAllowed(req.URL); err != nil {
		return nil, err
	}

	teeReq := types.SecureHTTPRequest{
		Method:  req.Method,
		URL:     req.URL,
		Headers: req.Headers,
		Body:    req.Body,
		Timeout: req.Timeout,
	}

	resp, err := n.network.Fetch(ctx, teeReq)
	if err != nil {
		return nil, err
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Headers,
		Body:       resp.Body,
	}, nil
}

func (n *networkAPIImpl) FetchWithSecret(ctx context.Context, req HTTPRequest, secretName string, authType AuthType) (*HTTPResponse, error) {
	if err := n.ctx.RequireCapability(CapNetwork); err != nil {
		return nil, err
	}

	if err := n.ensureAllowed(req.URL); err != nil {
		return nil, err
	}

	teeReq := types.SecureHTTPRequest{
		Method:  req.Method,
		URL:     req.URL,
		Headers: req.Headers,
		Body:    req.Body,
		Timeout: req.Timeout,
	}

	var teeAuthType types.AuthType
	switch authType {
	case AuthBearer:
		teeAuthType = types.AuthTypeBearer
	case AuthBasic:
		teeAuthType = types.AuthTypeBasic
	case AuthAPIKey:
		teeAuthType = types.AuthTypeAPIKey
	}

	resp, err := n.network.FetchWithSecret(ctx, teeReq, n.namespace, secretName, teeAuthType)
	if err != nil {
		return nil, err
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Headers,
		Body:       resp.Body,
	}, nil
}

func (n *networkAPIImpl) RPC(ctx context.Context, endpoint, method string, params any) ([]byte, error) {
	if err := n.ctx.RequireCapability(CapNetwork); err != nil {
		return nil, err
	}

	if err := n.ensureAllowed(endpoint); err != nil {
		return nil, err
	}

	result, err := n.network.RPC(ctx, endpoint, method, params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (n *networkAPIImpl) RPCWithSecret(ctx context.Context, endpoint, method string, params any, secretName string) ([]byte, error) {
	if err := n.ctx.RequireCapability(CapNetwork); err != nil {
		return nil, err
	}

	if err := n.ensureAllowed(endpoint); err != nil {
		return nil, err
	}

	result, err := n.network.RPCWithSecret(ctx, endpoint, method, params, n.namespace, secretName)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ensureAllowed enforces per-service outbound host allowlist (if provided).
func (n *networkAPIImpl) ensureAllowed(rawURL string) error {
	if len(n.allowedHosts) == 0 {
		return nil
	}
	host, err := parseHost(rawURL)
	if err != nil {
		return err
	}
	for _, allowed := range n.allowedHosts {
		if allowed == host {
			return nil
		}
	}
	return NewOSError(ErrCodeNetworkError, "host not allowed: "+host)
}

// parseHost extracts the hostname from a URL (supports full URLs or host:port).
func parseHost(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty url")
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", err
		}
		return u.Hostname(), nil
	}
	// Fallback: treat as host[:port]
	host, _, err := net.SplitHostPort(raw)
	if err == nil {
		return host, nil
	}
	// If no port, the whole string is host
	return raw, nil
}
