//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/chain"
)

func main() {
	// Load contract addresses from env
	os.Setenv("CONTRACT_PRICEFEED_HASH", "0x7507972a4c97ccaffe7af5d1179081492882b1d6")
	os.Setenv("CONTRACT_RANDOMNESSLOG_HASH", "0x0536c9ee25a6a9cbbae6d824c32ba1ec259d9810")
	os.Setenv("CONTRACT_AUTOMATIONANCHOR_HASH", "0x85c4697f9a2b0b1fe7325ffb73b362e198b16bf7")
	os.Setenv("CONTRACT_PAYMENTHUB_HASH", "0x94955e10072c701aa17e85283b0a799f0eb9ff23")

	addresses := chain.ContractAddressesFromEnv()
	fmt.Println("=== Contract Addresses Loaded ===")
	data, _ := json.MarshalIndent(addresses, "", "  ")
	fmt.Println(string(data))

	// Create chain client
	client, err := chain.NewClient(chain.Config{
		RPCURL:    "https://testnet1.neo.coz.io:443",
		NetworkID: 894710606,
		Timeout:   30 * time.Second,
	})
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	ctx := context.Background()

	// Test DataFeeds (legacy PriceFeed) contract with correct method name
	fmt.Println("\n=== Testing DataFeeds Contract (getLatestPrice) ===")
	result, err := client.Call(ctx, "invokefunction", []interface{}{
		addresses.PriceFeed,
		"getLatestPrice", // Legacy method name
		[]interface{}{
			map[string]interface{}{"type": "String", "value": "BTCUSD"},
		},
	})
	if err != nil {
		fmt.Printf("DataFeeds.getLatestPrice error: %v\n", err)
	} else {
		fmt.Printf("DataFeeds.getLatestPrice result: %s\n", truncate(string(result), 300))
	}

	// Test VRF (legacy RandomnessLog) contract - getVRFPublicKey
	fmt.Println("\n=== Testing VRF Contract (getVRFPublicKey) ===")
	result, err = client.Call(ctx, "invokefunction", []interface{}{
		addresses.RandomnessLog,
		"getVRFPublicKey", // Legacy method name
		[]interface{}{},
	})
	if err != nil {
		fmt.Printf("VRF.getVRFPublicKey error: %v\n", err)
	} else {
		fmt.Printf("VRF.getVRFPublicKey result: %s\n", truncate(string(result), 300))
	}

	// Test Automation (legacy AutomationAnchor) contract - admin
	fmt.Println("\n=== Testing Automation Contract (admin) ===")
	result, err = client.Call(ctx, "invokefunction", []interface{}{
		addresses.AutomationAnchor,
		"admin", // Legacy method name
		[]interface{}{},
	})
	if err != nil {
		fmt.Printf("Automation.admin error: %v\n", err)
	} else {
		fmt.Printf("Automation.admin result: %s\n", truncate(string(result), 300))
	}

	// Test Gateway (legacy PaymentHub) contract - admin
	fmt.Println("\n=== Testing Gateway Contract (admin) ===")
	result, err = client.Call(ctx, "invokefunction", []interface{}{
		addresses.PaymentHub,
		"admin", // Legacy method name
		[]interface{}{},
	})
	if err != nil {
		fmt.Printf("Gateway.admin error: %v\n", err)
	} else {
		fmt.Printf("Gateway.admin result: %s\n", truncate(string(result), 300))
	}

	fmt.Println("\n✅ Contract invocation test completed!")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
