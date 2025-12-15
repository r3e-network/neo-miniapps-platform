package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	signerpb "github.com/R3E-Network/service_layer/api/gen/signer"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/R3E-Network/service_layer/services/teesigner/signer"
	"google.golang.org/grpc/test/bufconn"
)

func TestListenPort(t *testing.T) {
	orig, had := os.LookupEnv("PORT")
	if had {
		defer os.Setenv("PORT", orig)
	} else {
		defer os.Unsetenv("PORT")
	}

	os.Setenv("PORT", "1234")
	if got := listenPort(); got != 1234 {
		t.Fatalf("expected 1234, got %d", got)
	}

	os.Setenv("PORT", "0")
	if got := listenPort(); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}

	os.Setenv("PORT", "70000")
	if got := listenPort(); got != 9090 {
		t.Fatalf("expected default 9090, got %d", got)
	}

	os.Setenv("PORT", "not-a-number")
	if got := listenPort(); got != 9090 {
		t.Fatalf("expected default 9090, got %d", got)
	}
}

func TestNewGRPCServer(t *testing.T) {
	if srv := newGRPCServer(nil); srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv := newGRPCServer(&tls.Config{}); srv == nil {
		t.Fatal("expected non-nil TLS server")
	}
}

func TestInternalHTTPPort(t *testing.T) {
	orig, had := os.LookupEnv("INTERNAL_HTTP_PORT")
	if had {
		defer os.Setenv("INTERNAL_HTTP_PORT", orig)
	} else {
		defer os.Unsetenv("INTERNAL_HTTP_PORT")
	}

	_ = os.Unsetenv("INTERNAL_HTTP_PORT")
	if _, ok := internalHTTPPort(); ok {
		t.Fatal("expected internal http port to be disabled by default")
	}

	_ = os.Setenv("INTERNAL_HTTP_PORT", "8080")
	if port, ok := internalHTTPPort(); !ok || port != 8080 {
		t.Fatalf("expected enabled port 8080, got %d ok=%v", port, ok)
	}

	_ = os.Setenv("INTERNAL_HTTP_PORT", "0")
	if _, ok := internalHTTPPort(); ok {
		t.Fatal("expected port 0 to disable internal http")
	}

	_ = os.Setenv("INTERNAL_HTTP_PORT", "70000")
	if _, ok := internalHTTPPort(); ok {
		t.Fatal("expected invalid port to disable internal http")
	}

	_ = os.Setenv("INTERNAL_HTTP_PORT", "not-a-number")
	if _, ok := internalHTTPPort(); ok {
		t.Fatal("expected invalid value to disable internal http")
	}
}

func TestInternalRotateKeyHandler(t *testing.T) {
	current := time.Unix(1, 0).UTC()
	nowFn := func() time.Time { return current }

	svc, err := signer.NewService(signer.ServiceConfig{
		MasterKeySeed: []byte("seed"),
		Now:           nowFn,
		RateLimiter:   signer.NewClientRateLimiter(1000, 1000, nowFn),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	handler := newInternalHTTPHandler(svc)

	getReq := httptest.NewRequest(http.MethodGet, "/internal/rotate-key", nil)
	getResp := httptest.NewRecorder()
	handler.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", getResp.Code)
	}

	current = time.Unix(2, 0).UTC()
	postReq := httptest.NewRequest(http.MethodPost, "/internal/rotate-key", nil)
	postResp := httptest.NewRecorder()
	handler.ServeHTTP(postResp, postReq)
	if postResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", postResp.Code)
	}

	var rotation signer.RotationResult
	if err := json.Unmarshal(postResp.Body.Bytes(), &rotation); err != nil {
		t.Fatalf("unmarshal rotation response: %v", err)
	}
	if rotation.NewVersion != "v2" {
		t.Fatalf("expected new_version=v2, got %q", rotation.NewVersion)
	}

	signResp, err := svc.Sign(context.Background(), &signerpb.SignRequest{TxHash: "0x" + strings.Repeat("11", 32)})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if signResp.GetKeyVersion() != "v2" {
		t.Fatalf("expected sign key_version=v2, got %q", signResp.GetKeyVersion())
	}
}

func TestInitSupabase_DoesNotRequireEnvInDev(t *testing.T) {
	m, err := marble.New(marble.Config{MarbleType: "tee-signer"})
	if err != nil {
		t.Fatalf("marble.New: %v", err)
	}
	repo, err := initSupabase(m)
	if err != nil {
		t.Fatalf("initSupabase: %v", err)
	}
	if repo == nil {
		t.Fatal("expected repo to be non-nil")
	}
}

func TestRun_MissingMasterKeySeed(t *testing.T) {
	withEnv(t, "MASTER_KEY_SEED", "", func() {
		ctx, cancel := contextWithTimeout(250 * time.Millisecond)
		defer cancel()
		if err := run(ctx); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRun_StrictModeRequiresTLS(t *testing.T) {
	withEnv(t, "MARBLE_ENV", "production", func() {
		withEnv(t, "MASTER_KEY_SEED", "seed", func() {
			ctx, cancel := contextWithTimeout(250 * time.Millisecond)
			defer cancel()
			if err := run(ctx); err == nil {
				t.Fatal("expected error in strict mode without TLS")
			}
		})
	})
}

func TestRun_MarbleTLSMissingRootCA(t *testing.T) {
	withEnv(t, "MASTER_KEY_SEED", "seed", func() {
		withEnv(t, "MARBLE_CERT", "not-a-cert", func() {
			withEnv(t, "MARBLE_KEY", "not-a-key", func() {
				withEnv(t, "MARBLE_ROOT_CA", "", func() {
					ctx, cancel := contextWithTimeout(250 * time.Millisecond)
					defer cancel()
					if err := run(ctx); err == nil {
						t.Fatal("expected initialize error")
					}
				})
			})
		})
	})
}

func TestRun_ListenFailsOnPrivilegedPort(t *testing.T) {
	withEnv(t, "MASTER_KEY_SEED", "seed", func() {
		oldListen := listenFunc
		listenFunc = func(network, address string) (net.Listener, error) {
			return nil, errors.New("listen failed")
		}
		t.Cleanup(func() { listenFunc = oldListen })

		ctx, cancel := contextWithTimeout(250 * time.Millisecond)
		defer cancel()
		if err := run(ctx); err == nil {
			t.Fatal("expected listen error")
		}
	})
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	withEnv(t, "MASTER_KEY_SEED", "seed", func() {
		withEnv(t, "PORT", "0", func() {
			oldListen := listenFunc
			listenFunc = func(network, address string) (net.Listener, error) {
				return bufconn.Listen(1024 * 1024), nil
			}
			t.Cleanup(func() { listenFunc = oldListen })

			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(10*time.Millisecond, cancel)
			if err := run(ctx); err != nil {
				t.Fatalf("run: %v", err)
			}
		})
	})
}

func withEnv(t *testing.T, key, value string, fn func()) {
	t.Helper()
	orig, had := os.LookupEnv(key)
	if value == "" {
		_ = os.Unsetenv(key)
	} else {
		_ = os.Setenv(key, value)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, orig)
		} else {
			_ = os.Unsetenv(key)
		}
	})
	fn()
}

func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
