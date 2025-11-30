// Package mixer provides a privacy-preserving transaction mixing service.
//
// # Overview
//
// The mixer service enables users to mix their transactions through TEE-managed
// pool accounts, providing privacy through transaction obfuscation. It uses
// Zero-Knowledge Proofs (ZKP) and Trusted Execution Environment (TEE) signatures
// for on-chain verification.
//
// # Architecture
//
// This package is self-contained with the following components:
//
//   - domain.go: Type definitions (MixRequest, PoolAccount, etc.)
//   - store.go: Store interface and dependency interfaces
//   - store_postgres.go: PostgreSQL implementation
//   - service.go: Core business logic
//   - http.go: HTTP API handlers
//   - package.go: Service registration and initialization
//
// # API Endpoints
//
// The service exposes the following HTTP endpoints:
//
//	GET  /accounts/{id}/mixer          - List mix requests
//	POST /accounts/{id}/mixer          - Create mix request
//	GET  /accounts/{id}/mixer/{reqID}  - Get mix request details
//	POST /accounts/{id}/mixer/{reqID}/deposit - Confirm deposit
//	POST /accounts/{id}/mixer/{reqID}/claim   - Create withdrawal claim
//	GET  /mixer/stats                  - Get global mixing statistics
//
// # Mix Durations
//
// Supported mixing durations:
//
//   - 30min: Quick mixing for small amounts
//   - 1h: Standard mixing (default)
//   - 24h: Extended mixing for better privacy
//   - 7d: Maximum privacy mixing
//
// # Security Model
//
// 1. TEE-Managed Pool Accounts: Private keys are generated and stored within TEE
// 2. ZKP Verification: Proofs are generated for on-chain verification
// 3. Emergency Withdrawal: 7-day waiting period for claims when service unavailable
// 4. Service Collateral: Guarantee deposit for user protection
//
// # Usage Example
//
//	// Create a mix request
//	req := mixer.MixRequest{
//	    AccountID:    "acc_123",
//	    SourceWallet: "0x...",
//	    Amount:       "1000000000000000000", // 1 ETH in wei
//	    MixDuration:  mixer.MixDuration1Hour,
//	    Targets: []mixer.MixTarget{
//	        {Address: "0x...", Amount: "500000000000000000"},
//	        {Address: "0x...", Amount: "500000000000000000"},
//	    },
//	}
//	created, err := svc.CreateMixRequest(ctx, req)
//
// # Database Schema
//
// The service uses the following tables (see migrations/0031_mixer.sql):
//
//   - mixer_requests: Mix request records
//   - mixer_pool_accounts: TEE-managed pool wallets
//   - mixer_transactions: Internal obfuscation transactions
//   - mixer_withdrawal_claims: Emergency withdrawal claims
//   - mixer_service_deposit: Service collateral tracking
package mixer
