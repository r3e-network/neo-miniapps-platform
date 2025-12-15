// Package main provides the Service Layer CLI for user authentication and service management.
//
// Usage:
//
//	slcli login --token <TOKEN>                      - Login with API token
//	slcli logout                                     - Logout and clear credentials
//	slcli whoami                                     - Show current user info
//	slcli balance                                    - Check user balance
//	slcli neovault request --amount <AMOUNT>            - Create neovault request
//	slcli neovault status --request-id <ID>             - Check neovault request status
//	slcli vrf request --seed <SEED>                  - Request VRF random number
//	slcli vrf get --request-id <ID>                  - Get VRF result
//	slcli secrets create --name <NAME> --value <VAL> - Create secret
//	slcli secrets list                               - List secrets
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

const (
	defaultAPIURL   = "http://localhost:8080/api/v1"
	credentialsFile = ".slcli/credentials" // #nosec G101 -- file path, not credentials
	configFile      = ".slcli/config"
	envTokenKey     = "SLCLI_TOKEN" // #nosec G101 -- env var name, not a credential
	envAPIURLKey    = "SLCLI_API_URL"
)

// Credentials stores user authentication information
type Credentials struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	Address   string    `json:"address"`
	Email     string    `json:"email,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Config stores CLI configuration
type Config struct {
	APIURL string `json:"api_url"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "login":
		cmdLogin(args)
	case "logout":
		cmdLogout(args)
	case "whoami":
		cmdWhoami(args)
	case "balance":
		cmdBalance(args)
	case "neovault":
		cmdNeoVault(args)
	case "vrf":
		cmdVRF(args)
	case "secrets":
		cmdSecrets(args)
	case "help", "-h", "--help":
		printUsage()
	case "version", "-v", "--version":
		fmt.Println("slcli version 1.0.0")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Service Layer CLI - User Authentication and Service Management

Usage:
  slcli <command> [arguments]

Authentication Commands:
  login --token <TOKEN>              Login with API token
  logout                             Logout and clear credentials
  whoami                             Show current user info

Service Commands:
  balance                            Check user GAS balance
  neovault request --amount <AMOUNT>    Create neovault request
  neovault status --request-id <ID>     Check neovault request status
  neovault list                         List neovault requests
  vrf request --seed <SEED>          Request VRF random number
  vrf get --request-id <ID>          Get VRF result
  vrf list                           List VRF requests
  secrets create --name <N> --value <V>  Create secret
  secrets list                       List secrets
  secrets delete --name <NAME>       Delete secret
  secrets permissions --name <NAME>  Get secret permissions

Environment Variables:
  SLCLI_TOKEN      API token (alternative to login)
  SLCLI_API_URL    API base URL (default: http://localhost:8080/api/v1)

Examples:
  slcli login --token eyJhbGc...
  slcli whoami
  slcli balance
  slcli neovault request --amount 100000000
  slcli vrf request --seed "my-random-seed"
  slcli secrets create --name api_key --value "secret-value"
  slcli logout`)
}

// =============================================================================
// Authentication Commands
// =============================================================================

func cmdLogin(args []string) {
	if len(args) < 2 || args[0] != "--token" {
		fmt.Fprintln(os.Stderr, "Usage: slcli login --token <TOKEN>")
		os.Exit(1)
	}

	token := args[1]
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: Token cannot be empty")
		os.Exit(1)
	}

	creds, err := loginWithToken(token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Login successful")
	fmt.Printf("User ID:  %s\n", creds.UserID)
	fmt.Printf("Address:  %s\n", creds.Address)
	if creds.Email != "" {
		fmt.Printf("Email:    %s\n", creds.Email)
	}
	fmt.Printf("Expires:  %s\n", creds.ExpiresAt.Format(time.RFC3339))
}

func loginWithToken(token string) (*Credentials, error) {
	// Verify token by calling /me endpoint
	apiURL := getAPIURL()
	req, err := http.NewRequest(http.MethodGet, apiURL+"/me", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, truncated, readErr := httputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return nil, fmt.Errorf("invalid token (HTTP %d): failed to read body: %w", resp.StatusCode, readErr)
		}
		msg := string(body)
		if truncated {
			msg += "...(truncated)"
		}
		return nil, fmt.Errorf("invalid token (HTTP %d): %s", resp.StatusCode, msg)
	}

	// Parse user info
	var meResp struct {
		User struct {
			ID      string `json:"id"`
			Address string `json:"address"`
			Email   string `json:"email"`
		} `json:"user"`
	}
	data, err := httputil.ReadAllStrict(resp.Body, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if err := json.Unmarshal(data, &meResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Save credentials
	creds := &Credentials{
		Token:     token,
		UserID:    meResp.User.ID,
		Address:   meResp.User.Address,
		Email:     meResp.User.Email,
		ExpiresAt: time.Now().Add(24 * time.Hour), // Assume 24h expiry
		CreatedAt: time.Now(),
	}

	if err := saveCredentials(creds); err != nil {
		return nil, fmt.Errorf("failed to save credentials: %w", err)
	}

	return creds, nil
}

func cmdLogout(args []string) {
	_ = args
	credsPath := getCredentialsPath()
	if err := os.Remove(credsPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Failed to remove credentials: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Logout successful")
}

func cmdWhoami(args []string) {
	_ = args
	creds, err := loadCredentials()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: Not logged in. Use 'slcli login --token <TOKEN>' to login.")
		os.Exit(1)
	}

	// Check if token is expired
	if time.Now().After(creds.ExpiresAt) {
		fmt.Fprintln(os.Stderr, "Error: Token expired. Please login again.")
		os.Exit(1)
	}

	fmt.Printf("User ID:  %s\n", creds.UserID)
	fmt.Printf("Address:  %s\n", creds.Address)
	if creds.Email != "" {
		fmt.Printf("Email:    %s\n", creds.Email)
	}
	fmt.Printf("Logged in: %s\n", creds.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Expires:   %s\n", creds.ExpiresAt.Format(time.RFC3339))

	// Calculate time remaining
	remaining := time.Until(creds.ExpiresAt)
	if remaining > 0 {
		fmt.Printf("Time left: %s\n", formatDuration(remaining))
	}
}

// =============================================================================
// Service Commands
// =============================================================================

func cmdBalance(args []string) {
	_ = args
	data, err := apiRequest("GET", "/gasbank/account", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var account struct {
		Balance  int64 `json:"balance"`
		Reserved int64 `json:"reserved"`
	}
	if err := json.Unmarshal(data, &account); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to parse response: %v\n", err)
		os.Exit(1)
	}

	available := account.Balance - account.Reserved
	fmt.Printf("Balance:   %d (%.8f GAS)\n", account.Balance, float64(account.Balance)/1e8)
	fmt.Printf("Reserved:  %d (%.8f GAS)\n", account.Reserved, float64(account.Reserved)/1e8)
	fmt.Printf("Available: %d (%.8f GAS)\n", available, float64(available)/1e8)
}

func cmdNeoVault(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: slcli neovault <request|status|list>")
		os.Exit(1)
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "request":
		cmdNeoVaultRequest(subargs)
	case "status":
		cmdNeoVaultStatus(subargs)
	case "list":
		cmdNeoVaultList(subargs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown neovault subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func cmdNeoVaultRequest(args []string) {
	if len(args) < 2 || args[0] != "--amount" {
		fmt.Fprintln(os.Stderr, "Usage: slcli neovault request --amount <AMOUNT>")
		os.Exit(1)
	}

	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || amount <= 0 {
		fmt.Fprintln(os.Stderr, "Error: amount must be a positive integer (in smallest units, e.g. 100000000 = 1 GAS)")
		os.Exit(1)
	}

	address, err := getUserAddress()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to resolve user address: %v\n", err)
		os.Exit(1)
	}

	payload := map[string]interface{}{
		"user_address": address,
		"total_amount": amount,
	}

	data, err := apiRequest("POST", "/neovault/request", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var resp struct {
		RequestID      string `json:"request_id"`
		DepositAddress string `json:"deposit_address"`
		TotalAmount    int64  `json:"total_amount"`
		ServiceFee     int64  `json:"service_fee"`
		NetAmount      int64  `json:"net_amount"`
		Deadline       int64  `json:"deadline"`
		ExpiresAt      string `json:"expires_at"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		fmt.Println(string(data))
		return
	}

	fmt.Println("✓ Neovault request created")
	fmt.Printf("Request ID:      %s\n", resp.RequestID)
	fmt.Printf("Deposit Address: %s\n", resp.DepositAddress)
	fmt.Printf("Total Amount:    %d\n", resp.TotalAmount)
	fmt.Printf("Service Fee:     %d\n", resp.ServiceFee)
	fmt.Printf("Net Amount:      %d\n", resp.NetAmount)
	fmt.Printf("Deadline:        %d\n", resp.Deadline)
	fmt.Printf("Expires At:      %s\n", resp.ExpiresAt)
}

func cmdNeoVaultStatus(args []string) {
	if len(args) < 2 || args[0] != "--request-id" {
		fmt.Fprintln(os.Stderr, "Usage: slcli neovault status --request-id <ID>")
		os.Exit(1)
	}

	requestID := args[1]
	data, err := apiRequest("GET", "/neovault/request/"+requestID, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}

func cmdNeoVaultList(args []string) {
	_ = args
	data, err := apiRequest("GET", "/neovault/requests", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var requests []map[string]interface{}
	if err := json.Unmarshal(data, &requests); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to parse response: %v\n", err)
		os.Exit(1)
	}

	if len(requests) == 0 {
		fmt.Println("No neovault requests found")
		return
	}

	fmt.Printf("Found %d neovault request(s):\n\n", len(requests))
	for i, req := range requests {
		fmt.Printf("%d. Request ID: %v\n", i+1, req["request_id"])
		fmt.Printf("   Amount: %v\n", req["amount"])
		fmt.Printf("   Status: %v\n", req["status"])
		fmt.Printf("   Created: %v\n\n", req["created_at"])
	}
}

func cmdVRF(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: slcli vrf <request|get|list>")
		os.Exit(1)
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "request":
		cmdVRFRequest(subargs)
	case "get":
		cmdVRFGet(subargs)
	case "list":
		cmdVRFList(subargs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown vrf subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func cmdVRFRequest(args []string) {
	if len(args) < 2 || args[0] != "--seed" {
		fmt.Fprintln(os.Stderr, "Usage: slcli vrf request --seed <SEED>")
		os.Exit(1)
	}

	seed := args[1]
	payload := map[string]interface{}{
		"seed":      seed,
		"num_words": 1,
	}

	data, err := apiRequest("POST", "/vrf/random", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ VRF request created")
	fmt.Println(string(data))
}

func cmdVRFGet(args []string) {
	if len(args) < 2 || args[0] != "--request-id" {
		fmt.Fprintln(os.Stderr, "Usage: slcli vrf get --request-id <ID>")
		os.Exit(1)
	}

	fmt.Println("Note: VRF get endpoint not yet implemented")
}

func cmdVRFList(args []string) {
	fmt.Println("Note: VRF list endpoint not yet implemented")
}

func cmdSecrets(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: slcli secrets <create|list|delete|permissions>")
		os.Exit(1)
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "create":
		cmdSecretsCreate(subargs)
	case "list":
		cmdSecretsList(subargs)
	case "delete":
		cmdSecretsDelete(subargs)
	case "permissions":
		cmdSecretsPermissions(subargs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown secrets subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func cmdSecretsCreate(args []string) {
	if len(args) < 4 || args[0] != "--name" || args[2] != "--value" {
		fmt.Fprintln(os.Stderr, "Usage: slcli secrets create --name <NAME> --value <VALUE>")
		os.Exit(1)
	}

	name := args[1]
	value := args[3]

	payload := map[string]interface{}{
		"name":  name,
		"value": value,
	}

	data, err := apiRequest("POST", "/secrets/secrets", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Secret '%s' created successfully\n", name)
	fmt.Println(string(data))
}

func cmdSecretsList(args []string) {
	_ = args
	data, err := apiRequest("GET", "/secrets/secrets", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var secrets []map[string]interface{}
	if err := json.Unmarshal(data, &secrets); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to parse response: %v\n", err)
		os.Exit(1)
	}

	if len(secrets) == 0 {
		fmt.Println("No secrets found")
		return
	}

	fmt.Printf("Found %d secret(s):\n\n", len(secrets))
	for i, secret := range secrets {
		fmt.Printf("%d. Name: %v\n", i+1, secret["name"])
		fmt.Printf("   ID: %v\n", secret["id"])
		fmt.Printf("   Version: %v\n", secret["version"])
		fmt.Printf("   Created: %v\n\n", secret["created_at"])
	}
}

func cmdSecretsDelete(args []string) {
	if len(args) < 2 || args[0] != "--name" {
		fmt.Fprintln(os.Stderr, "Usage: slcli secrets delete --name <NAME>")
		os.Exit(1)
	}

	name := args[1]
	_, err := apiRequest("DELETE", "/secrets/secrets/"+name, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Secret '%s' deleted successfully\n", name)
}

func cmdSecretsPermissions(args []string) {
	if len(args) < 2 || args[0] != "--name" {
		fmt.Fprintln(os.Stderr, "Usage: slcli secrets permissions --name <NAME>")
		os.Exit(1)
	}

	name := args[1]
	data, err := apiRequest("GET", "/secrets/secrets/"+name+"/permissions", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Permissions for secret '%s':\n", name)
	fmt.Println(string(data))
}

// =============================================================================
// Helper Functions
// =============================================================================

func getCredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to get home directory: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(home, credentialsFile)
}

func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to get home directory: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(home, configFile)
}

func saveCredentials(creds *Credentials) error {
	credsPath := getCredentialsPath()
	credsDir := filepath.Dir(credsPath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(credsDir, 0o700); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}

	// Marshal credentials to JSON
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Write to file with restricted permissions
	if err := os.WriteFile(credsPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

func loadCredentials() (*Credentials, error) {
	// Check environment variable first
	if token := os.Getenv(envTokenKey); token != "" {
		return &Credentials{
			Token:     token,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}, nil
	}

	// Load from file
	credsPath := getCredentialsPath()
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return &creds, nil
}

func getUserAddress() (string, error) {
	creds, err := loadCredentials()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(creds.Address) != "" {
		return creds.Address, nil
	}

	// When running via SLCLI_TOKEN without a stored profile, resolve address via /me.
	data, err := apiRequest(http.MethodGet, "/me", nil)
	if err != nil {
		return "", err
	}
	var meResp struct {
		User struct {
			Address string `json:"address"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &meResp); err != nil {
		return "", fmt.Errorf("parse /me response: %w", err)
	}
	if meResp.User.Address == "" {
		return "", fmt.Errorf("missing address in /me response")
	}
	return meResp.User.Address, nil
}

func getAPIURL() string {
	// Check environment variable
	if url := os.Getenv(envAPIURLKey); url != "" {
		return strings.TrimSuffix(url, "/")
	}

	// Check config file
	configPath := getConfigPath()
	if data, err := os.ReadFile(configPath); err == nil {
		var config Config
		if err := json.Unmarshal(data, &config); err == nil && config.APIURL != "" {
			return strings.TrimSuffix(config.APIURL, "/")
		}
	}

	return defaultAPIURL
}

func apiRequest(method, endpoint string, payload interface{}) ([]byte, error) {
	creds, err := loadCredentials()
	if err != nil {
		return nil, fmt.Errorf("not logged in. Use 'slcli login --token <TOKEN>' to login")
	}

	apiURL := getAPIURL()
	var body io.Reader
	if payload != nil {
		payloadBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", marshalErr)
		}
		body = strings.NewReader(string(payloadBytes))
	}

	req, err := http.NewRequest(method, apiURL+endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+creds.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, truncated, readErr := httputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read response: %w", readErr)
		}
		msg := string(respBody)
		if truncated {
			msg += "...(truncated)"
		}
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, msg)
	}

	respBody, err := httputil.ReadAllStrict(resp.Body, 8<<20)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return respBody, nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}
