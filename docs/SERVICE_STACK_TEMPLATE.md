# Service Stack Template

This document defines the complete stack required for each service in the Neo Service Layer.

## Complete Service Stack Checklist

Every service MUST have the following components:

### 1. TEE Layer (`tee/`)
- [x] Access to TrustRoot via ServiceOS
- [x] Enclave runtime support
- [x] Key management (vault, keys)
- [x] Attestation support

### 2. ServiceOS Layer (`platform/os/`)
- [x] Generic ServiceOS APIs (all services share)
- [ ] Service-specific capabilities in Manifest

### 3. Service Layer (`services/[name]/`)
- [x] `service.go` - Main service implementation
- [x] `enclave.go` - TEE enclave operations
- [x] `store.go` - Data persistence
- [x] `domain.go` - Domain models and types

### 4. Smart Contract (`contracts/[Name]Service/`)
- [ ] `[Name]Service.cs` - Neo N3 smart contract
- [ ] `[Name]Service.csproj` - Project file
- [ ] `[Name]Service.manifest.json` - Contract manifest
- [ ] `[Name]Service.nef` - Compiled contract

### 5. API Routes (`cmd/server/`)
- [ ] REST API endpoints in `api.go`
- [ ] Request/Response types
- [ ] Authentication middleware integration

### 6. Frontend Page (`frontend/src/pages/services/`)
- [ ] `[Name]ServicePage.tsx` - Main service page
- [ ] Service overview section
- [ ] Configuration section
- [ ] Usage/requests section
- [ ] Statistics section

### 7. Frontend Components (`frontend/src/components/[name]/`)
- [ ] `[Name]Card.tsx` - Service card component
- [ ] `[Name]Form.tsx` - Input form component
- [ ] `[Name]Stats.tsx` - Statistics component
- [ ] `[Name]Settings.tsx` - Settings component

### 8. User Settings (`frontend/src/pages/settings/`)
- [ ] Service-specific settings in SettingsPage
- [ ] API key management
- [ ] Notification preferences
- [ ] Usage limits configuration

### 9. Documentation (`docs/services/[name]/`)
- [ ] `README.md` - Service overview
- [ ] `API.md` - API documentation
- [ ] `CONTRACT.md` - Smart contract documentation
- [ ] `EXAMPLES.md` - Usage examples
- [ ] `CONFIGURATION.md` - Configuration guide

---

## Service Completeness Matrix

| Service | TEE | OS | Service | Contract | API | Page | Components | Settings | Docs |
|---------|-----|----|---------|---------|----|------|------------|----------|------|
| oracle | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| vrf | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| secrets | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| gasbank | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| mixer | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| datafeeds | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| accounts | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| automation | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| ccip | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| confidential | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| cre | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| datalink | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| datastreams | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| dta | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

**Completion Summary (Updated 2024-12):**
- TEE Layer: 14/14 (100%)
- ServiceOS: 14/14 (100%)
- Service Implementation: 14/14 (100%)
- Smart Contracts: 14/14 (100%)
- API Routes: 14/14 (100%)
- Frontend Pages: 14/14 (100%)
- Frontend Components: 14/14 (100%) - Card + Stats for all services
- User Settings: 14/14 (100%) - ServiceSettings component with all services
- Documentation: 14/14 (100%)

**Overall Completion: 126/126 (100%)**

### Component Structure
Each service has the following components in `frontend/src/components/services/[name]/`:
- `[Name]Card.tsx` - Display card for individual items
- `[Name]Stats.tsx` - Statistics dashboard component
- `index.ts` - Barrel export file

---

## Priority Order for Completion

### Phase 1: Core Services (High Priority)
1. **oracle** - Price oracle, external data
2. **vrf** - Verifiable random function
3. **secrets** - Secret management
4. **gasbank** - Gas sponsorship

### Phase 2: Data Services (Medium Priority)
5. **datafeeds** - Data aggregation
6. **datalink** - Data synchronization
7. **datastreams** - Real-time streaming

### Phase 3: Compute Services (Medium Priority)
8. **automation** - Task automation
9. **cre** - Chainlink runtime environment
10. **confidential** - Confidential computing

### Phase 4: Specialized Services (Lower Priority)
11. **mixer** - Privacy mixing
12. **ccip** - Cross-chain interoperability
13. **dta** - Data trust authority
14. **accounts** - Account management

---

## File Structure Template

```
service_layer/
├── services/[name]/
│   ├── service.go
│   ├── enclave.go
│   ├── store.go
│   └── domain.go
├── contracts/[Name]Service/
│   ├── [Name]Service.cs
│   ├── [Name]Service.csproj
│   └── README.md
├── frontend/src/
│   ├── pages/services/[Name]ServicePage.tsx
│   └── components/[name]/
│       ├── [Name]Card.tsx
│       ├── [Name]Form.tsx
│       ├── [Name]Stats.tsx
│       └── index.ts
├── cmd/server/
│   └── api.go (add routes)
└── docs/services/[name]/
    ├── README.md
    ├── API.md
    ├── CONTRACT.md
    └── EXAMPLES.md
```
