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
		return handleJAMStatus(ctx, client, args[1:])
	case "reports":
		return handleJAMReports(ctx, client, args[1:])
	case "receipt":
		return handleJAMReceipt(ctx, client, args[1:])
	case "receipts":
		return handleJAMReceipts(ctx, client, args[1:])
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
	includeReceipt := fs.Bool("include-receipt", false, "include accumulator receipt in response")
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
	path := "/jam/packages"
	if *includeReceipt {
		path += "?include_receipt=true"
	}
	data, err := client.request(ctx, "POST", path, pkg)
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
	includeReceipt := fs.Bool("include-receipt", false, "include receipts in response")
	tabular := fs.Bool("table", false, "print packages in a table")
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
	if *includeReceipt {
		query += "&include_receipt=true"
	}
	data, err := client.request(ctx, "GET", query, nil)
	if err != nil {
		return err
	}
	if *tabular {
		var envelope struct {
			Items []struct {
				ID        string `json:"id"`
				ServiceID string `json:"service_id"`
				Status    string `json:"status"`
				CreatedAt string `json:"created_at"`
			} `json:"items"`
			NextOffset int `json:"next_offset"`
		}
		if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Items) > 0 {
			fmt.Println("Packages:")
			for _, item := range envelope.Items {
				fmt.Printf("- id=%s service=%s status=%s created_at=%s\n", item.ID, item.ServiceID, item.Status, item.CreatedAt)
			}
			fmt.Printf("next_offset=%d\n", envelope.NextOffset)
			return nil
		}
	}
	prettyPrint(data)
	return nil
}

func handleJAMReports(ctx context.Context, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("jam reports", flag.ContinueOnError)
	service := fs.String("service", "", "filter by service id")
	limit := fs.Int("limit", 50, "max reports to return")
	offset := fs.Int("offset", 0, "offset for pagination")
	includeReceipt := fs.Bool("include-receipt", false, "include receipts in response")
	tabular := fs.Bool("table", false, "print reports in a table")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	query := fmt.Sprintf("/jam/reports?limit=%d&offset=%d", *limit, *offset)
	if strings.TrimSpace(*service) != "" {
		query += "&service_id=" + url.QueryEscape(*service)
	}
	if *includeReceipt {
		query += "&include_receipt=true"
	}
	data, err := client.request(ctx, "GET", query, nil)
	if err != nil {
		return err
	}
	if *tabular {
		var envelope struct {
			Items []struct {
				ID        string `json:"id"`
				ServiceID string `json:"service_id"`
				PackageID string `json:"package_id"`
				CreatedAt string `json:"created_at"`
			} `json:"items"`
			NextOffset int `json:"next_offset"`
		}
		if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Items) > 0 {
			fmt.Println("Reports:")
			for _, item := range envelope.Items {
				fmt.Printf("- id=%s service=%s package=%s created_at=%s\n", item.ID, item.ServiceID, item.PackageID, item.CreatedAt)
			}
			fmt.Printf("next_offset=%d\n", envelope.NextOffset)
			return nil
		}
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
	includeReceipt := fs.Bool("include-receipt", false, "include receipt in response (if available)")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	if *pkgID == "" {
		return usageError(errors.New("package id is required"))
	}
	path := "/jam/packages/" + *pkgID + "/report"
	if *includeReceipt {
		path += "?include_receipt=true"
	}
	data, err := client.request(ctx, "GET", path, nil)
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

func handleJAMStatus(ctx context.Context, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("jam status", flag.ContinueOnError)
	service := fs.String("service", "", "service id to include accumulator root (optional)")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	path := "/jam/status"
	if strings.TrimSpace(*service) != "" {
		path += "?service_id=" + url.QueryEscape(*service)
	}
	data, err := client.request(ctx, "GET", path, nil)
	if err != nil {
		return err
	}
	var payload struct {
		Enabled             bool             `json:"enabled"`
		Store               string           `json:"store"`
		RateLimitPerMinute  int              `json:"rate_limit_per_min"`
		MaxPreimageBytes    int64            `json:"max_preimage_bytes"`
		MaxPendingPackages  int              `json:"max_pending_packages"`
		AuthRequired        bool             `json:"auth_required"`
		LegacyListResponse  bool             `json:"legacy_list_response"`
		AccumulatorsEnabled bool             `json:"accumulators_enabled"`
		AccumulatorHash     string           `json:"accumulator_hash"`
		AccumulatorRoot     map[string]any   `json:"accumulator_root"`
		AccumulatorRoots    []map[string]any `json:"accumulator_roots"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode status: %w", err)
	}
	fmt.Printf("JAM enabled=%t", payload.Enabled)
	if payload.Store != "" {
		fmt.Printf(" store=%s", payload.Store)
	}
	if payload.AccumulatorsEnabled {
		fmt.Printf(" accumulators_enabled=%t", payload.AccumulatorsEnabled)
	}
	if payload.AccumulatorHash != "" {
		fmt.Printf(" accumulator_hash=%s", payload.AccumulatorHash)
	}
	fmt.Println()
	if len(payload.AccumulatorRoot) > 0 {
		pretty, _ := json.MarshalIndent(payload.AccumulatorRoot, "", "  ")
		fmt.Printf("Accumulator root:\n%s\n", string(pretty))
	}
	if len(payload.AccumulatorRoots) > 0 {
		fmt.Println("Accumulator roots:")
		for _, root := range payload.AccumulatorRoots {
			svc, _ := root["service_id"].(string)
			seq, _ := toInt64(root["seq"])
			r, _ := root["root"].(string)
			fmt.Printf("  - %s seq=%d root=%s\n", svc, seq, r)
		}
	}
	return nil
}

func handleJAMReceipt(ctx context.Context, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("jam receipt", flag.ContinueOnError)
	hash := fs.String("hash", "", "receipt hash to fetch")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	if *hash == "" {
		if len(args) > 0 {
			*hash = args[0]
		}
		if *hash == "" {
			return usageError(errors.New("hash is required"))
		}
	}
	path := "/jam/receipts/" + url.PathEscape(strings.TrimSpace(*hash))
	data, err := client.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	prettyPrint(data)
	return nil
}

func handleJAMReceipts(ctx context.Context, client *apiClient, args []string) error {
	fs := flag.NewFlagSet("jam receipts", flag.ContinueOnError)
	service := fs.String("service", "", "filter by service id")
	limit := fs.Int("limit", 50, "limit results")
	offset := fs.Int("offset", 0, "offset for pagination")
	tabular := fs.Bool("table", true, "print receipts in a table")
	if err := fs.Parse(args); err != nil {
		return usageError(err)
	}
	query := fmt.Sprintf("/jam/receipts?limit=%d&offset=%d", *limit, *offset)
	if strings.TrimSpace(*service) != "" {
		query += "&service_id=" + url.QueryEscape(strings.TrimSpace(*service))
	}
	data, err := client.request(ctx, http.MethodGet, query, nil)
	if err != nil {
		return err
	}
	var envelope struct {
		Items      []map[string]any `json:"items"`
		NextOffset int              `json:"next_offset"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Items) > 0 {
		if *tabular {
			for _, item := range envelope.Items {
				hash, _ := item["hash"].(string)
				svc, _ := item["service_id"].(string)
				et, _ := item["entry_type"].(string)
				seq, _ := toInt64(item["seq"])
				root, _ := item["new_root"].(string)
				fmt.Printf("- hash=%s service=%s entry_type=%s seq=%d new_root=%s\n", hash, svc, et, seq, root)
			}
			fmt.Printf("next_offset=%d\n", envelope.NextOffset)
			return nil
		}
	}
	prettyPrint(data)
	return nil
}
