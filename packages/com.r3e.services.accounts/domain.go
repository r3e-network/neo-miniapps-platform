package accounts

import (
	"fmt"
	"strings"
	"time"
)

// Account represents a logical tenant or owner of resources within the service
// layer.
type Account struct {
	ID        string
	Owner     string
	Metadata  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WorkspaceWallet captures a wallet registered to a workspace/account.
type WorkspaceWallet struct {
	ID            string
	WorkspaceID   string
	WalletAddress string
	Label         string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

const (
	walletHexLength    = 40
	walletStringLength = walletHexLength + 2 // 0x + 40 hex chars
)

// NormalizeWalletAddress trims whitespace and lowercases a wallet address.
func NormalizeWalletAddress(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}

// ValidateWalletAddress enforces a non-empty 0x-prefixed hex wallet address.
func ValidateWalletAddress(addr string) error {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return fmt.Errorf("wallet_address is required")
	}
	if !strings.HasPrefix(trimmed, "0x") {
		return fmt.Errorf("wallet_address must start with 0x")
	}
	if len(trimmed) != walletStringLength {
		return fmt.Errorf("wallet_address must be %d hex characters", walletHexLength)
	}
	for _, ch := range trimmed[2:] {
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
			continue
		}
		return fmt.Errorf("wallet_address must be hexadecimal")
	}
	return nil
}

// IsValidWalletAddress reports whether the provided address meets validation rules.
func IsValidWalletAddress(addr string) bool {
	return ValidateWalletAddress(addr) == nil
}
