# MarbleRun-Based Refactoring Plan

## Executive Summary

This document outlines the refactoring plan to align the Neo Service Layer with MarbleRun's proven confidential microservices architecture patterns.

## Architecture Comparison

### MarbleRun Architecture
```
┌─────────────────────────────────────────────────────────────┐
│                      Coordinator                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  Manifest   │  │   Secrets   │  │  Recovery Manager   │  │
│  │  Validator  │  │   Manager   │  │  (Multi-party RSA)  │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Package   │  │    User     │  │    TLS Manager      │  │
│  │  Verifier   │  │   Manager   │  │  (mTLS, Policies)   │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
              ┌─────▼─────┐       ┌─────▼─────┐
              │  Marble   │       │  Marble   │
              │ (Service) │       │ (Service) │
              │           │       │           │
              │ UniqueID  │       │ UniqueID  │
              │ SignerID  │       │ SignerID  │
              │ ProductID │       │ ProductID │
              └───────────┘       └───────────┘
```

### Current Architecture
```
┌─────────────────────────────────────────────────────────────┐
│                      Foundation                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  TrustRoot  │  │   Supabase  │  │    ServiceOS        │  │
│  │  (TEE)      │  │   Client    │  │    Factory          │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
              ┌─────▼─────┐       ┌─────▼─────┐
              │  Service  │       │  Service  │
              │ (Oracle)  │       │  (VRF)    │
              │           │       │           │
              │ Manifest  │       │ Manifest  │
              │ Caps Only │       │ Caps Only │
              └───────────┘       └───────────┘
```

## Key Differences & Gaps

| Feature | MarbleRun | Current | Gap |
|---------|-----------|---------|-----|
| **Manifest** | 8 sections (Packages, Marbles, Secrets, Users, Roles, RecoveryKeys, TLS, Config) | Simple (ServiceID, Version, Capabilities) | Major |
| **Package Verification** | MRENCLAVE, MRSIGNER, ProductID, SecurityVersion | EnclaveID only | Major |
| **Secrets** | Types (symmetric-key, cert-*, plain), Shared/UserDefined | Simple key-value | Medium |
| **Users & Roles** | Full RBAC with certificate-based auth | Basic capability system | Medium |
| **Recovery** | Multi-party RSA key recovery | None | Major |
| **TLS** | Transparent mTLS with policies | Manual cert pinning | Medium |
| **Sealing** | ProductKey, UniqueKey, Disabled modes | Single mode | Minor |

## Refactoring Plan

### Phase 1: Enhanced Manifest Structure

**Goal**: Align Manifest with MarbleRun's comprehensive configuration model.

**New Manifest Structure**:
```go
// platform/os/manifest.go

// Manifest is the complete service configuration (MarbleRun-style)
type Manifest struct {
    // Packages defines enclave software packages
    Packages map[string]Package `json:"Packages"`

    // Marbles defines service instances
    Marbles map[string]Marble `json:"Marbles"`

    // Secrets defines cryptographic secrets
    Secrets map[string]Secret `json:"Secrets"`

    // Users defines authorized users
    Users map[string]User `json:"Users"`

    // Roles defines permission roles
    Roles map[string]Role `json:"Roles"`

    // RecoveryKeys for multi-party recovery
    RecoveryKeys map[string]string `json:"RecoveryKeys,omitempty"`

    // TLS defines TLS policies
    TLS TLSConfig `json:"TLS,omitempty"`

    // Config holds service-specific configuration
    Config map[string]ConfigEntry `json:"Config,omitempty"`
}

// Package defines an enclave software package
type Package struct {
    // UniqueID is MRENCLAVE - hash of enclave code
    UniqueID string `json:"UniqueID,omitempty"`

    // SignerID is MRSIGNER - hash of signing key
    SignerID string `json:"SignerID,omitempty"`

    // ProductID identifies the product
    ProductID uint16 `json:"ProductID,omitempty"`

    // SecurityVersion is the minimum security version
    SecurityVersion uint16 `json:"SecurityVersion,omitempty"`

    // Debug allows debug mode (development only)
    Debug bool `json:"Debug,omitempty"`
}

// Marble defines a service instance
type Marble struct {
    // Package references a Package definition
    Package string `json:"Package"`

    // Parameters are environment variables
    Parameters MarbleParameters `json:"Parameters,omitempty"`

    // TLS defines TLS configuration for this marble
    TLS MarbleTLS `json:"TLS,omitempty"`

    // MaxActivations limits concurrent instances
    MaxActivations uint `json:"MaxActivations,omitempty"`
}

// Secret defines a cryptographic secret
type Secret struct {
    // Type: symmetric-key, cert-rsa, cert-ecdsa, cert-ed25519, plain
    Type SecretType `json:"Type"`

    // Size in bits (for symmetric-key)
    Size uint `json:"Size,omitempty"`

    // Shared indicates if secret is shared across marbles
    Shared bool `json:"Shared,omitempty"`

    // UserDefined indicates secret is provided by user
    UserDefined bool `json:"UserDefined,omitempty"`

    // Cert for certificate secrets
    Cert *CertConfig `json:"Cert,omitempty"`

    // Private for private key secrets
    Private []byte `json:"Private,omitempty"`

    // Public for public key secrets
    Public []byte `json:"Public,omitempty"`
}

// User defines an authorized user
type User struct {
    // Certificate is the user's X.509 certificate (PEM)
    Certificate string `json:"Certificate"`

    // Roles assigned to this user
    Roles []string `json:"Roles"`
}

// Role defines a permission role
type Role struct {
    // ResourceType: Packages, Marbles, Secrets, etc.
    ResourceType string `json:"ResourceType"`

    // ResourceNames are specific resources (empty = all)
    ResourceNames []string `json:"ResourceNames,omitempty"`

    // Actions: ReadSecret, WriteSecret, SignQuote, etc.
    Actions []string `json:"Actions"`
}
```

**Files to Create/Modify**:
- `platform/os/manifest.go` - New comprehensive manifest types
- `platform/os/manifest_validator.go` - Manifest validation logic
- `platform/os/manifest_loader.go` - Load from JSON/YAML

### Phase 2: Coordinator Pattern

**Goal**: Refactor Foundation to act as a Coordinator that verifies and bootstraps services.

**New Coordinator Structure**:
```go
// platform/coordinator/coordinator.go

// Coordinator manages the confidential microservices mesh
type Coordinator struct {
    mu sync.RWMutex

    // Core components
    trustRoot    *tee.TrustRoot
    manifest     *Manifest
    state        *CoordinatorState

    // Managers
    secretMgr    *SecretManager
    recoveryMgr  *RecoveryManager
    tlsMgr       *TLSManager
    packageMgr   *PackageManager
    userMgr      *UserManager

    // Service registry
    marbles      map[string]*MarbleInstance
}

// CoordinatorState tracks coordinator lifecycle
type CoordinatorState int

const (
    StateUninitialized CoordinatorState = iota
    StateRecovery      // Waiting for recovery keys
    StateAcceptingManifest
    StateAcceptingMarbles
    StateMax
)

// Activate activates a marble (service) after verification
func (c *Coordinator) Activate(ctx context.Context, req *ActivationRequest) (*ActivationResponse, error) {
    // 1. Verify quote (MRENCLAVE, MRSIGNER)
    // 2. Check package definition in manifest
    // 3. Generate/retrieve secrets for this marble
    // 4. Issue TLS certificates
    // 5. Return activation response with secrets
}
```

**Files to Create**:
- `platform/coordinator/coordinator.go` - Main coordinator
- `platform/coordinator/state.go` - State machine
- `platform/coordinator/activation.go` - Marble activation
- `platform/coordinator/secret_manager.go` - Secret generation/distribution
- `platform/coordinator/recovery_manager.go` - Multi-party recovery
- `platform/coordinator/tls_manager.go` - TLS certificate management
- `platform/coordinator/package_manager.go` - Package verification
- `platform/coordinator/user_manager.go` - User/role management

### Phase 3: Package Verification

**Goal**: Implement MRENCLAVE/MRSIGNER verification for services.

**Package Verifier**:
```go
// platform/coordinator/package_manager.go

// PackageManager verifies enclave packages
type PackageManager struct {
    packages map[string]*Package
}

// VerifyQuote verifies a marble's attestation quote against package definition
func (pm *PackageManager) VerifyQuote(quote *Quote, packageName string) error {
    pkg, ok := pm.packages[packageName]
    if !ok {
        return ErrPackageNotFound
    }

    // Verify MRENCLAVE (UniqueID)
    if pkg.UniqueID != "" && quote.MREnclave != pkg.UniqueID {
        return ErrMREnclaveMismatch
    }

    // Verify MRSIGNER (SignerID)
    if pkg.SignerID != "" && quote.MRSigner != pkg.SignerID {
        return ErrMRSignerMismatch
    }

    // Verify ProductID and SecurityVersion
    if pkg.ProductID != 0 && quote.ProductID != pkg.ProductID {
        return ErrProductIDMismatch
    }

    if pkg.SecurityVersion != 0 && quote.SecurityVersion < pkg.SecurityVersion {
        return ErrSecurityVersionTooLow
    }

    return nil
}
```

**Update Quote Structure**:
```go
// platform/os/types.go - Update Quote

type Quote struct {
    RawQuote        []byte    `json:"raw_quote"`
    UserData        []byte    `json:"user_data"`
    MREnclave       string    `json:"mr_enclave"`
    MRSigner        string    `json:"mr_signer"`
    ProductID       uint16    `json:"product_id"`       // NEW
    SecurityVersion uint16    `json:"security_version"` // NEW
    Timestamp       time.Time `json:"timestamp"`
}
```

### Phase 4: Multi-Party Recovery System

**Goal**: Implement MarbleRun-style recovery with RSA key shares.

**Recovery Manager**:
```go
// platform/coordinator/recovery_manager.go

// RecoveryManager handles multi-party key recovery
type RecoveryManager struct {
    mu sync.Mutex

    // Recovery configuration
    threshold    int                    // Minimum keys needed
    recoveryKeys map[string]*rsa.PublicKey // User -> RSA public key

    // Recovery state
    encryptedState []byte
    keyShares      map[string][]byte // Encrypted shares per user
}

// SetRecoveryKeys configures recovery keys from manifest
func (rm *RecoveryManager) SetRecoveryKeys(keys map[string]string) error {
    rm.mu.Lock()
    defer rm.mu.Unlock()

    rm.recoveryKeys = make(map[string]*rsa.PublicKey, len(keys))
    for name, pemKey := range keys {
        pubKey, err := parseRSAPublicKey(pemKey)
        if err != nil {
            return fmt.Errorf("parse recovery key %s: %w", name, err)
        }
        rm.recoveryKeys[name] = pubKey
    }

    return nil
}

// GenerateRecoveryData creates encrypted key shares for each recovery key
func (rm *RecoveryManager) GenerateRecoveryData(sealingKey []byte) (map[string][]byte, error) {
    rm.mu.Lock()
    defer rm.mu.Unlock()

    shares := make(map[string][]byte, len(rm.recoveryKeys))
    for name, pubKey := range rm.recoveryKeys {
        // Encrypt sealing key with user's RSA public key
        encrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, sealingKey, nil)
        if err != nil {
            return nil, fmt.Errorf("encrypt for %s: %w", name, err)
        }
        shares[name] = encrypted
    }

    return shares, nil
}

// RecoverState recovers coordinator state using provided key shares
func (rm *RecoveryManager) RecoverState(shares map[string][]byte) ([]byte, error) {
    // Decrypt shares and reconstruct sealing key
    // Unseal coordinator state
}
```

### Phase 5: Transparent TLS Configuration

**Goal**: Implement MarbleRun-style TLS policies for inter-service communication.

**TLS Manager**:
```go
// platform/coordinator/tls_manager.go

// TLSManager handles TLS certificate generation and policies
type TLSManager struct {
    rootCA      *x509.Certificate
    rootKey     *ecdsa.PrivateKey

    // TLS policies from manifest
    incoming    map[string][]string // Marble -> allowed client marbles
    outgoing    map[string][]string // Marble -> allowed server marbles
}

// TLSConfig from manifest
type TLSConfig struct {
    // Incoming defines which marbles can connect to which
    Incoming map[string]TLSPolicy `json:"Incoming,omitempty"`

    // Outgoing defines which marbles can be connected to
    Outgoing map[string]TLSPolicy `json:"Outgoing,omitempty"`
}

type TLSPolicy struct {
    // Port to apply policy to
    Port string `json:"Port"`

    // Cert is the certificate secret name
    Cert string `json:"Cert"`

    // AllowedMarbles that can connect/be connected to
    AllowedMarbles []string `json:"AllowedMarbles,omitempty"`

    // DisableClientAuth disables mTLS
    DisableClientAuth bool `json:"DisableClientAuth,omitempty"`
}

// IssueCertificate issues a TLS certificate for a marble
func (tm *TLSManager) IssueCertificate(marbleName string, publicKey any) (*x509.Certificate, error) {
    template := &x509.Certificate{
        SerialNumber: big.NewInt(time.Now().UnixNano()),
        Subject: pkix.Name{
            CommonName:   marbleName,
            Organization: []string{"Neo Service Layer"},
        },
        DNSNames:    []string{marbleName},
        NotBefore:   time.Now(),
        NotAfter:    time.Now().Add(365 * 24 * time.Hour),
        KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
        ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
    }

    certDER, err := x509.CreateCertificate(rand.Reader, template, tm.rootCA, publicKey, tm.rootKey)
    if err != nil {
        return nil, err
    }

    return x509.ParseCertificate(certDER)
}
```

### Phase 6: Enhanced Secret Types

**Goal**: Support MarbleRun's secret types (symmetric-key, cert-*, plain).

**Secret Types**:
```go
// platform/coordinator/secret_manager.go

type SecretType string

const (
    SecretTypeSymmetricKey SecretType = "symmetric-key"
    SecretTypeCertRSA      SecretType = "cert-rsa"
    SecretTypeCertECDSA    SecretType = "cert-ecdsa"
    SecretTypeCertED25519  SecretType = "cert-ed25519"
    SecretTypePlain        SecretType = "plain"
)

// SecretManager generates and distributes secrets
type SecretManager struct {
    mu sync.RWMutex

    // Secret definitions from manifest
    definitions map[string]*Secret

    // Generated secrets (sealed)
    generated map[string]*GeneratedSecret

    // User-defined secrets
    userDefined map[string][]byte
}

// GeneratedSecret holds a generated secret
type GeneratedSecret struct {
    Type       SecretType
    Private    []byte // Private key (sealed)
    Public     []byte // Public key/cert
    Cert       []byte // Certificate (PEM)
    SharedWith []string // Marbles that share this secret
}

// GenerateSecret generates a secret based on definition
func (sm *SecretManager) GenerateSecret(name string, def *Secret) (*GeneratedSecret, error) {
    switch def.Type {
    case SecretTypeSymmetricKey:
        return sm.generateSymmetricKey(name, def.Size)
    case SecretTypeCertRSA:
        return sm.generateCertRSA(name, def.Cert)
    case SecretTypeCertECDSA:
        return sm.generateCertECDSA(name, def.Cert)
    case SecretTypeCertED25519:
        return sm.generateCertED25519(name, def.Cert)
    case SecretTypePlain:
        if def.UserDefined {
            return nil, ErrUserDefinedSecretRequired
        }
        return nil, ErrPlainSecretNotGenerated
    default:
        return nil, ErrUnknownSecretType
    }
}
```

## Implementation Order

### Week 1-2: Manifest & Package Verification
1. Create new manifest types (`platform/os/manifest.go`)
2. Implement manifest validator
3. Update Quote with ProductID/SecurityVersion
4. Implement PackageManager

### Week 3-4: Coordinator Core
1. Create Coordinator structure
2. Implement state machine
3. Implement marble activation flow
4. Migrate Foundation to use Coordinator

### Week 5-6: Secret Management
1. Implement SecretManager with all types
2. Add secret generation for each type
3. Implement secret distribution to marbles
4. Update services to receive secrets via activation

### Week 7-8: Recovery & TLS
1. Implement RecoveryManager
2. Add multi-party key recovery
3. Implement TLSManager
4. Add transparent mTLS between services

### Week 9-10: Integration & Testing
1. Update all services to new activation flow
2. Create comprehensive test suite
3. Update documentation
4. Performance testing

## Migration Strategy

### Backward Compatibility
- Keep existing `os.Manifest` as `LegacyManifest`
- Auto-convert legacy manifests to new format
- Deprecation warnings for old format

### Service Updates
Each service needs:
1. Package definition with MRENCLAVE/MRSIGNER
2. Marble definition with parameters
3. Secret definitions for required secrets
4. TLS configuration for inter-service communication

### Example Migration

**Before (Current)**:
```go
manifest := &os.Manifest{
    ServiceID: "oracle",
    Version: "1.0.0",
    RequiredCapabilities: []os.Capability{
        os.CapSecrets,
        os.CapNetwork,
    },
}
```

**After (MarbleRun-style)**:
```json
{
  "Packages": {
    "oracle-pkg": {
      "SignerID": "abc123...",
      "ProductID": 1,
      "SecurityVersion": 1
    }
  },
  "Marbles": {
    "oracle": {
      "Package": "oracle-pkg",
      "Parameters": {
        "Env": {
          "SERVICE_ID": "oracle",
          "VERSION": "1.0.0"
        },
        "Files": {
          "/config/oracle.json": "{{ raw .Secrets.oracle_config }}"
        }
      },
      "TLS": {
        "Incoming": {
          "Port": "8443",
          "Cert": "oracle-cert"
        }
      }
    }
  },
  "Secrets": {
    "oracle-cert": {
      "Type": "cert-ecdsa",
      "Cert": {
        "Subject": {"CommonName": "oracle.service-layer.local"}
      }
    },
    "oracle_config": {
      "Type": "plain",
      "UserDefined": true
    }
  }
}
```

## Success Metrics

1. **Security**: All services verified via MRENCLAVE/MRSIGNER
2. **Recovery**: Multi-party recovery tested and documented
3. **TLS**: All inter-service communication uses mTLS
4. **Compatibility**: Existing services work with new system
5. **Performance**: < 100ms activation latency

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking changes | High | Backward compatibility layer |
| Performance regression | Medium | Benchmark before/after |
| Complex migration | Medium | Automated migration tools |
| SGX hardware requirements | Low | Simulation mode support |

## References

- [MarbleRun Documentation](https://docs.edgeless.systems/marblerun)
- [MarbleRun GitHub](https://github.com/edgelesssys/marblerun)
- [SGX Attestation](https://software.intel.com/content/www/us/en/develop/topics/software-guard-extensions/attestation-services.html)
