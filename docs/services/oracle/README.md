# Oracle Service

## Overview

The Oracle Service provides secure, TEE-protected external data fetching capabilities for smart contracts and applications on the Neo N3 blockchain. It enables contracts to access off-chain data sources (APIs, price feeds, weather data, etc.) with cryptographic proof of authenticity.

## Features

### Core Capabilities
- **Secure HTTP Requests**: Fetch data from external APIs with TEE protection
- **Data Feed Management**: Configure and manage recurring data feeds
- **Authentication Support**: Multiple auth types (Bearer, API Key, Basic Auth)
- **Response Signing**: TEE-signed responses for verifiability
- **JSON Path Extraction**: Extract specific fields from JSON responses
- **Caching**: Intelligent response caching for GET requests
- **Supabase Integration**: Optional data publishing to Supabase

### Security Features
- **TEE Isolation**: All data fetching occurs within Trusted Execution Environment
- **Secret Management**: API keys stored securely in TEE vault
- **Host Allowlisting**: Restrict requests to approved domains
- **Request Signing**: Cryptographic signatures on all responses
- **Attestation**: Remote attestation support for trust verification

## Architecture

### Component Structure

```
Oracle Service
├── Service Layer (service.go)
│   ├── HTTP Request Handling
│   ├── Data Feed Management
│   ├── Contract Integration
│   └── Metrics Collection
├── Enclave Layer (enclave.go)
│   ├── Secure HTTP Client
│   ├── Response Signing
│   ├── JSON Path Extraction
│   └── Secret Resolution
└── Storage Layer (store.go)
    ├── Feed Configuration
    ├── Request History
    └── Response Cache
```

### Data Flow

1. **Request Initiation**: Contract or API client submits oracle request
2. **Validation**: Service validates URL, method, and permissions
3. **Secret Resolution**: Enclave retrieves API keys from vault
4. **Secure Fetch**: HTTP request executed within TEE
5. **Response Processing**: Data extracted and signed
6. **Callback**: Result delivered to contract or client

## Service Manifest

```yaml
Service ID: oracle
Version: 1.0.0
Description: Oracle service for fetching external data with TEE protection

Required Capabilities:
  - CapSecrets: Secret management
  - CapNetwork: HTTP requests
  - CapKeys: Cryptographic operations
  - CapStorage: Data persistence

Optional Capabilities:
  - CapSecretsWrite: Store new secrets
  - CapKeysSign: Sign responses
  - CapAttestation: Remote attestation
  - CapDatabase: Supabase integration
  - CapCache: Response caching
  - CapMetrics: Performance monitoring
  - CapScheduler: Scheduled feeds
  - CapContract: Contract callbacks

Resource Limits:
  - Max Memory: 128 MB
  - Max CPU Time: 30 seconds
  - Max Network Requests: 100
  - Max Secrets: 50
```

## Use Cases

### 1. Price Oracles
Fetch cryptocurrency or asset prices from exchanges and price aggregators.

### 2. Weather Data
Retrieve weather information for parametric insurance or prediction markets.

### 3. Sports Results
Get game scores and outcomes for betting or fantasy sports contracts.

### 4. Random Data
Fetch entropy from external randomness beacons.

### 5. API Integration
Connect smart contracts to any REST API (payment processors, IoT devices, etc.).

## Configuration

### Sealed Configuration

The service supports sealed configuration stored in TEE:

```json
{
  "allowed_hosts": [
    "api.coingecko.com",
    "api.binance.com",
    "api.openweathermap.org"
  ],
  "supabase": {
    "project_url": "https://your-project.supabase.co",
    "api_key_secret": "supabase_api_key",
    "table": "oracle_data"
  }
}
```

Configuration is loaded from sealed storage at path: `oracle/config`

### Supabase Integration

Optional Supabase configuration for data publishing:

```json
{
  "project_url": "https://your-project.supabase.co",
  "api_key_secret": "supabase_api_key",
  "table": "price_data",
  "allowed_hosts": ["*.supabase.co"],
  "default_headers": {
    "Content-Type": "application/json"
  }
}
```

## Metrics

The service exposes the following metrics:

- `oracle_fetch_total`: Total number of fetch requests
- `oracle_fetch_errors`: Total number of fetch errors
- `oracle_cache_hits`: Total number of cache hits
- `oracle_cache_misses`: Total number of cache misses
- `oracle_fetch_duration_seconds`: Fetch request duration histogram
- `oracle_active_feeds`: Number of active data feeds (gauge)

## Data Feeds

### Feed Configuration

Data feeds enable scheduled, recurring data fetching:

```go
type DataFeed struct {
    Name        string            // Feed identifier
    Description string            // Human-readable description
    URL         string            // API endpoint
    Method      string            // HTTP method (GET, POST)
    Headers     map[string]string // Custom headers
    SecretName  string            // API key secret reference
    AuthType    AuthType          // Authentication type
    Schedule    string            // Cron expression
    Interval    time.Duration     // Polling interval
    Active      bool              // Enable/disable feed
}
```

### Supported Auth Types

- `AuthTypeNone`: No authentication
- `AuthTypeBearer`: Bearer token in Authorization header
- `AuthTypeAPIKey`: API key in custom header
- `AuthTypeBasic`: HTTP Basic authentication

## Integration Points

### Contract Integration

The service listens for contract requests via the ServiceOS event bus:

```go
type OracleContractRequest struct {
    URL      string            // Target URL
    Method   string            // HTTP method
    Headers  map[string]string // Request headers
    Body     []byte            // Request body
    JSONPath string            // Optional JSON extraction path
    AuthType string            // Authentication type
}
```

### Response Format

```go
type FetchResponse struct {
    StatusCode int               // HTTP status code
    Headers    map[string]string // Response headers
    Body       []byte            // Response body
    Signature  []byte            // TEE signature
    Timestamp  time.Time         // Fetch timestamp
}
```

## Performance Characteristics

- **Latency**: 100-500ms for typical API requests
- **Throughput**: Up to 100 requests/second
- **Cache Hit Rate**: 60-80% for GET requests with 5-minute TTL
- **Success Rate**: 99%+ for reliable endpoints

## Security Considerations

### Trust Model

- All HTTP requests execute within TEE
- API keys never leave the secure enclave
- Responses are cryptographically signed
- Host allowlisting prevents unauthorized requests

### Threat Mitigation

- **MITM Attacks**: TLS verification enforced
- **Data Tampering**: TEE signatures provide integrity
- **Secret Exposure**: Keys stored in sealed storage
- **DoS**: Rate limiting and resource constraints

## Troubleshooting

### Common Issues

**Issue**: Fetch requests timing out
- **Solution**: Check network connectivity, increase timeout, verify URL

**Issue**: Authentication failures
- **Solution**: Verify secret name, check auth type, validate API key

**Issue**: Cache not working
- **Solution**: Ensure CapCache capability is enabled

**Issue**: Supabase integration failing
- **Solution**: Verify project URL, check API key, validate table name

## Related Services

- **Secrets Service**: Manages API keys and credentials
- **DataFeeds Service**: Advanced data aggregation and processing
- **Automation Service**: Scheduled feed execution

## References

- [API Documentation](./API.md)
- [Contract Documentation](./CONTRACT.md)
- [Usage Examples](./EXAMPLES.md)
- [Service Architecture](../../architecture/ARCHITECTURE.md)
