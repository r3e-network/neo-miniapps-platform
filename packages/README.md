# Service Packages Architecture

Each service package is **self-contained** and follows the principle:
> "Service engine should be generic and unaware of any specific service"

## Package Structure

Every service package should contain:

```
packages/com.r3e.services.<name>/
├── doc.go              # Package documentation and API reference
├── domain.go           # Type definitions (models, enums, constants)
├── store.go            # Store interface and dependency interfaces
├── store_postgres.go   # PostgreSQL implementation
├── service.go          # Core business logic
├── service_test.go     # Unit tests
├── http.go             # HTTP API handlers (self-registered)
├── package.go          # Service registration and initialization
└── testing.go          # Test helpers and mocks
```

## Self-Registration Pattern

Services register themselves via `init()` functions:

```go
// package.go
func init() {
    pkg.MustRegisterPackage("com.r3e.services.myservice", func() (pkg.ServicePackage, error) {
        return &Package{...}, nil
    })
}
```

## HTTP Handler Registration

Each service registers its own HTTP routes:

```go
// http.go
type HTTPHandler struct {
    svc *Service
}

func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux, basePath string) {
    mux.HandleFunc(basePath, h.handleRoot)
    mux.HandleFunc(basePath+"/", h.handleWithID)
}
```

## Key Principles

1. **No service-specific code in engine layer** - Engine is generic
2. **Services own their HTTP handlers** - Not in applications/httpapi/
3. **Services own their documentation** - doc.go in each package
4. **Services own their contracts** - domain.go defines all types
5. **Services self-register** - Via init() functions
6. **Services declare dependencies** - Via interfaces, not concrete types

## Migration Status

| Service | Self-Contained | HTTP Handler | Documentation |
|---------|---------------|--------------|---------------|
| mixer | ✅ | ✅ | ✅ |
| accounts | ⏳ | ❌ | ❌ |
| functions | ⏳ | ❌ | ❌ |
| gasbank | ⏳ | ❌ | ❌ |
| automation | ⏳ | ❌ | ❌ |
| oracle | ⏳ | ❌ | ❌ |
| vrf | ⏳ | ❌ | ❌ |
| datafeeds | ⏳ | ❌ | ❌ |
| datastreams | ⏳ | ❌ | ❌ |
| datalink | ⏳ | ❌ | ❌ |
| ccip | ⏳ | ❌ | ❌ |
| cre | ⏳ | ❌ | ❌ |
| dta | ⏳ | ❌ | ❌ |
| confidential | ⏳ | ❌ | ❌ |
| secrets | ⏳ | ❌ | ❌ |

Legend: ✅ Complete | ⏳ Partial | ❌ Not Started
