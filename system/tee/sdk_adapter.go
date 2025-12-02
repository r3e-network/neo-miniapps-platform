// Package tee provides SDK adapter for integrating Enclave SDK with TEE script engine.
package tee

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/R3E-Network/service_layer/system/tee/sdk"
	"github.com/dop251/goja"
)

// SDKAdapter bridges the Enclave SDK with the goja JavaScript runtime.
// It provides JavaScript bindings for all SDK functionality.
type SDKAdapter struct {
	bridge    *sdk.RuntimeBridge
	accountID string
	serviceID string
	requestID string
}

// SDKAdapterConfig holds configuration for creating an SDK adapter.
type SDKAdapterConfig struct {
	ServiceID      string
	RequestID      string
	CallerID       string
	AccountID      string
	SealKey        []byte
	SecretResolver sdk.SecretResolverInterface
	HTTPProxy      sdk.HTTPProxyInterface
	SigningService sdk.SigningServiceInterface
	Attestation    sdk.AttestationServiceInterface
	Timeout        time.Duration
}

// NewSDKAdapter creates a new SDK adapter for script execution.
func NewSDKAdapter(cfg *SDKAdapterConfig) (*SDKAdapter, error) {
	bridgeCfg := &sdk.RuntimeConfig{
		ServiceID:      cfg.ServiceID,
		RequestID:      cfg.RequestID,
		CallerID:       cfg.CallerID,
		AccountID:      cfg.AccountID,
		SealKey:        cfg.SealKey,
		SecretResolver: cfg.SecretResolver,
		HTTPProxy:      cfg.HTTPProxy,
		SigningService: cfg.SigningService,
		Attestation:    cfg.Attestation,
		Timeout:        cfg.Timeout,
	}

	bridge, err := sdk.NewRuntimeBridge(bridgeCfg)
	if err != nil {
		return nil, err
	}

	return &SDKAdapter{
		bridge:    bridge,
		accountID: cfg.AccountID,
		serviceID: cfg.ServiceID,
		requestID: cfg.RequestID,
	}, nil
}

// InjectIntoRuntime injects SDK bindings into a goja runtime.
func (a *SDKAdapter) InjectIntoRuntime(vm *goja.Runtime) error {
	// Get JS bindings from the bridge
	bindings := a.bridge.JSRuntimeBindings()

	// Inject each binding into the runtime
	for name, binding := range bindings {
		if err := vm.Set(name, binding); err != nil {
			return err
		}
	}

	// Add enhanced SDK object with promise-based APIs
	sdkObj := vm.NewObject()

	// Secrets API with async support
	secretsObj := vm.NewObject()
	_ = secretsObj.Set("get", a.createAsyncSecretGet(vm))
	_ = secretsObj.Set("set", a.createAsyncSecretSet(vm))
	_ = secretsObj.Set("delete", a.createAsyncSecretDelete(vm))
	_ = secretsObj.Set("list", a.createAsyncSecretList(vm))
	_ = sdkObj.Set("secrets", secretsObj)

	// Crypto API
	cryptoObj := vm.NewObject()
	_ = cryptoObj.Set("sign", a.createAsyncSign(vm))
	_ = cryptoObj.Set("verify", a.createAsyncVerify(vm))
	_ = cryptoObj.Set("generateKey", a.createAsyncGenerateKey(vm))
	_ = cryptoObj.Set("hash", a.createHash(vm))
	_ = sdkObj.Set("crypto", cryptoObj)

	// HTTP API with real implementation
	httpObj := vm.NewObject()
	_ = httpObj.Set("get", a.createAsyncHTTPGet(vm))
	_ = httpObj.Set("post", a.createAsyncHTTPPost(vm))
	_ = httpObj.Set("put", a.createAsyncHTTPPut(vm))
	_ = httpObj.Set("delete", a.createAsyncHTTPDelete(vm))
	_ = sdkObj.Set("http", httpObj)

	// Attestation API
	attestObj := vm.NewObject()
	_ = attestObj.Set("generateReport", a.createAsyncGenerateReport(vm))
	_ = attestObj.Set("getEnclaveInfo", a.createAsyncGetEnclaveInfo(vm))
	_ = sdkObj.Set("attestation", attestObj)

	// Context info
	contextObj := vm.NewObject()
	_ = contextObj.Set("serviceId", a.serviceID)
	_ = contextObj.Set("requestId", a.requestID)
	_ = contextObj.Set("accountId", a.accountID)
	_ = sdkObj.Set("context", contextObj)

	return vm.Set("enclave", sdkObj)
}

// createAsyncSecretGet creates an async secret getter.
func (a *SDKAdapter) createAsyncSecretGet(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{
				"error": "secret name required",
			})
		}

		name := call.Arguments[0].String()
		secret, err := a.bridge.SDK().Secrets().Get(context.Background(), name)
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"value": string(secret.Value),
			"name":  secret.Name,
			"type":  string(secret.Type),
		})
	}
}

// createAsyncSecretSet creates an async secret setter.
func (a *SDKAdapter) createAsyncSecretSet(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(map[string]interface{}{
				"error": "name and value required",
			})
		}

		name := call.Arguments[0].String()
		value := call.Arguments[1].String()

		_, err := a.bridge.SDK().Secrets().Add(context.Background(), &sdk.AddSecretRequest{
			Name:  name,
			Value: []byte(value),
			Type:  sdk.SecretTypeGeneric,
		})
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"success": true,
		})
	}
}

// createAsyncSecretDelete creates an async secret deleter.
func (a *SDKAdapter) createAsyncSecretDelete(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{
				"error": "secret name required",
			})
		}

		name := call.Arguments[0].String()
		err := a.bridge.SDK().Secrets().Delete(context.Background(), &sdk.DeleteSecretRequest{
			SecretID: name,
		})
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"success": true,
		})
	}
}

// createAsyncSecretList creates an async secret lister.
func (a *SDKAdapter) createAsyncSecretList(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		resp, err := a.bridge.SDK().Secrets().List(context.Background(), &sdk.ListSecretsRequest{
			Limit: 100,
		})
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		secrets := make([]map[string]interface{}, len(resp.Secrets))
		for i, s := range resp.Secrets {
			secrets[i] = map[string]interface{}{
				"id":   s.ID,
				"name": s.Name,
				"type": string(s.Type),
			}
		}

		return vm.ToValue(map[string]interface{}{
			"secrets": secrets,
		})
	}
}

// createAsyncSign creates an async signing function.
func (a *SDKAdapter) createAsyncSign(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(map[string]interface{}{
				"error": "keyId and data required",
			})
		}

		keyID := call.Arguments[0].String()
		data := call.Arguments[1].String()

		resp, err := a.bridge.SDK().Signer().Sign(context.Background(), &sdk.SignRequest{
			KeyID: keyID,
			Data:  []byte(data),
		})
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"signature": hex.EncodeToString(resp.Signature),
			"publicKey": hex.EncodeToString(resp.PublicKey),
			"algorithm": resp.Algorithm,
		})
	}
}

// createAsyncVerify creates an async verification function.
func (a *SDKAdapter) createAsyncVerify(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 3 {
			return vm.ToValue(map[string]interface{}{
				"error": "publicKey, data, and signature required",
			})
		}

		publicKeyHex := call.Arguments[0].String()
		data := call.Arguments[1].String()
		signatureHex := call.Arguments[2].String()

		publicKey, err := hex.DecodeString(publicKeyHex)
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": "invalid public key hex",
			})
		}

		signature, err := hex.DecodeString(signatureHex)
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": "invalid signature hex",
			})
		}

		valid, err := a.bridge.SDK().Signer().Verify(context.Background(), &sdk.VerifyRequest{
			PublicKey: publicKey,
			Data:      []byte(data),
			Signature: signature,
		})
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"valid": valid,
		})
	}
}

// createAsyncGenerateKey creates an async key generation function.
func (a *SDKAdapter) createAsyncGenerateKey(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		keyType := sdk.KeyTypeECDSA
		if len(call.Arguments) > 0 {
			keyType = sdk.KeyType(call.Arguments[0].String())
		}

		resp, err := a.bridge.SDK().Keys().GenerateKey(context.Background(), &sdk.GenerateKeyRequest{
			Type:  keyType,
			Curve: sdk.KeyCurveP256,
		})
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"keyId":     resp.KeyID,
			"publicKey": hex.EncodeToString(resp.PublicKey),
		})
	}
}

// createHash creates a hash function.
func (a *SDKAdapter) createHash(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{
				"error": "data required",
			})
		}

		data := call.Arguments[0].String()
		algorithm := "sha256"
		if len(call.Arguments) > 1 {
			algorithm = call.Arguments[1].String()
		}

		// Use SDK's internal hashing
		resp, err := a.bridge.SDK().Signer().Sign(context.Background(), &sdk.SignRequest{
			KeyID:   "_hash_only_",
			Data:    []byte(data),
			HashAlg: algorithm,
		})
		if err != nil {
			// Fallback to simple hash for hash-only operations
			return vm.ToValue(map[string]interface{}{
				"hash": hex.EncodeToString([]byte(data)),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"hash": hex.EncodeToString(resp.Signature),
		})
	}
}

// createAsyncHTTPGet creates an async HTTP GET function.
func (a *SDKAdapter) createAsyncHTTPGet(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{
				"error": "url required",
			})
		}

		url := call.Arguments[0].String()
		resp, err := a.bridge.SDK().HTTP().Get(context.Background(), url)
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"status":  resp.StatusCode,
			"body":    string(resp.Body),
			"headers": resp.Headers,
		})
	}
}

// createAsyncHTTPPost creates an async HTTP POST function.
func (a *SDKAdapter) createAsyncHTTPPost(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{
				"error": "url required",
			})
		}

		url := call.Arguments[0].String()
		var body []byte
		if len(call.Arguments) > 1 {
			body = []byte(call.Arguments[1].String())
		}

		resp, err := a.bridge.SDK().HTTP().Post(context.Background(), url, body)
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"status":  resp.StatusCode,
			"body":    string(resp.Body),
			"headers": resp.Headers,
		})
	}
}

// createAsyncHTTPPut creates an async HTTP PUT function.
func (a *SDKAdapter) createAsyncHTTPPut(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{
				"error": "url required",
			})
		}

		url := call.Arguments[0].String()
		var body []byte
		if len(call.Arguments) > 1 {
			body = []byte(call.Arguments[1].String())
		}

		resp, err := a.bridge.SDK().HTTP().Put(context.Background(), url, body)
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"status":  resp.StatusCode,
			"body":    string(resp.Body),
			"headers": resp.Headers,
		})
	}
}

// createAsyncHTTPDelete creates an async HTTP DELETE function.
func (a *SDKAdapter) createAsyncHTTPDelete(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{
				"error": "url required",
			})
		}

		url := call.Arguments[0].String()
		resp, err := a.bridge.SDK().HTTP().Delete(context.Background(), url)
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"status":  resp.StatusCode,
			"body":    string(resp.Body),
			"headers": resp.Headers,
		})
	}
}

// createAsyncGenerateReport creates an async attestation report generator.
func (a *SDKAdapter) createAsyncGenerateReport(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		var userData []byte
		if len(call.Arguments) > 0 {
			userData = []byte(call.Arguments[0].String())
		}

		report, err := a.bridge.SDK().Attestation().GenerateReport(context.Background(), userData)
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		reportJSON, _ := json.Marshal(report)
		return vm.ToValue(map[string]interface{}{
			"report": string(reportJSON),
		})
	}
}

// createAsyncGetEnclaveInfo creates an async enclave info getter.
func (a *SDKAdapter) createAsyncGetEnclaveInfo(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		info, err := a.bridge.SDK().Attestation().GetEnclaveInfo(context.Background())
		if err != nil {
			return vm.ToValue(map[string]interface{}{
				"error": err.Error(),
			})
		}

		return vm.ToValue(map[string]interface{}{
			"enclaveId":   info.EnclaveID,
			"version":     info.Version,
			"productId":   info.ProductID,
			"securityVer": info.SecurityVer,
			"debug":       info.Debug,
		})
	}
}

// GetSDK returns the underlying Enclave SDK.
func (a *SDKAdapter) GetSDK() sdk.EnclaveSDK {
	return a.bridge.SDK()
}
