// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"

	"github.com/R3E-Network/service_layer/tee/types"
)

// keysAPIImpl implements KeysAPI.
type keysAPIImpl struct {
	ctx      *ServiceContext
	keys     types.KeyManager
	basePath string
}

func newKeysAPI(ctx *ServiceContext, keys types.KeyManager, basePath string) *keysAPIImpl {
	return &keysAPIImpl{
		ctx:      ctx,
		keys:     keys,
		basePath: basePath,
	}
}

func (k *keysAPIImpl) DeriveKey(ctx context.Context, subPath string) (KeyHandle, error) {
	if err := k.ctx.RequireCapability(CapKeys); err != nil {
		return "", err
	}

	fullPath := k.basePath + "/" + subPath
	handle, err := k.keys.DeriveKey(ctx, fullPath)
	if err != nil {
		return "", err
	}
	return KeyHandle(handle), nil
}

func (k *keysAPIImpl) Sign(ctx context.Context, handle KeyHandle, data []byte) ([]byte, error) {
	if err := k.ctx.RequireCapability(CapKeysSign); err != nil {
		return nil, err
	}
	return k.keys.Sign(ctx, types.KeyHandle(handle), data)
}

func (k *keysAPIImpl) Verify(ctx context.Context, publicKey, data, signature []byte) (bool, error) {
	if err := k.ctx.RequireCapability(CapKeys); err != nil {
		return false, err
	}
	return k.keys.Verify(ctx, publicKey, data, signature)
}

func (k *keysAPIImpl) GetPublicKey(ctx context.Context, handle KeyHandle) ([]byte, error) {
	if err := k.ctx.RequireCapability(CapKeys); err != nil {
		return nil, err
	}
	return k.keys.GetPublicKey(ctx, types.KeyHandle(handle))
}

func (k *keysAPIImpl) GetAddress(ctx context.Context, handle KeyHandle, chain ChainType) (string, error) {
	if err := k.ctx.RequireCapability(CapKeys); err != nil {
		return "", err
	}

	var teeChain types.ChainType
	switch chain {
	case ChainEthereum:
		teeChain = types.ChainTypeEthereum
	case ChainNeo:
		teeChain = types.ChainTypeNeo
	}

	return k.keys.GetAddress(ctx, types.KeyHandle(handle), teeChain)
}
