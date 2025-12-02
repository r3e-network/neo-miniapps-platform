package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// Config controls snapshot generation for a given block height.
type Config struct {
	RPCURL     string
	Height     int64
	OutputDir  string
	Network    string
	Contracts  []string
	KVURLBase  string
	DSN        string
	SigningKey string
}

type snapshotManifest struct {
	Network     string    `json:"network"`
	Height      int64     `json:"height"`
	StateRoot   string    `json:"state_root"`
	Generated   time.Time `json:"generated_at"`
	KVPath      string    `json:"kv_path,omitempty"`
	KVURL       string    `json:"kv_url,omitempty"`
	KVHash      string    `json:"kv_sha256,omitempty"`
	KVBytes     int64     `json:"kv_bytes,omitempty"`
	KVDiffPath  string    `json:"kv_diff_path,omitempty"`
	KVDiffURL   string    `json:"kv_diff_url,omitempty"`
	KVDiffHash  string    `json:"kv_diff_sha256,omitempty"`
	KVDiffBytes int64     `json:"kv_diff_bytes,omitempty"`
	Contracts   []string  `json:"contracts,omitempty"`
	RPCURL      string    `json:"rpc_url,omitempty"`
	Signature   string    `json:"signature,omitempty"`
	SigningKey  string    `json:"signing_public_key,omitempty"`
}

func main() {
	var cfg Config
	flag.StringVar(&cfg.RPCURL, "rpc", core.EnvDefault("NEO_RPC_URL", "http://localhost:10332"), "NEO RPC endpoint")
	flag.Int64Var(&cfg.Height, "height", 0, "block height to snapshot (required)")
	flag.StringVar(&cfg.OutputDir, "out", core.EnvDefault("NEO_SNAPSHOT_OUT", "./snapshots"), "output directory for KV bundle + manifest")
	flag.StringVar(&cfg.Network, "network", core.EnvDefault("NEO_NETWORK", "mainnet"), "network label (mainnet|testnet)")
	flag.StringVar(&cfg.KVURLBase, "kv-url-base", core.EnvDefault("NEO_KV_URL_BASE", ""), "optional base URL to publish KV bundle links")
	flag.StringVar(&cfg.DSN, "dsn", core.EnvDefault("NEO_SNAPSHOT_DSN", ""), "Postgres DSN to reuse captured storage from neo_storage (optional)")
	flag.StringVar(&cfg.SigningKey, "signing-key", core.EnvDefault("NEO_SNAPSHOT_SIGNING_KEY", ""), "optional ed25519 private key (hex/base64) to sign manifest")
	contracts := flag.String("contracts", core.EnvDefault("NEO_CONTRACTS", ""), "comma-separated contract hashes to include (optional; empty means skip kv bundle)")
	flag.Parse()

	if cfg.Height <= 0 {
		log.Fatal("height must be > 0")
	}
	if strings.TrimSpace(*contracts) != "" {
		cfg.Contracts = strings.Split(*contracts, ",")
	}

	ctx := context.Background()
	storedKV := map[string][]stateEntry{}
	storedDiff := map[string][]stateEntry{}
	if strings.TrimSpace(cfg.DSN) != "" {
		db, err := sql.Open("postgres", cfg.DSN)
		if err != nil {
			log.Printf("snapshot: open db failed (dsn=%s): %v", cfg.DSN, err)
		} else {
			defer db.Close()
			if kvs, err := loadStorageFromDB(ctx, db, cfg.Height); err != nil {
				log.Printf("snapshot: load storage from db failed: %v", err)
			} else {
				storedKV = kvs
				log.Printf("snapshot: loaded %d contracts from neo_storage at height %d", len(kvs), cfg.Height)
			}
			if diffs, err := loadStorageDiffsFromDB(ctx, db, cfg.Height); err != nil {
				log.Printf("snapshot: load storage diffs from db failed: %v", err)
			} else {
				storedDiff = diffs
			}
		}
	}
	if len(cfg.Contracts) == 0 && (len(storedKV) > 0 || len(storedDiff) > 0) {
		for c := range storedKV {
			cfg.Contracts = append(cfg.Contracts, c)
		}
		for c := range storedDiff {
			cfg.Contracts = append(cfg.Contracts, c)
		}
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	manifestPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("block-%d.json", cfg.Height))
	kvPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("block-%d-kv.tar.gz", cfg.Height))

	stateRoot, err := fetchStateRoot(ctx, cfg.RPCURL, cfg.Height)
	if err != nil {
		log.Fatalf("fetch state root: %v", err)
	}

	manifest := snapshotManifest{
		Network:   cfg.Network,
		Height:    cfg.Height,
		StateRoot: stateRoot,
		Generated: time.Now().UTC(),
		RPCURL:    cfg.RPCURL,
	}

	if len(cfg.Contracts) > 0 {
		hash, bytesWritten, err := writeKVBundle(ctx, cfg.RPCURL, cfg.Contracts, storedKV, kvPath)
		if err != nil {
			log.Fatalf("build KV bundle: %v", err)
		}
		manifest.KVPath = kvPath
		manifest.KVHash = hash
		manifest.KVBytes = bytesWritten
		manifest.Contracts = cfg.Contracts
		if base := strings.TrimSuffix(strings.TrimSpace(cfg.KVURLBase), "/"); base != "" {
			manifest.KVURL = fmt.Sprintf("%s/%s", base, filepath.Base(kvPath))
		}
		if len(storedDiff) > 0 {
			diffPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("block-%d-kv-diff.tar.gz", cfg.Height))
			diffHash, diffBytes, err := writeKVBundle(ctx, cfg.RPCURL, cfg.Contracts, storedDiff, diffPath)
			if err != nil {
				log.Printf("warning: failed to build KV diff bundle: %v", err)
			} else {
				manifest.KVDiffPath = diffPath
				manifest.KVDiffHash = diffHash
				manifest.KVDiffBytes = diffBytes
				if base := strings.TrimSuffix(strings.TrimSpace(cfg.KVURLBase), "/"); base != "" {
					manifest.KVDiffURL = fmt.Sprintf("%s/%s", base, filepath.Base(diffPath))
				}
			}
		}
	}

	if pub, sig, err := signManifest(cfg, manifest); err == nil && sig != "" {
		manifest.Signature = sig
		manifest.SigningKey = pub
	} else if err != nil {
		log.Printf("warning: signing manifest failed: %v", err)
	}

	if err := writeManifest(manifestPath, manifest); err != nil {
		log.Fatalf("write manifest: %v", err)
	}
	log.Printf("snapshot written: height=%d state_root=%s manifest=%s kv=%s bytes=%d", cfg.Height, stateRoot, manifestPath, manifest.KVPath, manifest.KVBytes)
}


type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type stateRoot struct {
	Hash string `json:"hash"`
}

// stateEntry represents a contract storage key/value.
type stateEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func fetchStateRoot(ctx context.Context, rpcURL string, height int64) (string, error) {
	var sr stateRoot
	if err := postRPC(ctx, rpcURL, "getstateroot", []interface{}{height}, &sr); err != nil {
		return "", err
	}
	if strings.TrimSpace(sr.Hash) == "" {
		return "", fmt.Errorf("state root empty for height %d", height)
	}
	return sr.Hash, nil
}

func fetchContractStorage(ctx context.Context, rpcURL, contract string) ([]stateEntry, error) {
	var entries []stateEntry
	if err := postRPC(ctx, rpcURL, "getcontractstorage", []interface{}{contract}, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func postRPC(ctx context.Context, rpcURL, method string, params []interface{}, out interface{}) error {
	body, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params, ID: 1})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if out != nil && rpcResp.Result != nil {
		return json.Unmarshal(rpcResp.Result, out)
	}
	return nil
}

func writeKVBundle(ctx context.Context, rpcURL string, contracts []string, cached map[string][]stateEntry, outPath string) (string, int64, error) {
	f, err := os.Create(outPath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	hash := sha256.New()
	mw := io.MultiWriter(f, hash)

	gzw := gzip.NewWriter(mw)

	tw := tar.NewWriter(gzw)

	for _, c := range contracts {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		entries := cached[c]
		if len(entries) == 0 {
			entries, err = fetchContractStorage(ctx, rpcURL, c)
			if err != nil {
				return "", 0, fmt.Errorf("fetch contract storage %s: %w", c, err)
			}
		}
		payload, _ := json.Marshal(entries)
		hdr := &tar.Header{
			Name: fmt.Sprintf("%s.json", c),
			Mode: 0o644,
			Size: int64(len(payload)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return "", 0, err
		}
		if _, err := tw.Write(payload); err != nil {
			return "", 0, err
		}
	}
	if err := tw.Close(); err != nil {
		return "", 0, err
	}
	if err := gzw.Close(); err != nil {
		return "", 0, err
	}
	info, err := os.Stat(outPath)
	if err != nil {
		return "", 0, err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), info.Size(), nil
}

func writeManifest(path string, manifest snapshotManifest) error {
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func loadStorageFromDB(ctx context.Context, db *sql.DB, height int64) (map[string][]stateEntry, error) {
	out := make(map[string][]stateEntry)
	rows, err := db.QueryContext(ctx, `SELECT contract, kv FROM neo_storage WHERE height = $1`, height)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var contract string
		var raw []byte
		if err := rows.Scan(&contract, &raw); err != nil {
			return nil, err
		}
		var entries []stateEntry
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &entries)
		}
		out[contract] = entries
	}
	return out, rows.Err()
}

func loadStorageDiffsFromDB(ctx context.Context, db *sql.DB, height int64) (map[string][]stateEntry, error) {
	out := make(map[string][]stateEntry)
	rows, err := db.QueryContext(ctx, `SELECT contract, kv_diff FROM neo_storage_diffs WHERE height = $1`, height)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var contract string
		var raw []byte
		if err := rows.Scan(&contract, &raw); err != nil {
			return nil, err
		}
		var entries []stateEntry
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &entries)
		}
		out[contract] = entries
	}
	return out, rows.Err()
}

func signManifest(cfg Config, manifest snapshotManifest) (string, string, error) {
	key := strings.TrimSpace(cfg.SigningKey)
	if key == "" {
		return "", "", nil
	}
	raw, err := core.DecodeKey(key)
	if err != nil {
		return "", "", fmt.Errorf("decode signing key: %w", err)
	}
	if len(raw) != ed25519.PrivateKeySize {
		return "", "", fmt.Errorf("signing key must be ed25519 private key (%d bytes)", ed25519.PrivateKeySize)
	}
	priv := ed25519.PrivateKey(raw)
	pub := priv.Public().(ed25519.PublicKey)
	payload := fmt.Sprintf("%s|%d|%s|%s|%s", manifest.Network, manifest.Height, manifest.StateRoot, manifest.KVHash, manifest.KVDiffHash)
	sig := ed25519.Sign(priv, []byte(payload))
	return fmt.Sprintf("%x", pub), fmt.Sprintf("%x", sig), nil
}
