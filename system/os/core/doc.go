// Package engine provides the Service Engine (OS Kernel) for the service layer.
//
// # Architecture Overview
//
// The engine layer acts as an operating system kernel for blockchain services.
// It provides core runtime services that all applications (services) depend on:
//
//   - Lifecycle Management: Start, stop, restart services with proper state tracking
//   - Event System: Structured event logging and pub/sub
//   - Metrics Collection: Prometheus-compatible observability
//   - Auto-Recovery: Circuit breaker, exponential backoff, automatic restart
//   - Bus Management: Concurrency limiting for event, data, and compute buses
//   - Domain Engines: Specialized engines for DeFi, GameFi, NFT, RWA domains
//
// # Layered Architecture
//
// The engine sits between the platform layer (drivers) and the services layer:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                    Services Layer (Applications)                 │
//	│  ┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐  │
//	│  │Oracle │ │ VRF   │ │Func   │ │GasBank│ │Feeds  │ │Custom │  │
//	│  │Service│ │Service│ │Service│ │Service│ │Service│ │Service│  │
//	│  └───────┘ └───────┘ └───────┘ └───────┘ └───────┘ └───────┘  │
//	├─────────────────────────────────────────────────────────────────┤
//	│                    Engine Layer (This Package)                   │
//	│                                                                  │
//	│  ┌────────────────────────────────────────────────────────────┐│
//	│  │                    Core Runtime                             ││
//	│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          ││
//	│  │  │  State  │ │ Events  │ │ Metrics │ │Recovery │          ││
//	│  │  │ Machine │ │ Logger  │ │Collector│ │ Manager │          ││
//	│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘          ││
//	│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          ││
//	│  │  │   Bus   │ │Registry │ │Lifecycle│ │Dependency│          ││
//	│  │  │ Limiter │ │         │ │ Manager │ │ Graph   │          ││
//	│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘          ││
//	│  └────────────────────────────────────────────────────────────┘│
//	│                                                                  │
//	│  ┌────────────────────────────────────────────────────────────┐│
//	│  │                   Domain Engines                            ││
//	│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          ││
//	│  │  │  DeFi   │ │ GameFi  │ │   NFT   │ │   RWA   │          ││
//	│  │  │ Engine  │ │ Engine  │ │ Engine  │ │ Engine  │          ││
//	│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘          ││
//	│  └────────────────────────────────────────────────────────────┘│
//	│                                                                  │
//	│  ┌────────────────────────────────────────────────────────────┐│
//	│  │                   System Engines                            ││
//	│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          ││
//	│  │  │ Account │ │  Store  │ │ Compute │ │  Data   │          ││
//	│  │  │ Engine  │ │ Engine  │ │ Engine  │ │ Engine  │          ││
//	│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘          ││
//	│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          ││
//	│  │  │  Event  │ │ Ledger  │ │   RPC   │ │ Crypto  │          ││
//	│  │  │ Engine  │ │ Engine  │ │ Engine  │ │ Engine  │          ││
//	│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘          ││
//	│  └────────────────────────────────────────────────────────────┘│
//	│                                                                  │
//	│  ┌────────────────────────────────────────────────────────────┐│
//	│  │                Security & Access Control                    ││
//	│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          ││
//	│  │  │Security │ │Permissn │ │  Audit  │ │ Secrets │          ││
//	│  │  │ Engine  │ │ Engine  │ │ Engine  │ │ Engine  │          ││
//	│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘          ││
//	│  └────────────────────────────────────────────────────────────┘│
//	│                                                                  │
//	│  ┌────────────────────────────────────────────────────────────┐│
//	│  │                Infrastructure Engines                       ││
//	│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          ││
//	│  │  │  Cache  │ │  Queue  │ │Scheduler│ │ Notify  │          ││
//	│  │  │ Engine  │ │ Engine  │ │ Engine  │ │ Engine  │          ││
//	│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘          ││
//	│  └────────────────────────────────────────────────────────────┘│
//	│                                                                  │
//	│  ┌────────────────────────────────────────────────────────────┐│
//	│  │                Observability Engines                        ││
//	│  │  ┌─────────┐ ┌─────────┐                                   ││
//	│  │  │ Metrics │ │ Tracing │                                   ││
//	│  │  │ Engine  │ │ Engine  │                                   ││
//	│  │  └─────────┘ └─────────┘                                   ││
//	│  └────────────────────────────────────────────────────────────┘│
//	├─────────────────────────────────────────────────────────────────┤
//	│                    Platform Layer (Drivers)                      │
//	└─────────────────────────────────────────────────────────────────┘
//
// # Core Components
//
// ## State Machine (state/)
// Unified service state model with well-defined transitions:
//   - StatusUnknown → StatusRegistered → StatusStarting → StatusRunning
//   - StatusRunning → StatusStopping → StatusStopped
//   - Error states: StatusFailed, StatusStopFailed
//
// ## Event System (events/)
// Structured event logging with:
//   - Ring buffer storage with configurable size
//   - Event types for module lifecycle, bus operations, recovery
//   - Subscription support for real-time monitoring
//
// ## Metrics (metrics/)
// Prometheus-compatible metrics for:
//   - Module status and readiness
//   - Start/stop latency histograms
//   - Bus operations (publish, push, invoke)
//   - Recovery attempts and outcomes
//
// ## Recovery (recovery/)
// Automatic service recovery with:
//   - Restart strategy: Simple immediate restart
//   - Backoff strategy: Exponential backoff between retries
//   - Circuit breaker: Prevents cascading failures
//
// ## Bus Limiter (bus/)
// Concurrency control for service buses:
//   - Per-bus type limits (event, data, compute)
//   - Queue size limits
//   - Timeout handling
//
// ## Bridge (bridge/)
// Integration between Framework services and Engine:
//   - ServiceAdapter wraps services with state tracking
//   - Runtime bundles engine components
//   - Recovery registration helpers
//
// # Domain Engines
//
// Specialized engines for blockchain application domains:
//
// ## DeFi Engine (domains/defi/)
// Interfaces for decentralized finance:
//   - TokenEngine: Token info, balances, prices
//   - SwapEngine: Token swaps with routing
//   - LiquidityEngine: Pool management
//   - LendingEngine: Supply, borrow, repay
//   - StakingEngine: Stake/unstake operations
//   - YieldEngine: Yield farming aggregation
//
// ## GameFi Engine (domains/gamefi/)
// Interfaces for blockchain gaming:
//   - GameEngine: Game registration
//   - PlayerEngine: Player management
//   - AssetEngine: In-game assets
//   - MatchEngine: Game sessions
//   - LeaderboardEngine: Rankings
//   - AchievementEngine: Achievement tracking
//   - QuestEngine: Quest management
//   - TournamentEngine: Tournament operations
//
// ## NFT Engine (domains/nft/)
// Interfaces for non-fungible tokens:
//   - CollectionEngine: NFT collection management
//   - MintEngine: Minting operations
//   - MarketplaceEngine: Trading and auctions
//   - MetadataEngine: NFT metadata handling
//   - RoyaltyEngine: Royalty distribution
//
// ## RWA Engine (domains/rwa/)
// Interfaces for real-world assets:
//   - AssetEngine: RWA tokenization
//   - ComplianceEngine: KYC/AML compliance
//   - CustodyEngine: Asset custody
//   - ValuationEngine: Asset valuation
//   - DistributionEngine: Dividend/yield distribution
//
// # Security & Access Control Engines
//
// ## SecurityEngine
// Provides security policy enforcement and threat detection:
//   - ValidateToken: Validates authentication tokens and returns claims
//   - EnforcePolicy: Checks if actions are allowed by security policies
//
// ## PermissionEngine
// Manages fine-grained permissions and RBAC:
//   - CheckPermission: Verifies subject permissions for actions on resources
//   - GrantPermission/RevokePermission: Manages permission grants
//   - ListPermissions: Lists permissions for a subject
//
// ## AuditEngine
// Provides audit logging and compliance tracking:
//   - LogAuditEvent: Records audit events with actor, action, resource details
//   - QueryAuditLog: Queries audit events with filters
//
// ## SecretsEngine
// Provides secure secret storage and resolution for services:
//   - StoreSecret: Stores encrypted secrets for accounts
//   - GetSecret: Retrieves decrypted secrets by name
//   - DeleteSecret: Removes secrets from storage
//   - ListSecrets: Lists secret names (not values) for an account
//   - ResolveSecrets: Resolves multiple secrets by name
//
// # Infrastructure Engines
//
// ## CacheEngine
// Abstracts caching operations (Redis, Memcached, in-memory):
//   - Get/Set/Delete: Basic cache operations
//   - Exists: Check key existence
//   - TTL support for automatic expiration
//
// ## QueueEngine
// Abstracts message queue operations (RabbitMQ, Kafka, SQS):
//   - Enqueue/Dequeue: Basic queue operations
//   - Subscribe: Register handlers for queue messages
//
// ## SchedulerEngine
// Manages scheduled tasks and cron jobs:
//   - Schedule: Register tasks with cron expressions
//   - Cancel: Cancel scheduled tasks
//   - ListTasks: List all scheduled tasks
//
// ## NotificationEngine
// Handles notifications across channels:
//   - Send: Send notifications via email, SMS, push, or webhook
//
// # Observability Engines
//
// ## MetricsEngine
// Provides metrics collection and export:
//   - Counter: Increment counter metrics
//   - Gauge: Set gauge metrics
//   - Histogram: Record histogram observations
//
// ## TracingEngine
// Provides distributed tracing capabilities:
//   - StartSpan: Start trace spans with attributes
//
// # Usage
//
// Creating an engine with all components:
//
//	eng := engine.New(
//	    engine.WithEventBuffer(1000),
//	    engine.WithMetricsNamespace("myapp"),
//	    engine.WithRecoveryConfig(recovery.DefaultConfig()),
//	)
//
//	// Register services
//	eng.Register(oracleService)
//	eng.Register(vrfService)
//
//	// Start all services
//	if err := eng.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// # Design Principles
//
// 1. Separation of Concerns: Each component has a single responsibility
// 2. Dependency Inversion: Services depend on engine interfaces, not implementations
// 3. Fail-Safe: Automatic recovery prevents cascading failures
// 4. Observable: Comprehensive metrics and event logging
// 5. Extensible: Domain engines can be added without modifying core
package engine
