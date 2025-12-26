//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
)

var contracts = []string{
	"PaymentHubV2",
	"Governance",
	"PriceFeed",
	"RandomnessLog",
	"AppRegistry",
	"AutomationAnchor",
	"ServiceLayerGateway",
	"MiniAppServiceConsumer",
}

func main() {
	wif := strings.TrimSpace(os.Getenv("NEO_TESTNET_WIF"))
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

	buildDir := strings.TrimSpace(os.Getenv("CONTRACT_BUILD_DIR"))
	if buildDir == "" {
		buildDir = filepath.Join("contracts", "build")
	}

	fmt.Println("=== Contract Hash Calculation (offline) ===")
	fmt.Printf("Deployer: %s\n\n", deployerHash.StringLE())

	results := make(map[string]string)
	for _, name := range contracts {
		nefPath := filepath.Join(buildDir, name+".nef")
		manifestPath := filepath.Join(buildDir, name+".manifest.json")

		if _, statErr := os.Stat(nefPath); statErr != nil {
			continue
		}

		nefData, err := os.ReadFile(nefPath)
		if err != nil {
			fmt.Printf("%-22s %s\n", name+":", "NEF read failed")
			continue
		}
		nefFile, err := nef.FileFromBytes(nefData)
		if err != nil {
			fmt.Printf("%-22s %s\n", name+":", "NEF parse failed")
			continue
		}

		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			fmt.Printf("%-22s %s\n", name+":", "manifest read failed")
			continue
		}
		var m manifest.Manifest
		if err := json.Unmarshal(manifestData, &m); err != nil {
			fmt.Printf("%-22s %s\n", name+":", "manifest parse failed")
			continue
		}

		contractHash := state.CreateContractHash(deployerHash, nefFile.Checksum, m.Name)
		results[name] = "0x" + contractHash.StringLE()
	}

	keys := make([]string, 0, len(results))
	for key := range results {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		fmt.Printf("%-22s %s\n", key+":", results[key])
	}
}
