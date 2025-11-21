package devpack

import "time"

// Action represents a queued instruction emitted by a function.
type Action struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// ActionRef is a serializable reference to a queued action.
type ActionRef struct {
	Ref  bool                   `json:"__devpack_ref__"`
	ID   string                 `json:"id"`
	Type string                 `json:"type"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

// Response mirrors the Devpack JS helpers.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

const (
	ActionGasbankEnsureAccount   = "gasbank.ensureAccount"
	ActionGasbankWithdraw        = "gasbank.withdraw"
	ActionGasbankBalance         = "gasbank.balance"
	ActionGasbankListTransactions = "gasbank.listTransactions"
	ActionOracleCreateRequest    = "oracle.createRequest"
	ActionPricefeedSnapshot      = "pricefeed.recordSnapshot"
	ActionRandomGenerate         = "random.generate"
	ActionDatafeedSubmitUpdate   = "datafeeds.submitUpdate"
	ActionDatastreamPublishFrame = "datastreams.publishFrame"
	ActionDatalinkCreateDelivery = "datalink.createDelivery"
	ActionTriggersRegister       = "triggers.register"
	ActionAutomationSchedule     = "automation.schedule"
)

// ensure map is initialized once to avoid nil map panics when adding meta.
func params(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}

func newAction(id, t string, p map[string]interface{}) Action {
	return Action{ID: id, Type: t, Params: params(p)}
}

// EnsureGasAccount queues gasbank.ensureAccount.
func EnsureGasAccount(p map[string]interface{}) Action {
	return newAction("", ActionGasbankEnsureAccount, p)
}

// WithdrawGas queues gasbank.withdraw.
func WithdrawGas(p map[string]interface{}) Action {
	return newAction("", ActionGasbankWithdraw, p)
}

// BalanceGasAccount queues gasbank.balance.
func BalanceGasAccount(p map[string]interface{}) Action {
	return newAction("", ActionGasbankBalance, p)
}

// ListGasTransactions queues gasbank.listTransactions.
func ListGasTransactions(p map[string]interface{}) Action {
	return newAction("", ActionGasbankListTransactions, p)
}

// CreateOracleRequest queues oracle.createRequest.
func CreateOracleRequest(p map[string]interface{}) Action {
	return newAction("", ActionOracleCreateRequest, p)
}

// RecordPriceSnapshot queues pricefeed.recordSnapshot.
func RecordPriceSnapshot(p map[string]interface{}) Action {
	return newAction("", ActionPricefeedSnapshot, p)
}

// SubmitDataFeedUpdate queues datafeeds.submitUpdate.
func SubmitDataFeedUpdate(p map[string]interface{}) Action {
	return newAction("", ActionDatafeedSubmitUpdate, p)
}

// PublishDataStreamFrame queues datastreams.publishFrame.
func PublishDataStreamFrame(p map[string]interface{}) Action {
	return newAction("", ActionDatastreamPublishFrame, p)
}

// CreateDataLinkDelivery queues datalink.createDelivery.
func CreateDataLinkDelivery(p map[string]interface{}) Action {
	return newAction("", ActionDatalinkCreateDelivery, p)
}

// GenerateRandom queues random.generate (length defaults handled by runtime).
func GenerateRandom(p map[string]interface{}) Action {
	return newAction("", ActionRandomGenerate, p)
}

// RegisterTrigger queues triggers.register.
func RegisterTrigger(p map[string]interface{}) Action {
	return newAction("", ActionTriggersRegister, p)
}

// ScheduleAutomation queues automation.schedule.
func ScheduleAutomation(p map[string]interface{}) Action {
	return newAction("", ActionAutomationSchedule, p)
}

// AsResult converts an action into a reference with optional metadata.
func AsResult(a Action, meta map[string]interface{}) ActionRef {
	return ActionRef{
		Ref:  true,
		ID:   a.ID,
		Type: a.Type,
		Meta: meta,
	}
}

// Success builds a success response.
func Success(data interface{}, meta interface{}) Response {
	if data == nil {
		data = nil
	}
	if meta == nil {
		meta = nil
	}
	return Response{Success: true, Data: data, Meta: meta}
}

// Failure builds a failure response.
func Failure(err interface{}, meta interface{}) Response {
	if meta == nil {
		meta = nil
	}
	return Response{Success: false, Error: err, Meta: meta}
}

// Context holds metadata about the current execution.
type Context struct {
	FunctionID string
	AccountID  string
	Timestamp  time.Time
	Meta       map[string]interface{}
}
