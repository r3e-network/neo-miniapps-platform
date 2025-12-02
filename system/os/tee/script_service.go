package tee

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// ScriptService manages script definitions and executions within the TEE.
// This is the core service that replaces the functions service.
type ScriptService struct {
	engine          Engine
	store           ScriptStore
	secretManager   *SecretManager
	actionProcessor ActionProcessor
	accountChecker  core.AccountChecker
}

// ScriptServiceConfig configures the script service.
type ScriptServiceConfig struct {
	Engine          Engine
	Store           ScriptStore
	SecretManager   *SecretManager
	ActionProcessor ActionProcessor
	AccountChecker  core.AccountChecker
}

// NewScriptService creates a new script service.
func NewScriptService(cfg ScriptServiceConfig) *ScriptService {
	return &ScriptService{
		engine:          cfg.Engine,
		store:           cfg.Store,
		secretManager:   cfg.SecretManager,
		actionProcessor: cfg.ActionProcessor,
		accountChecker:  cfg.AccountChecker,
	}
}

// SetActionProcessor attaches an action processor for devpack actions.
func (s *ScriptService) SetActionProcessor(processor ActionProcessor) {
	s.actionProcessor = processor
}

// Create registers a new script definition.
func (s *ScriptService) Create(ctx context.Context, def ScriptDefinition) (ScriptDefinition, error) {
	if def.AccountID == "" {
		return ScriptDefinition{}, fmt.Errorf("account_id required")
	}
	if def.Name == "" {
		return ScriptDefinition{}, fmt.Errorf("name required")
	}
	if def.Source == "" {
		return ScriptDefinition{}, fmt.Errorf("source required")
	}

	// Validate account exists
	if s.accountChecker != nil {
		if err := s.accountChecker.AccountExists(ctx, def.AccountID); err != nil {
			return ScriptDefinition{}, fmt.Errorf("account validation failed: %w", err)
		}
	}

	// Validate secrets exist if specified
	if s.secretManager != nil && len(def.Secrets) > 0 {
		for _, secretName := range def.Secrets {
			_, err := s.secretManager.GetSecret(ctx, "scripts", def.AccountID, secretName)
			if err != nil {
				return ScriptDefinition{}, fmt.Errorf("secret validation failed for %s: %w", secretName, err)
			}
		}
	}

	// Validate script syntax
	if err := s.engine.(*engineImpl).scriptEngine.ValidateScript(ctx, def.Source); err != nil {
		return ScriptDefinition{}, fmt.Errorf("script validation failed: %w", err)
	}

	return s.store.CreateScript(ctx, def)
}

// Update modifies an existing script definition.
func (s *ScriptService) Update(ctx context.Context, def ScriptDefinition) (ScriptDefinition, error) {
	existing, err := s.store.GetScript(ctx, def.ID)
	if err != nil {
		return ScriptDefinition{}, err
	}

	// Merge fields
	if def.Name == "" {
		def.Name = existing.Name
	}
	if def.Description == "" {
		def.Description = existing.Description
	}
	if def.Source == "" {
		def.Source = existing.Source
	}
	if def.Secrets == nil {
		def.Secrets = existing.Secrets
	}
	def.AccountID = existing.AccountID

	// Validate new source if provided
	if def.Source != existing.Source {
		if err := s.engine.(*engineImpl).scriptEngine.ValidateScript(ctx, def.Source); err != nil {
			return ScriptDefinition{}, fmt.Errorf("script validation failed: %w", err)
		}
	}

	return s.store.UpdateScript(ctx, def)
}

// Get retrieves a script by ID.
func (s *ScriptService) Get(ctx context.Context, id string) (ScriptDefinition, error) {
	return s.store.GetScript(ctx, id)
}

// List returns scripts belonging to an account.
func (s *ScriptService) List(ctx context.Context, accountID string) ([]ScriptDefinition, error) {
	return s.store.ListScripts(ctx, accountID)
}

// Delete removes a script.
func (s *ScriptService) Delete(ctx context.Context, id string) error {
	return s.store.DeleteScript(ctx, id)
}

// Execute runs a script and records the execution.
func (s *ScriptService) Execute(ctx context.Context, scriptID string, payload map[string]any) (ScriptRun, error) {
	def, err := s.store.GetScript(ctx, scriptID)
	if err != nil {
		return ScriptRun{}, err
	}

	// Validate account
	if s.accountChecker != nil {
		if err := s.accountChecker.AccountExists(ctx, def.AccountID); err != nil {
			return ScriptRun{}, fmt.Errorf("account validation failed: %w", err)
		}
	}

	// Execute in TEE
	inputCopy := cloneMap(payload)
	result, execErr := s.executeInTEE(ctx, def, payload)

	// Process actions if execution succeeded
	var actionResults []ActionResult
	if execErr == nil && len(result.Actions) > 0 {
		actionResults = s.processActions(ctx, def.AccountID, result.Actions)
	}

	// Determine final status
	status := result.Status
	if execErr != nil {
		status = ScriptStatusFailed
		if result.Error == "" {
			result.Error = execErr.Error()
		}
	} else if status == "" {
		status = ScriptStatusSucceeded
	}

	// Create execution record
	run := ScriptRun{
		AccountID:   def.AccountID,
		ScriptID:    def.ID,
		Input:       inputCopy,
		Output:      result.Output,
		Logs:        result.Logs,
		Error:       result.Error,
		Status:      status,
		StartedAt:   result.StartedAt,
		CompletedAt: result.CompletedAt,
		Duration:    result.Duration,
		Actions:     actionResults,
	}

	// Persist execution record
	saved, storeErr := s.store.CreateScriptRun(ctx, run)
	if storeErr != nil {
		if execErr != nil {
			return ScriptRun{}, fmt.Errorf("execution failed (%v) and could not persist: %w", execErr, storeErr)
		}
		return ScriptRun{}, fmt.Errorf("could not persist execution: %w", storeErr)
	}

	if execErr != nil {
		return saved, execErr
	}

	return saved, nil
}

// executeInTEE runs the script in the TEE engine.
func (s *ScriptService) executeInTEE(ctx context.Context, def ScriptDefinition, payload map[string]any) (ScriptRunResult, error) {
	startedAt := time.Now().UTC()

	// Prepare input
	input := cloneMap(payload)
	input["_script"] = map[string]any{
		"id":          def.ID,
		"name":        def.Name,
		"description": def.Description,
	}

	// Wrap source for TEE execution
	wrappedScript := wrapScriptSource(def.Source)

	// Execute in TEE
	result, err := s.engine.Execute(ctx, ExecutionRequest{
		ServiceID:  "scripts",
		AccountID:  def.AccountID,
		Script:     wrappedScript,
		EntryPoint: "main",
		Input:      input,
		Secrets:    def.Secrets,
		Metadata: map[string]string{
			"script_id":   def.ID,
			"script_name": def.Name,
		},
	})

	completedAt := time.Now().UTC()

	if err != nil {
		return ScriptRunResult{
			ScriptID:    def.ID,
			Status:      ScriptStatusFailed,
			Error:       err.Error(),
			StartedAt:   startedAt,
			CompletedAt: completedAt,
			Duration:    completedAt.Sub(startedAt),
		}, err
	}

	// Map result
	runResult := ScriptRunResult{
		ScriptID:    def.ID,
		Output:      result.Output,
		Logs:        result.Logs,
		StartedAt:   result.StartedAt,
		CompletedAt: result.CompletedAt,
		Duration:    result.Duration,
	}

	// Map status
	switch result.Status {
	case ExecutionStatusSucceeded:
		runResult.Status = ScriptStatusSucceeded
	case ExecutionStatusFailed:
		runResult.Status = ScriptStatusFailed
		runResult.Error = result.Error
	case ExecutionStatusTimeout:
		runResult.Status = ScriptStatusFailed
		runResult.Error = "execution timeout"
	default:
		runResult.Status = ScriptStatusFailed
		runResult.Error = result.Error
	}

	// Extract actions from output
	if result.Output != nil {
		if actions, ok := result.Output["_actions"].([]any); ok {
			for _, a := range actions {
				if actionMap, ok := a.(map[string]any); ok {
					action := parseAction(actionMap)
					if action.Type != "" {
						runResult.Actions = append(runResult.Actions, action)
					}
				}
			}
			delete(result.Output, "_actions")
			runResult.Output = result.Output
		}
	}

	return runResult, nil
}

// processActions handles devpack actions emitted during execution.
func (s *ScriptService) processActions(ctx context.Context, accountID string, actions []Action) []ActionResult {
	if s.actionProcessor == nil {
		// No processor configured, mark all as pending
		results := make([]ActionResult, len(actions))
		for i, action := range actions {
			results[i] = ActionResult{
				Action: action,
				Status: ActionStatusPending,
				Error:  "action processor not configured",
			}
		}
		return results
	}

	results := make([]ActionResult, len(actions))
	for i, action := range actions {
		result := ActionResult{Action: action}

		if !s.actionProcessor.SupportsAction(action.Type) {
			result.Status = ActionStatusFailed
			result.Error = fmt.Sprintf("unsupported action type: %s", action.Type)
		} else {
			output, err := s.actionProcessor.ProcessAction(ctx, accountID, action.Type, action.Params)
			if err != nil {
				result.Status = ActionStatusFailed
				result.Error = err.Error()
			} else {
				result.Status = ActionStatusSucceeded
				result.Result = output
			}
		}

		results[i] = result
	}

	return results
}

// GetRun retrieves an execution record.
func (s *ScriptService) GetRun(ctx context.Context, id string) (ScriptRun, error) {
	return s.store.GetScriptRun(ctx, id)
}

// ListRuns returns execution history for a script.
func (s *ScriptService) ListRuns(ctx context.Context, scriptID string, limit int) ([]ScriptRun, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.store.ListScriptRuns(ctx, scriptID, limit)
}

// wrapScriptSource wraps user source code for TEE execution.
func wrapScriptSource(source string) string {
	return fmt.Sprintf(`
// Devpack runtime for TEE execution
var Devpack = {
	actions: [],
	context: {},
	setContext: function(ctx) { this.context = ctx; },
	__reset: function() { this.actions = []; },
	__flushActions: function() { return this.actions; },
	emit: function(type, params) {
		this.actions.push({ type: type, params: params, id: 'action_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9) });
	}
};

// User script
var userScript = %s;

// Main entry point for TEE engine
function main(input) {
	var params = input;
	var result;
	if (typeof userScript === 'function') {
		result = userScript(params, secrets);
	} else {
		result = userScript;
	}
	// Include any devpack actions in the result
	var actions = Devpack.__flushActions();
	if (actions && actions.length > 0) {
		if (typeof result === 'object' && result !== null) {
			result._actions = actions;
		} else {
			result = { result: result, _actions: actions };
		}
	}
	return result;
}
`, source)
}

// parseAction extracts an Action from a map.
func parseAction(m map[string]any) Action {
	action := Action{}
	if t, ok := m["type"].(string); ok {
		action.Type = t
	}
	if id, ok := m["id"].(string); ok {
		action.ID = id
	}
	if params, ok := m["params"].(map[string]any); ok {
		action.Params = params
	}
	return action
}

// cloneMap creates a shallow copy of a map.
func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// Invoke implements the compute engine interface for service engine integration.
func (s *ScriptService) Invoke(ctx context.Context, payload any) (any, error) {
	req, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invoke payload must be a map")
	}

	scriptID, _ := req["script_id"].(string)
	if scriptID == "" {
		scriptID, _ = req["function_id"].(string) // backward compatibility
	}
	accountID, _ := req["account_id"].(string)

	if scriptID == "" || accountID == "" {
		return nil, fmt.Errorf("account_id and script_id required")
	}

	// Validate account
	if s.accountChecker != nil {
		if err := s.accountChecker.AccountExists(ctx, accountID); err != nil {
			return nil, err
		}
	}

	// Get script and verify ownership
	def, err := s.store.GetScript(ctx, scriptID)
	if err != nil {
		return nil, err
	}
	if def.AccountID != accountID {
		return nil, fmt.Errorf("script belongs to different account")
	}

	// Extract input
	input := make(map[string]any)
	if rawInput, ok := req["input"]; ok {
		switch v := rawInput.(type) {
		case map[string]any:
			input = v
		case string:
			input["input"] = v
		}
	}

	return s.Execute(ctx, scriptID, input)
}

// Name returns the service name.
func (s *ScriptService) Name() string {
	return "scripts"
}

// Start initializes the service.
func (s *ScriptService) Start(ctx context.Context) error {
	// Register scripts service with TEE engine
	return s.engine.RegisterService(ctx, ServiceRegistration{
		ServiceID:               "scripts",
		AllowedSecretPatterns:   []string{"*"},
		MaxConcurrentExecutions: DefaultMaxConcurrent,
		DefaultTimeout:          DefaultExecutionTimeout,
		DefaultMemoryLimit:      DefaultMemoryLimit,
	})
}

// Stop shuts down the service.
func (s *ScriptService) Stop(ctx context.Context) error {
	return nil
}

// Ready checks if the service is ready.
func (s *ScriptService) Ready(ctx context.Context) error {
	return s.engine.Health(ctx)
}

// HTTP API Methods for automatic route discovery

// HTTPGetScripts handles GET /scripts - list all scripts for an account.
func (s *ScriptService) HTTPGetScripts(ctx context.Context, accountID string, query map[string]string, body map[string]any) (any, error) {
	return s.List(ctx, accountID)
}

// HTTPPostScripts handles POST /scripts - create a new script.
func (s *ScriptService) HTTPPostScripts(ctx context.Context, accountID string, query map[string]string, body map[string]any) (any, error) {
	name, _ := body["name"].(string)
	description, _ := body["description"].(string)
	source, _ := body["source"].(string)
	var secrets []string
	if rawSecrets, ok := body["secrets"].([]any); ok {
		for _, s := range rawSecrets {
			if str, ok := s.(string); ok {
				secrets = append(secrets, str)
			}
		}
	}

	def := ScriptDefinition{
		AccountID:   accountID,
		Name:        name,
		Description: description,
		Source:      source,
		Secrets:     secrets,
	}
	return s.Create(ctx, def)
}

// HTTPGetScriptsById handles GET /scripts/{id} - get a specific script.
func (s *ScriptService) HTTPGetScriptsById(ctx context.Context, accountID string, pathParams map[string]string, query map[string]string, body map[string]any) (any, error) {
	scriptID := pathParams["id"]
	def, err := s.Get(ctx, scriptID)
	if err != nil {
		return nil, err
	}
	if def.AccountID != accountID {
		return nil, fmt.Errorf("forbidden: script belongs to different account")
	}
	return def, nil
}

// HTTPPatchScriptsById handles PATCH /scripts/{id} - update a script.
func (s *ScriptService) HTTPPatchScriptsById(ctx context.Context, accountID string, pathParams map[string]string, query map[string]string, body map[string]any) (any, error) {
	scriptID := pathParams["id"]

	// Verify ownership
	existing, err := s.Get(ctx, scriptID)
	if err != nil {
		return nil, err
	}
	if existing.AccountID != accountID {
		return nil, fmt.Errorf("forbidden: script belongs to different account")
	}

	def := ScriptDefinition{ID: scriptID}
	if name, ok := body["name"].(string); ok {
		def.Name = name
	}
	if description, ok := body["description"].(string); ok {
		def.Description = description
	}
	if source, ok := body["source"].(string); ok {
		def.Source = source
	}
	if rawSecrets, ok := body["secrets"].([]any); ok {
		var secrets []string
		for _, s := range rawSecrets {
			if str, ok := s.(string); ok {
				secrets = append(secrets, str)
			}
		}
		def.Secrets = secrets
	}

	return s.Update(ctx, def)
}

// HTTPDeleteScriptsById handles DELETE /scripts/{id} - delete a script.
func (s *ScriptService) HTTPDeleteScriptsById(ctx context.Context, accountID string, pathParams map[string]string, query map[string]string, body map[string]any) (any, error) {
	scriptID := pathParams["id"]

	// Verify ownership
	existing, err := s.Get(ctx, scriptID)
	if err != nil {
		return nil, err
	}
	if existing.AccountID != accountID {
		return nil, fmt.Errorf("forbidden: script belongs to different account")
	}

	return nil, s.Delete(ctx, scriptID)
}

// HTTPPostScriptsIdExecute handles POST /scripts/{id}/execute - execute a script.
func (s *ScriptService) HTTPPostScriptsIdExecute(ctx context.Context, accountID string, pathParams map[string]string, query map[string]string, body map[string]any) (any, error) {
	scriptID := pathParams["id"]

	// Verify ownership
	def, err := s.Get(ctx, scriptID)
	if err != nil {
		return nil, err
	}
	if def.AccountID != accountID {
		return nil, fmt.Errorf("forbidden: script belongs to different account")
	}

	return s.Execute(ctx, scriptID, body)
}

// HTTPGetScriptsIdRuns handles GET /scripts/{id}/runs - list execution history.
func (s *ScriptService) HTTPGetScriptsIdRuns(ctx context.Context, accountID string, pathParams map[string]string, query map[string]string, body map[string]any) (any, error) {
	scriptID := pathParams["id"]

	// Verify ownership
	def, err := s.Get(ctx, scriptID)
	if err != nil {
		return nil, err
	}
	if def.AccountID != accountID {
		return nil, fmt.Errorf("forbidden: script belongs to different account")
	}

	limit := 25
	if limitStr, ok := query["limit"]; ok {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	return s.ListRuns(ctx, scriptID, limit)
}

// Backward compatibility aliases for functions API

// HTTPGetFunctions handles GET /functions - list all functions (alias for scripts).
func (s *ScriptService) HTTPGetFunctions(ctx context.Context, accountID string, query map[string]string, body map[string]any) (any, error) {
	scripts, err := s.List(ctx, accountID)
	if err != nil {
		return nil, err
	}
	// Convert to function format for backward compatibility
	return convertScriptsToFunctions(scripts), nil
}

// HTTPPostFunctions handles POST /functions - create a new function (alias for scripts).
func (s *ScriptService) HTTPPostFunctions(ctx context.Context, accountID string, query map[string]string, body map[string]any) (any, error) {
	result, err := s.HTTPPostScripts(ctx, accountID, query, body)
	if err != nil {
		return nil, err
	}
	if def, ok := result.(ScriptDefinition); ok {
		return convertScriptToFunction(def), nil
	}
	return result, nil
}

func convertScriptsToFunctions(scripts []ScriptDefinition) []map[string]any {
	result := make([]map[string]any, len(scripts))
	for i, s := range scripts {
		result[i] = convertScriptToFunction(s)
	}
	return result
}

func convertScriptToFunction(s ScriptDefinition) map[string]any {
	return map[string]any{
		"id":          s.ID,
		"account_id":  s.AccountID,
		"name":        s.Name,
		"description": s.Description,
		"source":      s.Source,
		"secrets":     s.Secrets,
		"created_at":  s.CreatedAt,
		"updated_at":  s.UpdatedAt,
	}
}

// Ensure ScriptService implements required interfaces
var _ interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
	Ready(context.Context) error
} = (*ScriptService)(nil)

// Helper to trim and validate strings
func trimString(s string) string {
	return strings.TrimSpace(s)
}
