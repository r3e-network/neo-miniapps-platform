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
- Enhanced Mixer service with ownership verification
- Added rate limiting to Automation service

### Removed
- Deprecated review documents (12 files)
- Deprecated scripts: `find_duplications.sh`, `split_large_files.sh`

### Fixed
- Security vulnerabilities in VRF, Mixer, and Automation services
- Authentication bypass issues
- Ownership verification in Mixer service

### Security
- Added JWT authentication middleware to all services
- Implemented rate limiting for resource-intensive endpoints
- Enhanced authorization checks across all service handlers

## [0.1.0] - 2024-12-10

### Added
- Initial release with MarbleRun + EGo + Supabase + Netlify architecture
- 9 core services: Gateway, VRF, Mixer, Oracle, Automation, AccountPool, Confidential, Secrets, DataFeeds
- Neo N3 smart contracts for service integration
- TEE protection with Intel SGX
- Remote attestation via MarbleRun
- Multi-tenant database with Row Level Security
- Deterministic Shared Seed Privacy Mixer (v4.1)

### Security
- All services run inside EGo SGX enclaves
- Secrets never leave the enclave
- TLS termination inside enclave
- ECDSA secp256r1 (Neo N3 compatible)
- AES-256-GCM encryption
- HKDF key derivation
- VRF (ECVRF-P256-SHA256-TAI)

[Unreleased]: https://github.com/R3E-Network/service_layer/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/R3E-Network/service_layer/releases/tag/v0.1.0
