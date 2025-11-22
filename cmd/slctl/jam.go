package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func handleJAM(ctx context.Context, client *apiClient, args []string) error {
	if len(args) == 0 {
		return usageError(errors.New("jam command required"))
	}
	switch args[0] {
	case "preimage":
		return handleJAMPreimage(ctx, client, args[1:])
	case "package":
		return handleJAMPackage(ctx, client, args[1:])
	case "packages":
		return handleJAMPackagesList(ctx, client, args[1:])
	case "process":
		return handleJAMProcess(ctx, client)
	case "report":
		return handleJAMReport(ctx, client, args[1:])
	case "status":
		return handleJAMStatus(ctx, client)
	case "reports":
		return handleJAMReports(ctx, client, args[1:])
	default:
		return usageError(fmt.Errorf("unknown jam command %q", args[0]))
	}
}

func handleJAMPreimage(ctx context.Context, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("jam preimage put", flag.ContinueOnError)
	hash := fs.String("hash", "", "sha256 hash of the preimage (computed if empty)")
	file := fs.String("file", "", "path to file to upload")
	contentType := fs.String("content-type", "application/octet-stream", "content type to send")
	stat := fs.Bool("stat", false, "return metadata (HEAD) instead of uploading content")
	meta := fs.Bool("meta", false, "return JSON metadata (GET /meta) instead of upload")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	if *stat {
		if *hash == "" {
			return usageError(errors.New("hash is required for stat"))
		}
		_, headers, err := client.requestWithHeaders(ctx, http.MethodHead, "/jam/preimages/"+*hash, nil)
		if err != nil {
			return err
		}
		fmt.Printf("hash=%s size=%s media_type=%s\n", headers.Get("X-Preimage-Hash"), headers.Get("X-Preimage-Size"), headers.Get("X-Preimage-Media-Type"))
		return nil
	}
	if *meta {
		if *hash == "" {
			return usageError(errors.New("hash is required for meta"))
		}
		data, err := client.request(ctx, http.MethodGet, "/jam/preimages/"+*hash+"/meta", nil)
		if err != nil {
			return err
		}
		prettyPrint(data)
		return nil
	}

	if *file == "" {
		return usageError(errors.New("file is required for upload"))
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	sum := *hash
	if sum == "" {
		h := sha256.Sum256(data)
		sum = hex.EncodeToString(h[:])
		fmt.Fprintf(os.Stderr, "computed hash: %s\n", sum)
	}
	_, _, err = client.requestRaw(ctx, "PUT", "/jam/preimages/"+sum, data, *contentType)
	if err != nil {
		return err
	}
	fmt.Println("uploaded preimage", sum)
	return nil
}

func handleJAMPackage(ctx context.Context, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("jam package submit", flag.ContinueOnError)
	serviceID := fs.String("service", "", "service id")
	itemKind := fs.String("kind", "", "work item kind")
	paramsHash := fs.String("params-hash", "", "hash of item parameters")
	preimages := fs.String("preimages", "", "comma-separated preimage hashes for package")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	if *serviceID == "" || *itemKind == "" || *paramsHash == "" {
		return usageError(errors.New("service, kind, and params-hash are required"))
	}
	item := map[string]any{
		"kind":        *itemKind,
		"params_hash": *paramsHash,
	}
	pkg := map[string]any{
		"service_id": *serviceID,
		"items":      []any{item},
	}
	if hashes := splitList(*preimages); len(hashes) > 0 {
		pkg["preimage_hashes"] = hashes
	}
	data, err := client.request(ctx, "POST", "/jam/packages", pkg)
	if err != nil {
		return err
	}
	prettyPrint(data)
	return nil
}

func handleJAMPackagesList(ctx context.Context, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("jam packages", flag.ContinueOnError)
	status := fs.String("status", "", "filter by status (pending|applied|disputed)")
	service := fs.String("service", "", "filter by service id")
	limit := fs.Int("limit", 50, "max packages to return")
	offset := fs.Int("offset", 0, "offset for pagination")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	query := fmt.Sprintf("/jam/packages?limit=%d&offset=%d", *limit, *offset)
	if strings.TrimSpace(*status) != "" {
		query += "&status=" + url.QueryEscape(*status)
	}
	if strings.TrimSpace(*service) != "" {
		query += "&service_id=" + url.QueryEscape(*service)
	}
	data, err := client.request(ctx, "GET", query, nil)
	if err != nil {
		return err
	}
	var envelope struct {
		Items      []any `json:"items"`
		NextOffset any   `json:"next_offset"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Items) > 0 {
		pretty, _ := json.MarshalIndent(envelope, "", "  ")
		fmt.Println(string(pretty))
		return nil
	}
	prettyPrint(data)
	return nil
}

func handleJAMReports(ctx context.Context, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("jam reports", flag.ContinueOnError)
	service := fs.String("service", "", "filter by service id")
	limit := fs.Int("limit", 50, "max reports to return")
	offset := fs.Int("offset", 0, "offset for pagination")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	query := fmt.Sprintf("/jam/reports?limit=%d&offset=%d", *limit, *offset)
	if strings.TrimSpace(*service) != "" {
		query += "&service_id=" + url.QueryEscape(*service)
	}
	data, err := client.request(ctx, "GET", query, nil)
	if err != nil {
		return err
	}
	var envelope struct {
		Items      []any `json:"items"`
		NextOffset any   `json:"next_offset"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Items) > 0 {
		pretty, _ := json.MarshalIndent(envelope, "", "  ")
		fmt.Println(string(pretty))
		return nil
	}
	prettyPrint(data)
	return nil
}

func handleJAMProcess(ctx context.Context, client *apiClient) error {
	data, err := client.request(ctx, "POST", "/jam/process", nil)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) > 0 {
		prettyPrint(data)
	} else {
		fmt.Println("processed next package (if any)")
	}
	return nil
}

func handleJAMReport(ctx context.Context, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("jam report", flag.ContinueOnError)
	pkgID := fs.String("package", "", "package id")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	if *pkgID == "" {
		return usageError(errors.New("package id is required"))
	}
	data, err := client.request(ctx, "GET", "/jam/packages/"+*pkgID+"/report", nil)
	if err != nil {
		return err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		fmt.Println(string(data))
		return nil
	}
	pretty, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(pretty))
	return nil
}

func handleJAMStatus(ctx context.Context, client *apiClient) error {
	data, err := client.request(ctx, "GET", "/system/status", nil)
	if err != nil {
		return err
	}
	var payload struct {
		JAM map[string]any `json:"jam"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode status: %w", err)
	}
	enabled, _ := payload.JAM["enabled"].(bool)
	store, _ := payload.JAM["store"].(string)
	fmt.Printf("JAM enabled=%t", enabled)
	if store != "" {
		fmt.Printf(" store=%s", store)
	}
	fmt.Println()
	return nil
}
