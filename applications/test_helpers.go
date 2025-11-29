package app

import "github.com/R3E-Network/service_layer/applications/storage/memory"

// NewMemoryStoresForTest creates a Stores instance backed by the in-memory
// storage implementation. This is intended for use in tests and local development.
func NewMemoryStoresForTest() Stores {
	store := memory.New()
	return Stores{
		Accounts:         store,
		Functions:        store,
		GasBank:          store,
		Automation:       store,
		DataFeeds:        store,
		DataStreams:      store,
		DataLink:         store,
		DTA:              store,
		Confidential:     store,
		Oracle:           store,
		Secrets:          store,
		CRE:              store,
		CCIP:             store,
		VRF:              store,
		WorkspaceWallets: store,
	}
}
