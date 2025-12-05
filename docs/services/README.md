# Service Layer Documentation

## Overview

This directory contains comprehensive documentation for all **14 services** in the Neo Service Layer platform. Each service runs as an **EGo SGX Marble**, activated by the **MarbleRun Coordinator**, with Supabase integration for request queuing and real-time updates.

**Architecture**: MarbleRun + EGo + Supabase + Netlify + Neo N3

## Service Categories

### Core Services (High Priority)

#### 1. [Oracle Service](./oracle/)
External data fetching service with TEE protection for smart contracts.

- **Purpose**: Fetch data from external APIs with cryptographic proof
- **Use Cases**: Price feeds, weather data, sports results, API integration
- **Key Features**: Secure HTTP requests, data feed management, JSON path extraction
- **Documentation**: [README](./oracle/README.md) | [API](./oracle/API.md) | [Contract](./oracle/CONTRACT.md) | [Examples](./oracle/EXAMPLES.md)

#### 2. [VRF Service](./vrf/)
Verifiable Random Function service for provably fair randomness.

- **Purpose**: Generate cryptographically secure, verifiable random numbers
- **Use Cases**: Lotteries, gaming, NFT minting, validator selection
- **Key Features**: VRF proofs, deterministic verification, TEE protection
- **Documentation**: [README](./vrf/README.md) | [API](./vrf/API.md) | [Contract](./vrf/CONTRACT.md) | [Examples](./vrf/EXAMPLES.md)

#### 3. [Secrets Service](./secrets/)
Secure secret management with TEE-backed encryption.

- **Purpose**: Store and manage sensitive data (API keys, passwords, credentials)
- **Use Cases**: API key management, database credentials, OAuth tokens
- **Key Features**: Sealed storage, access control, secret rotation, audit logging
- **Documentation**: [README](./secrets/README.md) | [API](./secrets/API.md) | [Contract](./secrets/CONTRACT.md) | [Examples](./secrets/EXAMPLES.md)

#### 4. [GasBank Service](./gasbank/)
Gas fee management and transaction sponsorship.

- **Purpose**: Abstract gas fees and sponsor transactions for users
- **Use Cases**: Gasless transactions, fee subsidies, user onboarding
- **Key Features**: Fee sponsorship, balance management, settlement tracking
- **Documentation**: [README](./gasbank/README.md) | [API](./gasbank/API.md) | [Contract](./gasbank/CONTRACT.md) | [Examples](./gasbank/EXAMPLES.md)

---

### Data Services (Medium Priority)

#### 5. [DataFeeds Service](./datafeeds/)
Data feed aggregation and management for price feeds.

- **Purpose**: Aggregate and manage multiple data sources
- **Use Cases**: Price oracles, market data, aggregated feeds
- **Key Features**: Multi-source aggregation, confidence scoring, feed management
- **Documentation**: [README](./datafeeds/README.md) | [API](./datafeeds/API.md) | [Contract](./datafeeds/CONTRACT.md) | [Examples](./datafeeds/EXAMPLES.md)

#### 6. [DataLink Service](./datalink/)
Cross-chain data synchronization and delivery.

- **Purpose**: Synchronize data across multiple blockchains
- **Use Cases**: Cross-chain oracles, data bridges, multi-chain applications
- **Key Features**: Data synchronization, cross-chain delivery, proof generation
- **Documentation**: [README](./datalink/README.md) | [API](./datalink/API.md) | [Contract](./datalink/CONTRACT.md) | [Examples](./datalink/EXAMPLES.md)

#### 7. [DataStreams Service](./datastreams/)
Real-time data streaming for live market data.

- **Purpose**: Stream real-time data to applications and contracts
- **Use Cases**: Live price feeds, market data streams, event streaming
- **Key Features**: Real-time streaming, WebSocket support, data buffering
- **Documentation**: [README](./datastreams/README.md) | [API](./datastreams/API.md) | [Contract](./datastreams/CONTRACT.md) | [Examples](./datastreams/EXAMPLES.md)

---

### Compute Services (Medium Priority)

#### 8. [Automation Service](./automation/)
Task automation and scheduling service.

- **Purpose**: Schedule and automate recurring tasks and operations
- **Use Cases**: Scheduled feeds, automated trading, periodic maintenance
- **Key Features**: Cron scheduling, task queuing, retry logic, distributed locking
- **Documentation**: [README](./automation/README.md) | [API](./automation/API.md) | [Contract](./automation/CONTRACT.md) | [Examples](./automation/EXAMPLES.md)

#### 9. [CRE Service](./cre/)
Chainlink Runtime Environment for executing Chainlink functions.

- **Purpose**: Execute Chainlink-compatible functions in TEE
- **Use Cases**: Chainlink Functions, custom computation, data processing
- **Key Features**: JavaScript runtime, HTTP requests, secret access
- **Documentation**: [README](./cre/README.md) | [API](./cre/API.md) | [Contract](./cre/CONTRACT.md) | [Examples](./cre/EXAMPLES.md)

#### 10. [Confidential Computing Service](./confidential/)
Privacy-preserving computation service.

- **Purpose**: Execute sensitive computations with privacy guarantees
- **Use Cases**: Private data analysis, confidential transactions, secure computation
- **Key Features**: TEE isolation, encrypted inputs/outputs, zero-knowledge proofs
- **Documentation**: [README](./confidential/README.md) | [API](./confidential/API.md) | [Contract](./confidential/CONTRACT.md) | [Examples](./confidential/EXAMPLES.md)

---

### Specialized Services (Lower Priority)

#### 11. [Mixer Service](./mixer/)
Privacy-preserving transaction mixing service.

- **Purpose**: Mix transactions to enhance privacy and anonymity
- **Use Cases**: Private transfers, anonymity sets, transaction obfuscation
- **Key Features**: TEE-based mixing, proof generation, multi-pool architecture
- **Documentation**: [README](./mixer/README.md) | [API](./mixer/API.md) | [Contract](./mixer/CONTRACT.md) | [Examples](./mixer/EXAMPLES.md)

#### 12. [CCIP Service](./ccip/)
Cross-Chain Interoperability Protocol service.

- **Purpose**: Enable cross-chain communication and asset transfers
- **Use Cases**: Cross-chain messaging, token bridges, multi-chain dApps
- **Key Features**: Message routing, proof verification, cross-chain calls
- **Documentation**: [README](./ccip/README.md) | [API](./ccip/API.md) | [Contract](./ccip/CONTRACT.md) | [Examples](./ccip/EXAMPLES.md)

#### 13. [DTA Service](./dta/)
Data Trust Authority for data provenance and verification.

- **Purpose**: Verify data authenticity and track provenance
- **Use Cases**: Data certification, provenance tracking, trust verification
- **Key Features**: Data signing, provenance chains, trust attestation
- **Documentation**: [README](./dta/README.md) | [API](./dta/API.md) | [Contract](./dta/CONTRACT.md) | [Examples](./dta/EXAMPLES.md)

#### 14. [Accounts Service](./accounts/)
Account management and user profile service.

- **Purpose**: Manage user accounts, profiles, and wallet associations
- **Use Cases**: User onboarding, profile management, wallet linking
- **Key Features**: Account creation, profile storage, wallet management
- **Documentation**: [README](./accounts/README.md) | [API](./accounts/API.md) | [Contract](./accounts/CONTRACT.md) | [Examples](./accounts/EXAMPLES.md)

---

## Documentation Structure

Each service directory contains four comprehensive documentation files:

### 1. README.md
- Service overview and features
- Architecture and component structure
- Service manifest and capabilities
- Use cases and integration points
- Performance characteristics
- Security considerations

### 2. API.md
- REST API endpoints and methods
- Request/response formats
- Authentication and authorization
- Data types and schemas
- Error handling
- SDK examples (JavaScript, Go, Python)
- Rate limiting and quotas

### 3. CONTRACT.md
- Smart contract documentation
- Contract methods and signatures
- Events and notifications
- Storage schema
- Integration guide with code examples
- Gas costs and optimization
- Deployment instructions

### 4. EXAMPLES.md
- Basic usage examples
- Smart contract integration
- Advanced use cases
- Error handling patterns
- Testing examples
- Production best practices

---

## Quick Start

### 1. Choose a Service

Browse the service categories above and select the service that matches your use case.

### 2. Read the Overview

Start with the service's `README.md` to understand its purpose, features, and architecture.

### 3. Explore the API

Review the `API.md` for REST endpoints, request formats, and SDK usage examples.

### 4. Integrate with Contracts

Check the `CONTRACT.md` for smart contract integration patterns and on-chain methods.

### 5. Learn from Examples

Study the `EXAMPLES.md` for practical code examples and common patterns.

---

## Common Integration Patterns

### Pattern 1: Oracle Data Fetching

```typescript
// 1. Use Oracle service to fetch external data
const oracleClient = new OracleClient({ apiKey: 'key' });
const data = await oracleClient.fetch({ url: 'https://api.example.com/price' });

// 2. Verify TEE signature
const valid = await oracleClient.verifySignature(data);

// 3. Use data in your application
console.log('Price:', JSON.parse(data.body).price);
```

### Pattern 2: Verifiable Randomness

```typescript
// 1. Generate random number with VRF
const vrfClient = new VRFClient({ apiKey: 'key' });
const output = await vrfClient.generateRandomness({ seed: 'user_seed' });

// 2. Verify proof
const valid = await vrfClient.verifyRandomness(output);

// 3. Use randomness
const winnerIndex = BigInt(output.randomness) % BigInt(participants.length);
```

### Pattern 3: Secret Management

```typescript
// 1. Store secret securely
const secretsClient = new SecretsClient({ apiKey: 'key' });
await secretsClient.storeSecret({ key: 'api_key', value: 'secret_value' });

// 2. Retrieve secret in TEE
const secret = await secretsClient.getSecret({ key: 'api_key' });

// 3. Use secret in oracle request
await oracleClient.fetch({
  url: 'https://api.example.com/data',
  secret_name: 'api_key',
  auth_type: 'bearer'
});
```

---

## Service Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Frontend (Netlify)                        │
│  React + Vite + Supabase Auth + Realtime                    │
└─────────────────────────────────────────────────────────────┘
                              │ HTTPS
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Supabase (PostgreSQL)                     │
│  service_requests table • RLS policies • Realtime           │
└─────────────────────────────────────────────────────────────┘
                              │ Polling
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              MarbleRun Coordinator                           │
│  Quote verification • Secret provisioning • mTLS            │
└─────────────────────────────────────────────────────────────┘
                              │ Activation
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ Core Marbles │    │Data Marbles  │    │Compute Marbles│
│  (EGo SGX)   │    │  (EGo SGX)   │    │  (EGo SGX)   │
│              │    │              │    │              │
│ • Oracle     │    │ • DataFeeds  │    │ • Automation │
│ • VRF        │    │ • DataLink   │    │ • CRE        │
│ • Secrets    │    │ • DataStreams│    │ • Confidential│
│ • GasBank    │    │              │    │              │
└──────────────┘    └──────────────┘    └──────────────┘
        │                     │                     │
        └─────────────────────┼─────────────────────┘
                              │ RPC
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Neo N3 Blockchain                         │
│  Smart contracts • On-chain TEE signature verification      │
└─────────────────────────────────────────────────────────────┘
```

---

## Development Workflow

### 1. Local Development

```bash
# Start service layer
cd /home/neo/git/service_layer
make run

# Test service endpoint
curl http://localhost:8080/api/health
```

### 2. Service Testing

```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Run specific service tests
go test ./services/oracle/...
```

### 3. Contract Deployment

```bash
# Compile contracts
cd contracts
make build

# Deploy to testnet
neo-express contract deploy OracleService.nef
```

---

## Security Best Practices

### 1. TEE Verification

Always verify TEE attestation before trusting service outputs:

```typescript
const attestation = await client.getAttestation();
const valid = await verifyAttestation(attestation);
```

### 2. Signature Verification

Verify cryptographic signatures on all service responses:

```typescript
const valid = await client.verifySignature(response.data, response.signature);
```

### 3. Access Control

Implement proper access control for sensitive operations:

```typescript
await secretsClient.storeSecret({
  key: 'api_key',
  value: 'secret',
  access_policy: {
    allowed_services: ['oracle'],
    allowed_accounts: ['user123']
  }
});
```

---

## Performance Optimization

### 1. Caching

Use caching for frequently accessed data:

```typescript
// Oracle service automatically caches GET requests
const data = await oracleClient.fetch({ url: 'https://api.example.com/data' });
```

### 2. Batch Requests

Batch multiple requests when possible:

```typescript
const requests = urls.map(url => oracleClient.fetch({ url }));
const responses = await Promise.all(requests);
```

### 3. Async Processing

Use async patterns for long-running operations:

```typescript
// Submit request
const requestId = await service.submitRequest({ ... });

// Poll for result
const result = await service.waitForResult(requestId);
```

---

## Troubleshooting

### Common Issues

**Issue**: Service not responding
- **Solution**: Check service status at `/api/services/{service}/status`

**Issue**: Authentication failures
- **Solution**: Verify API key and token expiration

**Issue**: Rate limit exceeded
- **Solution**: Implement exponential backoff and request throttling

**Issue**: TEE attestation failed
- **Solution**: Verify TEE mode and attestation configuration

---

## Additional Resources

- [MarbleRun Architecture](../architecture/MARBLERUN_ARCHITECTURE.md) - Primary architecture document
- [TEE Architecture](../architecture/TEE_SUPABASE_ARCHITECTURE.md) - TEE integration details
- [Deployment Guide](../DEPLOYMENT.md) - Production deployment
- [Documentation Index](../README.md) - All documentation

---

## Contributing

To add or update service documentation:

1. Follow the 4-file structure (README, API, CONTRACT, EXAMPLES)
2. Use consistent formatting and examples
3. Include working code samples
4. Test all examples before committing
5. Update this index file with new services

---

## Support

For questions or issues:

- GitHub Issues: https://github.com/R3E-Network/service_layer/issues
- Documentation: https://docs.r3e.network
- Community: https://discord.gg/r3e-network

---

**Last Updated**: 2025-12-04
**Total Services**: 14
**Total Documentation Files**: 56 (14 services × 4 files each)
**Architecture**: MarbleRun + EGo + Supabase + Netlify + Neo N3
