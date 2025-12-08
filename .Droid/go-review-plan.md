# Go File Review Plan - SERVICE_LAYER_FUNCTIONALITY_CN.md Consistency

## Overview
Total Go files to review: 90
Document version: 3.2.0

## Review Checklist Per File

### Architecture Consistency
- [ ] Uses MarbleRun pattern (reads MARBLE_* env vars)
- [ ] Uses EGo for SGX (imports ego/enclave, ego/attestation where needed)
- [ ] Uses Supabase for persistence (internal/database)
- [ ] Follows 4-layer architecture (Client → Gateway → Services → Data)

### Security Patterns
- [ ] UseSecret() callback pattern for sensitive data
- [ ] HKDF key derivation without enclave ID
- [ ] No hardcoded secrets
- [ ] mTLS for inter-service communication

### Service-Specific Checks

## Review Groups

### Group 1: Core Infrastructure (Priority: Critical)
```
internal/marble/marble.go          - Marble SDK core
internal/marble/service.go         - Service base class
internal/marble/config.go          - Configuration
internal/marble/worker.go          - Background workers
internal/crypto/crypto.go          - Crypto utilities (HKDF, AES-GCM, ECDSA, VRF)
internal/database/supabase_*.go    - Supabase integration (11 files)
internal/gasbank/gasbank.go        - Gas fee management
internal/chain/*.go                - Neo N3 blockchain interaction
```

### Group 2: Entry Points (Priority: High)
```
cmd/gateway/main.go                - API Gateway
cmd/gateway/handlers_*.go          - HTTP handlers
cmd/gateway/middleware.go          - Auth middleware
cmd/marble/main.go                 - Generic marble entry
cmd/slcli/main.go                  - CLI tool
```

### Group 3: Services (Priority: High)
```
services/vrf/vrf.go                - VRF service
services/mixer/service.go          - Mixer service
services/mixer/handlers.go         - Mixer HTTP handlers
services/mixer/mixing.go           - Mixing logic
services/mixer/pool.go             - Pool management
services/accountpool/service.go    - Account pool (internal)
services/accountpool/pool.go       - HD key derivation
services/accountpool/handlers.go   - Internal API
services/accountpool/signing.go    - Transaction signing
services/datafeeds/datafeeds.go    - DataFeeds service
services/datafeeds/config.go       - DataFeeds config
services/automation/automation_*.go - Automation service
services/confidential/confidential.go - Confidential computing
```

### Group 4: Deployment Tools (Priority: Medium)
```
cmd/deploy-testnet/*.go            - Testnet deployment
cmd/deploy-fairy/*.go              - Fairy deployment
deploy/testnet/*.go                - Deployment utilities
```

### Group 5: Tests (Priority: Low - verify coverage)
```
*_test.go files                    - All test files
```

## Document Requirements to Verify

### 1. Marble SDK (internal/marble/)
- [ ] Marble struct has: marbleType, uuid, cert, rootCA, tlsConfig, secrets, report
- [ ] Initialize() reads: MARBLE_CERT, MARBLE_KEY, MARBLE_ROOT_CA, MARBLE_UUID, MARBLE_SECRETS
- [ ] UseSecret() pattern implemented with zeroing
- [ ] TLSConfig() returns mTLS configuration
- [ ] IsEnclave() checks attestation report

### 2. Crypto Library (internal/crypto/)
- [ ] DeriveKey() - HKDF-SHA256 (NO enclave ID in derivation)
- [ ] Encrypt()/Decrypt() - AES-256-GCM
- [ ] Sign()/Verify() - ECDSA P-256
- [ ] GenerateVRF()/VerifyVRF() - VRF implementation
- [ ] Hash256()/Hash160() - SHA256, RIPEMD160
- [ ] ScriptHashToAddress() - Neo N3 address
- [ ] HMACSign()/HMACVerify() - HMAC-SHA256

### 3. GasBank (internal/gasbank/)
- [ ] Deposit() - User deposit
- [ ] Withdraw() - User withdrawal
- [ ] Reserve() - Pre-service fee reservation
- [ ] Release() - Failed service fee release
- [ ] Consume() - Post-service fee deduction
- [ ] ChargeServiceFee() - Direct fee charge
- [ ] Fee constants: VRF=0.001, Automation=0.0005, DataFeeds=0.0001, Mixer=0.05+0.5%

### 4. VRF Service (services/vrf/)
- [ ] POST /vrf/random endpoint
- [ ] POST /vrf/verify endpoint
- [ ] Uses VRF_PRIVATE_KEY from Marble.Secret()
- [ ] Returns: seed, random_words, proof, public_key, timestamp

### 5. Mixer Service (services/mixer/)
- [ ] GET /mixer/info endpoint
- [ ] POST /mixer/request endpoint
- [ ] GET /mixer/status/{requestId} endpoint
- [ ] GET /mixer/requests endpoint
- [ ] Uses AccountPool for account management (NOT direct key access)
- [ ] requestHash + TEE signature pattern
- [ ] Amount limits: ≤10,000 per request, ≤100,000 total pool

### 6. AccountPool Service (services/accountpool/)
- [ ] Internal service (not externally exposed)
- [ ] POST /request - Request and lock accounts
- [ ] POST /release - Release accounts
- [ ] POST /sign - Sign transaction
- [ ] POST /batch-sign - Batch sign
- [ ] POST /balance - Update balance
- [ ] HD key derivation: HKDF(masterKey, accountID, "pool-account")
- [ ] Private keys NEVER leave this service
- [ ] 24-hour lock timeout

### 7. DataFeeds Service (services/datafeeds/)
- [ ] GET /datafeeds/prices endpoint
- [ ] GET /datafeeds/prices/{pair} endpoint
- [ ] GET /datafeeds/sources endpoint
- [ ] Multi-source aggregation
- [ ] Median/weighted average calculation
- [ ] TEE signature on prices

### 8. Automation Service (services/automation/)
- [ ] GET/POST /automation/triggers endpoint
- [ ] GET/DELETE /automation/triggers/{id} endpoint
- [ ] POST /automation/triggers/{id}/enable endpoint
- [ ] POST /automation/triggers/{id}/disable endpoint
- [ ] Trigger types: cron, condition, event

### 9. Confidential Service (services/confidential/)
- [ ] Status: Planned (placeholder)
- [ ] POST /confidential/execute (planned)
- [ ] GET /confidential/jobs/{id} (planned)

### 10. Gateway (cmd/gateway/)
- [ ] JWT authentication
- [ ] Request routing
- [ ] Rate limiting
- [ ] TLS termination in enclave
- [ ] Health check: /health
- [ ] Attestation: /attestation
- [ ] Auth endpoints: /api/v1/auth/register, /api/v1/auth/login

## Execution Plan

### Phase 1: Core Infrastructure Review (Day 1)
1. Review internal/marble/*.go
2. Review internal/crypto/crypto.go
3. Review internal/database/supabase_*.go
4. Review internal/gasbank/gasbank.go
5. Review internal/chain/*.go

### Phase 2: Entry Points Review (Day 1)
1. Review cmd/gateway/*.go
2. Review cmd/marble/main.go

### Phase 3: Services Review (Day 2)
1. Review services/vrf/*.go
2. Review services/mixer/*.go
3. Review services/accountpool/*.go
4. Review services/datafeeds/*.go
5. Review services/automation/*.go
6. Review services/confidential/*.go

### Phase 4: Deployment & Tests (Day 2)
1. Review deploy tools
2. Verify test coverage

## Expected Issues to Look For

1. **Security Issues**
   - Hardcoded secrets
   - Missing UseSecret() pattern
   - Enclave ID in key derivation
   - Direct private key exposure

2. **Architecture Violations**
   - Direct database access bypassing repository
   - Missing mTLS configuration
   - Incorrect MarbleRun integration

3. **Missing Features**
   - Incomplete API endpoints
   - Missing error handling
   - Missing logging/metrics

4. **Documentation Drift**
   - Code doesn't match documented behavior
   - Missing endpoints
   - Different data structures

## Output Format

For each file reviewed, generate:
```
## File: path/to/file.go

### Status: ✅ OK | ⚠️ Issues | ❌ Critical

### Document Consistency:
- [ ] Feature X: Status
- [ ] Feature Y: Status

### Issues Found:
1. Issue description
2. Issue description

### Recommendations:
1. Recommendation
2. Recommendation
```
