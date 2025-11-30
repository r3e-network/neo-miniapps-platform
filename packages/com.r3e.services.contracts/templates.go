package contracts

import (
	"context"
	"fmt"
	"strings"

	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// CreateTemplate registers a contract template.
func (s *Service) CreateTemplate(ctx context.Context, t Template) (Template, error) {
	if err := s.normalizeTemplate(&t); err != nil {
		return Template{}, err
	}
	created, err := s.store.CreateTemplate(ctx, t)
	if err != nil {
		return Template{}, err
	}
	s.log.WithField("template_id", created.ID).WithField("name", created.Name).Info("contract template registered")
	return created, nil
}

// UpdateTemplate updates template metadata.
func (s *Service) UpdateTemplate(ctx context.Context, t Template) (Template, error) {
	stored, err := s.store.GetTemplate(ctx, t.ID)
	if err != nil {
		return Template{}, err
	}
	t.ServiceID = stored.ServiceID // Preserve owning service
	if err := s.normalizeTemplate(&t); err != nil {
		return Template{}, err
	}
	updated, err := s.store.UpdateTemplate(ctx, t)
	if err != nil {
		return Template{}, err
	}
	s.log.WithField("template_id", t.ID).Info("contract template updated")
	return updated, nil
}

// GetTemplate fetches a template.
func (s *Service) GetTemplate(ctx context.Context, templateID string) (Template, error) {
	return s.store.GetTemplate(ctx, templateID)
}

// ListTemplates lists templates by category.
func (s *Service) ListTemplates(ctx context.Context, category TemplateCategory) ([]Template, error) {
	return s.store.ListTemplates(ctx, category)
}

// ListTemplatesByService lists templates for a specific service.
func (s *Service) ListTemplatesByService(ctx context.Context, serviceID string) ([]Template, error) {
	return s.store.ListTemplatesByService(ctx, serviceID)
}

// ListEngineTemplates lists all engine-level templates.
func (s *Service) ListEngineTemplates(ctx context.Context) ([]Template, error) {
	return s.store.ListTemplates(ctx, TemplateCategoryEngine)
}

// DeployFromTemplate creates a contract from a template and deploys it.
func (s *Service) DeployFromTemplate(ctx context.Context, accountID, templateID string, name string, constructorArgs map[string]any, gasLimit int64, metadata map[string]string) (Contract, Deployment, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return Contract{}, Deployment{}, err
	}

	tpl, err := s.store.GetTemplate(ctx, templateID)
	if err != nil {
		return Contract{}, Deployment{}, err
	}
	if tpl.Status != TemplateStatusActive {
		return Contract{}, Deployment{}, fmt.Errorf("template %s is not active", templateID)
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = tpl.Name
	}

	// Determine contract type
	contractType := ContractTypeUser
	if tpl.Category == TemplateCategoryEngine {
		contractType = ContractTypeEngine
	} else if tpl.ServiceID != "" {
		contractType = ContractTypeService
	}

	// Default to first supported network
	network := NetworkNeoN3
	if len(tpl.Networks) > 0 {
		network = tpl.Networks[0]
	}

	// Create contract record
	c := Contract{
		AccountID:    accountID,
		ServiceID:    tpl.ServiceID,
		Name:         name,
		Symbol:       tpl.Symbol,
		Description:  tpl.Description,
		Type:         contractType,
		Network:      network,
		CodeHash:     tpl.CodeHash,
		Version:      tpl.Version,
		ABI:          tpl.ABI,
		Bytecode:     tpl.Bytecode,
		Status:       ContractStatusDraft,
		Capabilities: tpl.Capabilities,
		DependsOn:    tpl.DependsOn,
		Metadata:     core.NormalizeMetadata(metadata),
	}

	createdContract, err := s.store.CreateContract(ctx, c)
	if err != nil {
		return Contract{}, Deployment{}, err
	}

	// Deploy the contract
	deployment, err := s.Deploy(ctx, accountID, createdContract.ID, constructorArgs, gasLimit, metadata)
	if err != nil {
		// Mark contract as failed deployment
		createdContract.Status = ContractStatusDraft
		_, _ = s.store.UpdateContract(ctx, createdContract)
		return createdContract, deployment, err
	}

	s.log.WithField("contract_id", createdContract.ID).WithField("template_id", templateID).Info("contract deployed from template")
	return createdContract, deployment, nil
}

// CreateServiceBinding binds a service to a contract.
func (s *Service) CreateServiceBinding(ctx context.Context, binding ServiceContractBinding) (ServiceContractBinding, error) {
	if err := s.base.EnsureAccount(ctx, binding.AccountID); err != nil {
		return ServiceContractBinding{}, err
	}
	// Verify contract exists and account has access
	if _, err := s.GetContract(ctx, binding.AccountID, binding.ContractID); err != nil {
		return ServiceContractBinding{}, err
	}

	binding.ServiceID = strings.TrimSpace(binding.ServiceID)
	if binding.ServiceID == "" {
		return ServiceContractBinding{}, core.RequiredError("service_id")
	}
	binding.Role = strings.ToLower(strings.TrimSpace(binding.Role))
	if binding.Role == "" {
		binding.Role = "consumer"
	}
	binding.Enabled = true
	binding.Metadata = core.NormalizeMetadata(binding.Metadata)

	created, err := s.store.CreateServiceBinding(ctx, binding)
	if err != nil {
		return ServiceContractBinding{}, err
	}
	s.log.WithField("binding_id", created.ID).WithField("service_id", created.ServiceID).Info("service contract binding created")
	return created, nil
}

// GetServiceBinding fetches a service binding.
func (s *Service) GetServiceBinding(ctx context.Context, bindingID string) (ServiceContractBinding, error) {
	return s.store.GetServiceBinding(ctx, bindingID)
}

// ListServiceBindings lists bindings for a service.
func (s *Service) ListServiceBindings(ctx context.Context, serviceID string) ([]ServiceContractBinding, error) {
	return s.store.ListServiceBindings(ctx, serviceID)
}

// ListAccountBindings lists all bindings for an account.
func (s *Service) ListAccountBindings(ctx context.Context, accountID string) ([]ServiceContractBinding, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListAccountBindings(ctx, accountID)
}

func (s *Service) normalizeTemplate(t *Template) error {
	t.Name = strings.TrimSpace(t.Name)
	t.Symbol = strings.ToUpper(strings.TrimSpace(t.Symbol))
	t.Description = strings.TrimSpace(t.Description)
	t.Version = strings.TrimSpace(t.Version)
	t.ABI = strings.TrimSpace(t.ABI)
	t.Bytecode = strings.TrimSpace(t.Bytecode)
	t.CodeHash = strings.TrimSpace(t.CodeHash)
	t.SourceCode = strings.TrimSpace(t.SourceCode)
	t.SourceLang = strings.ToLower(strings.TrimSpace(t.SourceLang))
	t.Metadata = core.NormalizeMetadata(t.Metadata)
	t.Tags = core.NormalizeTags(t.Tags)

	if t.Name == "" {
		return core.RequiredError("name")
	}
	if t.ABI == "" {
		return core.RequiredError("abi")
	}
	if t.Bytecode == "" {
		return core.RequiredError("bytecode")
	}
	if t.Version == "" {
		t.Version = "1.0.0"
	}

	category := TemplateCategory(strings.ToLower(strings.TrimSpace(string(t.Category))))
	if category == "" {
		category = TemplateCategoryCustom
	}
	t.Category = category

	status := TemplateStatus(strings.ToLower(strings.TrimSpace(string(t.Status))))
	if status == "" {
		status = TemplateStatusDraft
	}
	switch status {
	case TemplateStatusDraft, TemplateStatusActive, TemplateStatusDeprecated:
		t.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}

	return nil
}
