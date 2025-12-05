package coordinator

import (
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/tee/types"
)

// PackageManager verifies enclave packages against manifest definitions.
type PackageManager struct {
	mu       sync.RWMutex
	packages map[string]*os.Package
}

// NewPackageManager creates a new PackageManager.
func NewPackageManager() *PackageManager {
	return &PackageManager{
		packages: make(map[string]*os.Package),
	}
}

// SetPackages sets the package definitions from the manifest.
func (pm *PackageManager) SetPackages(packages map[string]os.Package) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.packages = make(map[string]*os.Package, len(packages))
	for name, pkg := range packages {
		p := pkg // Copy to avoid reference issues
		pm.packages[name] = &p
	}
}

// GetPackage retrieves a package definition.
func (pm *PackageManager) GetPackage(name string) (*os.Package, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pkg, ok := pm.packages[name]
	return pkg, ok
}

// VerifyQuote verifies an attestation quote against a package definition.
func (pm *PackageManager) VerifyQuote(quote *types.Quote, packageName string) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pkg, ok := pm.packages[packageName]
	if !ok {
		return fmt.Errorf("package not found: %s", packageName)
	}

	// Debug mode allows any quote
	if pkg.Debug {
		return nil
	}

	// Verify MRENCLAVE (UniqueID)
	if pkg.UniqueID != "" {
		if quote.MREnclave != pkg.UniqueID {
			return &QuoteVerificationError{
				Field:    "MRENCLAVE",
				Expected: pkg.UniqueID,
				Actual:   quote.MREnclave,
			}
		}
	}

	// Verify MRSIGNER (SignerID)
	if pkg.SignerID != "" {
		if quote.MRSigner != pkg.SignerID {
			return &QuoteVerificationError{
				Field:    "MRSIGNER",
				Expected: pkg.SignerID,
				Actual:   quote.MRSigner,
			}
		}
	}

	// Verify ProductID
	if pkg.ProductID != 0 {
		if quote.ProductID != pkg.ProductID {
			return &QuoteVerificationError{
				Field:    "ProductID",
				Expected: fmt.Sprintf("%d", pkg.ProductID),
				Actual:   fmt.Sprintf("%d", quote.ProductID),
			}
		}
	}

	// Verify SecurityVersion (SVN)
	if pkg.SecurityVersion != 0 {
		if quote.SecurityVersion < pkg.SecurityVersion {
			return &QuoteVerificationError{
				Field:    "SecurityVersion",
				Expected: fmt.Sprintf(">= %d", pkg.SecurityVersion),
				Actual:   fmt.Sprintf("%d", quote.SecurityVersion),
			}
		}
	}

	// Verify TCB status if specified
	if len(pkg.AcceptedTCBStatuses) > 0 {
		if !contains(pkg.AcceptedTCBStatuses, quote.TCBStatus) {
			return &QuoteVerificationError{
				Field:    "TCBStatus",
				Expected: fmt.Sprintf("one of %v", pkg.AcceptedTCBStatuses),
				Actual:   quote.TCBStatus,
			}
		}
	}

	return nil
}

// QuoteVerificationError indicates a quote verification failure.
type QuoteVerificationError struct {
	Field    string
	Expected string
	Actual   string
}

func (e *QuoteVerificationError) Error() string {
	return fmt.Sprintf("quote verification failed: %s mismatch (expected %s, got %s)",
		e.Field, e.Expected, e.Actual)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
