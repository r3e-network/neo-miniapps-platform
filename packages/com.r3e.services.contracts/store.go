package contracts

import "context"

// Store defines the persistence interface for the contracts service.
type Store interface {
	// Contract operations
	CreateContract(ctx context.Context, c Contract) (Contract, error)
	UpdateContract(ctx context.Context, c Contract) (Contract, error)
	GetContract(ctx context.Context, id string) (Contract, error)
	GetContractByAddress(ctx context.Context, network Network, address string) (Contract, error)
	ListContracts(ctx context.Context, accountID string) ([]Contract, error)
	ListContractsByService(ctx context.Context, serviceID string) ([]Contract, error)
	ListContractsByNetwork(ctx context.Context, network Network) ([]Contract, error)

	// Template operations
	CreateTemplate(ctx context.Context, t Template) (Template, error)
	UpdateTemplate(ctx context.Context, t Template) (Template, error)
	GetTemplate(ctx context.Context, id string) (Template, error)
	ListTemplates(ctx context.Context, category TemplateCategory) ([]Template, error)
	ListTemplatesByService(ctx context.Context, serviceID string) ([]Template, error)

	// Deployment operations
	CreateDeployment(ctx context.Context, d Deployment) (Deployment, error)
	UpdateDeployment(ctx context.Context, d Deployment) (Deployment, error)
	GetDeployment(ctx context.Context, id string) (Deployment, error)
	ListDeployments(ctx context.Context, contractID string, limit int) ([]Deployment, error)

	// Invocation operations
	CreateInvocation(ctx context.Context, inv Invocation) (Invocation, error)
	UpdateInvocation(ctx context.Context, inv Invocation) (Invocation, error)
	GetInvocation(ctx context.Context, id string) (Invocation, error)
	ListInvocations(ctx context.Context, contractID string, limit int) ([]Invocation, error)
	ListAccountInvocations(ctx context.Context, accountID string, limit int) ([]Invocation, error)

	// Service binding operations
	CreateServiceBinding(ctx context.Context, b ServiceContractBinding) (ServiceContractBinding, error)
	GetServiceBinding(ctx context.Context, id string) (ServiceContractBinding, error)
	ListServiceBindings(ctx context.Context, serviceID string) ([]ServiceContractBinding, error)
	ListAccountBindings(ctx context.Context, accountID string) ([]ServiceContractBinding, error)

	// Network config operations
	GetNetworkConfig(ctx context.Context, network Network) (NetworkConfig, error)
	ListNetworkConfigs(ctx context.Context) ([]NetworkConfig, error)
	SaveNetworkConfig(ctx context.Context, cfg NetworkConfig) (NetworkConfig, error)
}
