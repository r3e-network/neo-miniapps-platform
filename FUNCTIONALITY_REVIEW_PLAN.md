# Functionality Review Plan

## Objective
Review the service_layer codebase for functionality correctness, completeness, and consistency across all services.

## Review Scope

### Services to Review
1. **VRF Service** - Verifiable Random Function
2. **Mixer Service** - Privacy mixing
3. **Automation Service** - Trigger-based automation
4. **Secrets Service** - Secret management
5. **DataFeeds Service** - Price feed oracle
6. **AccountPool Service** - Account pool management
7. **Confidential Service** - Confidential computing
8. **Oracle Service** - General oracle service

### Architecture Layers
Each service has three layers:
- `marble/` - TEE service implementation (handlers, lifecycle, core logic)
- `supabase/` - Database operations (repository, models)
- `chain/` - Blockchain interaction (contract, events)

---

## Phase 1: Architecture Consistency Review

### 1.1 Service Structure Consistency
Check that each service follows the standard structure:
```
services/{service}/
├── marble/
│   ├── service.go      # Service struct, New(), ID(), Name(), Version()
│   ├── handlers.go     # HTTP handlers
│   ├── api.go          # Route registration
│   ├── types.go        # Request/Response types
│   ├── lifecycle.go    # Start(), Stop()
│   └── service_test.go # Unit tests
├── supabase/
│   ├── repository.go   # Database operations
│   ├── models.go       # Database models
│   └── repository_test.go
└── chain/
    ├── contract.go     # Smart contract interaction
    ├── events.go       # Event handling
    └── contract_test.go
```

### 1.2 Interface Compliance
Verify each service implements required interfaces:
- `marble.Service` interface (ID, Name, Version, Router, Start, Stop)
- `supabase.RepositoryInterface` for each service
- `chain.ServiceChainModule` for chain modules

---

## Phase 2: Functionality Correctness Review

### 2.1 VRF Service
- [ ] `GenerateRandomness()` produces valid VRF output
- [ ] `VerifyRandomness()` correctly validates proofs
- [ ] Request lifecycle: pending → fulfilled/failed
- [ ] Chain event handling for VRF requests
- [ ] Database persistence of requests

### 2.2 Mixer Service
- [ ] `randomSplit()` correctly splits amounts
- [ ] Request lifecycle: pending → deposited → mixing → delivered
- [ ] AccountPool integration for pool accounts
- [ ] Target address validation
- [ ] Completion proof generation
- [ ] Dispute mechanism

### 2.3 Automation Service
- [ ] Trigger types: schedule, price, chain, manual
- [ ] Condition evaluation logic
- [ ] Action dispatch (http, contract, notification)
- [ ] Execution logging
- [ ] Enable/disable triggers

### 2.4 Secrets Service
- [ ] Secret encryption/decryption
- [ ] Service authorization (AllowedServices)
- [ ] Policy-based access control
- [ ] User-scoped secrets

### 2.5 DataFeeds Service
- [ ] Price feed updates
- [ ] Chainlink integration
- [ ] Deviation threshold checks
- [ ] Heartbeat monitoring

### 2.6 AccountPool Service
- [ ] Account lifecycle: active → locked → retiring
- [ ] Balance tracking
- [ ] Service-based locking
- [ ] Transfer operations

---

## Phase 3: Completeness Review

### 3.1 API Endpoints
For each service, verify all required endpoints exist:
- Health check endpoint
- CRUD operations
- Service-specific operations

### 3.2 Error Handling
- [ ] All handlers return appropriate HTTP status codes
- [ ] Error messages are informative but not leaking sensitive info
- [ ] Database errors are wrapped properly
- [ ] Chain errors are handled gracefully

### 3.3 Authentication/Authorization
- [ ] User ID extraction from headers
- [ ] Service-to-service authentication
- [ ] Resource ownership validation

### 3.4 Database Operations
- [ ] All CRUD operations implemented
- [ ] Proper validation before operations
- [ ] Transaction handling where needed

---

## Phase 4: Consistency Review

### 4.1 Naming Conventions
- [ ] Service constants: ServiceID, ServiceName, Version
- [ ] Handler naming: handle{Action}
- [ ] Type naming: {Action}Request, {Action}Response
- [ ] Status constants: Status{State}

### 4.2 Error Handling Patterns
- [ ] Consistent use of httputil package
- [ ] Consistent error wrapping
- [ ] Consistent logging patterns

### 4.3 Code Patterns
- [ ] Consistent service initialization
- [ ] Consistent route registration
- [ ] Consistent database query patterns
- [ ] Consistent chain interaction patterns

---

## Phase 5: Cross-Service Integration Review

### 5.1 Service Dependencies
- Mixer → AccountPool (account management)
- Mixer → Secrets (optional secret storage)
- Automation → All services (action dispatch)
- All services → Gateway (contract calls)

### 5.2 Shared Components
- `internal/marble` - Marble SDK
- `internal/chain` - Chain client
- `internal/database` - Database client
- `internal/httputil` - HTTP utilities

---

## Review Checklist Template

For each service, complete:

```markdown
## {Service} Review

### Structure
- [ ] service.go exists with New(), ID(), Name(), Version()
- [ ] handlers.go exists with all required handlers
- [ ] api.go exists with route registration
- [ ] types.go exists with request/response types
- [ ] lifecycle.go exists with Start(), Stop()

### Functionality
- [ ] Core business logic is correct
- [ ] All handlers work as expected
- [ ] Database operations are correct
- [ ] Chain operations are correct (if applicable)

### Completeness
- [ ] All required endpoints exist
- [ ] All error cases handled
- [ ] All validation in place

### Consistency
- [ ] Follows naming conventions
- [ ] Follows error handling patterns
- [ ] Follows code patterns
```

---

## Execution Plan

1. **Phase 1**: Run structure analysis across all services
2. **Phase 2**: Deep dive into each service's core logic
3. **Phase 3**: Verify API completeness
4. **Phase 4**: Cross-check consistency patterns
5. **Phase 5**: Validate integration points

## Output
Generate a detailed report with:
- Issues found (categorized by severity)
- Recommendations for fixes
- Code snippets showing problems
- Suggested improvements
