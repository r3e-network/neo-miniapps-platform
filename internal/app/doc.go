// Package app provides the Application Composition Layer for the service layer.
//
// # Architecture Role
//
// The app package sits above the core layers (platform, framework, engine, services)
// and is responsible for composing them into a running application. It is NOT a
// business logic layer - business logic belongs in internal/services/.
//
// # Package Structure
//
//	internal/app/
//	├── application.go      # Main application struct, wiring, and lifecycle
//	├── domain/             # Domain models (pure data structures)
//	│   ├── account/        # Account models
//	│   ├── function/       # Function definitions and executions
//	│   ├── gasbank/        # Gas bank accounts and transactions
//	│   └── ...             # Other domain models
//	├── storage/            # Storage interfaces and implementations
//	│   ├── interfaces.go   # Store interfaces (AccountStore, FunctionStore, etc.)
//	│   ├── memory/         # In-memory implementation for testing
//	│   └── postgres/       # PostgreSQL implementation for production
//	├── httpapi/            # HTTP API handlers and routing
//	│   ├── handler.go      # Request handlers
//	│   ├── router.go       # Route definitions
//	│   └── middleware.go   # HTTP middleware
//	├── auth/               # Authentication and authorization
//	├── system/             # System management (lifecycle, descriptors)
//	├── jam/                # JAM protocol integration
//	└── metrics/            # Application metrics
//
// # Responsibilities
//
// The app package is responsible for:
//
//   - Composing services from internal/services/ with their dependencies
//   - Defining storage interfaces that services depend on
//   - Providing domain models shared across services
//   - Exposing HTTP API endpoints for external access
//   - Managing application-level concerns (auth, metrics, system status)
//
// # What Belongs Here vs internal/services/
//
//	┌─────────────────────────────────────────────────────────────────────┐
//	│                      internal/app/ (Composition)                     │
//	├─────────────────────────────────────────────────────────────────────┤
//	│ ✓ Application struct and wiring                                      │
//	│ ✓ Domain models (pure data, no business logic)                       │
//	│ ✓ Storage interfaces (repository pattern)                            │
//	│ ✓ HTTP handlers (request/response handling)                          │
//	│ ✓ Authentication and authorization                                   │
//	│ ✓ Application metrics and observability                              │
//	│ ✗ Business logic (belongs in internal/services/)                     │
//	│ ✗ Service implementations (belongs in internal/services/)            │
//	└─────────────────────────────────────────────────────────────────────┘
//
//	┌─────────────────────────────────────────────────────────────────────┐
//	│                    internal/services/ (Business Logic)               │
//	├─────────────────────────────────────────────────────────────────────┤
//	│ ✓ Service implementations (accounts, functions, oracle, etc.)        │
//	│ ✓ Business rules and validation                                      │
//	│ ✓ Service-to-service coordination                                    │
//	│ ✓ Domain-specific operations                                         │
//	│ ✗ HTTP handling (belongs in internal/app/httpapi/)                   │
//	│ ✗ Storage implementations (belongs in internal/app/storage/)         │
//	└─────────────────────────────────────────────────────────────────────┘
//
// # Dependency Direction
//
// The dependency flow is:
//
//	cmd/appserver/
//	      │
//	      ▼
//	internal/app/ (composition)
//	      │
//	      ├──► internal/services/ (business logic)
//	      │           │
//	      │           ├──► internal/framework/ (service base)
//	      │           │
//	      │           └──► internal/engine/ (interfaces only)
//	      │
//	      ├──► internal/engine/ (orchestration)
//	      │           │
//	      │           └──► internal/framework/
//	      │
//	      └──► internal/platform/ (drivers)
//
// # Example: Adding a New Domain
//
// When adding a new domain (e.g., "rewards"):
//
//  1. Create domain models in internal/app/domain/rewards/
//  2. Add storage interface to internal/app/storage/interfaces.go
//  3. Implement storage in internal/app/storage/postgres/ and memory/
//  4. Create service in internal/services/rewards/service.go
//  5. Wire service in internal/app/application.go
//  6. Add HTTP handlers in internal/app/httpapi/handler_rewards.go
//
// # Related Packages
//
//   - internal/services: Business logic services
//   - internal/engine: Service orchestration and lifecycle
//   - internal/framework: Service development tools
//   - internal/platform: Low-level drivers
package app
