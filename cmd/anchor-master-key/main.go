package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/httputil"
)

type masterKeyResponse struct {
	Hash   string `json:"hash"`
	PubKey string `json:"pubkey"`
}

func main() {
	if err := run(); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	rpc := flag.String("rpc", "", "Neo RPC URL")
	gateway := flag.String("gateway", "", "Gateway contract hash (0x-prefixed)")
	privHex := flag.String("priv", "", "Hex-encoded private key (admin)")
	pubKeyHex := flag.String("pubkey", "", "Compressed pubkey hex to anchor (optional if --neoaccounts is set)")
	pubKeyHashHex := flag.String("pubkey-hash", "", "SHA-256 hash of pubkey (hex, 32 bytes; optional if --neoaccounts is set)")
	attestHashHex := flag.String("attest-hash", "", "Attestation bundle hash/CID (hex; optional if --bundle is set)")
	neoAccounts := flag.String("neoaccounts", "", "NeoAccounts (or gateway) base URL; if set, fetch pubkey/hash from /master-key")
	bundleURI := flag.String("bundle", "", "Optional bundle URI (file:/// or https://) to compute attestation hash automatically")
	flag.Parse()

	if *rpc == "" || *gateway == "" || *privHex == "" || (*attestHashHex == "" && *bundleURI == "") {
		flag.Usage()
		return fmt.Errorf("missing required flags")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	httpClient := &http.Client{Timeout: 30 * time.Second}

	mk := masterKeyResponse{Hash: *pubKeyHashHex, PubKey: *pubKeyHex}
	if *neoAccounts != "" {
		fetched, err := fetchMasterKey(ctx, httpClient, *neoAccounts)
		if err != nil {
			return fmt.Errorf("fetch master key: %w", err)
		}
		mk = fetched
	}
	if mk.PubKey == "" || mk.Hash == "" {
		flag.Usage()
		return fmt.Errorf("pubkey and pubkey-hash are required (or set --neoaccounts)")
	}

	client, err := chain.NewClient(chain.Config{RPCURL: *rpc})
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}

	fulfiller, err := chain.NewTEEFulfiller(client, trim0x(*gateway), trim0x(*privHex))
	if err != nil {
		return fmt.Errorf("fulfiller: %w", err)
	}

	pubKey, err := hex.DecodeString(trim0x(mk.PubKey))
	if err != nil {
		return fmt.Errorf("pubkey decode: %w", err)
	}
	pubKeyHash, err := hex.DecodeString(trim0x(mk.Hash))
	if err != nil || len(pubKeyHash) != sha256.Size {
		return fmt.Errorf("pubkey-hash decode: %w", err)
	}
	attestHex := *attestHashHex
	if attestHex == "" && *bundleURI != "" {
		bundleHash, bundleErr := hashBundle(ctx, httpClient, *bundleURI)
		if bundleErr != nil {
			return fmt.Errorf("bundle hash: %w", bundleErr)
		}
		attestHex = bundleHash
	}
	attestHash, err := hex.DecodeString(trim0x(attestHex))
	if err != nil {
		return fmt.Errorf("attestation-hash decode: %w", err)
	}

	txResult, err := fulfiller.SetTEEMasterKey(ctx, pubKey, pubKeyHash, attestHash)
	if err != nil {
		return fmt.Errorf("set master key: %w", err)
	}

	fmt.Printf("Anchored master key. Tx: %s VMState: %s\n", txResult.TxHash, txResult.VMState)
	return nil
}

func fetchMasterKey(ctx context.Context, httpClient *http.Client, baseURL string) (masterKeyResponse, error) {
	rawURL, err := resolveURL(baseURL, "/master-key")
	if err != nil {
		return masterKeyResponse{}, err
	}

	resp, err := httpGet(ctx, httpClient, rawURL)
	if err != nil {
		return masterKeyResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, truncated, readErr := httputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return masterKeyResponse{}, fmt.Errorf("http %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		msg := string(b)
		if truncated {
			msg += "...(truncated)"
		}
		return masterKeyResponse{}, fmt.Errorf("http %d: %s", resp.StatusCode, msg)
	}
	var body masterKeyResponse
	data, err := httputil.ReadAllStrict(resp.Body, 1<<20)
	if err != nil {
		return masterKeyResponse{}, fmt.Errorf("read /master-key response: %w", err)
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return masterKeyResponse{}, fmt.Errorf("decode /master-key response: %w", err)
	}
	if body.PubKey == "" || body.Hash == "" {
		return masterKeyResponse{}, fmt.Errorf("/master-key missing pubkey/hash")
	}
	return body, nil
}

func hashBundle(ctx context.Context, httpClient *http.Client, uri string) (string, error) {
	data, err := fetch(ctx, httpClient, uri)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func resolveURL(base, path string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid base URL: %q", base)
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func httpGet(ctx context.Context, httpClient *http.Client, rawURL string) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, err
	}
	return httpClient.Do(req)
}

func fetch(ctx context.Context, httpClient *http.Client, uri string) ([]byte, error) {
	if strings.HasPrefix(uri, "file://") {
		path := strings.TrimPrefix(uri, "file://")
		return os.ReadFile(path)
	}

	resp, err := httpGet(ctx, httpClient, uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, truncated, readErr := httputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return nil, fmt.Errorf("http %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		msg := string(b)
		if truncated {
			msg += "...(truncated)"
		}
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, msg)
	}
	body, err := httputil.ReadAllStrict(resp.Body, 64<<20)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func trim0x(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, "0x") {
		return s[2:]
	}
	return s
}
