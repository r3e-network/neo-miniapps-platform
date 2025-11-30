package contracts

import "time"

// Network identifies the blockchain network a contract is deployed on.
type Network string

const (
	NetworkNeoN3     Network = "neo-n3"
	NetworkNeoX      Network = "neo-x"
	NetworkEthereum  Network = "ethereum"
	NetworkPolygon   Network = "polygon"
	NetworkArbitrum  Network = "arbitrum"
	NetworkOptimism  Network = "optimism"
	NetworkBase      Network = "base"
	NetworkAvalanche Network = "avalanche"
	NetworkBSC       Network = "bsc"
	NetworkTestnet   Network = "testnet"
	NetworkLocalPriv Network = "local-priv"
)

// DefaultNetwork returns the default deployment network (Neo N3).
func DefaultNetwork() Network { return NetworkNeoN3 }

// ContractType categorizes contracts by their role in the system.
type ContractType string

const (
	ContractTypeEngine  ContractType = "engine"
	ContractTypeService ContractType = "service"
	ContractTypeUser    ContractType = "user"
)

// ContractStatus tracks the lifecycle of a deployed contract.
type ContractStatus string

const (
	ContractStatusDraft      ContractStatus = "draft"
	ContractStatusDeploying  ContractStatus = "deploying"
	ContractStatusActive     ContractStatus = "active"
	ContractStatusPaused     ContractStatus = "paused"
	ContractStatusUpgrading  ContractStatus = "upgrading"
	ContractStatusDeprecated ContractStatus = "deprecated"
	ContractStatusRevoked    ContractStatus = "revoked"
)

// Contract represents a deployed smart contract.
type Contract struct {
	ID           string            `json:"id"`
	AccountID    string            `json:"account_id"`
	ServiceID    string            `json:"service_id,omitempty"`
	Name         string            `json:"name"`
	Symbol       string            `json:"symbol,omitempty"`
	Description  string            `json:"description,omitempty"`
	Type         ContractType      `json:"type"`
	Network      Network           `json:"network"`
	Address      string            `json:"address,omitempty"`
	CodeHash     string            `json:"code_hash,omitempty"`
	ConfigHash   string            `json:"config_hash,omitempty"`
	Version      string            `json:"version"`
	ABI          string            `json:"abi,omitempty"`
	Bytecode     string            `json:"bytecode,omitempty"`
	SourceHash   string            `json:"source_hash,omitempty"`
	Status       ContractStatus    `json:"status"`
	Capabilities []string          `json:"capabilities,omitempty"`
	DependsOn    []string          `json:"depends_on,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	DeployedAt   time.Time         `json:"deployed_at,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// ContractMethod describes a callable method on a contract.
type ContractMethod struct {
	ID              string            `json:"id"`
	ContractID      string            `json:"contract_id"`
	Name            string            `json:"name"`
	Selector        string            `json:"selector,omitempty"`
	Inputs          []MethodParam     `json:"inputs,omitempty"`
	Outputs         []MethodParam     `json:"outputs,omitempty"`
	StateMutability string            `json:"state_mutability"`
	Description     string            `json:"description,omitempty"`
	GasEstimate     int64             `json:"gas_estimate,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// MethodParam describes a parameter in a contract method.
type MethodParam struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Indexed bool   `json:"indexed,omitempty"`
}

// ContractEvent describes an event emitted by a contract.
type ContractEvent struct {
	ID          string            `json:"id"`
	ContractID  string            `json:"contract_id"`
	Name        string            `json:"name"`
	Signature   string            `json:"signature,omitempty"`
	Params      []MethodParam     `json:"params,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// InvocationStatus tracks the lifecycle of a contract invocation.
type InvocationStatus string

const (
	InvocationStatusPending   InvocationStatus = "pending"
	InvocationStatusSubmitted InvocationStatus = "submitted"
	InvocationStatusConfirmed InvocationStatus = "confirmed"
	InvocationStatusFailed    InvocationStatus = "failed"
	InvocationStatusReverted  InvocationStatus = "reverted"
)

// Invocation records a contract method call.
type Invocation struct {
	ID          string            `json:"id"`
	AccountID   string            `json:"account_id"`
	ContractID  string            `json:"contract_id"`
	MethodName  string            `json:"method_name"`
	Args        map[string]any    `json:"args,omitempty"`
	GasLimit    int64             `json:"gas_limit,omitempty"`
	GasUsed     int64             `json:"gas_used,omitempty"`
	GasPrice    string            `json:"gas_price,omitempty"`
	Value       string            `json:"value,omitempty"`
	TxHash      string            `json:"tx_hash,omitempty"`
	BlockNumber int64             `json:"block_number,omitempty"`
	BlockHash   string            `json:"block_hash,omitempty"`
	Status      InvocationStatus  `json:"status"`
	Result      any               `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	Logs        []EventLog        `json:"logs,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	SubmittedAt time.Time         `json:"submitted_at"`
	ConfirmedAt time.Time         `json:"confirmed_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// EventLog represents an event emitted during contract execution.
type EventLog struct {
	ContractID  string         `json:"contract_id"`
	EventName   string         `json:"event_name"`
	Topics      []string       `json:"topics,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
	LogIndex    int            `json:"log_index"`
	BlockNumber int64          `json:"block_number"`
	TxHash      string         `json:"tx_hash"`
}

// DeploymentStatus tracks the lifecycle of a deployment.
type DeploymentStatus string

const (
	DeploymentStatusPending   DeploymentStatus = "pending"
	DeploymentStatusSubmitted DeploymentStatus = "submitted"
	DeploymentStatusConfirmed DeploymentStatus = "confirmed"
	DeploymentStatusFailed    DeploymentStatus = "failed"
)

// Deployment tracks a contract deployment operation.
type Deployment struct {
	ID              string            `json:"id"`
	AccountID       string            `json:"account_id"`
	ContractID      string            `json:"contract_id"`
	Network         Network           `json:"network"`
	Bytecode        string            `json:"bytecode"`
	ConstructorArgs map[string]any    `json:"constructor_args,omitempty"`
	GasLimit        int64             `json:"gas_limit,omitempty"`
	GasUsed         int64             `json:"gas_used,omitempty"`
	GasPrice        string            `json:"gas_price,omitempty"`
	TxHash          string            `json:"tx_hash,omitempty"`
	Address         string            `json:"address,omitempty"`
	BlockNumber     int64             `json:"block_number,omitempty"`
	Status          DeploymentStatus  `json:"status"`
	Error           string            `json:"error,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	SubmittedAt     time.Time         `json:"submitted_at,omitempty"`
	ConfirmedAt     time.Time         `json:"confirmed_at,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// TemplateCategory groups templates by purpose.
type TemplateCategory string

const (
	TemplateCategoryEngine     TemplateCategory = "engine"
	TemplateCategoryToken      TemplateCategory = "token"
	TemplateCategoryOracle     TemplateCategory = "oracle"
	TemplateCategoryVRF        TemplateCategory = "vrf"
	TemplateCategoryFeed       TemplateCategory = "feed"
	TemplateCategoryDeFi       TemplateCategory = "defi"
	TemplateCategoryVault      TemplateCategory = "vault"
	TemplateCategoryStake      TemplateCategory = "stake"
	TemplateCategoryProxy      TemplateCategory = "proxy"
	TemplateCategoryMultisig   TemplateCategory = "multisig"
	TemplateCategoryGovernance TemplateCategory = "governance"
	TemplateCategoryCustom     TemplateCategory = "custom"
)

// TemplateStatus tracks template availability.
type TemplateStatus string

const (
	TemplateStatusDraft      TemplateStatus = "draft"
	TemplateStatusActive     TemplateStatus = "active"
	TemplateStatusDeprecated TemplateStatus = "deprecated"
)

// Template defines a reusable contract template.
type Template struct {
	ID           string            `json:"id"`
	ServiceID    string            `json:"service_id,omitempty"`
	Name         string            `json:"name"`
	Symbol       string            `json:"symbol,omitempty"`
	Description  string            `json:"description,omitempty"`
	Category     TemplateCategory  `json:"category"`
	Networks     []Network         `json:"networks"`
	Version      string            `json:"version"`
	ABI          string            `json:"abi"`
	Bytecode     string            `json:"bytecode"`
	SourceCode   string            `json:"source_code,omitempty"`
	SourceLang   string            `json:"source_lang,omitempty"`
	CodeHash     string            `json:"code_hash"`
	Audited      bool              `json:"audited"`
	AuditReport  string            `json:"audit_report,omitempty"`
	Params       []TemplateParam   `json:"params,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	DependsOn    []string          `json:"depends_on,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Status       TemplateStatus    `json:"status"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// TemplateParam describes a constructor parameter for template deployment.
type TemplateParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
}

// ServiceContractBinding describes how a service binds to on-chain contracts.
type ServiceContractBinding struct {
	ID         string            `json:"id"`
	ServiceID  string            `json:"service_id"`
	AccountID  string            `json:"account_id"`
	ContractID string            `json:"contract_id"`
	Network    Network           `json:"network"`
	Role       string            `json:"role"`
	Enabled    bool              `json:"enabled"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// NetworkConfig holds network-specific configuration.
type NetworkConfig struct {
	Network         Network           `json:"network"`
	ChainID         int64             `json:"chain_id"`
	RPCEndpoint     string            `json:"rpc_endpoint"`
	WSEndpoint      string            `json:"ws_endpoint,omitempty"`
	ExplorerURL     string            `json:"explorer_url,omitempty"`
	NativeToken     string            `json:"native_token"`
	NativeDecimals  int               `json:"native_decimals"`
	BlockTime       int               `json:"block_time_seconds"`
	Confirmations   int               `json:"confirmations"`
	EngineContracts map[string]string `json:"engine_contracts,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Enabled         bool              `json:"enabled"`
}

// EngineContracts defines the standard engine contract set.
var EngineContracts = []string{
	"Manager",
	"AccountManager",
	"ServiceRegistry",
	"GasBank",
	"OracleHub",
	"RandomnessHub",
	"DataFeedHub",
	"AutomationScheduler",
	"SecretsVault",
	"JAMInbox",
}

// DefaultNeoN3Config returns the default NetworkConfig for Neo N3 mainnet.
func DefaultNeoN3Config() NetworkConfig {
	return NetworkConfig{
		Network:        NetworkNeoN3,
		ChainID:        860833102,
		RPCEndpoint:    "https://mainnet1.neo.coz.io:443",
		WSEndpoint:     "wss://mainnet1.neo.coz.io:443/ws",
		ExplorerURL:    "https://dora.coz.io",
		NativeToken:    "GAS",
		NativeDecimals: 8,
		BlockTime:      15,
		Confirmations:  1,
		Enabled:        true,
	}
}

// DefaultNeoN3TestnetConfig returns the default NetworkConfig for Neo N3 testnet.
func DefaultNeoN3TestnetConfig() NetworkConfig {
	return NetworkConfig{
		Network:        NetworkTestnet,
		ChainID:        894710606,
		RPCEndpoint:    "https://testnet1.neo.coz.io:443",
		WSEndpoint:     "wss://testnet1.neo.coz.io:443/ws",
		ExplorerURL:    "https://dora.coz.io/testnet",
		NativeToken:    "GAS",
		NativeDecimals: 8,
		BlockTime:      15,
		Confirmations:  1,
		Enabled:        true,
	}
}

// DefaultNeoPrivnetConfig returns the default NetworkConfig for local Neo privnet.
func DefaultNeoPrivnetConfig() NetworkConfig {
	return NetworkConfig{
		Network:        NetworkLocalPriv,
		ChainID:        1234567890,
		RPCEndpoint:    "http://localhost:20332",
		NativeToken:    "GAS",
		NativeDecimals: 8,
		BlockTime:      1,
		Confirmations:  1,
		Enabled:        true,
	}
}
