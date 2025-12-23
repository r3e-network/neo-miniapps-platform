//go:build ignore

// Contract simulation script that invokes actual smart contract methods:
// - PriceFeed.Update - Update price feeds (BTC, ETH, NEO, GAS)
// - RandomnessLog.Record - Record randomness values
// - PaymentHub.Pay - Make payments to MiniApps

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

const rpcURL = "https://testnet1.neo.coz.io:443"

// Contract addresses (new v2.0 platform contracts)
var contracts = map[string]string{
	"PriceFeed":     "0xc5d9117d255054489d1cf59b2c1d188c01bc9954",
	"RandomnessLog": "0x76dfee17f2f4b9fa8f32bd3f4da6406319ab7b39",
	"PaymentHub":    "0x45777109546ceaacfbeed9336d695bb8b8bd77ca",
}

// Price feed symbols and base prices (in 8 decimals)
var priceFeeds = map[string]int64{
	"BTCUSD": 10500000000000, // $105,000
	"ETHUSD": 390000000000,   // $3,900
	"NEOUSD": 1500000000,     // $15
	"GASUSD": 700000000,      // $7
}

// MiniApps
var miniApps = []string{"builtin-lottery", "builtin-coin-flip", "builtin-dice-game"}

// Statistics
var stats struct {
	priceFeedUpdates  int64
	randomnessRecords int64
	paymentHubPays    int64
	errors            int64
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

	fmt.Printf("Deployer: %s\n", address.Uint160ToString(privateKey.GetScriptHash()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		fmt.Printf("Failed to create RPC client: %v\n", err)
		os.Exit(1)
	}

	acc := wallet.NewAccountFromPrivateKey(privateKey)
	acc.Label = "simulation"

	// Actor with CalledByEntry scope for PriceFeed/RandomnessLog
	act, err := actor.NewSimple(client, acc)
	if err != nil {
		fmt.Printf("Failed to create actor: %v\n", err)
		os.Exit(1)
	}

	// Actor with Global scope for PaymentHub (needed for GAS.Transfer)
	globalSigner := actor.SignerAccount{
		Signer: transaction.Signer{
			Account: acc.ScriptHash(),
			Scopes:  transaction.Global,
		},
		Account: acc,
	}
	actGlobal, err := actor.New(client, []actor.SignerAccount{globalSigner})
	if err != nil {
		fmt.Printf("Failed to create global actor: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Starting Contract Simulation ===")
	fmt.Println("Press Ctrl+C to stop")

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Start PriceFeed updater (every 5 seconds)
	wg.Add(1)
	go func() {
		defer wg.Done()
		runPriceFeedUpdater(ctx, act)
	}()

	// Start RandomnessLog recorder (every 10 seconds)
	wg.Add(1)
	go func() {
		defer wg.Done()
		runRandomnessRecorder(ctx, act)
	}()

	// Start PaymentHub payer for each MiniApp (every 3 seconds each)
	for _, appID := range miniApps {
		wg.Add(1)
		go func(app string) {
			defer wg.Done()
			runPaymentHubPayer(ctx, actGlobal, app)
		}(appID)
	}

	// Start stats reporter
	wg.Add(1)
	go func() {
		defer wg.Done()
		runStatsReporter(ctx)
	}()

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\n\nShutting down...")
	cancel()
	wg.Wait()

	fmt.Println("\n=== Final Statistics ===")
	fmt.Printf("PriceFeed Updates:   %d\n", atomic.LoadInt64(&stats.priceFeedUpdates))
	fmt.Printf("Randomness Records:  %d\n", atomic.LoadInt64(&stats.randomnessRecords))
	fmt.Printf("PaymentHub Pays:     %d\n", atomic.LoadInt64(&stats.paymentHubPays))
	fmt.Printf("Errors:              %d\n", atomic.LoadInt64(&stats.errors))
}

func parseContractHash(hashStr string) (util.Uint160, error) {
	hashStr = strings.TrimPrefix(hashStr, "0x")
	return util.Uint160DecodeStringLE(hashStr)
}

func runPriceFeedUpdater(ctx context.Context, act *actor.Actor) {
	contractHash, _ := parseContractHash(contracts["PriceFeed"])
	roundID := int64(time.Now().Unix())

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for symbol, basePrice := range priceFeeds {
				roundID++
				price := generatePrice(basePrice, 2) // 2% variance
				timestamp := uint64(time.Now().Unix())
				attestationHash := generateRandomBytes(32)
				sourceSetID := int64(1)

				txHash, _, err := act.SendCall(
					contractHash,
					"update",
					symbol,
					roundID,
					price,
					timestamp,
					attestationHash,
					sourceSetID,
				)
				if err != nil {
					fmt.Printf("[PriceFeed] %s update failed: %v\n", symbol, err)
					atomic.AddInt64(&stats.errors, 1)
					continue
				}

				fmt.Printf("[PriceFeed] %s updated: price=%d, roundId=%d, tx=%s\n",
					symbol, price, roundID, txHash.StringLE()[:16]+"...")
				atomic.AddInt64(&stats.priceFeedUpdates, 1)

				time.Sleep(500 * time.Millisecond) // Small delay between updates
			}
		}
	}
}

func runRandomnessRecorder(ctx context.Context, act *actor.Actor) {
	contractHash, _ := parseContractHash(contracts["RandomnessLog"])

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			requestID := generateRequestID()
			randomness := generateRandomBytes(32)
			attestationHash := generateRandomBytes(32)
			timestamp := uint64(time.Now().Unix())

			txHash, _, err := act.SendCall(
				contractHash,
				"record",
				requestID,
				randomness,
				attestationHash,
				timestamp,
			)
			if err != nil {
				fmt.Printf("[RandomnessLog] record failed: %v\n", err)
				atomic.AddInt64(&stats.errors, 1)
				continue
			}

			fmt.Printf("[RandomnessLog] recorded: requestId=%s, tx=%s\n",
				requestID[:16]+"...", txHash.StringLE()[:16]+"...")
			atomic.AddInt64(&stats.randomnessRecords, 1)
		}
	}
}

func runPaymentHubPayer(ctx context.Context, act *actor.Actor, appID string) {
	contractHash, _ := parseContractHash(contracts["PaymentHub"])

	// Stagger start times
	time.Sleep(time.Duration(len(appID)%3) * time.Second)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	paymentCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			paymentCount++
			amount := int64(100000) // 0.001 GAS
			memo := fmt.Sprintf("sim-payment-%d", paymentCount)

			txHash, _, err := act.SendCall(
				contractHash,
				"pay",
				appID,
				amount,
				memo,
			)
			if err != nil {
				fmt.Printf("[PaymentHub] %s pay failed: %v\n", appID, err)
				atomic.AddInt64(&stats.errors, 1)
				continue
			}

			fmt.Printf("[PaymentHub] %s payment: amount=%d, tx=%s\n",
				appID, amount, txHash.StringLE()[:16]+"...")
			atomic.AddInt64(&stats.paymentHubPays, 1)
		}
	}
}

func runStatsReporter(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Printf("\n--- Stats @ %s ---\n", time.Now().Format("15:04:05"))
			fmt.Printf("PriceFeed: %d | Randomness: %d | Payments: %d | Errors: %d\n",
				atomic.LoadInt64(&stats.priceFeedUpdates),
				atomic.LoadInt64(&stats.randomnessRecords),
				atomic.LoadInt64(&stats.paymentHubPays),
				atomic.LoadInt64(&stats.errors))
		}
	}
}

func generatePrice(basePrice int64, variancePercent int) int64 {
	variance := basePrice * int64(variancePercent) / 100
	n, _ := rand.Int(rand.Reader, big.NewInt(variance*2))
	return basePrice - variance + n.Int64()
}

func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
