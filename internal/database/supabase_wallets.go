package database

import (
	"context"
	"encoding/json"
	"fmt"
)

// =============================================================================
// Wallet Operations
// =============================================================================

// CreateWallet creates a new wallet binding.
func (r *Repository) CreateWallet(ctx context.Context, wallet *UserWallet) error {
	data, err := r.client.request(ctx, "POST", "user_wallets", wallet, "")
	if err != nil {
		return err
	}
	var wallets []UserWallet
	if err := json.Unmarshal(data, &wallets); err != nil {
		return err
	}
	if len(wallets) > 0 {
		wallet.ID = wallets[0].ID
	}
	return nil
}

// GetUserWallets retrieves all wallets for a user.
func (r *Repository) GetUserWallets(ctx context.Context, userID string) ([]UserWallet, error) {
	query := fmt.Sprintf("user_id=eq.%s&order=is_primary.desc,created_at.asc", userID)
	data, err := r.client.request(ctx, "GET", "user_wallets", nil, query)
	if err != nil {
		return nil, err
	}

	var wallets []UserWallet
	if err := json.Unmarshal(data, &wallets); err != nil {
		return nil, err
	}
	return wallets, nil
}

// GetWalletByAddress retrieves a wallet by address.
func (r *Repository) GetWalletByAddress(ctx context.Context, address string) (*UserWallet, error) {
	query := fmt.Sprintf("address=eq.%s&limit=1", address)
	data, err := r.client.request(ctx, "GET", "user_wallets", nil, query)
	if err != nil {
		return nil, err
	}

	var wallets []UserWallet
	if err := json.Unmarshal(data, &wallets); err != nil {
		return nil, err
	}
	if len(wallets) == 0 {
		return nil, fmt.Errorf("wallet not found")
	}
	return &wallets[0], nil
}

// GetWallet retrieves a wallet by ID for a specific user.
func (r *Repository) GetWallet(ctx context.Context, walletID, userID string) (*UserWallet, error) {
	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", walletID, userID)
	resp, err := r.client.request(ctx, "GET", "user_wallets", nil, query)
	if err != nil {
		return nil, err
	}
	var wallets []UserWallet
	if err := json.Unmarshal(resp, &wallets); err != nil {
		return nil, err
	}
	if len(wallets) == 0 {
		return nil, fmt.Errorf("wallet not found")
	}
	return &wallets[0], nil
}

// SetPrimaryWallet sets a wallet as primary.
func (r *Repository) SetPrimaryWallet(ctx context.Context, userID, walletID string) error {
	// First, unset all primary wallets for this user
	update := map[string]interface{}{"is_primary": false}
	_, err := r.client.request(ctx, "PATCH", "user_wallets", update, "user_id=eq."+userID)
	if err != nil {
		return err
	}

	// Then set the specified wallet as primary
	update = map[string]interface{}{"is_primary": true}
	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", walletID, userID)
	_, err = r.client.request(ctx, "PATCH", "user_wallets", update, query)
	return err
}

// VerifyWallet marks a wallet as verified.
func (r *Repository) VerifyWallet(ctx context.Context, walletID string, signature string) error {
	update := map[string]interface{}{
		"verified":               true,
		"verification_signature": signature,
	}
	_, err := r.client.request(ctx, "PATCH", "user_wallets", update, "id=eq."+walletID)
	return err
}

// DeleteWallet deletes a wallet binding.
func (r *Repository) DeleteWallet(ctx context.Context, walletID, userID string) error {
	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s&is_primary=eq.false", walletID, userID)
	_, err := r.client.request(ctx, "DELETE", "user_wallets", nil, query)
	return err
}
