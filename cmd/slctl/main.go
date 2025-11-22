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
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/version"
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
	showVersion := root.Bool("version", false, "Print slctl build information and exit")
	if err := root.Parse(args); err != nil {
		return usageError(err)
	}

	remaining := root.Args()
	if len(remaining) == 0 {
		return usageError(errors.New("no command specified"))
	}
	if *showVersion {
		fmt.Println(version.FullVersion())
		return nil
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
	case "health":
		return handleHealth(ctx, client, remaining[1:])
	case "pricefeeds":
		return handlePriceFeeds(ctx, client, remaining[1:])
	case "random":
		return handleRandom(ctx, client, remaining[1:])
	case "cre":
		return handleCRE(ctx, client, remaining[1:])
	case "ccip":
		return handleCCIP(ctx, client, remaining[1:])
	case "vrf":
		return handleVRF(ctx, client, remaining[1:])
	case "datalink":
		return handleDataLink(ctx, client, remaining[1:])
	case "dta":
		return handleDTA(ctx, client, remaining[1:])
	case "datastreams":
		return handleDataStreams(ctx, client, remaining[1:])
	case "confcompute":
		return handleConfCompute(ctx, client, remaining[1:])
	case "workspace-wallets":
		return handleWorkspaceWallets(ctx, client, remaining[1:])
	case "jam":
		return handleJAM(ctx, client, remaining[1:])
	case "status":
		return handleStatus(ctx, client)
	case "services":
		return handleServices(ctx, client, remaining[1:])
	case "version":
		return handleVersion(ctx, client)
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
  --version    Print CLI build information and exit

Commands:
  accounts     Manage accounts
  functions    Manage functions and executions
  automation   Manage automation jobs
  secrets      Manage account secrets
  gasbank      Manage gas bank accounts and transfers
  oracle       Manage oracle sources and requests
  health       Show oracle/datafeed health for an account
  pricefeeds   Manage price feed definitions and snapshots
  random       Request deterministic randomness
  cre          Inspect Chainlink Reliability Engine assets
  ccip         Inspect cross-chain lanes and messages
  vrf          Inspect VRF keys and requests
  datalink     Inspect channel configurations and deliveries
  dta          Inspect digital transfer agency products and orders
  datastreams  Inspect stream configurations and frames
  confcompute  Inspect confidential-compute enclaves
  workspace-wallets Inspect linked signing wallets
  jam          Interact with JAM prototype (preimages, packages, reports)
  services     Introspect service descriptors
  status       Show health/version/descriptors summary (uses /system/status; health at /healthz is unauthenticated)
  version      Show CLI and server version information`)
}

type apiClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func (c *apiClient) request(ctx context.Context, method, path string, payload any) ([]byte, error) {
	data, _, err := c.requestWithHeaders(ctx, method, path, payload)
	return data, err
}

// requestRaw sends a request with an arbitrary body and content type.
func (c *apiClient) requestRaw(ctx context.Context, method, path string, body []byte, contentType string) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.Header, err
	}
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		if len(msg) > 0 {
			var parsed map[string]any
			if err := json.Unmarshal(data, &parsed); err == nil {
				if errStr, ok := parsed["error"].(string); ok && errStr != "" {
					msg = errStr
				}
				if code, ok := parsed["code"].(string); ok && code != "" {
					msg = fmt.Sprintf("%s (%s)", msg, code)
				}
			}
		}
		return nil, resp.Header, fmt.Errorf("%s %s: %s (status %d)", method, path, msg, resp.StatusCode)
	}
	return data, resp.Header, nil
}

func (c *apiClient) requestWithHeaders(ctx context.Context, method, path string, payload any) ([]byte, http.Header, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, fmt.Errorf("encode payload: %w", err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.Header, err
	}

	if resp.StatusCode >= 300 {
		return nil, resp.Header, fmt.Errorf("%s %s: %s (status %d)", method, path, strings.TrimSpace(string(data)), resp.StatusCode)
	}
	return data, resp.Header, nil
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

func parseJSONMap(input string) (map[string]any, error) {
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func splitList(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ';'
	})
	var out []string
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func toInt(v any) (int, bool) {
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int:
		return val, true
	case int64:
		return int(val), true
	}
	return 0, false
}

func toInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case float64:
		return int64(val), true
	case int:
		return int64(val), true
	case int64:
		return val, true
	}
	return 0, false
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

func handleStatus(ctx context.Context, client *apiClient) error {
	data, err := client.request(ctx, http.MethodGet, "/system/status", nil)
	if err != nil {
		return err
	}
	var payload struct {
		Status  string `json:"status"`
		Version struct {
			Version   string `json:"version"`
			Commit    string `json:"commit"`
			BuiltAt   string `json:"built_at"`
			GoVersion string `json:"go_version"`
		} `json:"version"`
		Services []map[string]any `json:"services"`
		JAM      map[string]any   `json:"jam"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode status payload: %w", err)
	}
	fmt.Printf("Status: %s\n", payload.Status)
	fmt.Printf("Version: %s (commit %s, built %s, %s)\n", payload.Version.Version, payload.Version.Commit, payload.Version.BuiltAt, payload.Version.GoVersion)
	if len(payload.JAM) > 0 {
		enabled, _ := payload.JAM["enabled"].(bool)
		store, _ := payload.JAM["store"].(string)
		rate, _ := toInt(payload.JAM["rate_limit_per_min"])
		preimageMax, _ := toInt64(payload.JAM["max_preimage_bytes"])
		pendingMax, _ := toInt(payload.JAM["max_pending_packages"])
		authReq, _ := payload.JAM["auth_required"].(bool)
		legacyList, _ := payload.JAM["legacy_list_response"].(bool)
		fmt.Printf("JAM: enabled=%t", enabled)
		if store != "" {
			fmt.Printf(" store=%s", store)
		}
		if rate > 0 {
			fmt.Printf(" rate_limit_per_min=%d", rate)
		}
		if preimageMax > 0 {
			fmt.Printf(" max_preimage_bytes=%d", preimageMax)
		}
		if pendingMax > 0 {
			fmt.Printf(" max_pending_packages=%d", pendingMax)
		}
		if authReq {
			fmt.Printf(" auth_required=%t", authReq)
		}
		if legacyList {
			fmt.Printf(" legacy_list_response=%t", legacyList)
		}
		fmt.Println()
	}
	if len(payload.Services) > 0 {
		fmt.Println("Services:")
		for _, svc := range payload.Services {
			name, _ := svc["Name"].(string)
			domain, _ := svc["Domain"].(string)
			caps, _ := svc["Capabilities"].([]any)
			var capStrings []string
			for _, capVal := range caps {
				if s, ok := capVal.(string); ok {
					capStrings = append(capStrings, s)
				}
			}
			fmt.Printf("  - %s (%s) caps=%s\n", name, domain, strings.Join(capStrings, ","))
		}
	}
	return nil
}

// handleHealth inspects oracle/datafeed health for an account.
func handleHealth(ctx context.Context, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var accountID string
	includeOracle := fs.Bool("oracle", true, "Include oracle health")
	includeDatafeeds := fs.Bool("datafeeds", true, "Include datafeed health")
	fs.StringVar(&accountID, "account", "", "Account ID (required)")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	if accountID == "" {
		return usageError(errors.New("account is required (use --account)"))
	}

	if *includeOracle {
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/oracle/requests", nil)
		if err != nil {
			return err
		}
		var requests []struct {
			Status    string `json:"status"`
			Attempts  int    `json:"attempts"`
			CreatedAt string `json:"created_at"`
		}
		_ = json.Unmarshal(data, &requests)
		var pending, running, failed, succeeded, maxAttempts int
		var oldestPending time.Time
		for _, req := range requests {
			switch strings.ToLower(req.Status) {
			case "pending":
				pending++
				if t, err := time.Parse(time.RFC3339, req.CreatedAt); err == nil {
					if oldestPending.IsZero() || t.Before(oldestPending) {
						oldestPending = t
					}
				}
			case "running":
				running++
			case "failed":
				failed++
			case "succeeded":
				succeeded++
			}
			if req.Attempts > maxAttempts {
				maxAttempts = req.Attempts
			}
		}
		fmt.Printf("Oracle: pending=%d running=%d failed=%d succeeded=%d max_attempts=%d", pending, running, failed, succeeded, maxAttempts)
		if !oldestPending.IsZero() {
			fmt.Printf(" oldest_pending=%s", time.Since(oldestPending).Round(time.Second))
		}
		fmt.Println()
	}

	if *includeDatafeeds {
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/datafeeds", nil)
		if err != nil {
			return err
		}
		var feeds []struct {
			ID           string   `json:"id"`
			Pair         string   `json:"pair"`
			Heartbeat    int64    `json:"heartbeat"`
			ThresholdPPM int      `json:"threshold_ppm"`
			SignerSet    []string `json:"signer_set"`
			Decimals     int      `json:"decimals"`
		}
		_ = json.Unmarshal(data, &feeds)
		if len(feeds) == 0 {
			fmt.Println("Datafeeds: none configured")
			return nil
		}
		fmt.Println("Datafeeds:")
		for _, feed := range feeds {
			latestData, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/datafeeds/"+feed.ID+"/latest", nil)
			var latest struct {
				RoundID   int64  `json:"round_id"`
				Price     string `json:"price"`
				Timestamp string `json:"timestamp"`
				Signer    string `json:"signer"`
			}
			if err == nil {
				_ = json.Unmarshal(latestData, &latest)
			}
			heartbeat := time.Duration(feed.Heartbeat)
			if heartbeat == 0 && feed.Heartbeat > 0 {
				heartbeat = time.Duration(feed.Heartbeat)
			}
			var age time.Duration
			if latest.Timestamp != "" {
				if ts, err := time.Parse(time.RFC3339, latest.Timestamp); err == nil {
					age = time.Since(ts)
				}
			}
			status := "empty"
			if latest.Timestamp != "" {
				status = "healthy"
				if heartbeat > 0 && age > heartbeat {
					status = "stale"
				}
			}
			fmt.Printf("- %s (round %d): %s", feed.Pair, latest.RoundID, status)
			if latest.Price != "" {
				fmt.Printf(" price=%s", latest.Price)
			}
			if heartbeat > 0 {
				fmt.Printf(" heartbeat=%s", heartbeat)
			}
			if feed.ThresholdPPM > 0 {
				fmt.Printf(" deviation<=%dppm", feed.ThresholdPPM)
			}
			if age > 0 {
				fmt.Printf(" age=%s", age.Round(time.Second))
			}
			if len(feed.SignerSet) > 0 {
				fmt.Printf(" signers=%d", len(feed.SignerSet))
			}
			fmt.Println()
		}
	}

	return nil
}

func handleVersion(ctx context.Context, client *apiClient) error {
	fmt.Printf("slctl: %s\n", version.FullVersion())
	data, err := client.request(ctx, http.MethodGet, "/system/version", nil)
	if err != nil {
		return err
	}
	var payload struct {
		Version   string `json:"version"`
		Commit    string `json:"commit"`
		BuiltAt   string `json:"built_at"`
		GoVersion string `json:"go_version"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode server version: %w", err)
	}
	fmt.Printf("server[%s]: %s (commit %s, built %s, %s)\n", client.baseURL, payload.Version, payload.Commit, payload.BuiltAt, payload.GoVersion)
	return nil
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
  slctl gasbank summary --account <id>
  slctl gasbank ensure --account <id> [--wallet address]
  slctl gasbank list --account <id>
  slctl gasbank deposit --account <id> --gas-account <id> --amount <float> [--tx-id id] [--from addr] [--to addr]
  slctl gasbank withdraw --account <id> --gas-account <id> --amount <float> [--to addr]
  slctl gasbank transactions --account <id> --gas-account <id> [--status <status>] [--type <type>] [--limit N]
  slctl gasbank deposits list --account <id> --gas-account <id> [--limit N]
  slctl gasbank withdrawals list --account <id> --gas-account <id> [--status <status>] [--limit N]
  slctl gasbank approvals list --account <id> --transaction <id>
  slctl gasbank approvals submit --account <id> --transaction <id> --approver <id> [--approve] [--note text]
  slctl gasbank settlement deadletters list|retry|delete ...`)
		return nil
	}
	switch args[0] {
	case "summary":
		fs := flag.NewFlagSet("gasbank summary", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		data, err := client.request(ctx, http.MethodGet, "/accounts/"+accountID+"/gasbank/summary", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
		return nil
	case "ensure":
		fs := flag.NewFlagSet("gasbank ensure", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, wallet string
		var minBalance, dailyLimit, notificationThreshold floatFlag
		var requiredApprovals intFlag
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&wallet, "wallet", "", "Wallet address")
		fs.Var(&minBalance, "min-balance", "Minimum balance threshold")
		fs.Var(&dailyLimit, "daily-limit", "Daily withdrawal limit")
		fs.Var(&notificationThreshold, "notification-threshold", "Notification threshold for balances")
		fs.Var(&requiredApprovals, "required-approvals", "Required approvals for withdrawals")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		payload := map[string]any{"wallet_address": wallet}
		if minBalance.set {
			payload["min_balance"] = minBalance.value
		}
		if dailyLimit.set {
			payload["daily_limit"] = dailyLimit.value
		}
		if notificationThreshold.set {
			payload["notification_threshold"] = notificationThreshold.value
		}
		if requiredApprovals.set {
			payload["required_approvals"] = requiredApprovals.value
		}
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
		var accountID, gasAccountID, to, scheduleAt string
		var amount float64
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&gasAccountID, "gas-account", "", "Gas account ID (required)")
		fs.Float64Var(&amount, "amount", 0, "Amount to withdraw (required)")
		fs.StringVar(&to, "to", "", "Destination address")
		fs.StringVar(&scheduleAt, "schedule-at", "", "Schedule time (RFC3339)")
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
		if strings.TrimSpace(scheduleAt) != "" {
			payload["schedule_at"] = scheduleAt
		}
		data, err := client.request(ctx, http.MethodPost, "/accounts/"+accountID+"/gasbank/withdraw", payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "transactions":
		fs := flag.NewFlagSet("gasbank transactions", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, gasAccountID, status, txType string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&gasAccountID, "gas-account", "", "Gas account ID (required)")
		fs.StringVar(&status, "status", "", "Filter by transaction status")
		fs.StringVar(&txType, "type", "", "Filter by transaction type")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || gasAccountID == "" {
			return errors.New("account and gas-account are required")
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/transactions?gas_account_id=%s", accountID, gasAccountID)
		if strings.TrimSpace(status) != "" {
			path += "&status=" + url.QueryEscape(status)
		}
		if strings.TrimSpace(txType) != "" {
			path += "&type=" + url.QueryEscape(txType)
		}
		data, err := client.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "deposits":
		if len(args) < 2 {
			return fmt.Errorf("gasbank deposits requires a subcommand")
		}
		return handleGasBankDeposits(ctx, client, args[1:])
	case "withdrawals":
		if len(args) < 2 {
			return fmt.Errorf("gasbank withdrawals requires a subcommand")
		}
		return handleGasBankWithdrawals(ctx, client, args[1:])
	case "settlement":
		if len(args) < 2 {
			return fmt.Errorf("gasbank settlement requires a subcommand")
		}
		return handleGasBankSettlement(ctx, client, args[1:])
	case "approvals":
		if len(args) < 2 {
			return fmt.Errorf("gasbank approvals requires a subcommand")
		}
		return handleGasBankApprovals(ctx, client, args[1:])
	default:
		return fmt.Errorf("unknown gasbank subcommand %q", args[0])
	}
	return nil
}

func handleGasBankApprovals(ctx context.Context, client *apiClient, args []string) error {
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("gasbank approvals list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, transactionID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&transactionID, "transaction", "", "Transaction ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || transactionID == "" {
			return errors.New("account and transaction are required")
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/approvals/%s", accountID, transactionID)
		data, err := client.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "submit":
		fs := flag.NewFlagSet("gasbank approvals submit", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, transactionID, approver, note, signature string
		approve := fs.Bool("approve", false, "Approve (default false = reject)")
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&transactionID, "transaction", "", "Transaction ID (required)")
		fs.StringVar(&approver, "approver", "", "Approver identifier (required)")
		fs.StringVar(&note, "note", "", "Optional note")
		fs.StringVar(&signature, "signature", "", "Optional signature")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || transactionID == "" || approver == "" {
			return errors.New("account, transaction, and approver are required")
		}
		payload := map[string]any{
			"approver":  approver,
			"approve":   *approve,
			"note":      note,
			"signature": signature,
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/approvals/%s", accountID, transactionID)
		data, err := client.request(ctx, http.MethodPost, path, payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		return fmt.Errorf("unknown gasbank approvals subcommand %q", args[0])
	}
	return nil
}

func handleGasBankDeposits(ctx context.Context, client *apiClient, args []string) error {
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("gasbank deposits list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, gasAccountID string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&gasAccountID, "gas-account", "", "Gas account ID (required)")
		fs.IntVar(&limit, "limit", 25, "Maximum deposits to return")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || gasAccountID == "" {
			return errors.New("account and gas-account are required")
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/deposits?gas_account_id=%s&limit=%d", accountID, gasAccountID, limit)
		data, err := client.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		return fmt.Errorf("unknown gasbank deposits subcommand %q", args[0])
	}
	return nil
}

func handleGasBankWithdrawals(ctx context.Context, client *apiClient, args []string) error {
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("gasbank withdrawals list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, gasAccountID, status string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&gasAccountID, "gas-account", "", "Gas account ID (required)")
		fs.StringVar(&status, "status", "", "Filter by withdrawal status")
		fs.IntVar(&limit, "limit", 25, "Maximum withdrawals to return")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || gasAccountID == "" {
			return errors.New("account and gas-account are required")
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/withdrawals?gas_account_id=%s&limit=%d", accountID, gasAccountID, limit)
		if strings.TrimSpace(status) != "" {
			path += "&status=" + url.QueryEscape(status)
		}
		data, err := client.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "get":
		fs := flag.NewFlagSet("gasbank withdrawals get", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, transactionID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&transactionID, "transaction", "", "Transaction ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || transactionID == "" {
			return errors.New("account and transaction are required")
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/withdrawals/%s", accountID, transactionID)
		data, err := client.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "cancel":
		fs := flag.NewFlagSet("gasbank withdrawals cancel", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, transactionID, reason string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&transactionID, "transaction", "", "Transaction ID (required)")
		fs.StringVar(&reason, "reason", "", "Cancellation reason")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || transactionID == "" {
			return errors.New("account and transaction are required")
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/withdrawals/%s", accountID, transactionID)
		payload := map[string]any{"action": "cancel", "reason": reason}
		data, err := client.request(ctx, http.MethodPatch, path, payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "attempts":
		fs := flag.NewFlagSet("gasbank withdrawals attempts", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, transactionID string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&transactionID, "transaction", "", "Transaction ID (required)")
		fs.IntVar(&limit, "limit", 25, "Maximum attempts to return")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || transactionID == "" {
			return errors.New("account and transaction are required")
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/withdrawals/%s/attempts?limit=%d", accountID, transactionID, limit)
		data, err := client.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		return fmt.Errorf("unknown gasbank withdrawals subcommand %q", args[0])
	}
	return nil
}

func handleGasBankSettlement(ctx context.Context, client *apiClient, args []string) error {
	switch args[0] {
	case "deadletters":
		if len(args) < 2 {
			return fmt.Errorf("gasbank settlement deadletters requires a subcommand")
		}
		return handleGasBankDeadLetters(ctx, client, args[1:])
	default:
		return fmt.Errorf("unknown gasbank settlement subcommand %q", args[0])
	}
}

func handleGasBankDeadLetters(ctx context.Context, client *apiClient, args []string) error {
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("gasbank settlement deadletters list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.IntVar(&limit, "limit", 25, "Maximum dead letters to return")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/deadletters?limit=%d", accountID, limit)
		data, err := client.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "retry":
		fs := flag.NewFlagSet("gasbank settlement deadletters retry", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, transactionID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&transactionID, "transaction", "", "Transaction ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || transactionID == "" {
			return errors.New("account and transaction are required")
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/deadletters/%s/retry", accountID, transactionID)
		data, err := client.request(ctx, http.MethodPost, path, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "delete":
		fs := flag.NewFlagSet("gasbank settlement deadletters delete", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, transactionID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&transactionID, "transaction", "", "Transaction ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || transactionID == "" {
			return errors.New("account and transaction are required")
		}
		path := fmt.Sprintf("/accounts/%s/gasbank/deadletters/%s", accountID, transactionID)
		if _, err := client.request(ctx, http.MethodDelete, path, nil); err != nil {
			return err
		}
		fmt.Println("Dead letter deleted.")
	default:
		return fmt.Errorf("unknown gasbank settlement deadletters subcommand %q", args[0])
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
  slctl oracle requests list --account <id> [--limit n] [--status pending|running|failed|succeeded] [--cursor <id>] [--all]
  slctl oracle requests create --account <id> --source <id> [--payload JSON] [--payload-file path] [--alternate <id>[,<id>...]]
  slctl oracle requests retry --account <id> --request <id>

Runner callbacks:
  export ORACLE_RUNNER_TOKENS=runner-1,runner-2   # accepted tokens (also in config files)
  curl -X PATCH /accounts/<id>/oracle/requests/<req> \
    -H "Authorization: Bearer $TOKEN" \
    -H "X-Oracle-Runner-Token: runner-1" \
    -d '{"status":"running"}'`)
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

type floatFlag struct {
	set   bool
	value float64
}

func (f *floatFlag) String() string {
	if !f.set {
		return ""
	}
	return fmt.Sprintf("%f", f.value)
}

func (f *floatFlag) Set(v string) error {
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return err
	}
	f.value = parsed
	f.set = true
	return nil
}

type intFlag struct {
	set   bool
	value int
}

func (f *intFlag) String() string {
	if !f.set {
		return ""
	}
	return fmt.Sprintf("%d", f.value)
}

func (f *intFlag) Set(v string) error {
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return err
	}
	f.value = parsed
	f.set = true
	return nil
}

type stringSliceFlag struct {
	values []string
}

func (s *stringSliceFlag) String() string {
	return strings.Join(s.values, ",")
}

func (s *stringSliceFlag) Set(v string) error {
	parts := strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == ';'
	})
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		s.values = append(s.values, p)
	}
	return nil
}

func handleOracleRequests(ctx context.Context, client *apiClient, args []string) error {
	sub := args[0]
	fs := flag.NewFlagSet("oracle requests "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var accountID, sourceID, payloadRaw, payloadFile string
	var alternates stringSliceFlag
	var statusFilter string
	var limit int
	var cursor string
	var fetchAll bool
	fs.StringVar(&accountID, "account", "", "Account ID (required)")
	fs.StringVar(&sourceID, "source", "", "Source ID")
	fs.StringVar(&payloadRaw, "payload", "", "Inline JSON payload")
	fs.StringVar(&payloadFile, "payload-file", "", "Path to JSON payload file")
	fs.Var(&alternates, "alternate", "Alternate data source IDs (comma-separated, repeatable)")
	fs.StringVar(&statusFilter, "status", "", "Filter by status (pending,running,failed,succeeded)")
	fs.IntVar(&limit, "limit", 100, "Limit number of requests returned")
	fs.StringVar(&cursor, "cursor", "", "Cursor for pagination (use value from X-Next-Cursor)")
	fs.BoolVar(&fetchAll, "all", false, "Follow cursors until the queue is exhausted")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if accountID == "" {
		return errors.New("account is required")
	}
	switch sub {
	case "list":
		path := "/accounts/" + accountID + "/oracle/requests"
		params := url.Values{}
		if statusFilter != "" {
			params.Set("status", statusFilter)
		}
		if limit > 0 {
			params.Set("limit", strconv.Itoa(limit))
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		pagePath := path
		if len(params) > 0 {
			pagePath += "?" + params.Encode()
		}
		if !fetchAll {
			data, headers, err := client.requestWithHeaders(ctx, http.MethodGet, pagePath, nil)
			if err != nil {
				return err
			}
			prettyPrint(data)
			if next := headers.Get("X-Next-Cursor"); next != "" {
				fmt.Println("\nNext cursor:", next)
			}
			return nil
		}

		allItems := make([]json.RawMessage, 0)
		nextCursor := cursor
		const maxPages = 100
		for i := 0; i < maxPages; i++ {
			pageParams := url.Values{}
			for k, vals := range params {
				for _, v := range vals {
					pageParams.Add(k, v)
				}
			}
			if nextCursor != "" {
				pageParams.Set("cursor", nextCursor)
			}
			pageURL := path
			if len(pageParams) > 0 {
				pageURL += "?" + pageParams.Encode()
			}
			data, headers, err := client.requestWithHeaders(ctx, http.MethodGet, pageURL, nil)
			if err != nil {
				return err
			}
			var page []json.RawMessage
			if err := json.Unmarshal(data, &page); err != nil {
				return fmt.Errorf("decode page: %w", err)
			}
			allItems = append(allItems, page...)
			nextCursor = headers.Get("X-Next-Cursor")
			if nextCursor == "" || len(page) == 0 {
				break
			}
		}
		combined, err := json.MarshalIndent(allItems, "", "  ")
		if err != nil {
			return fmt.Errorf("encode combined: %w", err)
		}
		fmt.Println(string(combined))
		return nil
	case "create":
		if sourceID == "" {
			return errors.New("source is required")
		}
		payloadBody, err := loadJSONPayload(payloadRaw, payloadFile)
		if err != nil {
			return err
		}
		if len(alternates.values) > 0 {
			if payloadBody == nil {
				payloadBody = make(map[string]any)
			}
			if obj, ok := payloadBody.(map[string]any); ok {
				obj["alternate_source_ids"] = alternates.values
				payloadBody = obj
			} else {
				return fmt.Errorf("payload must be a JSON object when using --alternate")
			}
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
	case "retry":
		fs := flag.NewFlagSet("oracle requests retry", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var requestID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&requestID, "request", "", "Request ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || requestID == "" {
			return errors.New("account and request are required")
		}
		body := map[string]any{"status": "retry"}
		data, err := client.request(ctx, http.MethodPatch, "/accounts/"+accountID+"/oracle/requests/"+requestID, body)
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
// CRE

func handleCRE(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl cre playbooks --account <id>
  slctl cre executors --account <id>
  slctl cre runs --account <id> [--limit 25]`)
		return nil
	}
	resource := args[0]
	switch resource {
	case "playbooks":
		fs := flag.NewFlagSet("cre playbooks", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		url := fmt.Sprintf("/accounts/%s/cre/playbooks", accountID)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "executors":
		fs := flag.NewFlagSet("cre executors", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		url := fmt.Sprintf("/accounts/%s/cre/executors", accountID)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "runs":
		fs := flag.NewFlagSet("cre runs", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.IntVar(&limit, "limit", 25, "Number of runs to fetch")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		if limit <= 0 {
			limit = 25
		}
		url := fmt.Sprintf("/accounts/%s/cre/runs?limit=%d", accountID, limit)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		fmt.Println(`Usage:
  slctl cre playbooks --account <id>
  slctl cre executors --account <id>
  slctl cre runs --account <id> [--limit 25]`)
		return fmt.Errorf("unknown cre subcommand %q", resource)
	}
	return nil
}

// ---------------------------------------------------------------------
// CCIP

func handleCCIP(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl ccip lanes --account <id>
  slctl ccip messages --account <id> [--limit 50]`)
		return nil
	}
	switch args[0] {
	case "lanes":
		fs := flag.NewFlagSet("ccip lanes", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		url := fmt.Sprintf("/accounts/%s/ccip/lanes", accountID)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "messages":
		fs := flag.NewFlagSet("ccip messages", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.IntVar(&limit, "limit", 50, "Number of messages to fetch")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		if limit <= 0 {
			limit = 50
		}
		url := fmt.Sprintf("/accounts/%s/ccip/messages?limit=%d", accountID, limit)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		fmt.Println(`Usage:
  slctl ccip lanes --account <id>
  slctl ccip messages --account <id> [--limit 50]`)
		return fmt.Errorf("unknown ccip subcommand %q", args[0])
	}
	return nil
}

// ---------------------------------------------------------------------
// VRF

func handleVRF(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl vrf keys --account <id>
  slctl vrf requests --account <id> [--limit 50]`)
		return nil
	}
	switch args[0] {
	case "keys":
		fs := flag.NewFlagSet("vrf keys", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		url := fmt.Sprintf("/accounts/%s/vrf/keys", accountID)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "requests":
		fs := flag.NewFlagSet("vrf requests", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.IntVar(&limit, "limit", 50, "Number of requests to fetch")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		if limit <= 0 {
			limit = 50
		}
		url := fmt.Sprintf("/accounts/%s/vrf/requests?limit=%d", accountID, limit)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		fmt.Println(`Usage:
  slctl vrf keys --account <id>
  slctl vrf requests --account <id> [--limit 50]`)
		return fmt.Errorf("unknown vrf subcommand %q", args[0])
	}
	return nil
}

// ---------------------------------------------------------------------
// DataLink

func handleDataLink(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl datalink channels --account <id>
  slctl datalink channel-create --account <id> --name <name> --endpoint <url> [--signers w1,w2] [--status active] [--metadata '{"env":"dev"}']
  slctl datalink deliveries --account <id> [--limit 50]
  slctl datalink deliver --account <id> --channel <id> --payload '{"foo":"bar"}' [--metadata '{"trace":"abc"}']`)
		return nil
	}
	switch args[0] {
	case "channels":
		fs := flag.NewFlagSet("datalink channels", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		url := fmt.Sprintf("/accounts/%s/datalink/channels", accountID)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "channel-create":
		fs := flag.NewFlagSet("datalink channel-create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, name, endpoint, signerSet, status, metaStr string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&name, "name", "", "Channel name (required)")
		fs.StringVar(&endpoint, "endpoint", "", "Endpoint URL (required)")
		fs.StringVar(&signerSet, "signers", "", "Comma/semicolon separated signer wallets")
		fs.StringVar(&status, "status", "", "Channel status (inactive|active|suspended)")
		fs.StringVar(&metaStr, "metadata", "", "Metadata JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || name == "" || endpoint == "" {
			return errors.New("account, name, and endpoint are required")
		}
		metadata, err := parseJSONMap(metaStr)
		if err != nil {
			return fmt.Errorf("parse metadata: %w", err)
		}
		payload := map[string]any{
			"name":       name,
			"endpoint":   endpoint,
			"status":     status,
			"signer_set": splitList(signerSet),
			"metadata":   metadata,
		}
		url := fmt.Sprintf("/accounts/%s/datalink/channels", accountID)
		data, err := client.request(ctx, http.MethodPost, url, payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "deliveries":
		fs := flag.NewFlagSet("datalink deliveries", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.IntVar(&limit, "limit", 50, "Number of deliveries to fetch")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		if limit <= 0 {
			limit = 50
		}
		url := fmt.Sprintf("/accounts/%s/datalink/deliveries?limit=%d", accountID, limit)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "deliver":
		fs := flag.NewFlagSet("datalink deliver", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, channelID, payloadStr, metaStr string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&channelID, "channel", "", "Channel ID (required)")
		fs.StringVar(&payloadStr, "payload", "", "JSON payload (required)")
		fs.StringVar(&metaStr, "metadata", "", "JSON metadata map")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || channelID == "" {
			return errors.New("account and channel are required")
		}
		payload, err := parseJSONMap(payloadStr)
		if err != nil {
			return fmt.Errorf("parse payload: %w", err)
		}
		if payload == nil {
			return errors.New("payload is required")
		}
		metadata, err := parseJSONMap(metaStr)
		if err != nil {
			return fmt.Errorf("parse metadata: %w", err)
		}
		body := map[string]any{
			"payload":  payload,
			"metadata": metadata,
		}
		url := fmt.Sprintf("/accounts/%s/datalink/channels/%s/deliveries", accountID, channelID)
		data, err := client.request(ctx, http.MethodPost, url, body)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		fmt.Println(`Usage:
  slctl datalink channels --account <id>
  slctl datalink channel-create --account <id> --name <name> --endpoint <url> [--signers w1,w2] [--status active] [--metadata '{"env":"dev"}']
  slctl datalink deliveries --account <id> [--limit 50]
  slctl datalink deliver --account <id> --channel <id> --payload '{"foo":"bar"}' [--metadata '{"trace":"abc"}']`)
		return fmt.Errorf("unknown datalink subcommand %q", args[0])
	}
	return nil
}

// ---------------------------------------------------------------------
// DTA

func handleDTA(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl dta products --account <id>
  slctl dta orders --account <id> [--limit 50]`)
		return nil
	}
	switch args[0] {
	case "products":
		fs := flag.NewFlagSet("dta products", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		url := fmt.Sprintf("/accounts/%s/dta/products", accountID)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "orders":
		fs := flag.NewFlagSet("dta orders", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.IntVar(&limit, "limit", 50, "Number of orders to fetch")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		if limit <= 0 {
			limit = 50
		}
		url := fmt.Sprintf("/accounts/%s/dta/orders?limit=%d", accountID, limit)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		fmt.Println(`Usage:
  slctl dta products --account <id>
  slctl dta orders --account <id> [--limit 50]`)
		return fmt.Errorf("unknown dta subcommand %q", args[0])
	}
	return nil
}

// ---------------------------------------------------------------------
// DataStreams

func handleDataStreams(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl datastreams streams --account <id>
  slctl datastreams create --account <id> --name <name> --symbol <symbol> [--description <desc>] [--frequency "1s"] [--sla-ms 50] [--status active] [--metadata '{"env":"dev"}']
  slctl datastreams frames --account <id> --stream <id> [--limit 20]
  slctl datastreams publish --account <id> --stream <id> --sequence <n> [--payload '{"price":123}'] [--latency-ms 10] [--status delivered] [--metadata '{"trace":"abc"}']`)
		return nil
	}
	switch args[0] {
	case "streams":
		fs := flag.NewFlagSet("datastreams streams", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID string
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" {
			return errors.New("account is required")
		}
		url := fmt.Sprintf("/accounts/%s/datastreams", accountID)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "create":
		fs := flag.NewFlagSet("datastreams create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, name, symbol, description, frequency, status, metaStr string
		var slaMs int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&name, "name", "", "Stream name (required)")
		fs.StringVar(&symbol, "symbol", "", "Symbol/identifier (required)")
		fs.StringVar(&description, "description", "", "Description")
		fs.StringVar(&frequency, "frequency", "", "Update frequency (e.g. 1s)")
		fs.IntVar(&slaMs, "sla-ms", 0, "SLA in milliseconds")
		fs.StringVar(&status, "status", "", "Stream status (active|inactive|suspended)")
		fs.StringVar(&metaStr, "metadata", "", "Metadata JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || name == "" || symbol == "" {
			return errors.New("account, name, and symbol are required")
		}
		metadata, err := parseJSONMap(metaStr)
		if err != nil {
			return fmt.Errorf("parse metadata: %w", err)
		}
		payload := map[string]any{
			"name":        name,
			"symbol":      symbol,
			"description": description,
			"frequency":   frequency,
			"sla_ms":      slaMs,
			"status":      status,
			"metadata":    metadata,
		}
		url := fmt.Sprintf("/accounts/%s/datastreams", accountID)
		data, err := client.request(ctx, http.MethodPost, url, payload)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "frames":
		fs := flag.NewFlagSet("datastreams frames", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, streamID string
		var limit int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&streamID, "stream", "", "Stream ID (required)")
		fs.IntVar(&limit, "limit", 20, "Number of frames to fetch")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || streamID == "" {
			return errors.New("account and stream are required")
		}
		if limit <= 0 {
			limit = 20
		}
		url := fmt.Sprintf("/accounts/%s/datastreams/%s/frames?limit=%d", accountID, streamID, limit)
		data, err := client.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
	case "publish":
		fs := flag.NewFlagSet("datastreams publish", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var accountID, streamID, status, payloadStr, metaStr string
		var sequence, latency int
		fs.StringVar(&accountID, "account", "", "Account ID (required)")
		fs.StringVar(&streamID, "stream", "", "Stream ID (required)")
		fs.IntVar(&sequence, "sequence", 0, "Sequence number (required)")
		fs.IntVar(&latency, "latency-ms", 0, "Latency in milliseconds")
		fs.StringVar(&status, "status", "", "Frame status")
		fs.StringVar(&payloadStr, "payload", "", "JSON payload")
		fs.StringVar(&metaStr, "metadata", "", "JSON metadata")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if accountID == "" || streamID == "" || sequence <= 0 {
			return errors.New("account, stream, and positive sequence are required")
		}
		payload, err := parseJSONMap(payloadStr)
		if err != nil {
			return fmt.Errorf("parse payload: %w", err)
		}
		metadata, err := parseJSONMap(metaStr)
		if err != nil {
			return fmt.Errorf("parse metadata: %w", err)
		}
		body := map[string]any{
			"sequence":   sequence,
			"payload":    payload,
			"latency_ms": latency,
			"status":     status,
			"metadata":   metadata,
		}
		url := fmt.Sprintf("/accounts/%s/datastreams/%s/frames", accountID, streamID)
		data, err := client.request(ctx, http.MethodPost, url, body)
		if err != nil {
			return err
		}
		prettyPrint(data)
	default:
		fmt.Println(`Usage:
  slctl datastreams streams --account <id>
  slctl datastreams create --account <id> --name <name> --symbol <symbol> [--description <desc>] [--frequency "1s"] [--sla-ms 50] [--status active] [--metadata '{"env":"dev"}']
  slctl datastreams frames --account <id> --stream <id> [--limit 20]
  slctl datastreams publish --account <id> --stream <id> --sequence <n> [--payload '{"price":123}'] [--latency-ms 10] [--status delivered] [--metadata '{"trace":"abc"}']`)
		return fmt.Errorf("unknown datastreams subcommand %q", args[0])
	}
	return nil
}

// ---------------------------------------------------------------------
// Confidential Compute

func handleConfCompute(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl confcompute enclaves --account <id> [--limit 50]`)
		return nil
	}
	if args[0] != "enclaves" {
		fmt.Println(`Usage:
  slctl confcompute enclaves --account <id> [--limit 50]`)
		return fmt.Errorf("unknown confcompute subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("confcompute enclaves", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var accountID string
	var limit int
	fs.StringVar(&accountID, "account", "", "Account ID (required)")
	fs.IntVar(&limit, "limit", 50, "Number of enclaves to fetch")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if accountID == "" {
		return errors.New("account is required")
	}
	if limit <= 0 {
		limit = 50
	}
	url := fmt.Sprintf("/accounts/%s/confcompute/enclaves?limit=%d", accountID, limit)
	data, err := client.request(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	prettyPrint(data)
	return nil
}

// ---------------------------------------------------------------------
// Workspace Wallets

func handleWorkspaceWallets(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage:
  slctl workspace-wallets list --account <id> [--limit 50]`)
		return nil
	}
	if args[0] != "list" {
		fmt.Println(`Usage:
  slctl workspace-wallets list --account <id> [--limit 50]`)
		return fmt.Errorf("unknown workspace-wallets subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("workspace-wallets list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var accountID string
	var limit int
	fs.StringVar(&accountID, "account", "", "Account ID (required)")
	fs.IntVar(&limit, "limit", 50, "Number of wallets to fetch")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if accountID == "" {
		return errors.New("account is required")
	}
	if limit <= 0 {
		limit = 50
	}
	url := fmt.Sprintf("/accounts/%s/workspace-wallets?limit=%d", accountID, limit)
	data, err := client.request(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	prettyPrint(data)
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

func loadJSONPayload(inline, file string) (any, error) {
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

	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	return payload, nil
}
