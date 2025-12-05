# Mixer Service

## Overview

Privacy-preserving transaction mixing service with TEE-based custody

## Features

### Core Capabilities
- Secure TEE-protected operations
- Smart contract integration
- Real-time processing
- Comprehensive audit logging

### Security Features
- TEE isolation for all operations
- Cryptographic verification
- Access control and permissions
- Remote attestation support

## Architecture

```
Mixer Service
├── Service Layer (service.go)
│   ├── Request Processing
│   ├── Business Logic
│   └── Contract Integration
├── Enclave Layer (enclave.go)
│   ├── Secure Operations
│   ├── Key Management
│   └── Data Protection
└── Storage Layer (store.go)
    ├── Data Persistence
    └── State Management
```

## Service Manifest

```yaml
Service ID: mixer
Version: 1.0.0
Description: Privacy-preserving transaction mixing service with TEE-based custody

Required Capabilities:
  - CapStorage: Data persistence
  - CapKeys: Key management

Optional Capabilities:
  - CapNetwork: Network access
  - CapDatabase: Database storage
  - CapMetrics: Performance monitoring
  - CapContract: Contract integration
```

## References

- [API Documentation](./API.md)
- [Contract Documentation](./CONTRACT.md)
- [Usage Examples](./EXAMPLES.md)
- [Service Architecture](../../architecture/ARCHITECTURE.md)
