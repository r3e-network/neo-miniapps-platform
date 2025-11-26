package account

import appaccount "github.com/R3E-Network/service_layer/internal/app/domain/account"

// Account is a workspace/tenant record. Alias to keep services independent of the app wiring package.
type Account = appaccount.Account

// WorkspaceWallet represents a registered signer wallet for a workspace.
type WorkspaceWallet = appaccount.WorkspaceWallet

// NormalizeWalletAddress trims and standardizes wallet addresses.
func NormalizeWalletAddress(addr string) string {
	return appaccount.NormalizeWalletAddress(addr)
}

// ValidateWalletAddress enforces address format constraints.
func ValidateWalletAddress(addr string) error {
	return appaccount.ValidateWalletAddress(addr)
}

// IsValidWalletAddress reports whether an address passes validation.
func IsValidWalletAddress(addr string) bool {
	return appaccount.IsValidWalletAddress(addr)
}
