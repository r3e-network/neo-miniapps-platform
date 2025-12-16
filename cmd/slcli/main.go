// Package main provides the Service Layer CLI for user authentication and service management.
//
// Usage:
//
//	slcli login --token <TOKEN>                      - Login with API token
//	slcli logout                                     - Logout and clear credentials
//	slcli whoami                                     - Show current user info
//	slcli balance                                    - Check user balance
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
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/httputil"
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
