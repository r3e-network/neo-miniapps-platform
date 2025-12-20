// Package neoaccounts provides API routes for the neoaccounts service.
package neoaccounts

// =============================================================================
// API Routes
// =============================================================================

// registerRoutes registers service-specific HTTP handlers.
// Note: /health, /ready, and standard /info are registered by BaseService.RegisterStandardRoutes().
// /pool-info is the neoaccounts-specific endpoint for pool statistics.
func (s *Service) registerRoutes() {
	router := s.Router()
	router.HandleFunc("/master-key", s.handleMasterKey).Methods("GET")
	router.HandleFunc("/pool-info", s.handleInfo).Methods("GET")
	router.HandleFunc("/accounts", s.handleListAccounts).Methods("GET")
	router.HandleFunc("/request", s.handleRequestAccounts).Methods("POST")
	router.HandleFunc("/release", s.handleReleaseAccounts).Methods("POST")
	router.HandleFunc("/sign", s.handleSignTransaction).Methods("POST")
	router.HandleFunc("/batch-sign", s.handleBatchSign).Methods("POST")
	router.HandleFunc("/balance", s.handleUpdateBalance).Methods("POST")
	router.HandleFunc("/transfer", s.handleTransfer).Methods("POST")

	// Contract operations - all signing happens inside TEE
	router.HandleFunc("/deploy", s.handleDeployContract).Methods("POST")
	router.HandleFunc("/update-contract", s.handleUpdateContract).Methods("POST")
	router.HandleFunc("/invoke", s.handleInvokeContract).Methods("POST")
	router.HandleFunc("/simulate", s.handleSimulateContract).Methods("POST")
}
