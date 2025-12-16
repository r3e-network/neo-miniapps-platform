# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Sprint 1: Code quality baseline and security improvements
- Environment isolation configuration (development, testing, production)
- Unified error handling package (`internal/errors`)
- Unified structured logging package (`internal/logging`)
- Kubernetes secrets template (`k8s/secrets.yaml.template`)

### Changed

- Updated README.md with production-ready architecture
- Improved security across all services with JWT authentication
- Enhanced NeoVault service with ownership verification
- Added rate limiting to NeoFlow service

### Removed

- Deprecated review documents (12 files)
- Deprecated scripts: `find_duplications.sh`, `split_large_files.sh`
- NeoRand (VRF) service and contracts (randomness is provided via NeoCompute scripts)

### Fixed

- Security vulnerabilities in NeoRand (VRF), NeoVault, and NeoFlow services
- Authentication bypass issues
- Ownership verification in NeoVault service

### Security

- Added JWT authentication middleware to all services
- Implemented rate limiting for resource-intensive endpoints
- Enhanced authorization checks across all service handlers

## [0.1.0] - 2024-12-10

### Added

- Initial release with MarbleRun + EGo + Supabase + Vercel architecture
- 9 core services: Gateway, NeoRand (VRF), NeoVault, NeoOracle, NeoFlow, NeoAccounts (AccountPool), NeoCompute, NeoStore (Secrets), NeoFeeds
- Neo N3 smart contracts for service integration
- TEE protection with MarbleRun/EGo
- Remote attestation via MarbleRun
- Multi-tenant database with Row Level Security
- Deterministic Shared Seed Privacy NeoVault (v4.1)

### Security

- All services run inside EGo MarbleRun TEE
- Secrets never leave the TEE
- TLS termination inside TEE
- ECDSA secp256r1 (Neo N3 compatible)
- AES-256-GCM encryption
- HKDF key derivation
- VRF (ECVRF-P256-SHA256-TAI)

[Unreleased]: https://github.com/R3E-Network/service_layer/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/R3E-Network/service_layer/releases/tag/v0.1.0
