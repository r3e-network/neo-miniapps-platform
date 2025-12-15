package httputil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/R3E-Network/service_layer/internal/logging"
	"github.com/R3E-Network/service_layer/internal/serviceauth"
)

func TestGetUserID_ProductionRequiresContext(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(serviceauth.UserIDHeader, "user-123")

	if got := GetUserID(req); got != "" {
		t.Fatalf("GetUserID() = %q, want empty in production without auth context", got)
	}

	rr := httptest.NewRecorder()
	_, ok := RequireUserID(rr, req)
	if ok {
		t.Fatal("RequireUserID() should fail without authenticated context in production")
	}
	if rr.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Result().StatusCode)
	}
}

func TestGetUserID_ProductionUsesServiceAuthContext(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")

	ctx := serviceauth.WithUserID(context.Background(), "user-123")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	req.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{&x509.Certificate{}}},
	}

	if got := GetUserID(req); got != "user-123" {
		t.Fatalf("GetUserID() = %q, want user-123", got)
	}
}

func TestGetUserID_ProductionUsesAuthContext(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")

	ctx := logging.WithUserID(context.Background(), "user-456")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	if got := GetUserID(req); got != "user-456" {
		t.Fatalf("GetUserID() = %q, want user-456", got)
	}
}

func TestGetUserID_NonProductionAllowsHeaderFallback(t *testing.T) {
	t.Setenv("MARBLE_ENV", "development")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(serviceauth.UserIDHeader, "user-789")

	if got := GetUserID(req); got != "user-789" {
		t.Fatalf("GetUserID() = %q, want user-789", got)
	}
}

func TestGetUserID_StrictModeRequiresVerifiedMTLS(t *testing.T) {
	t.Setenv("MARBLE_ENV", "development")
	t.Setenv("OE_SIMULATION", "0")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(serviceauth.UserIDHeader, "user-789")

	if got := GetUserID(req); got != "" {
		t.Fatalf("GetUserID() = %q, want empty in strict mode without verified mTLS", got)
	}

	req.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{&x509.Certificate{}}},
	}
	if got := GetUserID(req); got != "user-789" {
		t.Fatalf("GetUserID() = %q, want user-789 with verified mTLS", got)
	}
}

func TestGetServiceID_HeaderFallbackNonProduction(t *testing.T) {
	t.Setenv("MARBLE_ENV", "development")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(serviceauth.ServiceIDHeader, "gateway")

	if got := GetServiceID(req); got != "gateway" {
		t.Fatalf("GetServiceID() = %q, want gateway", got)
	}
}

func TestGetServiceID_StrictModeRequiresVerifiedMTLS(t *testing.T) {
	t.Setenv("MARBLE_ENV", "development")
	t.Setenv("OE_SIMULATION", "0")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(serviceauth.ServiceIDHeader, "gateway")

	if got := GetServiceID(req); got != "" {
		t.Fatalf("GetServiceID() = %q, want empty in strict mode without verified mTLS", got)
	}

	req.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{&x509.Certificate{DNSNames: []string{"gateway"}}}},
	}
	if got := GetServiceID(req); got != "gateway" {
		t.Fatalf("GetServiceID() = %q, want gateway with verified mTLS", got)
	}
}

func TestGetServiceID_ProductionRequiresVerifiedMTLSForHeader(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(serviceauth.ServiceIDHeader, "gateway")

	if got := GetServiceID(req); got != "" {
		t.Fatalf("GetServiceID() = %q, want empty in production without verified mTLS", got)
	}

	rr := httptest.NewRecorder()
	_, ok := RequireServiceID(rr, req)
	if ok {
		t.Fatal("RequireServiceID() should fail in production without verified mTLS")
	}
	if rr.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Result().StatusCode)
	}

	req.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{&x509.Certificate{DNSNames: []string{"gateway"}}}},
	}
	if got := GetServiceID(req); got != "gateway" {
		t.Fatalf("GetServiceID() = %q, want gateway with verified mTLS", got)
	}
	rr = httptest.NewRecorder()
	if svcID, ok := RequireServiceID(rr, req); !ok || svcID != "gateway" {
		t.Fatalf("RequireServiceID() = (%q,%v), want (gateway,true)", svcID, ok)
	}
}

func TestGetServiceID_ProductionUsesServiceAuthContext(t *testing.T) {
	t.Setenv("MARBLE_ENV", "production")

	ctx := serviceauth.WithServiceID(context.Background(), "gateway")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	req.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{&x509.Certificate{DNSNames: []string{"gateway"}}}},
	}
	rr := httptest.NewRecorder()

	if got := GetServiceID(req); got != "gateway" {
		t.Fatalf("GetServiceID() = %q, want gateway", got)
	}
	if svcID, ok := RequireServiceID(rr, req); !ok || svcID != "gateway" {
		t.Fatalf("RequireServiceID() = (%q,%v), want (gateway,true)", svcID, ok)
	}
}

func TestWriteErrorHelpers(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteErrorWithCode(rr, http.StatusBadRequest, "bad", "nope")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	var body ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Message != "nope" {
		t.Fatalf("message = %q, want nope", body.Message)
	}
	if body.Code != "bad" {
		t.Fatalf("code = %q, want bad", body.Code)
	}

	rr = httptest.NewRecorder()
	Forbidden(rr, "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}

	rr = httptest.NewRecorder()
	NotFound(rr, "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}

	rr = httptest.NewRecorder()
	InternalError(rr, "")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}

	rr = httptest.NewRecorder()
	ServiceUnavailable(rr, "")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Value string `json:"value"`
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"value":"ok"}`))
	rr := httptest.NewRecorder()
	var v payload
	if ok := DecodeJSON(rr, req, &v); !ok {
		t.Fatalf("DecodeJSON() = false, want true")
	}
	if v.Value != "ok" {
		t.Fatalf("value = %q, want ok", v.Value)
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{invalid`))
	rr = httptest.NewRecorder()
	if ok := DecodeJSON(rr, req, &v); ok {
		t.Fatalf("DecodeJSON() = true, want false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"value":"ok"}`))
	rr = httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rr, req.Body, 4)
	if ok := DecodeJSON(rr, req, &v); ok {
		t.Fatalf("DecodeJSON() = true, want false for oversized body")
	}
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rr.Code)
	}
}

func TestDecodeJSONOptional(t *testing.T) {
	type payload struct {
		Value string `json:"value"`
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	var v payload
	if ok := DecodeJSONOptional(rr, req, &v); !ok {
		t.Fatalf("DecodeJSONOptional() = false, want true for empty body")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	if ok := DecodeJSONOptional(rr, req, &v); !ok {
		t.Fatalf("DecodeJSONOptional() = false, want true for EOF body")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{invalid`))
	if ok := DecodeJSONOptional(rr, req, &v); ok {
		t.Fatalf("DecodeJSONOptional() = true, want false for invalid JSON")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"value":"ok"}`))
	req.Body = http.MaxBytesReader(rr, req.Body, 4)
	if ok := DecodeJSONOptional(rr, req, &v); ok {
		t.Fatalf("DecodeJSONOptional() = true, want false for oversized body")
	}
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rr.Code)
	}
}

func TestPathAndQueryHelpers(t *testing.T) {
	if got := PathParam("/users/123/orders", "/users/", "/orders"); got != "123" {
		t.Fatalf("PathParam() = %q, want 123", got)
	}
	if got := PathParam("/users/123/orders/456", "/users/", "/orders"); got != "123" {
		t.Fatalf("PathParam() = %q, want 123", got)
	}
	if got := PathParam("/users/123/profile", "/users/", "/orders"); got != "123" {
		t.Fatalf("PathParam() = %q, want 123", got)
	}
	if got := PathParamAt("/users/123/orders/456", 1); got != "123" {
		t.Fatalf("PathParamAt() = %q, want 123", got)
	}
	if got := PathParamAt("/users/123/orders/456", 10); got != "" {
		t.Fatalf("PathParamAt() = %q, want empty", got)
	}

	req := httptest.NewRequest(http.MethodGet, "/?a=10&b=xyz&c=true&d=yes&e=0&f=123&s=hello", nil)
	if got := QueryInt(req, "a", 0); got != 10 {
		t.Fatalf("QueryInt(a) = %d, want 10", got)
	}
	if got := QueryInt(req, "b", 7); got != 7 {
		t.Fatalf("QueryInt(b) = %d, want 7", got)
	}
	if got := QueryInt(req, "missing", 7); got != 7 {
		t.Fatalf("QueryInt(missing) = %d, want 7", got)
	}
	if got := QueryInt64(req, "missing", 9); got != 9 {
		t.Fatalf("QueryInt64(missing) = %d, want 9", got)
	}
	if got := QueryInt64(req, "f", 0); got != 123 {
		t.Fatalf("QueryInt64(f) = %d, want 123", got)
	}
	if got := QueryString(req, "missing", "x"); got != "x" {
		t.Fatalf("QueryString(missing) = %q, want x", got)
	}
	if got := QueryString(req, "s", "x"); got != "hello" {
		t.Fatalf("QueryString(s) = %q, want hello", got)
	}
	if got := QueryBool(req, "c", false); got != true {
		t.Fatalf("QueryBool(c) = %v, want true", got)
	}
	if got := QueryBool(req, "d", false); got != true {
		t.Fatalf("QueryBool(d) = %v, want true", got)
	}
	if got := QueryBool(req, "e", true); got != false {
		t.Fatalf("QueryBool(e) = %v, want false", got)
	}
	if got := QueryBool(req, "missingBool", true); got != true {
		t.Fatalf("QueryBool(missingBool) = %v, want true", got)
	}
}

func TestRequireAdminRole(t *testing.T) {
	t.Setenv("MARBLE_ENV", "development")
	t.Setenv("OE_SIMULATION", "1")
	t.Setenv("MARBLE_CERT", "")
	t.Setenv("MARBLE_KEY", "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Role", "admin")
	rr := httptest.NewRecorder()
	if !RequireAdminRole(rr, req) {
		t.Fatalf("RequireAdminRole(admin) = false, want true")
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Role", "super_admin")
	rr = httptest.NewRecorder()
	if !RequireAdminRole(rr, req) {
		t.Fatalf("RequireAdminRole(super_admin) = false, want true")
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Role", "user")
	rr = httptest.NewRecorder()
	if RequireAdminRole(rr, req) {
		t.Fatalf("RequireAdminRole(user) = true, want false")
	}
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestGetUserRole_StrictModeRequiresVerifiedMTLS(t *testing.T) {
	t.Setenv("MARBLE_ENV", "development")
	t.Setenv("OE_SIMULATION", "0")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Role", "admin")

	if got := GetUserRole(req); got != "" {
		t.Fatalf("GetUserRole() = %q, want empty in strict mode without verified mTLS", got)
	}

	req.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{&x509.Certificate{}}},
	}
	if got := GetUserRole(req); got != "admin" {
		t.Fatalf("GetUserRole() = %q, want admin with verified mTLS", got)
	}
}

func TestPaginationParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?offset=-2&limit=999", nil)
	offset, limit := PaginationParams(req, 10, 100)
	if offset != 0 {
		t.Fatalf("offset = %d, want 0", offset)
	}
	if limit != 100 {
		t.Fatalf("limit = %d, want 100", limit)
	}

	req = httptest.NewRequest(http.MethodGet, "/?offset=3&limit=0", nil)
	offset, limit = PaginationParams(req, 10, 100)
	if offset != 3 {
		t.Fatalf("offset = %d, want 3", offset)
	}
	if limit != 1 {
		t.Fatalf("limit = %d, want 1", limit)
	}
}

func TestWrapError(t *testing.T) {
	if WrapError(nil, "context") != nil {
		t.Fatalf("WrapError(nil) should return nil")
	}

	err := WrapError(errors.New("boom"), "context")
	if err == nil {
		t.Fatalf("WrapError() returned nil")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Fatalf("wrapped error = %q, want to contain context", err.Error())
	}
}
