// Package engine provides the Service Engine for the Service Layer.
//
// The Engine is the central orchestrator, similar to Android's SystemServer.
// It manages:
//   - TEE Trust Root initialization
//   - Service lifecycle (registration, start, stop)
//   - Service discovery and routing
//   - Health monitoring
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                         Engine                                  │
//	│  ┌─────────────────────────────────────────────────────────┐   │
//	│  │                    Service Registry                      │   │
//	│  │  oracle │ mixer │ secrets │ vrf │ gasbank │ ...         │   │
//	│  └─────────────────────────────────────────────────────────┘   │
//	│                              ↑                                  │
//	│  ┌─────────────────────────────────────────────────────────┐   │
//	│  │                    Foundation                            │   │
//	│  │  TEE Trust Root │ Supabase │ ServiceOS Factory          │   │
//	│  └─────────────────────────────────────────────────────────┘   │
//	└─────────────────────────────────────────────────────────────────┘
//
// Usage:
//
//	// Create engine with configuration
//	engine := engine.New(engine.Config{
//	    EnclaveID: "my-enclave",
//	    Mode:      "simulation",
//	})
//
//	// Register services
//	engine.RegisterService(engine.ServiceDefinition{
//	    Manifest: oracle.Manifest(),
//	    Factory:  func(os os.ServiceOS) (base.Service, error) { return oracle.New(os) },
//	})
//
//	// Start engine
//	if err := engine.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer engine.Stop(ctx)
//
//	// Access services
//	svc, ok := engine.GetService("oracle")
package engine
