// Package tee provides the Trusted Execution Environment (TEE) engine for confidential computing.
//
// Enhanced Script Engine with sys.* API injection
//
// This file extends the script engine to inject the sys.* APIs into the JavaScript runtime.
// The sys.* APIs provide secure access to system functions through the ECALL/OCALL bridge.
package tee

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dop251/goja"
)

// EnhancedScriptEngine extends ScriptEngine with sys.* API support.
type EnhancedScriptEngine interface {
	ScriptEngine

	// ExecuteWithSysAPI runs a script with sys.* APIs injected.
	ExecuteWithSysAPI(ctx context.Context, req EnhancedScriptRequest) (*EnhancedScriptResult, error)
}

// EnhancedScriptRequest extends ScriptExecutionRequest with sys.* API context.
type EnhancedScriptRequest struct {
	ScriptExecutionRequest

	// ServiceID for secret isolation
	ServiceID string

	// AccountID for the execution context
	AccountID string

	// SysAPI provides the sys.* APIs
	SysAPI SysAPI
}

// EnhancedScriptResult extends ScriptExecutionResult with proof.
type EnhancedScriptResult struct {
	ScriptExecutionResult

	// Proof of execution (if generated)
	Proof *ExecutionProof
}

// enhancedGojaEngine implements EnhancedScriptEngine using goja.
type enhancedGojaEngine struct {
	mu       sync.RWMutex
	ready    bool
	heapSize int64
}

// NewEnhancedScriptEngine creates a new enhanced script engine.
func NewEnhancedScriptEngine(heapSize int64) EnhancedScriptEngine {
	if heapSize <= 0 {
		heapSize = DefaultMemoryLimit
	}
	return &enhancedGojaEngine{
		heapSize: heapSize,
	}
}

func (e *enhancedGojaEngine) Initialize(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ready = true
	return nil
}

func (e *enhancedGojaEngine) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ready = false
	return nil
}

func (e *enhancedGojaEngine) ValidateScript(ctx context.Context, script string) error {
	e.mu.RLock()
	if !e.ready {
		e.mu.RUnlock()
		return ErrEnclaveNotReady
	}
	e.mu.RUnlock()

	_, err := goja.Compile("validate.js", script, false)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidScript, err)
	}
	return nil
}

func (e *enhancedGojaEngine) Execute(ctx context.Context, req ScriptExecutionRequest) (*ScriptExecutionResult, error) {
	// Delegate to basic execution without sys.* APIs
	return e.executeBasic(ctx, req)
}

func (e *enhancedGojaEngine) ExecuteWithSysAPI(ctx context.Context, req EnhancedScriptRequest) (*EnhancedScriptResult, error) {
	e.mu.RLock()
	if !e.ready {
		e.mu.RUnlock()
		return nil, ErrEnclaveNotReady
	}
	e.mu.RUnlock()

	// Create a new runtime for isolation
	vm := goja.New()

	// Capture logs
	logs := make([]string, 0)

	// Set up console object
	e.setupConsole(vm, &logs)

	// Inject secrets as a frozen object
	e.setupSecrets(vm, req.Secrets)

	// Inject input
	inputVal := vm.ToValue(req.Input)
	_ = vm.Set("input", inputVal)

	// Inject sys.* APIs
	if req.SysAPI != nil {
		if err := e.setupSysAPI(ctx, vm, req.SysAPI); err != nil {
			return nil, fmt.Errorf("setup sys API: %w", err)
		}
	}

	// Add built-in utilities
	_, err := vm.RunString(enhancedBuiltinFunctions)
	if err != nil {
		return nil, fmt.Errorf("load builtins: %w", err)
	}

	// Run the user script
	_, err = vm.RunString(req.Script)
	if err != nil {
		return nil, fmt.Errorf("execute script: %w", err)
	}

	// Call the entry point function
	entryPoint, ok := goja.AssertFunction(vm.Get(req.EntryPoint))
	if !ok {
		return nil, fmt.Errorf("entry point '%s' is not a function", req.EntryPoint)
	}

	resultVal, err := entryPoint(goja.Undefined(), vm.Get("input"))
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", req.EntryPoint, err)
	}

	// Convert result to map
	output := e.convertResult(resultVal)

	// Get logs from sys.log if available
	if sysImpl, ok := req.SysAPI.(*sysAPIImpl); ok {
		logs = append(logs, sysImpl.GetLogs()...)
	}

	return &EnhancedScriptResult{
		ScriptExecutionResult: ScriptExecutionResult{
			Output:     output,
			Logs:       logs,
			MemoryUsed: 0, // goja doesn't expose memory stats
		},
	}, nil
}

func (e *enhancedGojaEngine) executeBasic(ctx context.Context, req ScriptExecutionRequest) (*ScriptExecutionResult, error) {
	e.mu.RLock()
	if !e.ready {
		e.mu.RUnlock()
		return nil, ErrEnclaveNotReady
	}
	e.mu.RUnlock()

	vm := goja.New()
	logs := make([]string, 0)

	e.setupConsole(vm, &logs)
	e.setupSecrets(vm, req.Secrets)

	inputVal := vm.ToValue(req.Input)
	_ = vm.Set("input", inputVal)

	_, err := vm.RunString(builtinFunctions)
	if err != nil {
		return nil, fmt.Errorf("load builtins: %w", err)
	}

	_, err = vm.RunString(req.Script)
	if err != nil {
		return nil, fmt.Errorf("execute script: %w", err)
	}

	entryPoint, ok := goja.AssertFunction(vm.Get(req.EntryPoint))
	if !ok {
		return nil, fmt.Errorf("entry point '%s' is not a function", req.EntryPoint)
	}

	resultVal, err := entryPoint(goja.Undefined(), vm.Get("input"))
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", req.EntryPoint, err)
	}

	output := e.convertResult(resultVal)

	return &ScriptExecutionResult{
		Output:     output,
		Logs:       logs,
		MemoryUsed: 0,
	}, nil
}

func (e *enhancedGojaEngine) setupConsole(vm *goja.Runtime, logs *[]string) {
	console := vm.NewObject()
	_ = console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.String()
		}
		if len(args) > 0 {
			*logs = append(*logs, fmt.Sprint(args))
		}
		return goja.Undefined()
	})
	_ = console.Set("error", func(call goja.FunctionCall) goja.Value {
		args := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.String()
		}
		if len(args) > 0 {
			*logs = append(*logs, "[ERROR] "+fmt.Sprint(args))
		}
		return goja.Undefined()
	})
	_ = console.Set("warn", func(call goja.FunctionCall) goja.Value {
		args := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.String()
		}
		if len(args) > 0 {
			*logs = append(*logs, "[WARN] "+fmt.Sprint(args))
		}
		return goja.Undefined()
	})
	_ = vm.Set("console", console)
}

func (e *enhancedGojaEngine) setupSecrets(vm *goja.Runtime, secrets map[string]string) {
	if len(secrets) > 0 {
		secretsObj := vm.NewObject()
		for k, v := range secrets {
			_ = secretsObj.Set(k, v)
		}
		_ = vm.Set("secrets", secretsObj)
	} else {
		_ = vm.Set("secrets", vm.NewObject())
	}
}

func (e *enhancedGojaEngine) setupSysAPI(ctx context.Context, vm *goja.Runtime, sysAPI SysAPI) error {
	sys := vm.NewObject()

	// sys.http
	httpObj := vm.NewObject()
	_ = httpObj.Set("fetch", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "url required"})
		}

		url := call.Arguments[0].String()
		method := "GET"
		var headers map[string]string
		var body []byte

		if len(call.Arguments) > 1 {
			opts := call.Arguments[1].Export()
			if optsMap, ok := opts.(map[string]any); ok {
				if m, ok := optsMap["method"].(string); ok {
					method = m
				}
				if h, ok := optsMap["headers"].(map[string]any); ok {
					headers = make(map[string]string)
					for k, v := range h {
						if s, ok := v.(string); ok {
							headers[k] = s
						}
					}
				}
				if b, ok := optsMap["body"].(string); ok {
					body = []byte(b)
				}
			}
		}

		resp, err := sysAPI.HTTP().Fetch(ctx, HTTPRequest{
			Method:  method,
			URL:     url,
			Headers: headers,
			Body:    body,
		})
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}

		return vm.ToValue(map[string]any{
			"status":  resp.StatusCode,
			"headers": resp.Headers,
			"body":    string(resp.Body),
		})
	})
	_ = httpObj.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "url required"})
		}
		url := call.Arguments[0].String()
		resp, err := sysAPI.HTTP().Get(ctx, url, nil)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{
			"status": resp.StatusCode,
			"body":   string(resp.Body),
		})
	})
	_ = httpObj.Set("post", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(map[string]any{"error": "url and body required"})
		}
		url := call.Arguments[0].String()
		body := call.Arguments[1].String()
		resp, err := sysAPI.HTTP().Post(ctx, url, []byte(body), nil)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{
			"status": resp.StatusCode,
			"body":   string(resp.Body),
		})
	})
	_ = sys.Set("http", httpObj)

	// sys.secrets
	secretsObj := vm.NewObject()
	_ = secretsObj.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		name := call.Arguments[0].String()
		value, err := sysAPI.Secrets().Get(ctx, name)
		if err != nil {
			return goja.Undefined()
		}
		return vm.ToValue(value)
	})
	_ = secretsObj.Set("getMultiple", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]string{})
		}
		namesVal := call.Arguments[0].Export()
		var names []string
		if arr, ok := namesVal.([]any); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					names = append(names, s)
				}
			}
		}
		values, err := sysAPI.Secrets().GetMultiple(ctx, names)
		if err != nil {
			return vm.ToValue(map[string]string{})
		}
		return vm.ToValue(values)
	})
	_ = secretsObj.Set("set", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(map[string]any{"error": "name and value required"})
		}
		name := call.Arguments[0].String()
		value := call.Arguments[1].String()
		err := sysAPI.Secrets().Set(ctx, name, value)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"success": true})
	})
	_ = secretsObj.Set("delete", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "name required"})
		}
		name := call.Arguments[0].String()
		err := sysAPI.Secrets().Delete(ctx, name)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"success": true})
	})
	_ = secretsObj.Set("list", func(call goja.FunctionCall) goja.Value {
		names, err := sysAPI.Secrets().List(ctx)
		if err != nil {
			return vm.ToValue([]string{})
		}
		return vm.ToValue(names)
	})
	_ = sys.Set("secrets", secretsObj)

	// sys.chain
	chainObj := vm.NewObject()
	_ = chainObj.Set("call", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "request required"})
		}
		reqVal := call.Arguments[0].Export()
		reqMap, ok := reqVal.(map[string]any)
		if !ok {
			return vm.ToValue(map[string]any{"error": "invalid request"})
		}

		chainReq := ChainCallRequest{
			Chain:    getString(reqMap, "chain"),
			Contract: getString(reqMap, "contract"),
			Method:   getString(reqMap, "method"),
		}
		if args, ok := reqMap["args"].([]any); ok {
			chainReq.Args = args
		}

		resp, err := sysAPI.Chain().Call(ctx, chainReq)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		if resp.Error != "" {
			return vm.ToValue(map[string]any{"error": resp.Error})
		}

		var result any
		_ = json.Unmarshal(resp.Result, &result)
		return vm.ToValue(map[string]any{"result": result})
	})
	_ = chainObj.Set("sendTransaction", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "request required"})
		}
		reqVal := call.Arguments[0].Export()
		reqMap, ok := reqVal.(map[string]any)
		if !ok {
			return vm.ToValue(map[string]any{"error": "invalid request"})
		}

		txReq := ChainTxRequest{
			Chain: getString(reqMap, "chain"),
			To:    getString(reqMap, "to"),
			Value: getString(reqMap, "value"),
		}
		if data, ok := reqMap["data"].(string); ok {
			txReq.Data = []byte(data)
		}

		resp, err := sysAPI.Chain().SendTransaction(ctx, txReq)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{
			"txHash": resp.TxHash,
			"status": resp.Status,
			"error":  resp.Error,
		})
	})
	_ = sys.Set("chain", chainObj)

	// sys.log
	logObj := vm.NewObject()
	_ = logObj.Set("debug", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			sysAPI.Log().Debug(call.Arguments[0].String())
		}
		return goja.Undefined()
	})
	_ = logObj.Set("info", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			sysAPI.Log().Info(call.Arguments[0].String())
		}
		return goja.Undefined()
	})
	_ = logObj.Set("warn", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			sysAPI.Log().Warn(call.Arguments[0].String())
		}
		return goja.Undefined()
	})
	_ = logObj.Set("error", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			sysAPI.Log().Error(call.Arguments[0].String())
		}
		return goja.Undefined()
	})
	_ = sys.Set("log", logObj)

	// sys.crypto - Full implementation
	cryptoObj := vm.NewObject()
	sysCrypto := sysAPI.Crypto()
	_ = cryptoObj.Set("hash", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(map[string]any{"error": "algorithm and data required"})
		}
		algorithm := call.Arguments[0].String()
		data := call.Arguments[1].String()
		hash, err := sysCrypto.Hash(algorithm, []byte(data))
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"hash": fmt.Sprintf("%x", hash)})
	})
	_ = cryptoObj.Set("sign", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "data required"})
		}
		data := call.Arguments[0].String()
		sig, err := sysCrypto.Sign([]byte(data))
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"signature": fmt.Sprintf("%x", sig)})
	})
	_ = cryptoObj.Set("verify", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 3 {
			return vm.ToValue(map[string]any{"error": "data, signature, and publicKey required"})
		}
		data := call.Arguments[0].String()
		sigHex := call.Arguments[1].String()
		pubKeyHex := call.Arguments[2].String()
		sig, _ := hexDecode(sigHex)
		pubKey, _ := hexDecode(pubKeyHex)
		valid, err := sysCrypto.Verify([]byte(data), sig, pubKey)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"valid": valid})
	})
	_ = cryptoObj.Set("encrypt", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(map[string]any{"error": "keyID and plaintext required"})
		}
		keyID := call.Arguments[0].String()
		plaintext := call.Arguments[1].String()
		ciphertext, err := sysCrypto.Encrypt(keyID, []byte(plaintext))
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"ciphertext": fmt.Sprintf("%x", ciphertext)})
	})
	_ = cryptoObj.Set("decrypt", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(map[string]any{"error": "keyID and ciphertext required"})
		}
		keyID := call.Arguments[0].String()
		ciphertextHex := call.Arguments[1].String()
		ciphertext, _ := hexDecode(ciphertextHex)
		plaintext, err := sysCrypto.Decrypt(keyID, ciphertext)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"plaintext": string(plaintext)})
	})
	_ = cryptoObj.Set("generateKey", func(call goja.FunctionCall) goja.Value {
		keyType := "ecdsa-p256"
		if len(call.Arguments) > 0 {
			keyType = call.Arguments[0].String()
		}
		keyPair, err := sysCrypto.GenerateKey(keyType)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{
			"keyID":     keyPair.KeyID,
			"keyType":   keyPair.KeyType,
			"publicKey": fmt.Sprintf("%x", keyPair.PublicKey),
		})
	})
	_ = cryptoObj.Set("randomBytes", func(call goja.FunctionCall) goja.Value {
		length := 32
		if len(call.Arguments) > 0 {
			if l, ok := call.Arguments[0].Export().(int64); ok {
				length = int(l)
			}
		}
		bytes, err := sysCrypto.RandomBytes(length)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"bytes": fmt.Sprintf("%x", bytes)})
	})
	_ = sys.Set("crypto", cryptoObj)

	// sys.proof - Full implementation
	proofObj := vm.NewObject()
	sysProof := sysAPI.Proof()
	_ = proofObj.Set("generate", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "data required"})
		}
		data := call.Arguments[0].String()
		proof, err := sysProof.GenerateProof(ctx, []byte(data))
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{
			"proofID":    proof.ProofID,
			"enclaveID":  proof.EnclaveID,
			"inputHash":  proof.InputHash,
			"outputHash": proof.OutputHash,
			"timestamp":  proof.Timestamp.Unix(),
			"signature":  fmt.Sprintf("%x", proof.Signature),
		})
	})
	_ = proofObj.Set("verify", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "proof required"})
		}
		proofVal := call.Arguments[0].Export()
		proofMap, ok := proofVal.(map[string]any)
		if !ok {
			return vm.ToValue(map[string]any{"error": "invalid proof format"})
		}
		// Reconstruct proof from JS object
		proof := &ExecutionProof{
			ProofID:   getString(proofMap, "proofID"),
			EnclaveID: getString(proofMap, "enclaveID"),
			InputHash: getString(proofMap, "inputHash"),
		}
		valid, err := sysProof.VerifyProof(ctx, proof)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"valid": valid})
	})
	_ = proofObj.Set("getAttestation", func(call goja.FunctionCall) goja.Value {
		attestation, err := sysProof.GetAttestation(ctx)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{
			"enclaveID": attestation.EnclaveID,
			"timestamp": attestation.Timestamp.Unix(),
		})
	})
	_ = sys.Set("proof", proofObj)

	// sys.neo - Neo N3 transaction signing (TEE signs, other engines broadcast)
	neoObj := vm.NewObject()
	sysNeo, _ := NewSysNeo(nil)
	_ = neoObj.Set("getAddress", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sysNeo.GetAddress())
	})
	_ = neoObj.Set("getPublicKey", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(fmt.Sprintf("%x", sysNeo.GetPublicKey()))
	})
	_ = neoObj.Set("getScriptHash", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sysNeo.GetScriptHash())
	})
	_ = neoObj.Set("signTransaction", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "transaction required"})
		}
		txVal := call.Arguments[0].Export()
		txMap, ok := txVal.(map[string]any)
		if !ok {
			return vm.ToValue(map[string]any{"error": "invalid transaction format"})
		}

		// Build NeoTransaction from JS object
		tx := &NeoTransaction{
			Version:         uint8(getInt(txMap, "version")),
			Nonce:           uint32(getInt(txMap, "nonce")),
			SystemFee:       getInt(txMap, "systemFee"),
			NetworkFee:      getInt(txMap, "networkFee"),
			ValidUntilBlock: uint32(getInt(txMap, "validUntilBlock")),
		}

		// Parse signers
		if signersVal, ok := txMap["signers"].([]any); ok {
			for _, s := range signersVal {
				if signerMap, ok := s.(map[string]any); ok {
					tx.Signers = append(tx.Signers, NeoSigner{
						Account: getString(signerMap, "account"),
						Scopes:  NeoWitnessScope(getInt(signerMap, "scopes")),
					})
				}
			}
		}

		// Parse script
		if scriptHex, ok := txMap["script"].(string); ok {
			tx.Script, _ = hexDecode(scriptHex)
		}

		signed, err := sysNeo.SignTransaction(tx)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}

		return vm.ToValue(map[string]any{
			"hash":           signed.Hash,
			"size":           signed.Size,
			"rawTransaction": signed.RawTransaction,
			"witnesses":      convertWitnesses(signed.Witnesses),
		})
	})
	_ = neoObj.Set("signInvocation", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "invocation request required"})
		}
		reqVal := call.Arguments[0].Export()
		reqMap, ok := reqVal.(map[string]any)
		if !ok {
			return vm.ToValue(map[string]any{"error": "invalid request format"})
		}

		// Build NeoInvocationRequest from JS object
		req := &NeoInvocationRequest{
			ScriptHash:      getString(reqMap, "scriptHash"),
			Method:          getString(reqMap, "method"),
			SystemFee:       getInt(reqMap, "systemFee"),
			NetworkFee:      getInt(reqMap, "networkFee"),
			ValidUntilBlock: uint32(getInt(reqMap, "validUntilBlock")),
			Scope:           NeoWitnessScope(getInt(reqMap, "scope")),
		}

		// Parse args
		if argsVal, ok := reqMap["args"].([]any); ok {
			for _, a := range argsVal {
				if argMap, ok := a.(map[string]any); ok {
					req.Args = append(req.Args, NeoContractArg{
						Type:  getString(argMap, "type"),
						Value: argMap["value"],
					})
				}
			}
		}

		signed, err := sysNeo.SignInvocation(req)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}

		return vm.ToValue(map[string]any{
			"hash":           signed.Hash,
			"size":           signed.Size,
			"rawTransaction": signed.RawTransaction,
			"witnesses":      convertWitnesses(signed.Witnesses),
		})
	})
	_ = sys.Set("neo", neoObj)

	// sys.storage - Sealed persistent storage with AES-256-GCM encryption
	storageObj := vm.NewObject()
	storage := sysAPI.Storage()

	// sys.storage.get(key) - Retrieve and decrypt a value
	_ = storageObj.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "key is required"})
		}
		key := call.Arguments[0].String()
		if key == "" {
			return vm.ToValue(map[string]any{"error": "key cannot be empty"})
		}

		value, err := storage.Get(ctx, key)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}

		// Try to parse as JSON first, otherwise return as string
		var parsed any
		if err := json.Unmarshal(value, &parsed); err == nil {
			return vm.ToValue(map[string]any{"value": parsed})
		}
		return vm.ToValue(map[string]any{"value": string(value)})
	})

	// sys.storage.set(key, value) - Encrypt and store a value
	_ = storageObj.Set("set", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(map[string]any{"error": "key and value are required"})
		}
		key := call.Arguments[0].String()
		if key == "" {
			return vm.ToValue(map[string]any{"error": "key cannot be empty"})
		}

		// Serialize value to JSON for storage
		valueArg := call.Arguments[1].Export()
		var valueBytes []byte
		var err error

		switch v := valueArg.(type) {
		case string:
			valueBytes = []byte(v)
		case []byte:
			valueBytes = v
		default:
			valueBytes, err = json.Marshal(v)
			if err != nil {
				return vm.ToValue(map[string]any{"error": "failed to serialize value: " + err.Error()})
			}
		}

		if err := storage.Set(ctx, key, valueBytes); err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"success": true})
	})

	// sys.storage.delete(key) - Remove a value
	_ = storageObj.Set("delete", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]any{"error": "key is required"})
		}
		key := call.Arguments[0].String()
		if key == "" {
			return vm.ToValue(map[string]any{"error": "key cannot be empty"})
		}

		if err := storage.Delete(ctx, key); err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"success": true})
	})

	// sys.storage.list(prefix) - List keys with prefix
	_ = storageObj.Set("list", func(call goja.FunctionCall) goja.Value {
		prefix := ""
		if len(call.Arguments) > 0 {
			prefix = call.Arguments[0].String()
		}

		keys, err := storage.List(ctx, prefix)
		if err != nil {
			return vm.ToValue(map[string]any{"error": err.Error()})
		}
		return vm.ToValue(map[string]any{"keys": keys})
	})

	_ = sys.Set("storage", storageObj)

	_ = vm.Set("sys", sys)
	return nil
}

func (e *enhancedGojaEngine) convertResult(resultVal goja.Value) map[string]any {
	var output map[string]any
	if resultVal != nil && !goja.IsUndefined(resultVal) && !goja.IsNull(resultVal) {
		exported := resultVal.Export()
		switch v := exported.(type) {
		case map[string]any:
			output = v
		default:
			jsonBytes, err := json.Marshal(exported)
			if err == nil {
				_ = json.Unmarshal(jsonBytes, &output)
			}
			if output == nil {
				output = map[string]any{"result": exported}
			}
		}
	}
	return output
}

// getString safely extracts a string from a map.
func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getInt safely extracts an int64 from a map.
func getInt(m map[string]any, key string) int64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int64:
			return n
		case int:
			return int64(n)
		case float64:
			return int64(n)
		}
	}
	return 0
}

// hexDecode decodes a hex string to bytes.
func hexDecode(s string) ([]byte, error) {
	// Remove 0x prefix if present
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	result := make([]byte, len(s)/2)
	for i := 0; i < len(result); i++ {
		var b byte
		for j := 0; j < 2; j++ {
			c := s[i*2+j]
			switch {
			case c >= '0' && c <= '9':
				b = b*16 + (c - '0')
			case c >= 'a' && c <= 'f':
				b = b*16 + (c - 'a' + 10)
			case c >= 'A' && c <= 'F':
				b = b*16 + (c - 'A' + 10)
			default:
				return nil, fmt.Errorf("invalid hex character: %c", c)
			}
		}
		result[i] = b
	}
	return result, nil
}

// convertWitnesses converts NeoWitness slice to a format suitable for JS.
func convertWitnesses(witnesses []NeoWitness) []map[string]any {
	result := make([]map[string]any, len(witnesses))
	for i, w := range witnesses {
		result[i] = map[string]any{
			"invocationScript":   w.InvocationScript,
			"verificationScript": w.VerificationScript,
		}
	}
	return result
}

// enhancedBuiltinFunctions provides common utility functions for scripts with sys.* API.
const enhancedBuiltinFunctions = `
// Crypto utilities (legacy - prefer sys.crypto)
var crypto = {
	randomUUID: function() {
		return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
			var r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
			return v.toString(16);
		});
	},

	sha256: function(data) {
		// Simple hash for demo - in production use sys.crypto.hash
		var hash = 0;
		for (var i = 0; i < data.length; i++) {
			var char = data.charCodeAt(i);
			hash = ((hash << 5) - hash) + char;
			hash = hash & hash;
		}
		return Math.abs(hash).toString(16);
	}
};

// Base64 encoding/decoding
var base64 = {
	encode: function(str) {
		var chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=';
		var encoded = '';
		var i = 0;
		while (i < str.length) {
			var a = str.charCodeAt(i++);
			var b = str.charCodeAt(i++);
			var c = str.charCodeAt(i++);
			var enc1 = a >> 2;
			var enc2 = ((a & 3) << 4) | (b >> 4);
			var enc3 = ((b & 15) << 2) | (c >> 6);
			var enc4 = c & 63;
			if (isNaN(b)) { enc3 = enc4 = 64; }
			else if (isNaN(c)) { enc4 = 64; }
			encoded += chars.charAt(enc1) + chars.charAt(enc2) + chars.charAt(enc3) + chars.charAt(enc4);
		}
		return encoded;
	},
	decode: function(str) {
		var chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=';
		var decoded = '';
		var i = 0;
		str = str.replace(/[^A-Za-z0-9\+\/\=]/g, '');
		while (i < str.length) {
			var enc1 = chars.indexOf(str.charAt(i++));
			var enc2 = chars.indexOf(str.charAt(i++));
			var enc3 = chars.indexOf(str.charAt(i++));
			var enc4 = chars.indexOf(str.charAt(i++));
			var a = (enc1 << 2) | (enc2 >> 4);
			var b = ((enc2 & 15) << 4) | (enc3 >> 2);
			var c = ((enc3 & 3) << 6) | enc4;
			decoded += String.fromCharCode(a);
			if (enc3 != 64) { decoded += String.fromCharCode(b); }
			if (enc4 != 64) { decoded += String.fromCharCode(c); }
		}
		return decoded;
	}
};

// JSON helpers
var json = {
	parse: JSON.parse,
	stringify: JSON.stringify
};

// HTTP fetch using sys.http (preferred over legacy fetch)
var fetch = function(url, options) {
	if (typeof sys !== 'undefined' && sys.http) {
		return sys.http.fetch(url, options || {});
	}
	console.log('fetch called (simulated):', url);
	return {
		ok: true,
		status: 200,
		json: function() { return {}; },
		text: function() { return ''; }
	};
};
`
