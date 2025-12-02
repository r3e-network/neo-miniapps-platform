// Package framework provides the unified Service OS layer for service packages.
// This file re-exports types from system/core so that services only need to
// import system/framework instead of multiple system packages.
//
// ARCHITECTURE: Services should only depend on this package (system/framework)
// for all their system-level needs. This simplifies the dependency graph:
//
//	Before: Services → {system/core, system/framework, system/framework/core, system/runtime}
//	After:  Services → system/framework (single dependency)
package framework

import (
	engine "github.com/R3E-Network/service_layer/system/core"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// =============================================================================
// Core Engine Types (re-exported from system/core)
// =============================================================================

// ServiceModule is the common contract every service must implement.
// Re-exported from system/core for convenience.
type ServiceModule = engine.ServiceModule

// NOTE: APISurface is already defined in builder.go as:
//   type APISurface = engine.APISurface

// API Surface constants
const (
	APISurfaceLifecycle  = engine.APISurfaceLifecycle
	APISurfaceReadiness  = engine.APISurfaceReadiness
	APISurfaceAccount    = engine.APISurfaceAccount
	APISurfaceStore      = engine.APISurfaceStore
	APISurfaceCompute    = engine.APISurfaceCompute
	APISurfaceData       = engine.APISurfaceData
	APISurfaceEvent      = engine.APISurfaceEvent
	APISurfaceRPC        = engine.APISurfaceRPC
	APISurfaceIndexer    = engine.APISurfaceIndexer
	APISurfaceLedger     = engine.APISurfaceLedger
	APISurfaceDataSource = engine.APISurfaceDataSource
	APISurfaceContracts  = engine.APISurfaceContracts
	APISurfaceGasBank    = engine.APISurfaceGasBank
	APISurfaceCrypto     = engine.APISurfaceCrypto
)

// APIDescriptor describes a standard API surface a module participates in.
type APIDescriptor = engine.APIDescriptor

// =============================================================================
// Engine Interfaces (re-exported from system/core)
// =============================================================================

// AccountEngine covers account lifecycle and tenancy.
type AccountEngine = engine.AccountEngine

// StoreEngine abstracts persistence.
type StoreEngine = engine.StoreEngine

// ComputeEngine abstracts execution of user functions or jobs.
type ComputeEngine = engine.ComputeEngine

// DataEngine abstracts data-plane services.
type DataEngine = engine.DataEngine

// EventEngine abstracts event dispatch/subscribe.
type EventEngine = engine.EventEngine

// SecretsEngine provides secure secret storage.
type SecretsEngine = engine.SecretsEngine

// ContractsEngine manages deployment/invocation of contracts.
type ContractsEngine = engine.ContractsEngine

// =============================================================================
// Service Registry (re-exported from system/core)
// =============================================================================

// ServiceRegistry provides type-safe service lookup for cross-service communication.
type ServiceRegistry = engine.ServiceRegistry

// FeeCollector is the interface for collecting service fees.
type FeeCollector = engine.FeeCollector

// =============================================================================
// Capability Markers (re-exported from system/core)
// =============================================================================

// ReadyChecker reports whether a module is ready to serve traffic.
type ReadyChecker = engine.ReadyChecker

// ReadySetter allows the engine to mark readiness explicitly.
type ReadySetter = engine.ReadySetter

// AccountCapable indicates whether a module supports account operations.
type AccountCapable = engine.AccountCapable

// ComputeCapable indicates whether a module supports compute operations.
type ComputeCapable = engine.ComputeCapable

// DataCapable indicates whether a module supports data operations.
type DataCapable = engine.DataCapable

// EventCapable indicates whether a module supports event operations.
type EventCapable = engine.EventCapable

// =============================================================================
// Framework Core Types (re-exported from system/framework/core)
// =============================================================================

// NOTE: Descriptor is already defined in builder.go as:
//   type Descriptor = service.Descriptor

// ObservationHooks provides lifecycle observation callbacks.
type ObservationHooks = core.ObservationHooks

// APIRequest represents a unified API request structure.
type APIRequest = core.APIRequest

// APIEndpoint represents endpoint metadata.
type APIEndpoint = core.APIEndpoint

// APIProvider is an optional interface for explicit endpoint declaration.
type APIProvider = core.APIProvider

// Tracer propagates tracing spans.
type Tracer = core.Tracer
