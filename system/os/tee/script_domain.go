package tee

import (
	"context"
	"time"
)

// ScriptDefinition describes a user-provided script that can be executed in the TEE.
// This replaces the functions.Definition type.
type ScriptDefinition struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Source      string    `json:"source"`
	Secrets     []string  `json:"secrets,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ScriptExecutionStatus describes the lifecycle result of a script run.
type ScriptExecutionStatus string

const (
	ScriptStatusSucceeded ScriptExecutionStatus = "succeeded"
	ScriptStatusFailed    ScriptExecutionStatus = "failed"
)

// ActionStatus captures the outcome of a devpack action.
type ActionStatus string

const (
	ActionStatusPending   ActionStatus = "pending"
	ActionStatusSucceeded ActionStatus = "succeeded"
	ActionStatusFailed    ActionStatus = "failed"
)

// Action defines an instruction emitted by the devpack runtime.
type Action struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Params map[string]any `json:"params,omitempty"`
}

// Action type constants for devpack integration.
const (
	ActionTypeGasBankEnsureAccount = "gasbank.ensureAccount"
	ActionTypeGasBankWithdraw      = "gasbank.withdraw"
	ActionTypeGasBankBalance       = "gasbank.balance"
	ActionTypeGasBankListTx        = "gasbank.listTransactions"
	ActionTypeOracleCreateRequest  = "oracle.createRequest"
	ActionTypePriceFeedSnapshot    = "pricefeed.recordSnapshot"
	ActionTypeRandomGenerate       = "random.generate"
	ActionTypeDataFeedSubmit       = "datafeeds.submitUpdate"
	ActionTypeDatastreamPublish    = "datastreams.publishFrame"
	ActionTypeDatalinkDeliver      = "datalink.createDelivery"
	ActionTypeTriggerRegister      = "triggers.register"
	ActionTypeAutomationSchedule   = "automation.schedule"
)

// ActionResult records the execution result for an action.
type ActionResult struct {
	Action
	Status ActionStatus   `json:"status"`
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

// ScriptRunResult captures the outcome returned by the script executor.
type ScriptRunResult struct {
	ScriptID      string                `json:"script_id"`
	Output        map[string]any        `json:"output"`
	Logs          []string              `json:"logs,omitempty"`
	Error         string                `json:"error,omitempty"`
	Status        ScriptExecutionStatus `json:"status"`
	StartedAt     time.Time             `json:"started_at"`
	CompletedAt   time.Time             `json:"completed_at"`
	Duration      time.Duration         `json:"duration"`
	Actions       []Action              `json:"actions,omitempty"`
	ActionResults []ActionResult        `json:"action_results,omitempty"`
}

// ScriptRun is the persisted record of a script execution.
type ScriptRun struct {
	ID          string                `json:"id"`
	AccountID   string                `json:"account_id"`
	ScriptID    string                `json:"script_id"`
	Input       map[string]any        `json:"input"`
	Output      map[string]any        `json:"output"`
	Logs        []string              `json:"logs,omitempty"`
	Error       string                `json:"error,omitempty"`
	Status      ScriptExecutionStatus `json:"status"`
	StartedAt   time.Time             `json:"started_at"`
	CompletedAt time.Time             `json:"completed_at"`
	Duration    time.Duration         `json:"duration"`
	Actions     []ActionResult        `json:"actions,omitempty"`
}

// ScriptStore defines the persistence interface for scripts and their executions.
type ScriptStore interface {
	// Script CRUD
	CreateScript(ctx context.Context, def ScriptDefinition) (ScriptDefinition, error)
	UpdateScript(ctx context.Context, def ScriptDefinition) (ScriptDefinition, error)
	GetScript(ctx context.Context, id string) (ScriptDefinition, error)
	ListScripts(ctx context.Context, accountID string) ([]ScriptDefinition, error)
	DeleteScript(ctx context.Context, id string) error

	// Execution history
	CreateScriptRun(ctx context.Context, run ScriptRun) (ScriptRun, error)
	GetScriptRun(ctx context.Context, id string) (ScriptRun, error)
	ListScriptRuns(ctx context.Context, scriptID string, limit int) ([]ScriptRun, error)
}

// ActionProcessor processes devpack actions emitted during script execution.
type ActionProcessor interface {
	// ProcessAction handles a single action and returns the result.
	ProcessAction(ctx context.Context, accountID string, actionType string, params map[string]any) (map[string]any, error)

	// SupportsAction checks if this processor can handle the given action type.
	SupportsAction(actionType string) bool
}

// ScriptSecretResolver resolves secret values for script execution.
type ScriptSecretResolver interface {
	// ResolveSecrets retrieves secret values by name for the given account.
	ResolveSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error)
}
