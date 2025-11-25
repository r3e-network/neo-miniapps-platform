# Documentation Index

This repository treats the [Neo Service Layer Specification](requirements.md)
as the single source of truth for the platform.

## Quick Navigation

| Category | Document | Description |
|----------|----------|-------------|
| **Start Here** | [Quickstart Tutorial](quickstart-tutorial.md) | End-to-end tutorial: zero to running in 15 minutes |
| **Services** | [Service Catalog](service-catalog.md) | Complete reference for all 17 services |
| **Development** | [Developer Guide](developer-guide.md) | Building and extending the Service Layer |
| **Architecture** | [Architecture Layers](architecture-layers.md) | 4-layer design (Platform → Framework → Engine → Services) |
| **Deep Dives** | [Framework Guide](framework-guide.md) | ServiceBase, Builder, Manifest, Testing |
| **Deep Dives** | [Engine Guide](engine-guide.md) | Registry, Lifecycle, Bus, Health monitoring |
| **Deployment** | [Deployment Guide](deployment-guide.md) | Production deployment with Docker/Kubernetes |
| **Specification** | [Requirements](requirements.md) | Single source of truth |

---

## Service Tutorials

### Core Services
| Service | Tutorial | Description |
|---------|----------|-------------|
| Accounts | [API Examples](examples/services.md#accounts) | Account management |
| Functions | [API Examples](examples/services.md#functions) | Serverless function execution |
| Automation | [Automation Guide](examples/automation.md) | Cron-style job scheduling |
| Triggers | [Triggers Guide](examples/triggers.md) | Event/webhook routing |
| Secrets | [Secrets Guide](examples/secrets.md) | Encrypted secret storage |

### Oracle & Data Services
| Service | Tutorial | Description |
|---------|----------|-------------|
| Oracle | [API Examples](examples/services.md#oracle-http-adapter) | HTTP data adapters |
| DataFeeds | [DataFeeds Quickstart](examples/datafeeds.md) | Chainlink-style signed feeds |
| PriceFeeds | [PriceFeeds Quickstart](examples/pricefeeds.md) | Deviation-based price aggregation |
| DataStreams | [API Examples](examples/services.md#data-streams) | Real-time data streams |
| DataLink | [DataLink Quickstart](examples/datalink.md) | Data delivery channels |

### Randomness Services
| Service | Tutorial | Description |
|---------|----------|-------------|
| Random | [Randomness Guide](examples/randomness.md#random-service) | ED25519 signed random |
| VRF | [Randomness Guide](examples/randomness.md#vrf-service) | Verifiable random functions |

### Financial Services
| Service | Tutorial | Description |
|---------|----------|-------------|
| GasBank | [GasBank Workflows](gasbank-workflows.md) | Gas account management |

### Cross-Chain & Advanced
| Service | Tutorial | Description |
|---------|----------|-------------|
| CCIP | [Cross-Chain Guide](examples/crosschain.md#ccip-service) | Cross-chain messaging |
| CRE | [Cross-Chain Guide](examples/crosschain.md#cre-service) | Workflow orchestration |
| Confidential | [Confidential Guide](examples/confidential.md#confidential-computing-service) | TEE enclave management |
| DTA | [Confidential Guide](examples/confidential.md#dta-service) | Trading infrastructure |

### Engine Integration
| Topic | Tutorial | Description |
|-------|----------|-------------|
| Event Bus | [Bus Quickstart](examples/bus.md) | Pub/sub messaging |
| System APIs | [API Examples](examples/services.md#discover-services-via-the-system-apis) | Module discovery |

---

## Architecture Documentation

### Getting Started
- [Quickstart Tutorial](quickstart-tutorial.md) - **NEW**: End-to-end tutorial (15 minutes)
- [Architecture Layers](architecture-layers.md) - **Start here**: 4-layer design guide

### Core Architecture
- [System Architecture](system-architecture.md) - Deployment topology and data flows
- [Service Engine Architecture](service-engine-architecture.md) - Detailed engine design
- [Service Engine (Android Model)](service-engine.md) - OS-style service model
- [Service Core Engine](core-engine.md) - Core engine internals

### Deep Dive Guides
- [Framework Guide](framework-guide.md) - **NEW**: ServiceBase, Builder, Manifest, Testing utilities
- [Engine Guide](engine-guide.md) - **NEW**: Registry, Lifecycle, Bus, Health, Dependencies

### Code Layout
```
internal/
├── platform/     # HAL/Drivers (RPC, Storage, Cache, Queue, Crypto)
├── framework/    # SDK (ServiceBase, Builder, Manifest)
├── engine/       # OS Kernel (Registry, Lifecycle, Bus, Health)
├── services/     # Business services (17 domains)
└── app/          # Application wiring (HTTP API, storage)
```

---

## Operations & Security

### Deployment
- [Deployment Guide](deployment-guide.md) - **NEW**: Production deployment with Docker/Kubernetes
- [Operations Runbook](ops-runbook.md) - Start/stop, monitoring, troubleshooting

### Dashboard
- [Dashboard Smoke Checklist](dashboard-smoke.md) - Dashboard verification
- [Dashboard E2E Testing](dashboard-e2e.md) - Playwright testing

### Security
- [Security & Production Hardening](security-hardening.md) - Production deployment
- [Tenant Quickstart](tenant-quickstart.md) - Multi-tenancy setup

### Auditing
- `/admin/audit?limit=...&offset=...` (admin JWT) with filters
- CLI helper: `slctl audit --limit 100 --user admin`

---

## Integration References

### NEO N3 Integration
- [NEO API Reference](neo-api.md) - Indexer and snapshot APIs
- [NEO Operations](neo-ops.md) - Running NEO nodes
- [NEO Layering Plan](neo-layering-summary.md) - Architecture roadmap
- [NEO Contract Set](neo-n3-contract-set.md) - Smart contract layout
- [Contract ↔ Service Alignment](neo-contracts-alignment.md) - Field mappings

### JAM Integration
- [JAM Integration Design](polkadot-jam-integration-design.md) - Overview
- [JAM Accumulator Plan](jam-accumulator-plan.md) - Implementation plan
- [JAM Receipts and Roots](jam-receipts-and-roots.md) - Receipt system
- [JAM Hardening](jam-hardening.md) - Security hardening

---

## API Quick Reference

### Core Bus Endpoints
```bash
# Publish events
POST /system/events
{"event": "my.event", "payload": {...}}

# Push data
POST /system/data
{"topic": "my.topic", "payload": {...}}

# Invoke compute
POST /system/compute
{"payload": {...}}

# CLI shortcuts
slctl bus events --event my.event --payload '{}'
slctl bus data --topic my.topic --payload '{}'
slctl bus compute --payload '{}'
```

### System Status
```bash
# Service status
GET /system/status

# Service descriptors
GET /system/descriptors

# Health checks
GET /readyz
GET /livez
```

---

## Testing

### Unit Tests
```bash
go test ./...
go test ./internal/services/... -cover
```

### Integration Tests
```bash
# In-memory
go test ./internal/app/httpapi -run IntegrationHTTPAPI

# PostgreSQL
go test -tags "integration postgres" ./internal/app/httpapi -run IntegrationPostgres
```

### Dashboard E2E
```bash
cd apps/dashboard
API_URL=http://localhost:8080 API_TOKEN=dev-token npm run e2e
```

---

## CLI Reference

See [CLI Quick Reference](../README.md#cli-quick-reference) in main README.

---

## Working With Documentation

1. **Start with Specification**: Update [requirements.md](requirements.md) first
2. **Add Examples**: Include curl/CLI snippets in relevant guides
3. **Link Implementation**: Reference file paths with line numbers
4. **Keep Realistic**: Use production-like payloads

---

## Document Status

| Document | Status | Last Updated |
|----------|--------|--------------|
| Quickstart Tutorial | ✅ Complete | 2025-11 |
| Service Catalog | ✅ Complete | 2025-11 |
| Developer Guide | ✅ Complete | 2025-11 |
| Framework Guide | ✅ Complete | 2025-11 |
| Engine Guide | ✅ Complete | 2025-11 |
| Deployment Guide | ✅ Complete | 2025-11 |
| All Service Tutorials | ✅ Complete | 2025-11 |
| Architecture Docs | ✅ Complete | 2025-11 |
| Operations Docs | ✅ Complete | 2025-11 |
