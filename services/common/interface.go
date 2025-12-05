// Package common provides consistent service interfaces for Neo Service Layer.
// All 14 services implement these interfaces for consistency and interoperability.
//
// Architecture: MarbleRun + EGo + Supabase + Netlify + Neo N3
package common

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/marble/sdk"
)

// =============================================================================
// Service Types (All 14 Services)
// =============================================================================

// ServiceType represents the type of service.
type ServiceType string

const (
	ServiceOracle       ServiceType = "oracle"
	ServiceVRF          ServiceType = "vrf"
	ServiceSecrets      ServiceType = "secrets"
	ServiceGasBank      ServiceType = "gasbank"
	ServiceMixer        ServiceType = "mixer"
	ServiceDataFeeds    ServiceType = "datafeeds"
	ServiceAccounts     ServiceType = "accounts"
	ServiceAutomation   ServiceType = "automation"
	ServiceCCIP         ServiceType = "ccip"
	ServiceConfidential ServiceType = "confidential"
	ServiceCRE          ServiceType = "cre"
	ServiceDataLink     ServiceType = "datalink"
	ServiceDataStreams  ServiceType = "datastreams"
	ServiceDTA          ServiceType = "dta"
)

// AllServiceTypes returns all 14 service types.
func AllServiceTypes() []ServiceType {
	return []ServiceType{
		ServiceOracle, ServiceVRF, ServiceSecrets, ServiceGasBank,
		ServiceMixer, ServiceDataFeeds, ServiceAccounts, ServiceAutomation,
		ServiceCCIP, ServiceConfidential, ServiceCRE, ServiceDataLink,
		ServiceDataStreams, ServiceDTA,
	}
}

// =============================================================================
// Service Registry
// =============================================================================

// ServiceRegistry manages all services.
type ServiceRegistry struct {
	services map[ServiceType]Service
}

// NewServiceRegistry creates a new service registry.
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[ServiceType]Service),
	}
}

// Register registers a service.
func (r *ServiceRegistry) Register(serviceType ServiceType, service Service) {
	r.services[serviceType] = service
}

// Get returns a service by type.
func (r *ServiceRegistry) Get(serviceType ServiceType) (Service, bool) {
	s, ok := r.services[serviceType]
	return s, ok
}

// All returns all registered services.
func (r *ServiceRegistry) All() map[ServiceType]Service {
	return r.services
}

// InitializeAll initializes all services.
func (r *ServiceRegistry) InitializeAll(ctx context.Context) error {
	for st, s := range r.services {
		if err := s.Initialize(ctx); err != nil {
			return fmt.Errorf("initialize %s: %w", st, err)
		}
	}
	return nil
}

// StartAll starts all services.
func (r *ServiceRegistry) StartAll(ctx context.Context) error {
	for st, s := range r.services {
		if err := s.Start(ctx); err != nil {
			return fmt.Errorf("start %s: %w", st, err)
		}
	}
	return nil
}

// StopAll stops all services.
func (r *ServiceRegistry) StopAll(ctx context.Context) error {
	var lastErr error
	for st, s := range r.services {
		if err := s.Stop(ctx); err != nil {
			lastErr = fmt.Errorf("stop %s: %w", st, err)
		}
	}
	return lastErr
}

// HealthAll returns health status for all services.
func (r *ServiceRegistry) HealthAll() map[ServiceType]HealthStatus {
	result := make(map[ServiceType]HealthStatus)
	for st, s := range r.services {
		result[st] = s.Health()
	}
	return result
}

// =============================================================================
// Inter-Service Communication
// =============================================================================

// ServiceClient provides inter-service communication.
type ServiceClient struct {
	registry *ServiceRegistry
	marble   *sdk.Marble
}

// NewServiceClient creates a new service client.
func NewServiceClient(registry *ServiceRegistry, marble *sdk.Marble) *ServiceClient {
	return &ServiceClient{
		registry: registry,
		marble:   marble,
	}
}

// Call calls another service.
func (c *ServiceClient) Call(ctx context.Context, serviceType ServiceType, operation string, payload json.RawMessage) (*ServiceResult, error) {
	service, ok := c.registry.Get(serviceType)
	if !ok {
		return nil, fmt.Errorf("service not found: %s", serviceType)
	}

	// Check if service implements ExtendedService
	extService, ok := service.(ExtendedService)
	if !ok {
		return nil, fmt.Errorf("service %s does not support request handling", serviceType)
	}

	req := &ServiceRequest{
		ID:          GenerateRequestID(),
		ServiceType: string(serviceType),
		Operation:   operation,
		Payload:     payload,
		CreatedAt:   time.Now(),
	}

	return extService.HandleRequest(ctx, req)
}

// CallAsync calls another service asynchronously.
func (c *ServiceClient) CallAsync(ctx context.Context, serviceType ServiceType, operation string, payload json.RawMessage, callback func(*ServiceResult, error)) {
	go func() {
		result, err := c.Call(ctx, serviceType, operation, payload)
		if callback != nil {
			callback(result, err)
		}
	}()
}

// =============================================================================
// Extended Service Interface
// =============================================================================

// ExtendedService extends the base Service interface with request handling.
type ExtendedService interface {
	Service
	// HandleRequest handles a service request
	HandleRequest(ctx context.Context, req *ServiceRequest) (*ServiceResult, error)
}

// HandleRequest is a default implementation that returns not implemented.
func (bs *BaseService) HandleRequest(ctx context.Context, req *ServiceRequest) (*ServiceResult, error) {
	return nil, fmt.Errorf("HandleRequest not implemented for %s", bs.name)
}

// Start is a default implementation.
func (bs *BaseService) Start(ctx context.Context) error {
	return nil
}

// Stop is a default implementation.
func (bs *BaseService) Stop(ctx context.Context) error {
	return nil
}


// =============================================================================
// Request ID Generation
// =============================================================================

// GenerateRequestID generates a unique request ID.
func GenerateRequestID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// =============================================================================
// Service Factory
// =============================================================================

// ServiceFactory creates services.
type ServiceFactory struct {
	marble *sdk.Marble
}

// NewServiceFactory creates a new service factory.
func NewServiceFactory(marble *sdk.Marble) *ServiceFactory {
	return &ServiceFactory{marble: marble}
}

// CreateService creates a service by type.
func (f *ServiceFactory) CreateService(serviceType ServiceType, version string) (*BaseService, error) {
	return NewBaseService(&ServiceConfig{
		Name:           string(serviceType),
		Version:        version,
		MarbleType:     string(serviceType),
		SimulationMode: f.marble.IsSimulation(),
	})
}
