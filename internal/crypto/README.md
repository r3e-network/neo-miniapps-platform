# Crypto Module

The `crypto` module provides cryptographic utilities for the Service Layer.

## Overview

This module provides secure cryptographic operations including:

- Hashing (SHA-256, RIPEMD-160)
- Key derivation (HKDF)
- Encryption/Decryption (AES-GCM)
- Digital signatures (ECDSA P-256)
- Neo N3 address utilities

## Functions

### Hashing

```go
// SHA-256 hash
hash := crypto.Hash256(data)

// RIPEMD-160 hash (for Neo addresses)
hash160 := crypto.Hash160(data)
```

### Key Derivation

```go
// Derive a key using HKDF
derivedKey := crypto.DeriveKey(masterKey, salt, info, keyLength)
```

### Encryption

```go
// AES-GCM encryption
ciphertext, err := crypto.Encrypt(key, plaintext)

// AES-GCM decryption
plaintext, err := crypto.Decrypt(key, ciphertext)
```

### Random Generation

```go
// Generate cryptographically secure random bytes
randomBytes, err := crypto.GenerateRandomBytes(32)
```

### Neo N3 Utilities

```go
// Convert public key to Neo script hash
scriptHash := crypto.PublicKeyToScriptHash(publicKey)

// Convert script hash to Neo address
address := crypto.ScriptHashToAddress(scriptHash)
```

### Memory Safety

```go
// Securely zero memory (for sensitive data)
crypto.ZeroBytes(sensitiveData)
```

## Security Considerations

1. **Key Material**: All key material should be zeroed after use
2. **Random Generation**: Uses `crypto/rand` for secure randomness
3. **Encryption**: AES-256-GCM with random nonces
4. **Key Derivation**: HKDF with SHA-256

## Usage Example

```go
package main

import (
    "github.com/R3E-Network/service_layer/internal/crypto"
)

func main() {
    // Generate a random key
    key, _ := crypto.GenerateRandomBytes(32)
    defer crypto.ZeroBytes(key)

    // Encrypt sensitive data
    plaintext := []byte("sensitive data")
    ciphertext, _ := crypto.Encrypt(key, plaintext)

    // Decrypt
    decrypted, _ := crypto.Decrypt(key, ciphertext)

    // Hash data
    hash := crypto.Hash256(decrypted)
}
```

## Testing

```bash
go test ./internal/crypto/... -v
```

Current test coverage: **71.9%**
