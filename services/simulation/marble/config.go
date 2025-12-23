// Package neosimulation provides simulation service for automated transaction testing.
package neosimulation

const (
	ServiceID   = "neosimulation"
	ServiceName = "Neo Simulation Service"
	Version     = "1.0.0"

	// Default simulation interval range (1-3 seconds target per miniapp)
	// With 2 concurrent workers per miniapp, each worker targets 2-6 seconds
	// This achieves ~1-3 seconds effective rate per miniapp
	DefaultMinIntervalMS = 2000 // 2 seconds minimum per worker
	DefaultMaxIntervalMS = 6000 // 6 seconds maximum per worker

	// Default number of concurrent workers per miniapp
	// With 2 workers and 2-6s interval, effective rate is 1-3s per miniapp
	DefaultWorkersPerApp = 2

	// Default simulation transaction amounts (in GAS smallest unit, 8 decimals)
	DefaultMinAmount = 1000000   // 0.01 GAS
	DefaultMaxAmount = 100000000 // 1 GAS
)

// Config holds simulation service configuration.
type Config struct {
	Marble           interface{} // *marble.Marble
	DB               interface{} // database.RepositoryInterface
	ChainClient      interface{} // *chain.Client
	AccountPoolURL   string
	MiniApps         []string      // List of app IDs to simulate
	MinIntervalMS    int           // Minimum interval between transactions (milliseconds)
	MaxIntervalMS    int           // Maximum interval between transactions (milliseconds)
	MinAmount        int64         // Minimum transaction amount
	MaxAmount        int64         // Maximum transaction amount
	WorkersPerApp    int           // Number of concurrent workers per miniapp
	AutoStart        bool          // Start simulation automatically on service start
}
