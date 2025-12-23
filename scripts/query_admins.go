//go:build ignore

// Script to query contract admin addresses

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const rpcURL = "https://testnet1.neo.coz.io:443"

var contracts = map[string]string{
	"PriceFeed":        "0xc5d9117d255054489d1cf59b2c1d188c01bc9954",
	"RandomnessLog":    "0x76dfee17f2f4b9fa8f32bd3f4da6406319ab7b39",
	"PaymentHub":       "0x45777109546ceaacfbeed9336d695bb8b8bd77ca",
	"AutomationAnchor": "0x1c888d699ce76b0824028af310d90c3c18adeab5",
}

func main() {
	wif := os.Getenv("NEO_TESTNET_WIF")
	if wif != "" {
		privateKey, _ := keys.NewPrivateKeyFromWIF(wif)
		deployerHash := privateKey.GetScriptHash()
		fmt.Printf("Expected Deployer: %s\n", address.Uint160ToString(deployerHash))
		fmt.Printf("Deployer Hash (LE): %s\n", deployerHash.StringLE())
	}

	ctx := context.Background()
	client, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		fmt.Printf("Failed to create RPC client: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Querying Contract Admins ===")

	for name, hashStr := range contracts {
		fmt.Printf("\n--- %s ---\n", name)
		contractHash, _ := parseContractHash(hashStr)

		// Call admin() method
		result, err := client.InvokeFunction(contractHash, "admin", nil, nil)
		if err != nil {
			fmt.Printf("Error calling admin(): %v\n", err)
			continue
		}

		fmt.Printf("State: %s\n", result.State)
		if result.FaultException != "" {
			fmt.Printf("Fault: %s\n", result.FaultException)
			continue
		}

		if len(result.Stack) > 0 {
			item := result.Stack[0]
			fmt.Printf("Type: %s\n", item.Type())

			// Try to decode as UInt160
			decoded, err := item.TryBytes()
			if err == nil && len(decoded) == 20 {
				adminHash, _ := util.Uint160DecodeBytesBE(decoded)
				fmt.Printf("Admin Address: %s\n", address.Uint160ToString(adminHash))
				fmt.Printf("Admin Hash (LE): %s\n", adminHash.StringLE())
			} else {
				fmt.Printf("Raw value: %v\n", item.Value())
			}
		}
	}
}

func parseContractHash(hashStr string) (util.Uint160, error) {
	hashStr = strings.TrimPrefix(hashStr, "0x")
	return util.Uint160DecodeStringLE(hashStr)
}
