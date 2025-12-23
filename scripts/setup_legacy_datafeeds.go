//go:build ignore

// Script to register TEE account and update prices in legacy DataFeeds contract

package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

const rpcURL = "https://testnet1.neo.coz.io:443"

// Legacy contract addresses
var legacyContracts = map[string]string{
	"DataFeeds": "0x7507972a4c97ccaffe7af5d1179081492882b1d6",
	"VRF":       "0x0536c9ee25a6a9cbbae6d824c32ba1ec259d9810",
	"Gateway":   "0x94955e10072c701aa17e85283b0a799f0eb9ff23",
}

// Price feeds to update
var priceFeeds = map[string]int64{
	"BTC/USD": 10500000000000, // $105,000 (8 decimals)
	"ETH/USD": 390000000000,   // $3,900
	"NEO/USD": 1500000000,     // $15
	"GAS/USD": 700000000,      // $7
}

func main() {
	wif := os.Getenv("NEO_TESTNET_WIF")
	if wif == "" {
		fmt.Println("NEO_TESTNET_WIF environment variable not set")
		os.Exit(1)
	}

	privateKey, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		fmt.Printf("Invalid WIF: %v\n", err)
		os.Exit(1)
	}

	deployerHash := privateKey.GetScriptHash()
	deployerAddr := address.Uint160ToString(deployerHash)
	pubKey := privateKey.PublicKey()

	fmt.Printf("Deployer: %s\n", deployerAddr)
	fmt.Printf("Public Key: %s\n", pubKey.StringCompressed())

	ctx := context.Background()
	client, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		fmt.Printf("Failed to create RPC client: %v\n", err)
		os.Exit(1)
	}

	acc := wallet.NewAccountFromPrivateKey(privateKey)
	acc.Label = "deployer"

	act, err := actor.NewSimple(client, acc)
	if err != nil {
		fmt.Printf("Failed to create actor: %v\n", err)
		os.Exit(1)
	}

	dataFeedsHash, _ := parseContractHash(legacyContracts["DataFeeds"])

	// Step 1: Check if already registered as TEE account
	fmt.Println("\n=== Checking TEE Registration ===")
	result, err := act.Call(dataFeedsHash, "isTEEAccount", deployerHash)
	if err != nil {
		fmt.Printf("Failed to check TEE account: %v\n", err)
	} else if result.State == "HALT" && len(result.Stack) > 0 {
		if b, ok := result.Stack[0].Value().(bool); ok && b {
			fmt.Println("✅ Already registered as TEE account")
		} else {
			fmt.Println("❌ Not registered as TEE account, registering...")
			if err := registerTEEAccount(ctx, client, act, dataFeedsHash, deployerHash, pubKey); err != nil {
				fmt.Printf("Failed to register TEE account: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✅ TEE account registered")
		}
	}

	// Step 2: Register feeds if not already registered
	fmt.Println("\n=== Checking Feed Registration ===")
	for feedId := range priceFeeds {
		result, err := act.Call(dataFeedsHash, "getFeedConfig", feedId)
		if err != nil || result.State != "HALT" {
			fmt.Printf("Feed %s not found, registering...\n", feedId)
			if err := registerFeed(ctx, client, act, dataFeedsHash, feedId); err != nil {
				fmt.Printf("Failed to register feed %s: %v\n", feedId, err)
			} else {
				fmt.Printf("✅ Feed %s registered\n", feedId)
			}
			time.Sleep(2 * time.Second)
		} else {
			fmt.Printf("✅ Feed %s already registered\n", feedId)
		}
	}

	// Step 3: Update prices
	fmt.Println("\n=== Updating Prices ===")
	nonce := time.Now().UnixNano()
	for feedId, basePrice := range priceFeeds {
		price := generatePrice(basePrice, 2) // 2% variance
		timestamp := time.Now().UnixMilli()  // Neo N3 uses milliseconds

		// Create signature
		signature := signPriceUpdate(privateKey, feedId, price, timestamp, nonce)

		fmt.Printf("Updating %s: price=%d, timestamp=%d\n", feedId, price, timestamp)

		txHash, _, err := act.SendCall(
			dataFeedsHash,
			"updatePrice",
			feedId,
			price,
			timestamp,
			nonce,
			signature,
		)
		if err != nil {
			fmt.Printf("❌ Failed to update %s: %v\n", feedId, err)
		} else {
			fmt.Printf("✅ %s updated, tx=%s\n", feedId, txHash.StringLE()[:16]+"...")
		}

		nonce++
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n=== Done ===")
}

func parseContractHash(hashStr string) (util.Uint160, error) {
	hashStr = strings.TrimPrefix(hashStr, "0x")
	return util.Uint160DecodeStringLE(hashStr)
}

func registerTEEAccount(ctx context.Context, client *rpcclient.Client, act *actor.Actor, contractHash util.Uint160, teeAccount util.Uint160, pubKey *keys.PublicKey) error {
	txHash, vub, err := act.SendCall(contractHash, "registerTEEAccount", teeAccount, pubKey.Bytes())
	if err != nil {
		return fmt.Errorf("send transaction: %w", err)
	}

	fmt.Printf("Transaction sent: %s (valid until block %d)\n", txHash.StringLE(), vub)
	return waitForTx(ctx, client, txHash)
}

func registerFeed(ctx context.Context, client *rpcclient.Client, act *actor.Actor, contractHash util.Uint160, feedId string) error {
	description := feedId + " Price Feed"
	decimals := int64(8)

	txHash, vub, err := act.SendCall(contractHash, "registerFeed", feedId, description, decimals)
	if err != nil {
		return fmt.Errorf("send transaction: %w", err)
	}

	fmt.Printf("Transaction sent: %s (valid until block %d)\n", txHash.StringLE(), vub)
	return waitForTx(ctx, client, txHash)
}

func signPriceUpdate(privateKey *keys.PrivateKey, feedId string, price int64, timestamp int64, nonce int64) []byte {
	// Create message to sign: feedId + price + timestamp + nonce
	msg := []byte(feedId)

	priceBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(priceBuf, uint64(price))
	msg = append(msg, priceBuf...)

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(timestamp))
	msg = append(msg, timestampBuf...)

	nonceBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonceBuf, uint64(nonce))
	msg = append(msg, nonceBuf...)

	// Hash the message
	hash := sha256.Sum256(msg)

	// Sign with ECDSA
	signature := privateKey.Sign(hash[:])
	return signature
}

func waitForTx(ctx context.Context, client *rpcclient.Client, txHash util.Uint256) error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	timeout := time.After(2 * time.Minute)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for transaction")
		case <-ticker.C:
			appLog, err := client.GetApplicationLog(txHash, nil)
			if err != nil {
				continue
			}

			if len(appLog.Executions) == 0 {
				continue
			}

			exec := appLog.Executions[0]
			if exec.VMState.HasFlag(1) {
				return nil
			}
			return fmt.Errorf("transaction failed: %s", exec.FaultException)
		}
	}
}

func generatePrice(basePrice int64, variancePercent int) int64 {
	variance := basePrice * int64(variancePercent) / 100
	// Simple random variance
	n := big.NewInt(variance * 2)
	return basePrice - variance + n.Int64()/2
}
