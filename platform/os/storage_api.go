// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"

	"github.com/R3E-Network/service_layer/tee/types"
)

// storageAPIImpl implements StorageAPI.
type storageAPIImpl struct {
	ctx       *ServiceContext
	namespace string
	vault     types.SecureVault
}

func newStorageAPI(ctx *ServiceContext, vault types.SecureVault, namespace string) *storageAPIImpl {
	return &storageAPIImpl{
		ctx:       ctx,
		namespace: namespace,
		vault:     vault,
	}
}

func (s *storageAPIImpl) Get(ctx context.Context, key string) ([]byte, error) {
	if err := s.ctx.RequireCapability(CapStorage); err != nil {
		return nil, err
	}

	var out []byte
	err := s.vault.Use(ctx, s.namespace, key, func(secret []byte) error {
		out = append(out, secret...)
		return nil
	})
	if err != nil {
		return nil, NewOSError(ErrCodeStorageError, err.Error())
	}
	return out, nil
}

func (s *storageAPIImpl) Use(ctx context.Context, key string, fn func(value []byte) error) error {
	if err := s.ctx.RequireCapability(CapStorage); err != nil {
		return err
	}
	return s.vault.Use(ctx, s.namespace, key, fn)
}

func (s *storageAPIImpl) Put(ctx context.Context, key string, value []byte) error {
	if err := s.ctx.RequireCapability(CapStorage); err != nil {
		return err
	}
	if err := s.vault.Store(ctx, s.namespace, key, value); err != nil {
		return NewOSError(ErrCodeStorageError, err.Error())
	}
	return nil
}

func (s *storageAPIImpl) Delete(ctx context.Context, key string) error {
	if err := s.ctx.RequireCapability(CapStorage); err != nil {
		return err
	}
	if err := s.vault.Delete(ctx, s.namespace, key); err != nil {
		return NewOSError(ErrCodeStorageError, err.Error())
	}
	return nil
}

func (s *storageAPIImpl) List(ctx context.Context, prefix string) ([]string, error) {
	if err := s.ctx.RequireCapability(CapStorage); err != nil {
		return nil, err
	}
	names, err := s.vault.List(ctx, s.namespace)
	if err != nil {
		return nil, NewOSError(ErrCodeStorageError, err.Error())
	}
	// Filter by prefix
	var filtered []string
	for _, n := range names {
		if len(prefix) == 0 || (len(n) >= len(prefix) && n[:len(prefix)] == prefix) {
			filtered = append(filtered, n)
		}
	}
	return filtered, nil
}

func (s *storageAPIImpl) Exists(ctx context.Context, key string) (bool, error) {
	if err := s.ctx.RequireCapability(CapStorage); err != nil {
		return false, err
	}
	return s.vault.Exists(ctx, s.namespace, key)
}
