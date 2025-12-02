package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
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

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

type manifest struct {
	Network          string `json:"network"`
	Height           int64  `json:"height"`
	StateRoot        string `json:"state_root"`
	KVURL            string `json:"kv_url,omitempty"`
	KVHash           string `json:"kv_sha256,omitempty"`
	KVDiffURL        string `json:"kv_diff_url,omitempty"`
	KVDiffHash       string `json:"kv_diff_sha256,omitempty"`
	Signature        string `json:"signature,omitempty"`
	SigningPublicKey string `json:"signing_public_key,omitempty"`
}

func handleManifest(ctx context.Context, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("manifest", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	urlFlag := fs.String("url", "", "Manifest URL (http/https)")
	pathFlag := fs.String("file", "", "Manifest file path (local)")
	outPath := fs.String("out", "", "Path to write the fetched manifest (optional)")
	downloadBundles := fs.Bool("download-bundles", false, "Download KV and diff bundles to disk when URLs are present")
	kvOut := fs.String("kv-out", "", "Path to write the KV bundle (defaults to ./block-<height>-kv.tar.gz)")
	kvDiffOut := fs.String("kv-diff-out", "", "Path to write the KV diff bundle (defaults to ./block-<height>-kv-diff.tar.gz)")
	timeout := fs.Duration("timeout", 20*time.Second, "Download timeout for bundles")
	verifyBundles := fs.Bool("verify-bundles", true, "Verify KV and diff bundles if URLs and hashes are present")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	if *urlFlag == "" && *pathFlag == "" {
		return usageError(errors.New("manifest requires --url or --file"))
	}

	var data []byte
	var err error
	if *urlFlag != "" {
		data, err = fetchBytes(ctx, client, *urlFlag, *timeout)
	} else {
		data, err = os.ReadFile(*pathFlag)
	}
	if err != nil {
		return err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}

	fmt.Printf("Manifest for %s height %d\n", m.Network, m.Height)
	fmt.Printf("State root: %s\n", m.StateRoot)
	if path := strings.TrimSpace(*outPath); path != "" {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write manifest: %w", err)
		}
		fmt.Println("Wrote manifest to", path)
	}

	// Verify bundle hashes if URLs/hashes present.
	if *verifyBundles {
		if m.KVURL != "" && m.KVHash != "" {
			if err := verifyResource(ctx, client, m.KVURL, m.KVHash, *timeout); err != nil {
				return fmt.Errorf("kv bundle verify failed: %w", err)
			}
			fmt.Println("KV bundle hash OK")
		} else {
			fmt.Println("KV bundle: no url/hash provided; skipped")
		}
		if m.KVDiffURL != "" && m.KVDiffHash != "" {
			if err := verifyResource(ctx, client, m.KVDiffURL, m.KVDiffHash, *timeout); err != nil {
				return fmt.Errorf("kv diff verify failed: %w", err)
			}
			fmt.Println("KV diff hash OK")
		} else if m.KVDiffURL != "" || m.KVDiffHash != "" {
			fmt.Println("KV diff: missing url or hash; skipped")
		}
	}

	// Optionally download bundles to disk.
	if *downloadBundles {
		baseHeight := m.Height
		if m.KVURL != "" {
			dest := strings.TrimSpace(*kvOut)
			if dest == "" {
				dest = fmt.Sprintf("block-%d-kv.tar.gz", baseHeight)
			}
			if err := downloadBundle(ctx, client, m.KVURL, m.KVHash, dest, *timeout); err != nil {
				return fmt.Errorf("download kv bundle: %w", err)
			}
			fmt.Println("KV bundle saved to", dest)
		}
		if m.KVDiffURL != "" {
			dest := strings.TrimSpace(*kvDiffOut)
			if dest == "" {
				dest = fmt.Sprintf("block-%d-kv-diff.tar.gz", baseHeight)
			}
			if err := downloadBundle(ctx, client, m.KVDiffURL, m.KVDiffHash, dest, *timeout); err != nil {
				return fmt.Errorf("download kv diff bundle: %w", err)
			}
			fmt.Println("KV diff bundle saved to", dest)
		}
	}

	// Verify signature if present.
	if m.SigningPublicKey != "" && m.Signature != "" {
		if err := verifyManifestSignature(m); err != nil {
			return fmt.Errorf("signature verify failed: %w", err)
		}
		fmt.Println("Manifest signature OK")
	} else {
		fmt.Println("Signature: not present; skipped")
	}

	return nil
}

func fetchBytes(ctx context.Context, client *apiClient, target string, timeout time.Duration) ([]byte, error) {
	// HTTP(S) with optional bearer token.
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		if client != nil && client.token != "" {
			req.Header.Set("Authorization", "Bearer "+client.token)
		}
		httpClient := &http.Client{Timeout: timeout}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("download %s: %d %s", target, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return io.ReadAll(resp.Body)
	}
	// API-relative path (requires client).
	if client != nil && strings.HasPrefix(target, "/") {
		return client.request(ctx, http.MethodGet, target, nil)
	}
	// Local file fallback.
	return os.ReadFile(target)
}

func verifyResource(ctx context.Context, client *apiClient, url string, expected string, timeout time.Duration) error {
	body, err := fetchBytes(ctx, client, url, timeout)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("expected %s, got %s", expected, got)
	}
	return nil
}

func downloadBundle(ctx context.Context, client *apiClient, url string, expectedSHA string, dest string, timeout time.Duration) error {
	body, err := fetchBytes(ctx, client, url, timeout)
	if err != nil {
		return err
	}
	if expectedSHA != "" {
		sum := sha256.Sum256(body)
		got := hex.EncodeToString(sum[:])
		if !strings.EqualFold(got, expectedSHA) {
			return fmt.Errorf("sha mismatch: expected %s got %s", expectedSHA, got)
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("prepare output dir: %w", err)
	}
	return os.WriteFile(dest, body, 0o644)
}

func verifyManifestSignature(m manifest) error {
	pub, err := core.DecodeKey(m.SigningPublicKey)
	if err != nil {
		return err
	}
	sig, err := core.DecodeKey(m.Signature)
	if err != nil {
		return err
	}
	payload := fmt.Sprintf("%s|%d|%s|%s|%s", m.Network, m.Height, m.StateRoot, m.KVHash, m.KVDiffHash)
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("signing public key must be %d bytes", ed25519.PublicKeySize)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("signature must be %d bytes", ed25519.SignatureSize)
	}
	if !ed25519.Verify(pub, []byte(payload), sig) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
