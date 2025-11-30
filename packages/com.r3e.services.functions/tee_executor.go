package functions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dop251/goja"
)

// TEEExecutor executes functions inside a Goja runtime and resolves secrets on demand.
type TEEExecutor struct {
	resolver SecretResolver
}

// NewTEEExecutor creates a new executor using the provided secret resolver.
func NewTEEExecutor(resolver SecretResolver) *TEEExecutor {
	return &TEEExecutor{resolver: resolver}
}

// SetSecretResolver implements SecretAwareExecutor.
func (e *TEEExecutor) SetSecretResolver(resolver SecretResolver) {
	e.resolver = resolver
}

// Execute runs the function source code with params and resolved secrets.
func (e *TEEExecutor) Execute(ctx context.Context, def Definition, payload map[string]any) (ExecutionResult, error) {
	secrets := map[string]string{}
	if len(def.Secrets) > 0 {
		if e.resolver == nil {
			return ExecutionResult{}, fmt.Errorf("secret resolver not configured")
		}
		resolved, err := e.resolver.ResolveSecrets(ctx, def.AccountID, def.Secrets)
		if err != nil {
			return ExecutionResult{}, err
		}
		secrets = resolved
	}

	rt := goja.New()
	if _, err := rt.RunString(devpackRuntimeSource); err != nil {
		return ExecutionResult{}, fmt.Errorf("load devpack runtime: %w", err)
	}
	if err := initialiseDevpack(rt, def); err != nil {
		return ExecutionResult{}, fmt.Errorf("init devpack: %w", err)
	}

	stop := make(chan struct{})
	defer close(stop)

	go func() {
		select {
		case <-ctx.Done():
			rt.Interrupt(ctx.Err())
		case <-stop:
		}
	}()

	var logs []string
	if err := attachConsole(rt, &logs); err != nil {
		return ExecutionResult{}, fmt.Errorf("attach console: %w", err)
	}
	if err := rt.Set("params", clonePayload(payload)); err != nil {
		return ExecutionResult{}, fmt.Errorf("set params: %w", err)
	}
	if err := rt.Set("secrets", secrets); err != nil {
		return ExecutionResult{}, fmt.Errorf("set secrets: %w", err)
	}

	script := fmt.Sprintf(`(function() {
	const entry = (%s);
	if (typeof entry === 'function') {
		return entry(params, secrets);
	}
	return entry;
})();`, def.Source)

	started := time.Now().UTC()
	val, err := rt.RunString(script)
	if err != nil {
		return ExecutionResult{}, runtimeError(err, ctx, "execute")
	}

	val, err = resolveValue(ctx, val)
	if err != nil {
		return ExecutionResult{}, runtimeError(err, ctx, "await function result")
	}

	actions, err := collectDevpackActions(rt)
	if err != nil {
		return ExecutionResult{}, runtimeError(err, ctx, "collect devpack actions")
	}

	exported := val.Export()
	var output map[string]any
	switch res := exported.(type) {
	case map[string]any:
		output = res
	case nil:
		output = map[string]any{}
	default:
		output = map[string]any{"result": res}
	}
	if len(logs) > 0 {
		output["logs"] = logs
	}

	completed := time.Now().UTC()
	return ExecutionResult{
		FunctionID:  def.ID,
		Output:      output,
		Logs:        logs,
		Status:      ExecutionStatusSucceeded,
		StartedAt:   started,
		CompletedAt: completed,
		Duration:    completed.Sub(started),
		Actions:     actions,
	}, nil
}

func attachConsole(vm *goja.Runtime, logs *[]string) error {
	console := vm.NewObject()
	logFn := func(call goja.FunctionCall) goja.Value {
		args := make([]any, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		*logs = append(*logs, fmt.Sprint(args...))
		return goja.Undefined()
	}
	if err := console.Set("log", logFn); err != nil {
		return err
	}
	if err := console.Set("info", logFn); err != nil {
		return err
	}
	if err := console.Set("warn", logFn); err != nil {
		return err
	}
	if err := console.Set("error", logFn); err != nil {
		return err
	}
	if err := vm.Set("console", console); err != nil {
		return err
	}
	return nil
}

func exportedPromise(val goja.Value) (*goja.Promise, bool) {
	exported := val.Export()
	if exported == nil {
		return nil, false
	}

	promise, ok := exported.(*goja.Promise)
	return promise, ok
}

func resolveValue(ctx context.Context, val goja.Value) (goja.Value, error) {
	if promise, ok := exportedPromise(val); ok {
		switch promise.State() {
		case goja.PromiseStateFulfilled:
			return promise.Result(), nil
		case goja.PromiseStateRejected:
			return nil, promiseRejectionError(promise.Result())
		case goja.PromiseStatePending:
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return nil, errors.New("function returned a promise that did not settle")
		}
	}
	return val, nil
}

func promiseRejectionError(reason goja.Value) error {
	if reason == nil {
		return errors.New("promise rejected")
	}
	if exported := reason.Export(); exported != nil {
		if err, ok := exported.(error); ok {
			return err
		}
		return fmt.Errorf("promise rejected: %v", exported)
	}
	return fmt.Errorf("promise rejected: %s", reason.String())
}

func runtimeError(err error, ctx context.Context, when string) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%s: %w", when, ctxErr)
	}
	switch typed := err.(type) {
	case *goja.InterruptedError:
		if val := typed.Value(); val != nil {
			if inner, ok := val.(error); ok {
				return fmt.Errorf("%s: %w", when, inner)
			}
			return fmt.Errorf("%s: %v", when, val)
		}
		return fmt.Errorf("%s: interrupted", when)
	case *goja.Exception:
		return fmt.Errorf("%s: %s", when, typed.Error())
	default:
		return fmt.Errorf("%s: %w", when, err)
	}
}

func initialiseDevpack(rt *goja.Runtime, def Definition) error {
	devpack := rt.Get("Devpack")
	if goja.IsUndefined(devpack) || goja.IsNull(devpack) {
		return nil
	}
	obj := devpack.ToObject(rt)
	if reset := obj.Get("__reset"); !goja.IsUndefined(reset) && !goja.IsNull(reset) {
		if fn, ok := goja.AssertFunction(reset); ok {
			if _, err := fn(devpack); err != nil {
				return err
			}
		}
	}
	if setCtx := obj.Get("setContext"); !goja.IsUndefined(setCtx) && !goja.IsNull(setCtx) {
		if fn, ok := goja.AssertFunction(setCtx); ok {
			contextPayload := map[string]any{
				"functionId": def.ID,
				"accountId":  def.AccountID,
			}
			if _, err := fn(devpack, rt.ToValue(contextPayload)); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectDevpackActions(rt *goja.Runtime) ([]Action, error) {
	devpack := rt.Get("Devpack")
	if goja.IsUndefined(devpack) || goja.IsNull(devpack) {
		return nil, nil
	}
	obj := devpack.ToObject(rt)
	flush := obj.Get("__flushActions")
	if goja.IsUndefined(flush) || goja.IsNull(flush) {
		return nil, nil
	}
	fn, ok := goja.AssertFunction(flush)
	if !ok {
		return nil, errors.New("devpack flush handler invalid")
	}
	value, err := fn(devpack)
	if err != nil {
		return nil, err
	}
	exported := value.Export()
	if exported == nil {
		return nil, nil
	}
	rawActions, ok := exported.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected devpack actions type %T", exported)
	}

	actions := make([]Action, 0, len(rawActions))
	for _, item := range rawActions {
		actionMap, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid action payload %T", item)
		}
		action, err := decodeAction(actionMap)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func decodeAction(raw map[string]any) (Action, error) {
	actionType := stringOrDefault(raw["type"], "")
	if actionType == "" {
		return Action{}, errors.New("devpack action missing type")
	}
	id := stringOrDefault(raw["id"], "")
	if id == "" {
		id = fmt.Sprintf("action_%d", time.Now().UnixNano())
	}
	params := mapFromAny(raw["params"])
	return Action{
		ID:     id,
		Type:   actionType,
		Params: params,
	}, nil
}

func stringOrDefault(value any, fallback string) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		if value == nil {
			return fallback
		}
		return fmt.Sprint(value)
	}
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case map[string]any:
		clone := make(map[string]any, len(v))
		for key, item := range v {
			clone[key] = mapCloneValue(item)
		}
		return clone
	default:
		return nil
	}
}

func mapCloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return mapFromAny(v)
	case []any:
		res := make([]any, len(v))
		for i, item := range v {
			res[i] = mapCloneValue(item)
		}
		return res
	default:
		return v
	}
}
