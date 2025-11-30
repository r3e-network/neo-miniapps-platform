// Package vrf provides the VRF Service as a ServicePackage.
package vrf

import (
	"context"

	"github.com/R3E-Network/service_layer/pkg/storage"
	domainvrf "github.com/R3E-Network/service_layer/domain/vrf"
)

// NewFromLegacyStores creates a VRF service from legacy storage interfaces.
// This is a transitional adapter for backward compatibility during migration.
// Used primarily for testing with memory.Store.
func NewFromLegacyStores(db any, accounts storage.AccountStore, vrfStore storage.VRFStore) (*Service, Store) {
	accountChecker := &legacyAccountChecker{store: accounts}
	store := &legacyStoreAdapter{store: vrfStore}
	return New(accountChecker, store, nil), store
}

// legacyAccountChecker adapts storage.AccountStore to AccountChecker interface.
type legacyAccountChecker struct {
	store storage.AccountStore
}

func (a *legacyAccountChecker) AccountExists(ctx context.Context, accountID string) error {
	_, err := a.store.GetAccount(ctx, accountID)
	return err
}

func (a *legacyAccountChecker) AccountTenant(ctx context.Context, accountID string) string {
	acct, err := a.store.GetAccount(ctx, accountID)
	if err != nil {
		return ""
	}
	if acct.Metadata != nil {
		return acct.Metadata["tenant"]
	}
	return ""
}

// legacyStoreAdapter adapts storage.VRFStore to local Store interface.
type legacyStoreAdapter struct {
	store storage.VRFStore
}

func (a *legacyStoreAdapter) CreateKey(ctx context.Context, key Key) (Key, error) {
	domainKey := keyToDomain(key)
	created, err := a.store.CreateVRFKey(ctx, domainKey)
	if err != nil {
		return Key{}, err
	}
	return keyFromDomain(created), nil
}

func (a *legacyStoreAdapter) UpdateKey(ctx context.Context, key Key) (Key, error) {
	domainKey := keyToDomain(key)
	updated, err := a.store.UpdateVRFKey(ctx, domainKey)
	if err != nil {
		return Key{}, err
	}
	return keyFromDomain(updated), nil
}

func (a *legacyStoreAdapter) GetKey(ctx context.Context, id string) (Key, error) {
	domainKey, err := a.store.GetVRFKey(ctx, id)
	if err != nil {
		return Key{}, err
	}
	return keyFromDomain(domainKey), nil
}

func (a *legacyStoreAdapter) ListKeys(ctx context.Context, accountID string) ([]Key, error) {
	domainKeys, err := a.store.ListVRFKeys(ctx, accountID)
	if err != nil {
		return nil, err
	}
	keys := make([]Key, len(domainKeys))
	for i, dk := range domainKeys {
		keys[i] = keyFromDomain(dk)
	}
	return keys, nil
}

func (a *legacyStoreAdapter) CreateRequest(ctx context.Context, req Request) (Request, error) {
	domainReq := requestToDomain(req)
	created, err := a.store.CreateVRFRequest(ctx, domainReq)
	if err != nil {
		return Request{}, err
	}
	return requestFromDomain(created), nil
}

func (a *legacyStoreAdapter) GetRequest(ctx context.Context, id string) (Request, error) {
	domainReq, err := a.store.GetVRFRequest(ctx, id)
	if err != nil {
		return Request{}, err
	}
	return requestFromDomain(domainReq), nil
}

func (a *legacyStoreAdapter) ListRequests(ctx context.Context, accountID string, limit int) ([]Request, error) {
	domainReqs, err := a.store.ListVRFRequests(ctx, accountID, limit)
	if err != nil {
		return nil, err
	}
	reqs := make([]Request, len(domainReqs))
	for i, dr := range domainReqs {
		reqs[i] = requestFromDomain(dr)
	}
	return reqs, nil
}

// LegacyWalletChecker adapts storage.WorkspaceWalletStore to WalletChecker interface.
type LegacyWalletChecker struct {
	Store storage.WorkspaceWalletStore
}

func (w *LegacyWalletChecker) WalletOwnedBy(ctx context.Context, accountID, wallet string) error {
	_, err := w.Store.FindWorkspaceWalletByAddress(ctx, accountID, wallet)
	return err
}

// Type conversion functions

func keyToDomain(k Key) domainvrf.Key {
	return domainvrf.Key{
		ID:            k.ID,
		AccountID:     k.AccountID,
		PublicKey:     k.PublicKey,
		Label:         k.Label,
		Status:        domainvrf.KeyStatus(k.Status),
		WalletAddress: k.WalletAddress,
		Attestation:   k.Attestation,
		Metadata:      k.Metadata,
		CreatedAt:     k.CreatedAt,
		UpdatedAt:     k.UpdatedAt,
	}
}

func keyFromDomain(k domainvrf.Key) Key {
	return Key{
		ID:            k.ID,
		AccountID:     k.AccountID,
		PublicKey:     k.PublicKey,
		Label:         k.Label,
		Status:        KeyStatus(k.Status),
		WalletAddress: k.WalletAddress,
		Attestation:   k.Attestation,
		Metadata:      k.Metadata,
		CreatedAt:     k.CreatedAt,
		UpdatedAt:     k.UpdatedAt,
	}
}

func requestToDomain(r Request) domainvrf.Request {
	return domainvrf.Request{
		ID:          r.ID,
		AccountID:   r.AccountID,
		KeyID:       r.KeyID,
		Consumer:    r.Consumer,
		Seed:        r.Seed,
		Status:      domainvrf.RequestStatus(r.Status),
		Result:      r.Result,
		Error:       r.Error,
		Metadata:    r.Metadata,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		FulfilledAt: r.FulfilledAt,
	}
}

func requestFromDomain(r domainvrf.Request) Request {
	return Request{
		ID:          r.ID,
		AccountID:   r.AccountID,
		KeyID:       r.KeyID,
		Consumer:    r.Consumer,
		Seed:        r.Seed,
		Status:      RequestStatus(r.Status),
		Result:      r.Result,
		Error:       r.Error,
		Metadata:    r.Metadata,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		FulfilledAt: r.FulfilledAt,
	}
}
