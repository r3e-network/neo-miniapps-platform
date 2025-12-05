// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"

	"github.com/R3E-Network/service_layer/tee/types"
)

// neoAPIImpl implements NeoAPI.
type neoAPIImpl struct {
	ctx       *ServiceContext
	neo       types.NeoSigner
	serviceID string
}

func newNeoAPI(ctx *ServiceContext, neo types.NeoSigner, serviceID string) *neoAPIImpl {
	return &neoAPIImpl{
		ctx:       ctx,
		neo:       neo,
		serviceID: serviceID,
	}
}

func (n *neoAPIImpl) CreateWallet(ctx context.Context, path string) (WalletHandle, error) {
	if err := n.ctx.RequireCapability(CapNeo); err != nil {
		return "", err
	}
	fullPath := n.serviceID + "/" + path
	handle, err := n.neo.CreateWallet(ctx, fullPath)
	if err != nil {
		return "", err
	}
	return WalletHandle(handle), nil
}

func (n *neoAPIImpl) DeriveWallet(ctx context.Context, seedPath string) (WalletHandle, error) {
	if err := n.ctx.RequireCapability(CapNeo); err != nil {
		return "", err
	}
	fullPath := n.serviceID + "/" + seedPath
	handle, err := n.neo.DeriveWallet(ctx, fullPath)
	if err != nil {
		return "", err
	}
	return WalletHandle(handle), nil
}

func (n *neoAPIImpl) GetAddress(ctx context.Context, handle WalletHandle) (string, error) {
	if err := n.ctx.RequireCapability(CapNeo); err != nil {
		return "", err
	}
	return n.neo.GetAddress(ctx, types.WalletHandle(handle))
}

func (n *neoAPIImpl) GetPublicKey(ctx context.Context, handle WalletHandle) ([]byte, error) {
	if err := n.ctx.RequireCapability(CapNeo); err != nil {
		return nil, err
	}
	return n.neo.GetPublicKey(ctx, types.WalletHandle(handle))
}

func (n *neoAPIImpl) GetScriptHash(ctx context.Context, handle WalletHandle) ([]byte, error) {
	if err := n.ctx.RequireCapability(CapNeo); err != nil {
		return nil, err
	}
	return n.neo.GetScriptHash(ctx, types.WalletHandle(handle))
}

func (n *neoAPIImpl) SignData(ctx context.Context, handle WalletHandle, data []byte) ([]byte, error) {
	if err := n.ctx.RequireCapability(CapNeoSign); err != nil {
		return nil, err
	}
	return n.neo.SignData(ctx, types.WalletHandle(handle), data)
}

func (n *neoAPIImpl) SignTransaction(ctx context.Context, handle WalletHandle, txHash []byte) ([]byte, error) {
	if err := n.ctx.RequireCapability(CapNeoSign); err != nil {
		return nil, err
	}
	return n.neo.SignTransaction(ctx, types.WalletHandle(handle), txHash)
}

func (n *neoAPIImpl) VerifySignature(ctx context.Context, publicKey, data, signature []byte) (bool, error) {
	if err := n.ctx.RequireCapability(CapNeo); err != nil {
		return false, err
	}
	return n.neo.VerifySignature(ctx, publicKey, data, signature)
}

func (n *neoAPIImpl) ListWallets(ctx context.Context) ([]WalletHandle, error) {
	if err := n.ctx.RequireCapability(CapNeo); err != nil {
		return nil, err
	}
	handles, err := n.neo.ListWallets(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]WalletHandle, len(handles))
	for i, h := range handles {
		result[i] = WalletHandle(h)
	}
	return result, nil
}

func (n *neoAPIImpl) DeleteWallet(ctx context.Context, handle WalletHandle) error {
	if err := n.ctx.RequireCapability(CapNeo); err != nil {
		return err
	}
	return n.neo.DeleteWallet(ctx, types.WalletHandle(handle))
}
