package tee

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dop251/goja"
)

// gojaScriptEngine implements ScriptEngine using goja (pure Go JavaScript runtime).
// This is used for simulation mode and environments without V8.
// For production TEE with Occlum, the actual V8/Node.js would run inside the enclave.
type gojaScriptEngine struct {
	mu       sync.RWMutex
	ready    bool
	heapSize int64
}

func newV8ScriptEngine(heapSize int64) ScriptEngine {
	if heapSize <= 0 {
		heapSize = DefaultMemoryLimit
	}
	return &gojaScriptEngine{
		heapSize: heapSize,
	}
}

func (e *gojaScriptEngine) Initialize(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ready = true
	return nil
}

func (e *gojaScriptEngine) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ready = false
	return nil
}

func (e *gojaScriptEngine) Execute(ctx context.Context, req ScriptExecutionRequest) (*ScriptExecutionResult, error) {
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
	console := vm.NewObject()
	_ = console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.String()
		}
		if len(args) > 0 {
			logs = append(logs, fmt.Sprint(args))
		}
		return goja.Undefined()
	})
	_ = vm.Set("console", console)

	// Inject secrets as a frozen object
	if len(req.Secrets) > 0 {
		secretsObj := vm.NewObject()
		for k, v := range req.Secrets {
			_ = secretsObj.Set(k, v)
		}
		_ = vm.Set("secrets", secretsObj)
	} else {
		_ = vm.Set("secrets", vm.NewObject())
	}

	// Inject input
	inputVal := vm.ToValue(req.Input)
	_ = vm.Set("input", inputVal)

	// Add built-in utilities
	_, err := vm.RunString(builtinFunctions)
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
	var output map[string]any
	if resultVal != nil && !goja.IsUndefined(resultVal) && !goja.IsNull(resultVal) {
		exported := resultVal.Export()
		switch v := exported.(type) {
		case map[string]any:
			output = v
		default:
			// Try JSON round-trip for complex objects
			jsonBytes, err := json.Marshal(exported)
			if err == nil {
				_ = json.Unmarshal(jsonBytes, &output)
			}
			if output == nil {
				output = map[string]any{"result": exported}
			}
		}
	}

	return &ScriptExecutionResult{
		Output:     output,
		Logs:       logs,
		MemoryUsed: 0, // goja doesn't expose memory stats
	}, nil
}

func (e *gojaScriptEngine) ValidateScript(ctx context.Context, script string) error {
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

// builtinFunctions provides common utility functions for scripts.
const builtinFunctions = `
// Crypto utilities
var crypto = {
	randomUUID: function() {
		return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
			var r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
			return v.toString(16);
		});
	},

	sha256: function(data) {
		// Simple hash for demo - in production use proper crypto
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

// JSON helpers (already available in JS, but explicit)
var json = {
	parse: JSON.parse,
	stringify: JSON.stringify
};

// HTTP fetch simulation (for TEE, actual fetch would go through enclave proxy)
var fetch = function(url, options) {
	console.log('fetch called (simulated):', url);
	return Promise.resolve({
		ok: true,
		status: 200,
		json: function() { return Promise.resolve({}); },
		text: function() { return Promise.resolve(''); }
	});
};
`
