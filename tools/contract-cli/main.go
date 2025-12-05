// Package main provides CLI tools for building, deploying, and testing Service Layer contracts.
package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Contract represents a Service Layer contract.
type Contract struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	ScriptHash  string `json:"script_hash,omitempty"`
	NEFPath     string `json:"nef_path,omitempty"`
	ManifestPath string `json:"manifest_path,omitempty"`
}

// NetworkConfig represents network configuration.
type NetworkConfig struct {
	Name       string `json:"name"`
	RPC        string `json:"rpc"`
	NetworkID  uint32 `json:"network_id"`
	WalletPath string `json:"wallet_path"`
	Account    string `json:"account"`
}

// DeploymentResult represents the result of a contract deployment.
type DeploymentResult struct {
	Contract   string    `json:"contract"`
	ScriptHash string    `json:"script_hash"`
	TxHash     string    `json:"tx_hash"`
	Network    string    `json:"network"`
	DeployedAt time.Time `json:"deployed_at"`
}

// Contracts to build and deploy
var contracts = []Contract{
	{Name: "ServiceLayerGateway", Path: "contracts/ServiceLayerGateway"},
	{Name: "OracleService", Path: "contracts/OracleService"},
	{Name: "VRFService", Path: "contracts/VRFService"},
	{Name: "DataFeedsService", Path: "contracts/DataFeedsService"},
	{Name: "AutomationService", Path: "contracts/AutomationService"},
	{Name: "SecretsService", Path: "contracts/SecretsService"},
	{Name: "GasBankService", Path: "contracts/GasBankService"},
}

// Network configurations
var networks = map[string]NetworkConfig{
	"neo-express": {
		Name:       "neo-express",
		RPC:        "http://localhost:50012",
		NetworkID:  1234567890,
		WalletPath: "test/neo-express/wallet.json",
		Account:    "genesis",
	},
	"testnet": {
		Name:       "testnet",
		RPC:        "http://seed1t5.neo.org:20332",
		NetworkID:  894710606,
		WalletPath: "config/testnet-wallet.json",
		Account:    "deployer",
	},
	"mainnet": {
		Name:       "mainnet",
		RPC:        "http://seed1.neo.org:10332",
		NetworkID:  860833102,
		WalletPath: "config/mainnet-wallet.json",
		Account:    "deployer",
	},
}

func main() {
	// Define commands
	buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
	buildAll := buildCmd.Bool("all", false, "Build all contracts")
	buildContract := buildCmd.String("contract", "", "Build specific contract")

	deployCmd := flag.NewFlagSet("deploy", flag.ExitOnError)
	deployNetwork := deployCmd.String("network", "neo-express", "Target network (neo-express, testnet, mainnet)")
	deployContract := deployCmd.String("contract", "", "Deploy specific contract (empty for all)")
	deployInit := deployCmd.Bool("init", true, "Initialize contracts after deployment")

	initCmd := flag.NewFlagSet("init", flag.ExitOnError)
	initNetwork := initCmd.String("network", "neo-express", "Target network")
	initGateway := initCmd.String("gateway", "", "Gateway contract hash")
	initServiceLayer := initCmd.String("service-layer", "", "Service Layer address")

	testCmd := flag.NewFlagSet("test", flag.ExitOnError)
	testIntegration := testCmd.Bool("integration", false, "Run integration tests")
	testUnit := testCmd.Bool("unit", false, "Run unit tests")

	expressCmd := flag.NewFlagSet("express", flag.ExitOnError)
	expressAction := expressCmd.String("action", "start", "Action: start, stop, reset, checkpoint")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		buildCmd.Parse(os.Args[2:])
		handleBuild(*buildAll, *buildContract)
	case "deploy":
		deployCmd.Parse(os.Args[2:])
		handleDeploy(*deployNetwork, *deployContract, *deployInit)
	case "init":
		initCmd.Parse(os.Args[2:])
		handleInit(*initNetwork, *initGateway, *initServiceLayer)
	case "test":
		testCmd.Parse(os.Args[2:])
		handleTest(*testIntegration, *testUnit)
	case "express":
		expressCmd.Parse(os.Args[2:])
		handleExpress(*expressAction)
	case "status":
		handleStatus()
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Service Layer Contract CLI

Usage:
  contract-cli <command> [options]

Commands:
  build     Build contracts
    -all              Build all contracts
    -contract <name>  Build specific contract

  deploy    Deploy contracts to network
    -network <name>   Target network (neo-express, testnet, mainnet)
    -contract <name>  Deploy specific contract (empty for all)
    -init             Initialize contracts after deployment (default: true)

  init      Initialize deployed contracts
    -network <name>       Target network
    -gateway <hash>       Gateway contract hash
    -service-layer <addr> Service Layer TEE address

  test      Run tests
    -integration  Run integration tests with neo-express
    -unit         Run unit tests

  express   Manage neo-express
    -action <name>  Action: start, stop, reset, checkpoint

  status    Show deployment status

  help      Show this help message

Examples:
  # Build all contracts
  contract-cli build -all

  # Deploy to neo-express and initialize
  contract-cli deploy -network neo-express -init

  # Run integration tests
  contract-cli test -integration

  # Start neo-express
  contract-cli express -action start`)
}

// =============================================================================
// Build Command
// =============================================================================

func handleBuild(all bool, contractName string) {
	fmt.Println("=== Building Service Layer Contracts ===")

	// Check for nccs (Neo C# Compiler)
	if !checkCommand("nccs") {
		fmt.Println("Error: nccs (Neo C# Compiler) not found")
		fmt.Println("Install with: dotnet tool install -g Neo.Compiler.CSharp")
		os.Exit(1)
	}

	var toBuild []Contract
	if all {
		toBuild = contracts
	} else if contractName != "" {
		for _, c := range contracts {
			if c.Name == contractName {
				toBuild = append(toBuild, c)
				break
			}
		}
		if len(toBuild) == 0 {
			fmt.Printf("Contract not found: %s\n", contractName)
			os.Exit(1)
		}
	} else {
		fmt.Println("Specify -all or -contract <name>")
		os.Exit(1)
	}

	buildDir := "contracts/build"
	os.MkdirAll(buildDir, 0755)

	results := make(map[string]bool)
	for _, contract := range toBuild {
		fmt.Printf("\nBuilding %s...\n", contract.Name)
		success := buildContract(contract, buildDir)
		results[contract.Name] = success
	}

	// Print summary
	fmt.Println("\n=== Build Summary ===")
	for name, success := range results {
		status := "✓ SUCCESS"
		if !success {
			status = "✗ FAILED"
		}
		fmt.Printf("  %s: %s\n", name, status)
	}
}

func buildContract(contract Contract, buildDir string) bool {
	// Find .csproj file
	csprojPath := filepath.Join(contract.Path, contract.Name+".csproj")
	if _, err := os.Stat(csprojPath); os.IsNotExist(err) {
		// Try to create a basic csproj
		if err := createCsproj(contract); err != nil {
			fmt.Printf("  Error creating csproj: %v\n", err)
			return false
		}
	}

	// Build with nccs
	cmd := exec.Command("nccs", contract.Path, "-o", buildDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("  Build failed: %v\n", err)
		return false
	}

	// Verify output files
	nefPath := filepath.Join(buildDir, contract.Name+".nef")
	manifestPath := filepath.Join(buildDir, contract.Name+".manifest.json")

	if _, err := os.Stat(nefPath); os.IsNotExist(err) {
		fmt.Printf("  NEF file not found: %s\n", nefPath)
		return false
	}
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		fmt.Printf("  Manifest file not found: %s\n", manifestPath)
		return false
	}

	fmt.Printf("  Output: %s, %s\n", nefPath, manifestPath)
	return true
}

func createCsproj(contract Contract) error {
	csprojContent := fmt.Sprintf(`<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net7.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Neo.SmartContract.Framework" Version="3.6.*" />
  </ItemGroup>
</Project>
`)
	csprojPath := filepath.Join(contract.Path, contract.Name+".csproj")
	return os.WriteFile(csprojPath, []byte(csprojContent), 0644)
}

// =============================================================================
// Deploy Command
// =============================================================================

func handleDeploy(networkName, contractName string, initialize bool) {
	fmt.Printf("=== Deploying to %s ===\n", networkName)

	network, ok := networks[networkName]
	if !ok {
		fmt.Printf("Unknown network: %s\n", networkName)
		os.Exit(1)
	}

	// Check for neoxp or neo-go
	var deployTool string
	if networkName == "neo-express" {
		if checkCommand("neoxp") {
			deployTool = "neoxp"
		} else {
			fmt.Println("Error: neoxp not found. Install neo-express.")
			os.Exit(1)
		}
	} else {
		if checkCommand("neo-go") {
			deployTool = "neo-go"
		} else {
			fmt.Println("Error: neo-go not found")
			os.Exit(1)
		}
	}

	var toDeploy []Contract
	if contractName != "" {
		for _, c := range contracts {
			if c.Name == contractName {
				toDeploy = append(toDeploy, c)
				break
			}
		}
	} else {
		toDeploy = contracts
	}

	results := []DeploymentResult{}
	for _, contract := range toDeploy {
		fmt.Printf("\nDeploying %s...\n", contract.Name)
		result, err := deployContract(contract, network, deployTool)
		if err != nil {
			fmt.Printf("  Deploy failed: %v\n", err)
			continue
		}
		results = append(results, *result)
		fmt.Printf("  Deployed: %s\n", result.ScriptHash)
	}

	// Save deployment results
	saveDeploymentResults(networkName, results)

	// Initialize if requested
	if initialize && len(results) > 0 {
		fmt.Println("\n=== Initializing Contracts ===")
		initializeContracts(network, results)
	}

	// Print summary
	fmt.Println("\n=== Deployment Summary ===")
	for _, r := range results {
		fmt.Printf("  %s: %s\n", r.Contract, r.ScriptHash)
	}
}

func deployContract(contract Contract, network NetworkConfig, tool string) (*DeploymentResult, error) {
	nefPath := filepath.Join("contracts/build", contract.Name+".nef")
	manifestPath := filepath.Join("contracts/build", contract.Name+".manifest.json")

	// Check files exist
	if _, err := os.Stat(nefPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("NEF file not found: %s (run build first)", nefPath)
	}

	var cmd *exec.Cmd
	if tool == "neoxp" {
		cmd = exec.Command("neoxp", "contract", "deploy",
			nefPath,
			network.Account,
			"--input", "test/neo-express/default.neo-express",
		)
	} else {
		cmd = exec.Command("neo-go", "contract", "deploy",
			"--in", nefPath,
			"--manifest", manifestPath,
			"--rpc-endpoint", network.RPC,
			"--wallet", network.WalletPath,
			"--address", network.Account,
		)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, string(output))
	}

	// Parse script hash from output
	scriptHash := parseScriptHash(string(output))

	return &DeploymentResult{
		Contract:   contract.Name,
		ScriptHash: scriptHash,
		TxHash:     parseTxHash(string(output)),
		Network:    network.Name,
		DeployedAt: time.Now(),
	}, nil
}

func parseScriptHash(output string) string {
	// Parse script hash from deployment output
	// Format varies by tool, this is a simplified parser
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Contract") && strings.Contains(line, "0x") {
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.HasPrefix(p, "0x") && len(p) == 42 {
					return p
				}
			}
		}
	}
	return ""
}

func parseTxHash(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "txid") || strings.Contains(line, "Transaction") {
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.HasPrefix(p, "0x") && len(p) == 66 {
					return p
				}
			}
		}
	}
	return ""
}

func saveDeploymentResults(network string, results []DeploymentResult) {
	filename := fmt.Sprintf("contracts/build/deployment-%s.json", network)
	data, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(filename, data, 0644)
	fmt.Printf("\nDeployment results saved to: %s\n", filename)
}

func initializeContracts(network NetworkConfig, results []DeploymentResult) {
	// Find gateway contract
	var gatewayHash string
	for _, r := range results {
		if r.Contract == "ServiceLayerGateway" {
			gatewayHash = r.ScriptHash
			break
		}
	}

	if gatewayHash == "" {
		fmt.Println("  Gateway contract not found in deployment results")
		return
	}

	// Register services with gateway
	for _, r := range results {
		if r.Contract == "ServiceLayerGateway" {
			continue
		}

		serviceId := strings.TrimSuffix(r.Contract, "Service")
		serviceId = strings.ToLower(serviceId)

		fmt.Printf("  Registering %s with gateway...\n", r.Contract)
		if err := registerService(network, gatewayHash, serviceId, r.ScriptHash); err != nil {
			fmt.Printf("    Failed: %v\n", err)
		} else {
			fmt.Printf("    Registered: %s -> %s\n", serviceId, r.ScriptHash)
		}
	}
}

func registerService(network NetworkConfig, gatewayHash, serviceId, contractHash string) error {
	// Use neoxp or neo-go to invoke RegisterService
	if network.Name == "neo-express" {
		cmd := exec.Command("neoxp", "contract", "invoke",
			gatewayHash, "registerService",
			serviceId, contractHash,
			"--account", network.Account,
			"--input", "test/neo-express/default.neo-express",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, string(output))
		}
	}
	return nil
}

// =============================================================================
// Init Command
// =============================================================================

func handleInit(networkName, gatewayHash, serviceLayerAddr string) {
	fmt.Printf("=== Initializing Contracts on %s ===\n", networkName)

	network, ok := networks[networkName]
	if !ok {
		fmt.Printf("Unknown network: %s\n", networkName)
		os.Exit(1)
	}

	// Load deployment results
	filename := fmt.Sprintf("contracts/build/deployment-%s.json", networkName)
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Deployment results not found: %s\n", filename)
		fmt.Println("Run 'contract-cli deploy' first")
		os.Exit(1)
	}

	var results []DeploymentResult
	json.Unmarshal(data, &results)

	if gatewayHash == "" {
		for _, r := range results {
			if r.Contract == "ServiceLayerGateway" {
				gatewayHash = r.ScriptHash
				break
			}
		}
	}

	if gatewayHash == "" {
		fmt.Println("Gateway hash not found. Specify with -gateway")
		os.Exit(1)
	}

	// Set service layer address on gateway
	if serviceLayerAddr != "" {
		fmt.Printf("Setting service layer address: %s\n", serviceLayerAddr)
		setServiceLayer(network, gatewayHash, serviceLayerAddr)
	}

	// Set gateway on all service contracts
	for _, r := range results {
		if r.Contract == "ServiceLayerGateway" {
			continue
		}
		fmt.Printf("Setting gateway on %s...\n", r.Contract)
		setGateway(network, r.ScriptHash, gatewayHash)
	}

	fmt.Println("\n=== Initialization Complete ===")
}

func setServiceLayer(network NetworkConfig, gatewayHash, serviceLayerAddr string) error {
	if network.Name == "neo-express" {
		cmd := exec.Command("neoxp", "contract", "invoke",
			gatewayHash, "setServiceLayer",
			serviceLayerAddr,
			"--account", network.Account,
			"--input", "test/neo-express/default.neo-express",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, string(output))
		}
	}
	return nil
}

func setGateway(network NetworkConfig, contractHash, gatewayHash string) error {
	if network.Name == "neo-express" {
		cmd := exec.Command("neoxp", "contract", "invoke",
			contractHash, "setGateway",
			gatewayHash,
			"--account", network.Account,
			"--input", "test/neo-express/default.neo-express",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, string(output))
		}
	}
	return nil
}

// =============================================================================
// Test Command
// =============================================================================

func handleTest(integration, unit bool) {
	if integration {
		runIntegrationTests()
	}
	if unit {
		runUnitTests()
	}
	if !integration && !unit {
		fmt.Println("Specify -integration or -unit")
	}
}

func runIntegrationTests() {
	fmt.Println("=== Running Integration Tests ===")

	// 1. Start neo-express if not running
	fmt.Println("\n1. Starting neo-express...")
	startNeoExpress()

	// 2. Reset to clean state
	fmt.Println("\n2. Resetting neo-express...")
	resetNeoExpress()

	// 3. Build contracts
	fmt.Println("\n3. Building contracts...")
	handleBuild(true, "")

	// 4. Deploy contracts
	fmt.Println("\n4. Deploying contracts...")
	handleDeploy("neo-express", "", true)

	// 5. Run Go integration tests
	fmt.Println("\n5. Running integration tests...")
	cmd := exec.Command("go", "test", "-v", "-tags=integration", "./test/contracts/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "NEO_EXPRESS=true")

	if err := cmd.Run(); err != nil {
		fmt.Printf("Integration tests failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Integration Tests Complete ===")
}

func runUnitTests() {
	fmt.Println("=== Running Unit Tests ===")
	cmd := exec.Command("go", "test", "-v", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Unit tests failed: %v\n", err)
		os.Exit(1)
	}
}

// =============================================================================
// Express Command
// =============================================================================

func handleExpress(action string) {
	switch action {
	case "start":
		startNeoExpress()
	case "stop":
		stopNeoExpress()
	case "reset":
		resetNeoExpress()
	case "checkpoint":
		checkpointNeoExpress()
	default:
		fmt.Printf("Unknown action: %s\n", action)
	}
}

func startNeoExpress() {
	fmt.Println("Starting neo-express...")

	// Check if already running
	cmd := exec.Command("neoxp", "show", "node", "--input", "test/neo-express/default.neo-express")
	if err := cmd.Run(); err == nil {
		fmt.Println("neo-express is already running")
		return
	}

	// Create config if not exists
	configPath := "test/neo-express/default.neo-express"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		os.MkdirAll("test/neo-express", 0755)
		cmd := exec.Command("neoxp", "create", "-f", "-o", configPath)
		cmd.Run()
	}

	// Start in background
	cmd = exec.Command("neoxp", "run", "--input", configPath, "--seconds-per-block", "1")
	cmd.Start()

	// Wait for startup
	time.Sleep(3 * time.Second)
	fmt.Println("neo-express started")
}

func stopNeoExpress() {
	fmt.Println("Stopping neo-express...")
	exec.Command("neoxp", "stop", "--input", "test/neo-express/default.neo-express").Run()
	fmt.Println("neo-express stopped")
}

func resetNeoExpress() {
	fmt.Println("Resetting neo-express...")
	exec.Command("neoxp", "reset", "-f", "--input", "test/neo-express/default.neo-express").Run()
	fmt.Println("neo-express reset")
}

func checkpointNeoExpress() {
	checkpointPath := fmt.Sprintf("test/neo-express/checkpoint-%d.neoxp-checkpoint", time.Now().Unix())
	fmt.Printf("Creating checkpoint: %s\n", checkpointPath)
	exec.Command("neoxp", "checkpoint", "create", checkpointPath, "--input", "test/neo-express/default.neo-express").Run()
}

// =============================================================================
// Status Command
// =============================================================================

func handleStatus() {
	fmt.Println("=== Service Layer Contract Status ===")

	// Check build status
	fmt.Println("\nBuild Status:")
	for _, contract := range contracts {
		nefPath := filepath.Join("contracts/build", contract.Name+".nef")
		if _, err := os.Stat(nefPath); err == nil {
			fmt.Printf("  ✓ %s: built\n", contract.Name)
		} else {
			fmt.Printf("  ✗ %s: not built\n", contract.Name)
		}
	}

	// Check deployment status for each network
	for networkName := range networks {
		filename := fmt.Sprintf("contracts/build/deployment-%s.json", networkName)
		data, err := os.ReadFile(filename)
		if err != nil {
			continue
		}

		var results []DeploymentResult
		json.Unmarshal(data, &results)

		if len(results) > 0 {
			fmt.Printf("\nDeployment Status (%s):\n", networkName)
			for _, r := range results {
				fmt.Printf("  ✓ %s: %s\n", r.Contract, r.ScriptHash)
			}
		}
	}

	// Check neo-express status
	fmt.Println("\nNeo-Express Status:")
	cmd := exec.Command("neoxp", "show", "node", "--input", "test/neo-express/default.neo-express")
	if err := cmd.Run(); err == nil {
		fmt.Println("  ✓ Running")
	} else {
		fmt.Println("  ✗ Not running")
	}
}

// =============================================================================
// Helpers
// =============================================================================

func checkCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func hexEncode(data []byte) string {
	return hex.EncodeToString(data)
}
