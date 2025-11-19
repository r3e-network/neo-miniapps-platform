package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	defaultAddr := getenv("SERVICE_LAYER_ADDR", "http://localhost:8080")
	defaultToken := os.Getenv("SERVICE_LAYER_TOKEN")

	root := flag.NewFlagSet("slctl", flag.ContinueOnError)
	root.SetOutput(io.Discard)
	addrFlag := root.String("addr", defaultAddr, "Service Layer base URL (default env SERVICE_LAYER_ADDR)")
	tokenFlag := root.String("token", defaultToken, "Bearer token for authentication (env SERVICE_LAYER_TOKEN)")
	timeoutFlag := root.Duration("timeout", 15*time.Second, "HTTP request timeout")
	if err := root.Parse(args); err != nil {
		return usageError(err)
	}

	remaining := root.Args()
	if len(remaining) == 0 {
		return usageError(errors.New("no command specified"))
	}

	httpClient := &http.Client{Timeout: *timeoutFlag}
	client := &apiClient{
		baseURL: strings.TrimRight(*addrFlag, "/"),
		token:   strings.TrimSpace(*tokenFlag),
		http:    httpClient,
	}

	switch remaining[0] {
	case "accounts":
		return handleAccounts(ctx, client, remaining[1:])
	case "functions":
		return handleFunctions(ctx, client, remaining[1:])
	case "automation":
		return handleAutomation(ctx, client, remaining[1:])
	case "secrets":
		return handleSecrets(ctx, client, remaining[1:])
	case "gasbank":
		return handleGasBank(ctx, client, remaining[1:])
	case "oracle":
		return handleOracle(ctx, client, remaining[1:])
	case "pricefeeds":
		return handlePriceFeeds(ctx, client, remaining[1:])
	case "random":
		return handleRandom(ctx, client, remaining[1:])
	case "services":
		return handleServices(ctx, client, remaining[1:])
	case "version":
		fmt.Println("Service Layer CLI (slctl)")
		return nil
	case "help", "-h", "--help":
		printRootUsage()
		return nil
	default:
		return usageError(fmt.Errorf("unknown command %q", remaining[0]))
	}
}

func usageError(err error) error {
	printRootUsage()
	return err
}

func printRootUsage() {
	fmt.Println(`Service Layer CLI (slctl)

Usage:
  slctl [global flags] <command> [subcommand] [flags]

Global Flags:
  --addr       Service Layer base URL (env SERVICE_LAYER_ADDR, default http://localhost:8080)
  --token      API bearer token (env SERVICE_LAYER_TOKEN)
  --timeout    HTTP timeout (default 15s)

Commands:
  accounts     Manage accounts
  functions    Manage functions and executions
  automation   Manage automation jobs
  secrets      Manage account secrets
  gasbank      Manage gas bank accounts and transfers
  oracle       Manage oracle sources and requests
  pricefeeds   Manage price feed definitions and snapshots
  random       Request deterministic randomness
  services     Introspect service descriptors
  version      Show CLI version information`)
}

type apiClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func (c *apiClient) request(ctx context.Context, method, path string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode payload: %w", err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: %s (status %d)", method, path, strings.TrimSpace(string(data)), resp.StatusCode)
	}
	return data, nil
}

func prettyPrint(data []byte) {
	if len(data) == 0 {
		fmt.Println("(empty)")
		return
	}
	var dst bytes.Buffer
	if err := json.Indent(&dst, data, "", "  "); err != nil {
		fmt.Println(string(data))
		return
	}
	fmt.Println(dst.String())
}

// ---------------------------------------------------------------------
// Services (introspection)

func handleServices(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 || args[0] == "list" {
		data, err := client.request(ctx, http.MethodGet, "/system/descriptors", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
		return nil
	}
	fmt.Println(`Usage:
  slctl services list`)
	return fmt.Errorf("unknown services subcommand %q", args[0])
}

// ---------------------------------------------------------------------
// Accounts

func handleAccounts(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl accounts list
  slctl accounts create --owner <owner> [--metadata key=value,...]
  slctl accounts get <account-id>
  slctl accounts delete <account-id>`)
		return nil
	}

	switch args[0] {
	case "list":
		data, err := client.request(ctx, http.MethodGet, "/accounts", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "create":
		fs := flag.NewFlagSet("accounts create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var owner string
		var metadataStr string
		fs.StringVar(&owner, "owner", "", "Account owner (required)")
		fs.StringVar(&metadataStr, "metadata", "", "Comma separated metadata key=value pairs")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if owner == "" {
			return errors.New("owner is required")
		}
		metadata, err := parseKeyValue(metadataStr)
		if err != nil {
			return fmt.Errorf("metadata: %w", err)
		}
		payload := map[string]any{
			"owner": owner,
		}
		if len(metadata) > 0 {
			payload["metadata"] = metadata
		}
		data, err := client.request(ctx, http.MethodPost, "/accounts", payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "get":
		if len(args) < 2 {
			return errors.New("account id required")
		}
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+args[1], nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "delete":
		if len(args) < 2 {
			return errors.New("account id required")
		}
		_, err := client.request(ctx, http.MethodDelete, "/accounts/"+args[1], nil)
		return err
	default:
		return fmt.Errorf("unknown accounts subcommand %q", args[0])
	}
	return nil
}

// ---------------------------------------------------------------------
// Functions

func handleFunctions(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl functions list --account <id>
  slctl functions create --account <id> --name <name> --source <file> [--description <text>] [--secret name,...]
  slctl functions execute --account <id> --function <id> [--payload JSON] [--payload-file path]
  slctl functions executions --account <id> --function <id> [--limit N]`)
		return nil
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("functions list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/functions", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "create":
		fs := flag.NewFlagSet("functions create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, name, sourcePath, description, secretsStr string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&name, "name", "", "Function name (required)")
		fs.StringVar(&sourcePath, "source", "", "Path to function source file (required)")
		fs.StringVar(&description, "description", "", "Optional description text")
		fs.StringVar(&secretsStr, "secrets", "", "Comma separated secret names")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || name == "" || sourcePath == "" {
			return errors.New("account, name, and source are required")
		}
		sourceBytes, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("read source file: %w", err)
		}
		var secrets []string
		if secretsStr != "" {
			secrets = splitCommaList(secretsStr)
		}
		payload := map[string]any{
			"name":        name,
			"source":      string(sourceBytes),
			"secrets":     secrets,
			"description": description,
		}
		data, err := client.request(ctx, http.MethodPost, "/accounts/"+accountID+"/functions", payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "execute":
		fs := flag.NewFlagSet("functions execute", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, functionID, payloadRaw, payloadFile string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&functionID, "function", "", "Function ID (required)")
		fs.StringVar(&payloadRaw, "payload", "", "Inline JSON payload")
		fs.StringVar(&payloadFile, "payload-file", "", "Path to JSON payload file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || functionID == "" {
			return errors.New("account and function are required")
		}
		payload, err := loadJSONPayload(payloadRaw, payloadFile)
		if err != nil {
			return err
		}
		data, err := client.request(ctx, http.MethodPost, fmt.Sprintf("/accounts/%s/functions/%s/execute", accountID, functionID), payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "executions":
		fs := flag.NewFlagSet("functions executions", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, functionID string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&functionID, "function", "", "Function ID (required)")
		fs.IntVar(&limit, "limit", 0, "Limit results (optional)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || functionID == "" {
			return errors.New("account and function are required")
		}
		path := fmt.Sprintf("/accounts/%s/functions/%s/executions", accountID, functionID)
		if limit > 0 {
			path += fmt.Sprintf("?limit=%d", limit)
		}
		data, err := client.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		return fmt.Errorf("unknown functions subcommand %q", args[0])
	}
	return nil
}

// ---------------------------------------------------------------------
// Automation

func handleAutomation(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl automation jobs list --account <id>
  slctl automation jobs create --account <id> --function <id> --name <name> --schedule <cron> [--description text]
  slctl automation jobs get --account <id> --job <id>
  slctl automation jobs set-enabled --account <id> --job <id> --enabled <true|false>`)
		return nil
	}
	if args[0] != "jobs" {
		return fmt.Errorf("unknown automation subcommand %q", args[0])
	}
	if len(args) < 2 {
		return fmt.Errorf("automation jobs requires a subcommand")
	}
	switch args[1] {
	case "list":
		fs := flag.NewFlagSet("automation jobs list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/automation/jobs", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "create":
		fs := flag.NewFlagSet("automation jobs create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, functionID, name, schedule, description string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&functionID, "function", "", "Function ID (required)")
		fs.StringVar(&name, "name", "", "Job name (required)")
		fs.StringVar(&schedule, "schedule", "", "Cron schedule (required)")
		fs.StringVar(&description, "description", "", "Description")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if accountID == "" || functionID == "" || name == "" || schedule == "" {
			return errors.New("account, function, name, and schedule are required")
		}
		payload := map[string]any{
			"function_id": functionID,
			"name":        name,
			"schedule":    schedule,
			"description": description,
		}
		data, err := client.request(ctx, http.MethodPost, "/accounts/"+accountID+"/automation/jobs", payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "get":
		fs := flag.NewFlagSet("automation jobs get", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, jobID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&jobID, "job", "", "Job ID (required)")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if accountID == "" || jobID == "" {
			return errors.New("account and job are required")
		}
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/automation/jobs/"+jobID, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "set-enabled":
		fs := flag.NewFlagSet("automation jobs set-enabled", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, jobID string
		var enabled bool
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&jobID, "job", "", "Job ID (required)")
		fs.BoolVar(&enabled, "enabled", false, "Enable or disable the job")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if accountID == "" || jobID == "" {
			return errors.New("account and job are required")
		}
		payload := map[string]any{"enabled": enabled}
		data, err := client.request(ctx, http.MethodPatch, "/accounts/"+accountID+"/automation/jobs/"+jobID, payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		return fmt.Errorf("unknown automation jobs subcommand %q", args[1])
	}
	return nil
}

// ---------------------------------------------------------------------
// Secrets

func handleSecrets(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl secrets list --account <id>
  slctl secrets create --account <id> --name <name> --value <value>
  slctl secrets get --account <id> --name <name>
  slctl secrets delete --account <id> --name <name>`)
		return nil
	}
	sub := args[0]
	fs := flag.NewFlagSet("secrets "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var accountID, name, value string
	fs.StringVar(&accountID, "account", "", "Account ID (required)")
	fs.StringVar(&name, "name", "", "Secret name")
	fs.StringVar(&value, "value", "", "Secret value (create only)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if accountID == "" {
		return errors.New("account is required")
	}
	switch sub {
	case "list":
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/secrets", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "create":
		if name == "" || value == "" {
			return errors.New("name and value are required")
		}
		payload := map[string]any{"name": name, "value": value}
		data, err := client.request(ctx, http.MethodPost, "/accounts/"+accountID+"/secrets", payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "get":
		if name == "" {
			return errors.New("name is required")
		}
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/secrets/"+name, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "delete":
		if name == "" {
			return errors.New("name is required")
		}
		_, err := client.request(ctx, http.MethodDelete, "/accounts/"+accountID+"/secrets/"+name, nil)
		return err
	default:
		return fmt.Errorf("unknown secrets subcommand %q", sub)
	}
	return nil
}

// ---------------------------------------------------------------------
// Gas Bank

func handleGasBank(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl gasbank ensure --account <id> [--wallet address]
  slctl gasbank list --account <id>
  slctl gasbank deposit --account <id> --gas-account <id> --amount <float> [--tx-id id] [--from addr] [--to addr]
  slctl gasbank withdraw --account <id> --gas-account <id> --amount <float> [--to addr]
  slctl gasbank transactions --account <id> --gas-account <id>`)
		return nil
	}
	switch args[0] {
	case "ensure":
		fs := flag.NewFlagSet("gasbank ensure", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, wallet string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&wallet, "wallet", "", "Wallet address")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		payload := map[string]any{"wallet_address": wallet}
		data, err := client.request(ctx, http.MethodPost, "/accounts/"+accountID+"/gasbank", payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "list":
		fs := flag.NewFlagSet("gasbank list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/gasbank", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "deposit":
		fs := flag.NewFlagSet("gasbank deposit", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, gasAccountID, txID, from, to string
		var amount float64
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&gasAccountID, "gas-account", "", "Gas account ID (required)")
		fs.Float64Var(&amount, "amount", 0, "Amount to deposit (required)")
		fs.StringVar(&txID, "tx-id", "", "Blockchain transaction ID")
		fs.StringVar(&from, "from", "", "From address")
		fs.StringVar(&to, "to", "", "To address")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || gasAccountID == "" || amount <= 0 {
			return errors.New("account, gas-account, and positive amount are required")
		}
		payload := map[string]any{
			"gas_account_id": gasAccountID,
			"amount":         amount,
			"tx_id":          txID,
			"from_address":   from,
			"to_address":     to,
		}
		data, err := client.request(ctx, http.MethodPost, "/accounts/"+accountID+"/gasbank/deposit", payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "withdraw":
		fs := flag.NewFlagSet("gasbank withdraw", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, gasAccountID, to string
		var amount float64
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&gasAccountID, "gas-account", "", "Gas account ID (required)")
		fs.Float64Var(&amount, "amount", 0, "Amount to withdraw (required)")
		fs.StringVar(&to, "to", "", "Destination address")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || gasAccountID == "" || amount <= 0 {
			return errors.New("account, gas-account, and positive amount are required")
		}
		payload := map[string]any{
			"gas_account_id": gasAccountID,
			"amount":         amount,
			"to_address":     to,
		}
		data, err := client.request(ctx, http.MethodPost, "/accounts/"+accountID+"/gasbank/withdraw", payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "transactions":
		fs := flag.NewFlagSet("gasbank transactions", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, gasAccountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&gasAccountID, "gas-account", "", "Gas account ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || gasAccountID == "" {
			return errors.New("account and gas-account are required")
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/transactions?gas_account_id=%s", accountID, gasAccountID)
		data, err := client.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		return fmt.Errorf("unknown gasbank subcommand %q", args[0])
	}
	return nil
}

// ---------------------------------------------------------------------
// Oracle

func handleOracle(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl oracle sources list --account <id>
  slctl oracle sources create --account <id> --name <name> --url <url> [--method GET] [--description text]
  slctl oracle sources get --account <id> --source <id>
  slctl oracle requests list --account <id>
  slctl oracle requests create --account <id> --source <id> [--payload JSON] [--payload-file path]`)
		return nil
	}
	switch args[0] {
	case "sources":
		if len(args) < 2 {
			return fmt.Errorf("oracle sources requires a subcommand")
		}
		return handleOracleSources(ctx, client, args[1:])
	case "requests":
		if len(args) < 2 {
			return fmt.Errorf("oracle requests requires a subcommand")
		}
		return handleOracleRequests(ctx, client, args[1:])
	default:
		return fmt.Errorf("unknown oracle subcommand %q", args[0])
	}
}

func handleOracleSources(ctx context.Context, client *apiClient, args []string) error {
	sub := args[0]
	fs := flag.NewFlagSet("oracle sources "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var accountID, sourceID, name, urlStr, method, description string
	fs.StringVar(&accountID, "account", "", "Account ID (required)")
	fs.StringVar(&sourceID, "source", "", "Source ID")
	fs.StringVar(&name, "name", "", "Source name")
	fs.StringVar(&urlStr, "url", "", "Source URL")
	fs.StringVar(&method, "method", "GET", "HTTP method")
	fs.StringVar(&description, "description", "", "Description")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if accountID == "" {
		return errors.New("account is required")
	}
	switch sub {
	case "list":
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/oracle/sources", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "create":
		if name == "" || urlStr == "" {
			return errors.New("name and url are required")
		}
		payload := map[string]any{
			"name":        name,
			"url":         urlStr,
			"method":      method,
			"description": description,
		}
		data, err := client.request(ctx, http.MethodPost, "/accounts/"+accountID+"/oracle/sources", payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "get":
		if sourceID == "" {
			return errors.New("source is required")
		}
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/oracle/sources/"+sourceID, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		return fmt.Errorf("unknown oracle sources subcommand %q", sub)
	}
	return nil
}

func handleOracleRequests(ctx context.Context, client *apiClient, args []string) error {
	sub := args[0]
	fs := flag.NewFlagSet("oracle requests "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var accountID, sourceID, payloadRaw, payloadFile string
	fs.StringVar(&accountID, "account", "", "Account ID (required)")
	fs.StringVar(&sourceID, "source", "", "Source ID")
	fs.StringVar(&payloadRaw, "payload", "", "Inline JSON payload")
	fs.StringVar(&payloadFile, "payload-file", "", "Path to JSON payload file")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if accountID == "" {
		return errors.New("account is required")
	}
	switch sub {
	case "list":
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/oracle/requests", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "create":
		if sourceID == "" {
			return errors.New("source is required")
		}
		payloadBody, err := loadJSONPayload(payloadRaw, payloadFile)
		if err != nil {
			return err
		}
		requestPayload := map[string]any{
			"data_source_id": sourceID,
		}
		if payloadBody != nil {
			requestPayload["payload"] = payloadBody
		}
		data, err := client.request(ctx, http.MethodPost, "/accounts/"+accountID+"/oracle/requests", requestPayload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		return fmt.Errorf("unknown oracle requests subcommand %q", sub)
	}
	return nil
}

// ---------------------------------------------------------------------
// Price Feeds

func handlePriceFeeds(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl pricefeeds list --account <id>
  slctl pricefeeds create --account <id> --base <asset> --quote <asset> [--interval "@every 1m"] [--heartbeat "@every 10m"] --deviation <float>
  slctl pricefeeds get --account <id> --feed <id>
  slctl pricefeeds snapshots --account <id> --feed <id>`)
		return nil
	}
	sub := args[0]
	fs := flag.NewFlagSet("pricefeeds "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var accountID, feedID, base, quote, interval, heartbeat string
	var deviation float64
	fs.StringVar(&accountID, "account", "", "Account ID (required)")
	fs.StringVar(&feedID, "feed", "", "Feed ID")
	fs.StringVar(&base, "base", "", "Base asset")
	fs.StringVar(&quote, "quote", "", "Quote asset")
	fs.StringVar(&interval, "interval", "", "Update interval")
	fs.StringVar(&heartbeat, "heartbeat", "", "Heartbeat interval")
	fs.Float64Var(&deviation, "deviation", 0, "Deviation percent")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if accountID == "" {
		return errors.New("account is required")
	}
	switch sub {
	case "list":
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/pricefeeds", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "create":
		if base == "" || quote == "" || deviation <= 0 {
			return errors.New("base, quote, and positive deviation are required")
		}
		payload := map[string]any{
			"base_asset":         base,
			"quote_asset":        quote,
			"update_interval":    interval,
			"heartbeat_interval": heartbeat,
			"deviation_percent":  deviation,
		}
		data, err := client.request(ctx, http.MethodPost, "/accounts/"+accountID+"/pricefeeds", payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "get":
		if feedID == "" {
			return errors.New("feed is required")
		}
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/pricefeeds/"+feedID, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "snapshots":
		if feedID == "" {
			return errors.New("feed is required")
		}
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/pricefeeds/"+feedID+"/snapshots", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		return fmt.Errorf("unknown pricefeeds subcommand %q", sub)
	}
	return nil
}

// ---------------------------------------------------------------------
// Randomness

func handleRandom(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl random list --account <id> [--limit 10]
  slctl random generate --account <id> [--length 32] [--request-id <id>]`)
		return nil
	}
	sub := args[0]
	switch sub {
	case "generate":
		fs := flag.NewFlagSet("random generate", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, requestID string
		var length int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.IntVar(&length, "length", 32, "Number of random bytes (1-1024)")
		fs.StringVar(&requestID, "request-id", "", "Optional request identifier")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		if length <= 0 {
			length = 32
		}
		payload := map[string]any{
			"length": length,
		}
		if strings.TrimSpace(requestID) != "" {
			payload["request_id"] = requestID
		}
		data, err := client.request(ctx, http.MethodPost, "/accounts/"+accountID+"/random", payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "list":
		fs := flag.NewFlagSet("random list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.IntVar(&limit, "limit", 10, "Number of results to show")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		if limit <= 0 {
			limit = 10
		}
		path := fmt.Sprintf("/accounts/%s/random/requests?limit=%d", accountID, limit)
		data, err := client.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		fmt.Println(`Usage:
  slctl random list --account <id> [--limit 10]
  slctl random generate --account <id> [--length 32] [--request-id <id>]`)
		return fmt.Errorf("unknown random subcommand %q", sub)
	}
	return nil
}

// ---------------------------------------------------------------------
// Helpers

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseKeyValue(input string) (map[string]string, error) {
	result := make(map[string]string)
	if strings.TrimSpace(input) == "" {
		return result, nil
	}
	pairs := strings.Split(input, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid entry %q (expected key=value)", pair)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("empty key in %q", pair)
		}
		result[key] = value
	}
	return result, nil
}

func splitCommaList(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	var result []string
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func loadJSONPayload(inline, file string) (map[string]any, error) {
	if inline != "" && file != "" {
		return nil, errors.New("specify either --payload or --payload-file, not both")
	}
	var data []byte
	switch {
	case inline != "":
		data = []byte(inline)
	case file != "":
		content, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			return nil, fmt.Errorf("read payload file: %w", err)
		}
		data = content
	default:
		return nil, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	return payload, nil
}
