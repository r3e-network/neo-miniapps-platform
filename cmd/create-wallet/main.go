// Command create-wallet creates a Neo N3 wallet from WIF.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

func main() {
	wif := os.Getenv("NEO_TESTNET_WIF")
	if wif == "" {
		log.Fatal("NEO_TESTNET_WIF not set")
	}

	privateKey, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		log.Fatalf("Invalid WIF: %v", err)
	}

	walletPath := "deploy/testnet/wallets/testnet.json"
	if len(os.Args) > 1 {
		walletPath = os.Args[1]
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(walletPath), 0o755); mkdirErr != nil {
		log.Fatalf("Failed to create wallet directory: %v", mkdirErr)
	}

	w, err := wallet.NewWallet(walletPath)
	if err != nil {
		log.Fatalf("Failed to create wallet: %v", err)
	}

	password := "testnetpassword"
	acc, err := wallet.NewAccountFromWIF(wif)
	if err != nil {
		log.Fatalf("Failed to create account: %v", err)
	}
	acc.Label = "deployer"

	if encryptErr := acc.Encrypt(password, w.Scrypt); encryptErr != nil {
		log.Fatalf("Failed to encrypt account: %v", encryptErr)
	}

	w.AddAccount(acc)
	if saveErr := w.Save(); saveErr != nil {
		log.Fatalf("Failed to save wallet: %v", saveErr)
	}

	fmt.Println("Wallet created successfully!")
	fmt.Printf("Path: %s\n", walletPath)
	fmt.Printf("Address: %s\n", acc.Address)
	fmt.Printf("Script Hash: %s\n", acc.ScriptHash())
	fmt.Printf("Public Key: %s\n", privateKey.PublicKey().String())
	fmt.Println("\nWallet JSON:")
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal wallet: %v", err)
	}
	fmt.Println(string(data))
}
