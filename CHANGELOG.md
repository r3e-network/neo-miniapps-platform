# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive security documentation for rate limiting, API key rotation, and audit logging
- Detailed security checklist and implementation status
- Complete DevOps infrastructure with Terraform and Helm charts
- Monitoring dashboards for key metrics
- Devpack runtime with declarative action queueing for functions
- Execution history persistence for action outcomes across storage backends
- JavaScript/TypeScript Devpack SDK (`@service-layer/devpack`) for local development
- Added Devpack 0.6.0 across TypeScript/Go/Rust/Python with random, data feed, data stream, DataLink, price feed helpers

### Changed
- Consolidated documentation under `docs/`, removed retired LaTeX/PDF exports, and clarified doc-first contribution guidance.
- Documented the supported toolchain versions (Go 1.24+, Node.js 20+, React/Vite dashboard stack).
- Corrected dashboard documentation to reflect the React + Vite implementation and updated setup guidance.
- Added README coverage for `configs/` and `scripts/` so configuration samples and automation tooling stay discoverable.
- Retired unused `devops/init-scripts` and updated docker-compose volumes to point at `configs/`.
- Updated the legacy GitHub workflow to use Go 1.24, the `cmd/appserver` target, and consistent Docker image names.
- Modernized `.github/workflows/ci-cd.yml` with the same Go/toolchain updates and renamed artifacts (`appserver`, `slctl`, and `service-layer` Docker tags).
- Simplified `.github/workflows/ci.yml` to run on Go 1.24, build `cmd/appserver`, and produce the same Docker image tag.
- Updated the root `Dockerfile` to Go 1.24 and ship the sample config for parity with other build targets.
- Switched `devops/Dockerfile` to Go 1.24 and the `cmd/appserver` binary to keep parity with the runtime build.
- Aligned `scripts/deploy_azure.sh` with the YAML config path used by the container images.
- Updated Makefile docker targets to use the `service-layer` image name.
- Updated the Helm chart metadata (description/keywords) to reflect the current Service Layer scope.
- Helm default values now use the `ghcr.io/r3e-network/service-layer` image repository.
- Enhanced `scripts/security_scan.sh` to install and run `gitleaks` alongside gosec/Nancy.
- `scripts/run_unit_tests.sh` now covers the `cmd/slctl` package so CLI regressions show up in local runs.
- Updated the root `docker-compose.yml` and `.env.example` to match the `service-layer` naming and credentials used elsewhere.
- Added `appserver` and `slctl` binaries to `.gitignore` so local builds don't pollute the repo.

### Fixed
- JavaScript runtime execution environment for TEE
- Configuration fields for Azure Confidential Computing
- Import path conflicts and type duplication issues

### Removed
- Deprecated `apps/web` prototype and its cached artifacts; `apps/dashboard` now serves as the sole maintained front-end surface.
- Stripped accidental `node_modules` and `dist` directories from the repo; regenerate them locally via `npm install`/`npm run build` when needed.
- Removed the obsolete `scripts/performance/` helpers that referenced non-existent APIs and load-test assets.

## [0.9.0] - 2024-03-20

### Added
- TEE integration with Azure Confidential Computing
- Memory limiting for JavaScript execution
- Timeout enforcement with VM interruption
- Function isolation with VM-per-execution model
- Sandbox security with frozen prototypes and strict mode
- Comprehensive tests for all security features
- Envelope encryption for secrets
- Data key rotation mechanism
- Comprehensive audit logging
- User isolation for secrets

### Changed
- Enhanced performance with database query optimizations
- Added performance-enhancing database indices
- Implemented schema denormalization for faster queries
- Created optimized repository pattern
- Added caching for frequently accessed data
- Implemented database connection pooling

### Fixed
- Fixed configuration system with proper fields
- Enhanced logging infrastructure
- Improved API server with proper middleware
- Added error handling throughout

## [0.8.0] - 2024-02-15

### Added
- Created robust Neo N3 compatibility layer
- Implemented transaction creation and signing
- Added support for Uint160/Uint256 conversions
- Created mock blockchain client for testing
- Integrated Prometheus metrics collection
- Implemented system metrics collector
- Added health check endpoints
- Created Grafana dashboards for key metrics

### Fixed
- Resolved integration issues with Neo N3 blockchain
- Fixed transaction submission and confirmation flow
- Addressed concurrency issues in transaction handling

## [0.7.0] - 2024-01-25

### Added
- Functions service implementation
- Secret management system
- Contract automation features
- Basic integration testing framework
- Initial version of web dashboard

### Changed
- Restructured project architecture
- Enhanced error handling system
- Improved configuration management

### Fixed
- Numerous early-stage implementation bugs
- JSON parsing issues in API responses
- Database connection handling

## [0.6.0] - 2023-12-10

### Added
- Initial API server implementation
- Basic blockchain communication
- Core service interfaces
- Database schema and migrations
- Authentication system

[Unreleased]: https://github.com/R3E-Network/service_layer/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/R3E-Network/service_layer/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/R3E-Network/service_layer/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/R3E-Network/service_layer/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/R3E-Network/service_layer/releases/tag/v0.6.0
