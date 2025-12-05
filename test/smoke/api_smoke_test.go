//go:build smoke

// Package smoke contains API smoke tests for RLS propagation.
package smoke

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestSmoke_API_RLS_WithBearerToken(t *testing.T) {
	baseURL := getEnv("API_BASE_URL", "http://localhost:8080")
	token := os.Getenv("SMOKE_BEARER_TOKEN")
	if token == "" {
		t.Skip("SMOKE_BEARER_TOKEN not set")
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// Oracle request
	oracleBody := map[string]any{
		"url":    "https://example.com",
		"method": "GET",
	}
	oracleResp, err := postJSON(client, baseURL+"/api/oracle/request", token, oracleBody, http.StatusOK)
	if err != nil {
		t.Fatalf("oracle request failed: %v", err)
	}
	if err := getWithToken(client, baseURL+"/api/oracle/requests", token, http.StatusOK); err != nil {
		t.Fatalf("oracle requests list failed: %v", err)
	}
	if err := getWithToken(client, baseURL+"/api/oracle/requests?status=pending", token, http.StatusOK); err != nil {
		t.Fatalf("oracle requests list (status filter) failed: %v", err)
	}
	if rid, ok := oracleResp["request_id"].(string); ok && rid != "" {
		_ = getWithToken(client, baseURL+"/api/oracle/requests/"+rid, token, http.StatusOK)
	}

	// VRF request
	vrfBody := map[string]any{
		"seed": "test-seed",
	}
	vrfResp, err := postJSON(client, baseURL+"/api/vrf/request", token, vrfBody, http.StatusOK)
	if err != nil {
		t.Fatalf("vrf request failed: %v", err)
	}
	if err := getWithToken(client, baseURL+"/api/vrf/requests", token, http.StatusOK); err != nil {
		t.Fatalf("vrf requests list failed: %v", err)
	}
	if err := getWithToken(client, baseURL+"/api/vrf/requests?status=pending", token, http.StatusOK); err != nil {
		t.Fatalf("vrf requests list (status filter) failed: %v", err)
	}
	if rid, ok := vrfResp["request_id"].(string); ok && rid != "" {
		_ = getWithToken(client, baseURL+"/api/vrf/requests/"+rid, token, http.StatusOK)
	}

	// Gasbank deposit
	gbBody := map[string]any{
		"address": "acct-smoke",
		"amount":  "1.0",
	}
	if _, err := postJSON(client, baseURL+"/api/gasbank/deposit", token, gbBody, http.StatusOK); err != nil {
		t.Fatalf("gasbank deposit failed: %v", err)
	}
	// Gasbank transactions list
	if err := getWithToken(client, baseURL+"/api/gasbank/transactions", token, http.StatusOK); err != nil {
		t.Fatalf("gasbank transactions list failed: %v", err)
	}
	if err := getWithToken(client, baseURL+"/api/gasbank/transactions?status=pending", token, http.StatusOK); err != nil {
		t.Fatalf("gasbank transactions list (status filter) failed: %v", err)
	}
	if err := getWithToken(client, baseURL+"/api/gasbank/transactions?account_id=acct-smoke", token, http.StatusOK); err != nil {
		t.Fatalf("gasbank transactions list (account filter) failed: %v", err)
	}

	// Datafeeds list
	if err := getWithToken(client, baseURL+"/api/datafeeds", token, http.StatusOK); err != nil {
		t.Fatalf("datafeeds list failed: %v", err)
	}

	// Datafeed create + price update + fetch
	dfResp, err := postJSON(client, baseURL+"/api/datafeeds", token, map[string]any{
		"name":             "smoke-feed",
		"symbol":           "SMK/USD",
		"sources":          []string{"smoke"},
		"interval_seconds": 60,
	}, http.StatusOK)
	if err != nil {
		t.Fatalf("datafeed create failed: %v", err)
	}
	feedID, _ := dfResp["id"].(string)
	if feedID != "" {
		if _, err := putJSON(client, baseURL+"/api/datafeeds/"+feedID, token, map[string]any{
			"price":  42.0,
			"source": "smoke",
		}, http.StatusOK); err != nil {
			t.Fatalf("datafeed price update failed: %v", err)
		}
		if err := getWithToken(client, baseURL+"/api/datafeeds/"+feedID, token, http.StatusOK); err != nil {
			t.Fatalf("datafeed fetch failed: %v", err)
		}
	}

	// Mixer request
	mixerBody := map[string]any{
		"depositor":         "NXXX",
		"amount":            "1",
		"encrypted_payload": "payload",
	}
	mixResp, err := postJSON(client, baseURL+"/api/mixer/mix", token, mixerBody, http.StatusOK)
	if err != nil {
		t.Fatalf("mixer request failed: %v", err)
	}

	// Mixer requests list (should be scoped to user via RLS)
	if err := getWithToken(client, baseURL+"/api/mixer/requests", token, http.StatusOK); err != nil {
		t.Fatalf("mixer requests list failed: %v", err)
	}
	if err := getWithToken(client, baseURL+"/api/mixer/requests?status=pending", token, http.StatusOK); err != nil {
		t.Fatalf("mixer requests list (status filter) failed: %v", err)
	}
	if err := getWithToken(client, baseURL+"/api/mixer/requests?account_id="+token, token, http.StatusOK); err != nil {
		t.Fatalf("mixer requests list (account filter) failed: %v", err)
	}
	if rid, ok := mixResp["request_id"].(string); ok && rid != "" {
		_ = getWithToken(client, baseURL+"/api/mixer/requests/"+rid, token, http.StatusOK)
		_ = getWithToken(client, baseURL+"/api/mixer/requests/"+rid+"/transactions?status=pending", token, http.StatusOK)
	}
	// Mixer transactions (per request) and refund
	mixReqResp, err := postJSON(client, baseURL+"/api/mixer/mix", token, mixerBody, http.StatusOK)
	if err == nil {
		if reqID, ok := mixReqResp["request_id"].(string); ok && reqID != "" {
			_ = getWithToken(client, baseURL+"/api/mixer/requests/"+reqID+"/transactions", token, http.StatusOK)
			_ = postJSON(client, baseURL+"/api/mixer/requests/"+reqID+"/refund", token, map[string]any{"tx_hash": "refund-smoke"}, http.StatusOK)
		}
	}

	// Accounts create
	accountBody := map[string]any{
		"email": "smoke@example.com",
		"name":  "smoke",
	}
	if _, err := postJSON(client, baseURL+"/api/accounts", token, accountBody, http.StatusOK); err != nil {
		t.Fatalf("accounts create failed: %v", err)
	}

	// Accounts list
	if err := getWithToken(client, baseURL+"/api/accounts", token, http.StatusOK); err != nil {
		t.Fatalf("accounts list failed: %v", err)
	}

	// Confidential compute
	confBody := map[string]any{
		"code":   "return 1",
		"inputs": map[string]any{"x": 1},
	}
	resp, err := postJSON(client, baseURL+"/api/confidential/compute", token, confBody, http.StatusOK)
	if err != nil {
		t.Fatalf("confidential compute failed: %v", err)
	}
	if id, ok := resp["compute_id"].(string); ok && id != "" {
		if err := getWithToken(client, baseURL+"/api/confidential/results/"+id, token, http.StatusOK); err != nil {
			t.Fatalf("confidential results fetch failed: %v", err)
		}
	}
}

func postJSON(client *http.Client, url, token string, body map[string]any, wantStatus int) (map[string]any, error) {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func putJSON(client *http.Client, url, token string, body map[string]any, wantStatus int) (map[string]any, error) {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func getWithToken(client *http.Client, url, token string, wantStatus int) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
