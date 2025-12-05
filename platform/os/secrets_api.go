// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"

	"github.com/R3E-Network/service_layer/tee/types"
)

// secretsAPIImpl implements SecretsAPI.
type secretsAPIImpl struct {
	ctx       *ServiceContext
	vault     types.SecureVault
	namespace string
}

func newSecretsAPI(ctx *ServiceContext, vault types.SecureVault, namespace string) *secretsAPIImpl {
	return &secretsAPIImpl{
		ctx:       ctx,
		vault:     vault,
		namespace: namespace,
	}
}

func (s *secretsAPIImpl) Store(ctx context.Context, name string, value []byte) error {
	if err := s.ctx.RequireCapability(CapSecretsWrite); err != nil {
		return err
	}
	return s.vault.Store(ctx, s.namespace, name, value)
}

func (s *secretsAPIImpl) Use(ctx context.Context, name string, fn func(secret []byte) error) error {
	if err := s.ctx.RequireCapability(CapSecrets); err != nil {
		return err
	}
	return s.vault.Use(ctx, s.namespace, name, fn)
}

func (s *secretsAPIImpl) UseMultiple(ctx context.Context, names []string, fn func(secrets map[string][]byte) error) error {
	if err := s.ctx.RequireCapability(CapSecrets); err != nil {
		return err
	}
	refs := make([]types.SecretRef, len(names))
	for i, name := range names {
		refs[i] = types.SecretRef{Namespace: s.namespace, Name: name, Alias: name}
	}
	return s.vault.UseMultiple(ctx, refs, fn)
}

func (s *secretsAPIImpl) Delete(ctx context.Context, name string) error {
	if err := s.ctx.RequireCapability(CapSecretsWrite); err != nil {
		return err
	}
	return s.vault.Delete(ctx, s.namespace, name)
}

func (s *secretsAPIImpl) List(ctx context.Context) ([]string, error) {
	if err := s.ctx.RequireCapability(CapSecrets); err != nil {
		return nil, err
	}
	return s.vault.List(ctx, s.namespace)
}

func (s *secretsAPIImpl) Exists(ctx context.Context, name string) (bool, error) {
	if err := s.ctx.RequireCapability(CapSecrets); err != nil {
		return false, err
	}
	return s.vault.Exists(ctx, s.namespace, name)
}
