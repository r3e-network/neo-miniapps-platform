// Package pkg provides the StoreProvider implementation.
package pkg

// storeProvider implements StoreProvider interface.
// It wraps the actual storage implementations from applications/storage.
type storeProvider struct {
	accounts         AccountStoreAPI
	functions        FunctionStoreAPI
	gasBank          GasBankStoreAPI
	automation       AutomationStoreAPI
	dataFeeds        DataFeedStoreAPI
	dataStreams      DataStreamStoreAPI
	dataLink         DataLinkStoreAPI
	dta              DTAStoreAPI
	confidential     ConfidentialStoreAPI
	oracle           OracleStoreAPI
	secrets          SecretStoreAPI
	cre              CREStoreAPI
	ccip             CCIPStoreAPI
	vrf              VRFStoreAPI
	workspaceWallets WorkspaceWalletStoreAPI
}

// StoreProviderConfig contains all storage implementations to inject.
type StoreProviderConfig struct {
	Accounts         AccountStoreAPI
	Functions        FunctionStoreAPI
	GasBank          GasBankStoreAPI
	Automation       AutomationStoreAPI
	DataFeeds        DataFeedStoreAPI
	DataStreams      DataStreamStoreAPI
	DataLink         DataLinkStoreAPI
	DTA              DTAStoreAPI
	Confidential     ConfidentialStoreAPI
	Oracle           OracleStoreAPI
	Secrets          SecretStoreAPI
	CRE              CREStoreAPI
	CCIP             CCIPStoreAPI
	VRF              VRFStoreAPI
	WorkspaceWallets WorkspaceWalletStoreAPI
}

// NewStoreProvider creates a StoreProvider from the given configuration.
func NewStoreProvider(cfg StoreProviderConfig) StoreProvider {
	return &storeProvider{
		accounts:         cfg.Accounts,
		functions:        cfg.Functions,
		gasBank:          cfg.GasBank,
		automation:       cfg.Automation,
		dataFeeds:        cfg.DataFeeds,
		dataStreams:      cfg.DataStreams,
		dataLink:         cfg.DataLink,
		dta:              cfg.DTA,
		confidential:     cfg.Confidential,
		oracle:           cfg.Oracle,
		secrets:          cfg.Secrets,
		cre:              cfg.CRE,
		ccip:             cfg.CCIP,
		vrf:              cfg.VRF,
		workspaceWallets: cfg.WorkspaceWallets,
	}
}

// Implement StoreProvider interface methods

func (s *storeProvider) AccountStore() AccountStoreAPI {
	return s.accounts
}

func (s *storeProvider) FunctionStore() FunctionStoreAPI {
	return s.functions
}

func (s *storeProvider) GasBankStore() GasBankStoreAPI {
	return s.gasBank
}

func (s *storeProvider) AutomationStore() AutomationStoreAPI {
	return s.automation
}

func (s *storeProvider) DataFeedStore() DataFeedStoreAPI {
	return s.dataFeeds
}

func (s *storeProvider) DataStreamStore() DataStreamStoreAPI {
	return s.dataStreams
}

func (s *storeProvider) DataLinkStore() DataLinkStoreAPI {
	return s.dataLink
}

func (s *storeProvider) DTAStore() DTAStoreAPI {
	return s.dta
}

func (s *storeProvider) ConfidentialStore() ConfidentialStoreAPI {
	return s.confidential
}

func (s *storeProvider) OracleStore() OracleStoreAPI {
	return s.oracle
}

func (s *storeProvider) SecretStore() SecretStoreAPI {
	return s.secrets
}

func (s *storeProvider) CREStore() CREStoreAPI {
	return s.cre
}

func (s *storeProvider) CCIPStore() CCIPStoreAPI {
	return s.ccip
}

func (s *storeProvider) VRFStore() VRFStoreAPI {
	return s.vrf
}

func (s *storeProvider) WorkspaceWalletStore() WorkspaceWalletStoreAPI {
	return s.workspaceWallets
}

// NilStoreProvider returns a StoreProvider with all nil stores.
// This is useful for testing or when stores are not yet available.
func NilStoreProvider() StoreProvider {
	return &storeProvider{}
}
