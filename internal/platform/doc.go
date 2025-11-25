// Package platform provides the Hardware Abstraction Layer (HAL) for the service layer.
//
// # Architecture Overview
//
// The service layer follows a clean, layered architecture similar to an operating system:
//
//	┌─────────────────────────────────────────────────────────────────────┐
//	│                      Services Layer (Applications)                   │
//	│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
//	│  │  DeFi    │ │ GameFi   │ │   NFT    │ │   RWA    │ │  Custom  │  │
//	│  │ Services │ │ Services │ │ Services │ │ Services │ │ Services │  │
//	│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘  │
//	├─────────────────────────────────────────────────────────────────────┤
//	│                       Engine Layer (OS Kernel)                       │
//	│  ┌─────────────────────────────────────────────────────────────┐   │
//	│  │                     Service Engine                           │   │
//	│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐            │   │
//	│  │  │ State   │ │ Events  │ │ Metrics │ │Recovery │            │   │
//	│  │  │ Machine │ │ System  │ │Collector│ │ Manager │            │   │
//	│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘            │   │
//	│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐            │   │
//	│  │  │   Bus   │ │Registry │ │Lifecycle│ │  Bridge │            │   │
//	│  │  │ Limiter │ │         │ │         │ │         │            │   │
//	│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘            │   │
//	│  └─────────────────────────────────────────────────────────────┘   │
//	├─────────────────────────────────────────────────────────────────────┤
//	│                      Platform Layer (HAL/Drivers)                    │
//	│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
//	│  │Blockchain│ │  Storage │ │  Cache   │ │  Queue   │ │  Crypto  │  │
//	│  │   RPC    │ │ (DB/KV)  │ │ (Redis)  │ │(RocketMQ)│ │  (HSM)   │  │
//	│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘  │
//	│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐              │
//	│  │   HTTP   │ │   gRPC   │ │WebSocket │ │  Oracle  │              │
//	│  │  Client  │ │  Client  │ │  Client  │ │  Client  │              │
//	│  └──────────┘ └──────────┘ └──────────┘ └──────────┘              │
//	└─────────────────────────────────────────────────────────────────────┘
//
// # Platform Layer
//
// The platform layer provides low-level drivers and adapters for external systems:
//
//   - Blockchain RPC: Connect to various blockchain networks (Neo, Ethereum, etc.)
//   - Storage: Database drivers (PostgreSQL, SQLite) and key-value stores
//   - Cache: Redis and in-memory caching
//   - Queue: Message queue integration (RocketMQ, Kafka)
//   - Crypto: Hardware security modules, key management
//   - HTTP/gRPC/WebSocket: Client libraries for external services
//   - Oracle: External data source connectors
//
// # Design Principles
//
// 1. Dependency Inversion: Upper layers depend on abstractions, not implementations
// 2. Single Responsibility: Each driver handles one external system
// 3. Interface Segregation: Small, focused interfaces for each capability
// 4. Open/Closed: New drivers can be added without modifying existing code
//
// # Usage
//
// Platform drivers are typically instantiated during application bootstrap and
// injected into the engine layer:
//
//	// Create platform drivers
//	rpcDriver := rpc.NewMultiChainDriver(config)
//	storageDriver := storage.NewPostgresDriver(dsn)
//	cacheDriver := cache.NewRedisDriver(redisURL)
//
//	// Inject into engine
//	engine := engine.New(
//	    engine.WithRPC(rpcDriver),
//	    engine.WithStorage(storageDriver),
//	    engine.WithCache(cacheDriver),
//	)
package platform
