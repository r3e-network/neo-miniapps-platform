//go:build ignore

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/database"
	neorequestsupabase "github.com/R3E-Network/service_layer/services/requests/supabase"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

const (
	defaultRPC   = "https://testnet1.neo.coz.io:443"
	defaultAppID = "com.test.consumer"

	appStatusApproved = 1
)

var supportedPermissions = map[string]struct{}{
	"wallet":     {},
	"payments":   {},
	"governance": {},
	"rng":        {},
	"datafeed":   {},
	"storage":    {},
	"oracle":     {},
	"compute":    {},
	"automation": {},
	"apps":       {},
	"secrets":    {},
}

type appInfo struct {
	AppID          string
	Developer      util.Uint160
	DeveloperKey   []byte
	EntryURL       string
	ManifestHash   []byte
	Status         int
	AllowlistHash  []byte
	RawStatusLabel string
}

func main() {
	devWIF := strings.TrimSpace(os.Getenv("MINIAPP_DEVELOPER_WIF"))
	if devWIF == "" {
		devWIF = strings.TrimSpace(os.Getenv("NEO_TESTNET_WIF"))
	}
	if devWIF == "" {
		fmt.Println("MINIAPP_DEVELOPER_WIF or NEO_TESTNET_WIF environment variable not set")
		os.Exit(1)
	}

	adminWIF := strings.TrimSpace(os.Getenv("MINIAPP_ADMIN_WIF"))
	if adminWIF == "" {
		adminWIF = devWIF
	}

	devKey, err := keys.NewPrivateKeyFromWIF(devWIF)
	if err != nil {
		fmt.Printf("Invalid developer WIF: %v\n", err)
		os.Exit(1)
	}

	adminKey, err := keys.NewPrivateKeyFromWIF(adminWIF)
	if err != nil {
		fmt.Printf("Invalid admin WIF: %v\n", err)
		os.Exit(1)
	}

	derivedPubKeyHex := strings.ToLower(devKey.PublicKey().StringCompressed())

	ctx := context.Background()
	manifest, source, err := loadManifest(ctx)
	if err != nil {
		fmt.Printf("Failed to load manifest: %v\n", err)
		os.Exit(1)
	}

	applyManifestOverrides(manifest, derivedPubKeyHex)

	canonical, err := canonicalizeManifest(manifest)
	if err != nil {
		fmt.Printf("Manifest validation failed: %v\n", err)
		os.Exit(1)
	}

	if err := enforceAssetPolicy(canonical); err != nil {
		fmt.Printf("Manifest policy error: %v\n", err)
		os.Exit(1)
	}

	manifestHashHex, err := computeManifestHashHex(canonical)
	if err != nil {
		fmt.Printf("Failed to compute manifest hash: %v\n", err)
		os.Exit(1)
	}

	if canonicalPath := strings.TrimSpace(os.Getenv("MINIAPP_CANONICAL_PATH")); canonicalPath != "" {
		payload, err := stableStringify(canonical)
		if err != nil {
			fmt.Printf("Failed to encode canonical manifest: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(canonicalPath, []byte(payload), 0o644); err != nil {
			fmt.Printf("Failed to write canonical manifest: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Canonical manifest written: %s\n", canonicalPath)
	}

	if raw := strings.TrimSpace(os.Getenv("MINIAPP_DRY_RUN")); raw != "" && parseEnvBool(raw) {
		fmt.Println("Dry run enabled; skipping on-chain registration.")
		fmt.Printf("Manifest hash: %s\n", manifestHashHex)
		return
	}

	appID := mustString(canonical, "app_id")
	entryURL := mustString(canonical, "entry_url")
	manifestPubKeyHex := mustString(canonical, "developer_pubkey")

	if manifestPubKeyHex != "" && derivedPubKeyHex != "" && !strings.EqualFold(manifestPubKeyHex, derivedPubKeyHex) {
		fmt.Printf("Developer pubkey mismatch (manifest=%s, wif=%s)\n", manifestPubKeyHex, derivedPubKeyHex)
		fmt.Println("Update manifest.developer_pubkey or set MINIAPP_DEVELOPER_PUBKEY/MINIAPP_DEVELOPER_WIF to match.")
		os.Exit(1)
	}

	manifestHashBytes, err := hex.DecodeString(manifestHashHex)
	if err != nil {
		fmt.Printf("Invalid manifest hash hex: %v\n", err)
		os.Exit(1)
	}

	developerKeyBytes, err := hex.DecodeString(manifestPubKeyHex)
	if err != nil {
		fmt.Printf("Invalid developer_pubkey hex: %v\n", err)
		os.Exit(1)
	}

	rpcURL := strings.TrimSpace(os.Getenv("NEO_RPC_URL"))
	if rpcURL == "" {
		rpcURL = defaultRPC
	}

	contractHash, err := resolveContractHash("CONTRACT_APPREGISTRY_HASH", "CONTRACT_APP_REGISTRY_HASH")
	if err != nil {
		fmt.Printf("Invalid AppRegistry hash: %v\n", err)
		os.Exit(1)
	}
	if contractHash == (util.Uint160{}) {
		fmt.Println("CONTRACT_APPREGISTRY_HASH not set")
		os.Exit(1)
	}

	fmt.Println("=== AppRegistry Registration ===")
	fmt.Printf("RPC: %s\n", rpcURL)
	fmt.Printf("Manifest source: %s\n", source)
	fmt.Printf("App ID: %s\n", appID)
	fmt.Printf("Entry URL: %s\n", entryURL)
	fmt.Printf("Manifest hash: %s\n", manifestHashHex)
	fmt.Printf("Developer: %s\n", address.Uint160ToString(devKey.GetScriptHash()))
	fmt.Printf("Admin: %s\n", address.Uint160ToString(adminKey.GetScriptHash()))

	client, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		fmt.Printf("Failed to create RPC client: %v\n", err)
		os.Exit(1)
	}

	devActor, err := newActor(client, devKey)
	if err != nil {
		fmt.Printf("Failed to create developer actor: %v\n", err)
		os.Exit(1)
	}

	adminActor, err := newActor(client, adminKey)
	if err != nil {
		fmt.Printf("Failed to create admin actor: %v\n", err)
		os.Exit(1)
	}

	existing, err := getAppInfo(devActor, contractHash, appID)
	if err != nil {
		fmt.Printf("Failed to query AppRegistry: %v\n", err)
		os.Exit(1)
	}

	needsApproval := existing == nil || existing.Status != appStatusApproved

	if existing == nil || existing.AppID == "" {
		fmt.Println("Registering new app...")
		if err := registerApp(ctx, client, devActor, contractHash, appID, manifestHashBytes, entryURL, developerKeyBytes); err != nil {
			fmt.Printf("Register failed: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("App already registered (status=%s)\n", existing.RawStatusLabel)
		needsUpdate := !bytesEqual(existing.ManifestHash, manifestHashBytes) || strings.TrimSpace(existing.EntryURL) != entryURL
		if needsUpdate {
			fmt.Println("Updating manifest on-chain...")
			if err := updateManifest(ctx, client, devActor, contractHash, appID, manifestHashBytes, entryURL); err != nil {
				fmt.Printf("Update manifest failed: %v\n", err)
				os.Exit(1)
			}
			needsApproval = true
		} else {
			fmt.Println("Manifest hash matches; skipping update")
		}
	}

	if needsApproval {
		if err := approveApp(ctx, client, adminActor, contractHash, appID); err != nil {
			fmt.Printf("SetStatus failed: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("App already approved; skipping setStatus")
	}

	fmt.Println("✅ AppRegistry registration complete")
}

func loadManifest(ctx context.Context) (map[string]any, string, error) {
	if path := strings.TrimSpace(os.Getenv("MINIAPP_MANIFEST_PATH")); path != "" {
		manifest, err := loadManifestFromFile(path)
		return manifest, "file:" + path, err
	}

	appID := strings.TrimSpace(os.Getenv("MINIAPP_APP_ID"))
	if appID == "" {
		appID = defaultAppID
	}
	manifest, err := loadManifestFromSupabase(ctx, appID)
	if err != nil {
		return nil, "", fmt.Errorf("%w (set MINIAPP_MANIFEST_PATH to load a local manifest)", err)
	}
	return manifest, "supabase:" + appID, nil
}

func loadManifestFromFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func loadManifestFromSupabase(ctx context.Context, appID string) (map[string]any, error) {
	client, err := database.NewClient(database.Config{})
	if err != nil {
		return nil, err
	}
	repo := database.NewRepository(client)
	miniapps := neorequestsupabase.NewRepository(repo)
	app, err := miniapps.GetMiniApp(ctx, appID)
	if err != nil {
		return nil, err
	}
	if len(app.Manifest) == 0 {
		return nil, fmt.Errorf("miniapps.manifest empty for %s", appID)
	}
	var manifest map[string]any
	if err := json.Unmarshal(app.Manifest, &manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func applyManifestOverrides(manifest map[string]any, derivedPubKeyHex string) {
	if manifest == nil {
		return
	}
	if appID := strings.TrimSpace(os.Getenv("MINIAPP_APP_ID")); appID != "" {
		manifest["app_id"] = appID
	}
	if entryURL := strings.TrimSpace(os.Getenv("MINIAPP_ENTRY_URL")); entryURL != "" {
		manifest["entry_url"] = entryURL
	}
	if devKey := strings.TrimSpace(os.Getenv("MINIAPP_DEVELOPER_PUBKEY")); devKey != "" {
		manifest["developer_pubkey"] = devKey
	} else if raw, ok := manifest["developer_pubkey"]; !ok || strings.TrimSpace(fmt.Sprint(raw)) == "" {
		if derivedPubKeyHex != "" {
			manifest["developer_pubkey"] = derivedPubKeyHex
		}
	}
	if callbackContract := strings.TrimSpace(os.Getenv("MINIAPP_CALLBACK_CONTRACT")); callbackContract != "" {
		manifest["callback_contract"] = callbackContract
	}
	if callbackMethod := strings.TrimSpace(os.Getenv("MINIAPP_CALLBACK_METHOD")); callbackMethod != "" {
		manifest["callback_method"] = callbackMethod
	}
}

func canonicalizeManifest(manifest map[string]any) (map[string]any, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest must be an object")
	}

	out := make(map[string]any, len(manifest))
	for k, v := range manifest {
		out[k] = v
	}

	appID := strings.TrimSpace(fmt.Sprint(manifest["app_id"]))
	entryURL := strings.TrimSpace(fmt.Sprint(manifest["entry_url"]))
	developerPubKey := strings.TrimSpace(fmt.Sprint(manifest["developer_pubkey"]))

	if appID == "" {
		return nil, fmt.Errorf("manifest.app_id required")
	}
	if entryURL == "" {
		return nil, fmt.Errorf("manifest.entry_url required")
	}
	if developerPubKey == "" {
		return nil, fmt.Errorf("manifest.developer_pubkey required")
	}

	out["app_id"] = appID
	out["entry_url"] = entryURL

	normalizedPubKey, err := normalizeHex(developerPubKey, "manifest.developer_pubkey")
	if err != nil {
		return nil, err
	}
	out["developer_pubkey"] = normalizedPubKey

	if raw, ok := manifest["name"]; ok {
		out["name"] = strings.TrimSpace(fmt.Sprint(raw))
	}
	if raw, ok := manifest["version"]; ok {
		out["version"] = strings.TrimSpace(fmt.Sprint(raw))
	}

	if raw, ok := manifest["assets_allowed"]; ok {
		list, err := normalizeStringList(raw, "manifest.assets_allowed", "upper")
		if err != nil {
			return nil, err
		}
		out["assets_allowed"] = list
	}
	if raw, ok := manifest["governance_assets_allowed"]; ok {
		list, err := normalizeStringList(raw, "manifest.governance_assets_allowed", "upper")
		if err != nil {
			return nil, err
		}
		out["governance_assets_allowed"] = list
	}
	if raw, ok := manifest["sandbox_flags"]; ok {
		list, err := normalizeStringList(raw, "manifest.sandbox_flags", "lower")
		if err != nil {
			return nil, err
		}
		out["sandbox_flags"] = list
	}
	if raw, ok := manifest["contracts_needed"]; ok {
		list, err := normalizeStringList(raw, "manifest.contracts_needed", "preserve")
		if err != nil {
			return nil, err
		}
		out["contracts_needed"] = list
	}
	if raw, ok := manifest["permissions"]; ok {
		permissions, err := normalizePermissions(raw)
		if err != nil {
			return nil, err
		}
		out["permissions"] = permissions
	}
	if raw, ok := manifest["limits"]; ok {
		limits, err := normalizeLimits(raw)
		if err != nil {
			return nil, err
		}
		out["limits"] = limits
	}

	callbackContract := strings.TrimSpace(fmt.Sprint(manifest["callback_contract"]))
	callbackMethod := strings.TrimSpace(fmt.Sprint(manifest["callback_method"]))
	if callbackContract != "" || callbackMethod != "" {
		if callbackContract == "" {
			return nil, fmt.Errorf("manifest.callback_contract required when callback_method is set")
		}
		if callbackMethod == "" {
			return nil, fmt.Errorf("manifest.callback_method required when callback_contract is set")
		}
		normalizedContract, err := normalizeHexBytes(callbackContract, 20, "manifest.callback_contract")
		if err != nil {
			return nil, err
		}
		out["callback_contract"] = normalizedContract
		out["callback_method"] = callbackMethod
	}

	return out, nil
}

func normalizeStringList(value any, label, mode string) ([]string, error) {
	var rawList []string
	switch v := value.(type) {
	case []any:
		for _, entry := range v {
			rawList = append(rawList, fmt.Sprint(entry))
		}
	case []string:
		rawList = append(rawList, v...)
	default:
		return nil, fmt.Errorf("%s must be an array", label)
	}

	items := make([]string, 0, len(rawList))
	for _, entry := range rawList {
		item := strings.TrimSpace(entry)
		if item == "" {
			continue
		}
		switch mode {
		case "upper":
			item = strings.ToUpper(item)
		case "lower":
			item = strings.ToLower(item)
		}
		items = append(items, item)
	}

	sort.Strings(items)
	items = uniqueStrings(items)
	return items, nil
}

func normalizePermissions(value any) (map[string]any, error) {
	if value == nil {
		return nil, fmt.Errorf("manifest.permissions must be an object or array")
	}

	switch list := value.(type) {
	case []any:
		out := map[string]any{}
		normalized, err := normalizeStringList(list, "manifest.permissions", "lower")
		if err != nil {
			return nil, err
		}
		for _, key := range normalized {
			if _, ok := supportedPermissions[key]; !ok {
				return nil, fmt.Errorf("manifest.permissions contains unsupported permission: %s", key)
			}
			if key == "wallet" {
				out[key] = []string{"read-address"}
			} else {
				out[key] = true
			}
		}
		return out, nil
	case []string:
		out := map[string]any{}
		normalized, err := normalizeStringList(list, "manifest.permissions", "lower")
		if err != nil {
			return nil, err
		}
		for _, key := range normalized {
			if _, ok := supportedPermissions[key]; !ok {
				return nil, fmt.Errorf("manifest.permissions contains unsupported permission: %s", key)
			}
			if key == "wallet" {
				out[key] = []string{"read-address"}
			} else {
				out[key] = true
			}
		}
		return out, nil
	}

	obj, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("manifest.permissions must be an object or array")
	}

	out := map[string]any{}
	for rawKey, rawVal := range obj {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		if _, ok := supportedPermissions[key]; !ok {
			return nil, fmt.Errorf("manifest.permissions contains unsupported permission: %s", key)
		}

		switch val := rawVal.(type) {
		case bool:
			if key == "wallet" && val {
				out[key] = []string{"read-address"}
			} else {
				out[key] = val
			}
		case []any:
			list, err := normalizeStringList(val, fmt.Sprintf("manifest.permissions.%s", key), "lower")
			if err != nil {
				return nil, err
			}
			if key == "wallet" {
				for _, entry := range list {
					if entry != "read-address" {
						return nil, fmt.Errorf("manifest.permissions.wallet contains unsupported entry: %s", entry)
					}
				}
			}
			out[key] = list
		case nil:
			out[key] = false
		default:
			return nil, fmt.Errorf("manifest.permissions.%s must be a boolean or array", key)
		}
	}

	return out, nil
}

func normalizeLimits(value any) (map[string]any, error) {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("manifest.limits must be an object")
	}

	out := map[string]any{}
	for rawKey, rawVal := range obj {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		val := strings.TrimSpace(fmt.Sprint(rawVal))
		if val == "" {
			continue
		}
		out[key] = val
	}
	return out, nil
}

func normalizeHex(value, label string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", fmt.Errorf("%s required", label)
	}
	raw = strings.TrimPrefix(raw, "0x")
	raw = strings.TrimPrefix(raw, "0X")
	raw = strings.ToLower(raw)
	if _, err := hex.DecodeString(raw); err != nil {
		return "", fmt.Errorf("%s must be hex: %w", label, err)
	}
	return raw, nil
}

func normalizeHexBytes(value string, length int, label string) (string, error) {
	normalized, err := normalizeHex(value, label)
	if err != nil {
		return "", err
	}
	if length > 0 && len(normalized) != length*2 {
		return "", fmt.Errorf("%s must be %d bytes", label, length)
	}
	return normalized, nil
}

func computeManifestHashHex(canonical map[string]any) (string, error) {
	payload, err := stableStringify(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:]), nil
}

func stableStringify(value any) (string, error) {
	var b strings.Builder
	if err := writeStableJSON(&b, value); err != nil {
		return "", err
	}
	return b.String(), nil
}

func writeStableJSON(b *strings.Builder, value any) error {
	switch v := value.(type) {
	case nil:
		b.WriteString("null")
		return nil
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			keyJSON, err := json.Marshal(key)
			if err != nil {
				return err
			}
			b.Write(keyJSON)
			b.WriteByte(':')
			if err := writeStableJSON(b, v[key]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
		return nil
	case []any:
		b.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeStableJSON(b, item); err != nil {
				return err
			}
		}
		b.WriteByte(']')
		return nil
	case []string:
		b.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			itemJSON, err := json.Marshal(item)
			if err != nil {
				return err
			}
			b.Write(itemJSON)
		}
		b.WriteByte(']')
		return nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return err
		}
		b.Write(encoded)
		return nil
	}
}

func enforceAssetPolicy(canonical map[string]any) error {
	assets, ok := canonical["assets_allowed"]
	if !ok {
		return fmt.Errorf("manifest.assets_allowed required")
	}
	assetsList, err := normalizeStringList(assets, "manifest.assets_allowed", "upper")
	if err != nil {
		return err
	}
	if len(assetsList) != 1 || assetsList[0] != "GAS" {
		return fmt.Errorf("manifest.assets_allowed must be exactly [\"GAS\"]")
	}

	governance, ok := canonical["governance_assets_allowed"]
	if !ok {
		return fmt.Errorf("manifest.governance_assets_allowed required")
	}
	governanceList, err := normalizeStringList(governance, "manifest.governance_assets_allowed", "upper")
	if err != nil {
		return err
	}
	if len(governanceList) != 1 || governanceList[0] != "NEO" {
		return fmt.Errorf("manifest.governance_assets_allowed must be exactly [\"NEO\"]")
	}
	return nil
}

func mustString(obj map[string]any, key string) string {
	raw, ok := obj[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func resolveContractHash(keys ...string) (util.Uint160, error) {
	for _, key := range keys {
		if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
			return parseHash160(raw)
		}
	}
	return util.Uint160{}, nil
}

func parseHash160(raw string) (util.Uint160, error) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "0x")
	return util.Uint160DecodeStringLE(raw)
}

func newActor(client *rpcclient.Client, key *keys.PrivateKey) (*actor.Actor, error) {
	acc := wallet.NewAccountFromPrivateKey(key)
	acc.Label = "account"
	return actor.NewSimple(client, acc)
}

func getAppInfo(act *actor.Actor, contract util.Uint160, appID string) (*appInfo, error) {
	result, err := act.Call(contract, "getApp", appID)
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" {
		return nil, fmt.Errorf("getApp failed: %s (fault: %s)", result.State, result.FaultException)
	}
	if len(result.Stack) == 0 {
		return nil, nil
	}

	structItem, ok := result.Stack[0].(*stackitem.Struct)
	if !ok {
		return nil, fmt.Errorf("unexpected getApp result")
	}
	items := structItem.Value().([]stackitem.Item)
	if len(items) < 7 {
		return nil, fmt.Errorf("unexpected getApp result length")
	}

	info := &appInfo{
		AppID:         bytesToString(items[0]),
		EntryURL:      bytesToString(items[3]),
		ManifestHash:  bytesFromItem(items[4]),
		Status:        intFromItem(items[5]),
		AllowlistHash: bytesFromItem(items[6]),
	}
	if info.AppID == "" {
		return nil, nil
	}

	info.Developer = uint160FromItem(items[1])
	info.DeveloperKey = bytesFromItem(items[2])
	info.RawStatusLabel = statusLabel(info.Status)

	return info, nil
}

func registerApp(ctx context.Context, client *rpcclient.Client, act *actor.Actor, contract util.Uint160, appID string, manifestHash []byte, entryURL string, developerPubKey []byte) error {
	testResult, err := act.Call(contract, "register", appID, manifestHash, entryURL, developerPubKey)
	if err != nil {
		return fmt.Errorf("test invoke failed: %w", err)
	}
	if testResult.State != "HALT" {
		return fmt.Errorf("test invoke failed: %s (fault: %s)", testResult.State, testResult.FaultException)
	}
	txHash, vub, err := act.SendCall(contract, "register", appID, manifestHash, entryURL, developerPubKey)
	if err != nil {
		return fmt.Errorf("send transaction: %w", err)
	}
	fmt.Printf("Register tx sent: %s (vub %d)\n", txHash.StringLE(), vub)
	return waitForTx(ctx, client, txHash)
}

func updateManifest(ctx context.Context, client *rpcclient.Client, act *actor.Actor, contract util.Uint160, appID string, manifestHash []byte, entryURL string) error {
	testResult, err := act.Call(contract, "updateManifest", appID, manifestHash, entryURL)
	if err != nil {
		return fmt.Errorf("test invoke failed: %w", err)
	}
	if testResult.State != "HALT" {
		return fmt.Errorf("test invoke failed: %s (fault: %s)", testResult.State, testResult.FaultException)
	}
	txHash, vub, err := act.SendCall(contract, "updateManifest", appID, manifestHash, entryURL)
	if err != nil {
		return fmt.Errorf("send transaction: %w", err)
	}
	fmt.Printf("UpdateManifest tx sent: %s (vub %d)\n", txHash.StringLE(), vub)
	return waitForTx(ctx, client, txHash)
}

func approveApp(ctx context.Context, client *rpcclient.Client, act *actor.Actor, contract util.Uint160, appID string) error {
	testResult, err := act.Call(contract, "setStatus", appID, appStatusApproved)
	if err != nil {
		return fmt.Errorf("test invoke failed: %w", err)
	}
	if testResult.State != "HALT" {
		return fmt.Errorf("test invoke failed: %s (fault: %s)", testResult.State, testResult.FaultException)
	}
	txHash, vub, err := act.SendCall(contract, "setStatus", appID, appStatusApproved)
	if err != nil {
		return fmt.Errorf("send transaction: %w", err)
	}
	fmt.Printf("SetStatus tx sent: %s (vub %d)\n", txHash.StringLE(), vub)
	return waitForTx(ctx, client, txHash)
}

func waitForTx(ctx context.Context, client *rpcclient.Client, txHash util.Uint256) error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	timeout := time.After(2 * time.Minute)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for transaction")
		case <-ticker.C:
			appLog, err := client.GetApplicationLog(txHash, nil)
			if err != nil {
				continue
			}
			if len(appLog.Executions) == 0 {
				continue
			}
			exec := appLog.Executions[0]
			if exec.VMState.HasFlag(1) {
				return nil
			}
			return fmt.Errorf("transaction failed: %s", exec.FaultException)
		}
	}
}

func bytesFromItem(item stackitem.Item) []byte {
	if item == nil || item.Value() == nil {
		return nil
	}
	switch v := item.Value().(type) {
	case []byte:
		return v
	case string:
		return []byte(v)
	default:
		return nil
	}
}

func bytesToString(item stackitem.Item) string {
	if item == nil || item.Value() == nil {
		return ""
	}
	switch v := item.Value().(type) {
	case []byte:
		return string(v)
	case string:
		return v
	case *big.Int:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func intFromItem(item stackitem.Item) int {
	if item == nil || item.Value() == nil {
		return 0
	}
	switch v := item.Value().(type) {
	case *big.Int:
		return int(v.Int64())
	case int64:
		return int(v)
	case int:
		return v
	case []byte:
		b := new(big.Int).SetBytes(v)
		return int(b.Int64())
	default:
		return 0
	}
}

func uint160FromItem(item stackitem.Item) util.Uint160 {
	if item == nil || item.Value() == nil {
		return util.Uint160{}
	}
	if b, ok := item.Value().([]byte); ok {
		hash, err := util.Uint160DecodeBytesBE(b)
		if err == nil {
			return hash
		}
	}
	return util.Uint160{}
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func statusLabel(status int) string {
	switch status {
	case 0:
		return "pending"
	case 1:
		return "approved"
	case 2:
		return "disabled"
	default:
		return fmt.Sprintf("unknown(%d)", status)
	}
}

func parseEnvBool(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
