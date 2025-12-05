# Neo Service Layer Testing Guide

Comprehensive testing guide for the Neo Service Layer platform.

## Test Categories

| Category | Purpose | Location | Run Command |
|----------|---------|----------|-------------|
| **Unit Tests** | Test individual components | `*_test.go` files | `go test ./...` |
| **Integration Tests** | Test component interactions | `test/integration/` | `go test -tags=integration ./test/integration/...` |
| **E2E Tests** | Test full request flows | `test/e2e/` | `go test -tags=e2e ./test/e2e/...` |
| **Smoke Tests** | Verify deployment health | `test/smoke/` | `go test -tags=smoke ./test/smoke/...` |

---

## Quick Start

### Run All Unit Tests

```bash
go test ./...
```

### Run Tests with Coverage

```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Run Specific Package Tests

```bash
# EGo runtime tests
go test ./ego/...

# Marble SDK tests
go test ./marble/sdk/...

# Supabase client tests
go test ./supabase/client/...

# Service base tests
go test ./services/base/...
```

---

## Unit Tests

### EGo Runtime (`ego/ego_test.go`)

Tests for SGX enclave simulation and sealing:

```bash
go test -v ./ego/...
```

**Test Coverage:**
- `TestNew` - Runtime creation
- `TestRuntime_SealUnseal` - AES-256-GCM sealing
- `TestRuntime_GenerateQuote` - Quote generation
- `TestRuntime_VerifyQuote` - Quote verification
- `TestRuntime_SealConcurrent` - Concurrent sealing

### Marble SDK (`marble/sdk/marble_test.go`)

Tests for Marble activation and secret management:

```bash
go test -v ./marble/sdk/...
```

**Test Coverage:**
- `TestNew` - Marble creation
- `TestMarble_Activate_Success` - Coordinator activation
- `TestMarble_SealUnseal` - Data sealing via Marble
- `TestMarble_Health` - Health status
- `TestQueryBuilder_*` - Supabase query builder

### Supabase Client (`supabase/client/resilience_test.go`)

Tests for resilience patterns:

```bash
go test -v ./supabase/client/...
```

**Test Coverage:**
- `TestCircuitBreaker_*` - Circuit breaker states
- `TestResilientClient_Do_*` - Retry logic
- `TestResilientClient_Metrics` - Request metrics

---

## Integration Tests

Integration tests verify component interactions.

### Setup

```bash
# Set environment
export INTEGRATION_TEST=true
export SUPABASE_URL=http://localhost:54321
export SUPABASE_SERVICE_KEY=your-service-key
```

### Run Integration Tests

```bash
go test -tags=integration -v ./test/integration/...
```

### Test Cases

| Test | Description |
|------|-------------|
| `TestEgoMarbleIntegration` | EGo + Marble SDK integration |
| `TestMarbleActivationFlow` | Full activation with mock coordinator |
| `TestSupabaseClientIntegration` | Supabase client with real database |
| `TestSupabaseRequestFlow` | Request insert/update flow |
| `TestCircuitBreakerWithRealRequests` | Circuit breaker with HTTP |
| `TestFullServiceFlow` | Complete service request flow |

---

## E2E Tests

End-to-end tests verify the complete request flow.

### Setup

```bash
# Start all services
docker-compose up -d

# Set environment
export E2E_TEST=true
export SUPABASE_URL=http://localhost:54321
export SUPABASE_ANON_KEY=your-anon-key
```

### Run E2E Tests

```bash
go test -tags=e2e -v ./test/e2e/...
```

### Test Cases

| Test | Description |
|------|-------------|
| `TestE2E_OracleFetch` | Oracle service fetch request |
| `TestE2E_VRFRandom` | VRF random number generation |
| `TestE2E_SecretsStoreGet` | Secrets store and retrieve |
| `TestE2E_GasBankBalance` | GasBank balance check |
| `TestE2E_FullOracleToBlockchain` | Complete oracle to blockchain flow |
| `TestE2E_ConcurrentRequests` | Concurrent request handling |

---

## Smoke Tests

Smoke tests verify deployment health.

### Setup

```bash
# Set environment
export SUPABASE_URL=http://localhost:54321
export SUPABASE_ANON_KEY=your-anon-key
export COORDINATOR_URL=https://localhost:4433
export FRONTEND_URL=http://localhost:5173
export NEO_RPC_URL=http://localhost:10332
```

### Run Smoke Tests

```bash
go test -tags=smoke -v ./test/smoke/...
```

### Quick Smoke Check

```bash
go test -tags=smoke -v -run TestSmoke_QuickCheck ./test/smoke/...
```

### Test Cases

| Test | Description |
|------|-------------|
| `TestSmoke_SupabaseHealth` | Supabase REST API health |
| `TestSmoke_SupabaseAuth` | Supabase Auth health |
| `TestSmoke_CoordinatorStatus` | MarbleRun Coordinator status |
| `TestSmoke_FrontendHealth` | Frontend serving |
| `TestSmoke_NeoRPCHealth` | Neo N3 RPC health |
| `TestSmoke_DatabaseTables` | Database schema verification |
| `TestSmoke_AllServicesConnectivity` | All services connectivity |

---

## Benchmarks

### Run Benchmarks

```bash
# EGo benchmarks
go test -bench=. ./ego/...

# Marble SDK benchmarks
go test -bench=. ./marble/sdk/...

# Supabase client benchmarks
go test -bench=. ./supabase/client/...
```

### Example Output

```
BenchmarkSeal-8              10000    105234 ns/op
BenchmarkUnseal-8            10000     98123 ns/op
BenchmarkGenerateQuote-8      5000    234567 ns/op
BenchmarkCircuitBreaker_Allow-8  1000000    1234 ns/op
```

---

## Test Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `INTEGRATION_TEST` | Enable integration tests | `false` |
| `E2E_TEST` | Enable E2E tests | `false` |
| `SUPABASE_URL` | Supabase URL | `http://localhost:54321` |
| `SUPABASE_ANON_KEY` | Supabase anon key | - |
| `SUPABASE_SERVICE_KEY` | Supabase service key | - |
| `COORDINATOR_URL` | Coordinator URL | `https://localhost:4433` |
| `FRONTEND_URL` | Frontend URL | `http://localhost:5173` |
| `NEO_RPC_URL` | Neo RPC URL | `http://localhost:10332` |
| `SIMULATION_MODE` | Enable SGX simulation | `true` |

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Tests

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - run: go test -v -cover ./...

  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: supabase/postgres:15
        env:
          POSTGRES_PASSWORD: postgres
        ports:
          - 5432:5432
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - run: |
          export INTEGRATION_TEST=true
          go test -tags=integration -v ./test/integration/...

  smoke-tests:
    runs-on: ubuntu-latest
    needs: [unit-tests]
    steps:
      - uses: actions/checkout@v3
      - run: docker-compose up -d
      - run: sleep 30  # Wait for services
      - run: go test -tags=smoke -v ./test/smoke/...
```

---

## Writing Tests

### Unit Test Template

```go
func TestMyFunction(t *testing.T) {
    // Arrange
    input := "test"
    expected := "result"

    // Act
    result := MyFunction(input)

    // Assert
    if result != expected {
        t.Errorf("MyFunction(%s) = %s, want %s", input, result, expected)
    }
}
```

### Table-Driven Tests

```go
func TestMyFunction(t *testing.T) {
    testCases := []struct {
        name     string
        input    string
        expected string
    }{
        {"empty", "", ""},
        {"simple", "hello", "HELLO"},
        {"special", "hello!", "HELLO!"},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            result := MyFunction(tc.input)
            if result != tc.expected {
                t.Errorf("got %s, want %s", result, tc.expected)
            }
        })
    }
}
```

### Mock Server

```go
func TestWithMockServer(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    }))
    defer server.Close()

    // Use server.URL in your test
    client := NewClient(server.URL)
    result, err := client.DoSomething()
    // Assert...
}
```

---

## Troubleshooting

### Common Issues

**Tests timeout:**
```bash
go test -timeout 60s ./...
```

**Database connection issues:**
```bash
# Check Supabase is running
curl http://localhost:54321/rest/v1/
```

**SGX simulation issues:**
```bash
export SIMULATION_MODE=true
go test ./ego/...
```

**Verbose output:**
```bash
go test -v -count=1 ./...
```

---

## Related Documentation

- [Deployment Guide](DEPLOYMENT.md)
- [Architecture](architecture/MARBLERUN_ARCHITECTURE.md)
- [Examples](../examples/README.md)
