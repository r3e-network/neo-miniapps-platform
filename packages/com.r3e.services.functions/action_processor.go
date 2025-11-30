package functions

import (
	"context"
)

// ActionProcessor defines the interface for processing devpack actions.
// This interface allows decoupling the Functions service from direct dependencies
// on other services (GasBank, Oracle, DataFeeds, etc.).
//
// Implementations of this interface are responsible for:
// 1. Routing actions to the appropriate service
// 2. Validating action parameters
// 3. Executing the action and returning results
//
// This design follows the Dependency Inversion Principle (DIP):
// - Functions service depends on the ActionProcessor abstraction
// - Concrete implementations depend on specific services
type ActionProcessor interface {
	// ProcessAction handles a single devpack action and returns the result.
	// The accountID is the owner of the function being executed.
	// The actionType identifies which service/operation to invoke.
	// The params contain action-specific parameters.
	ProcessAction(ctx context.Context, accountID string, actionType string, params map[string]any) (map[string]any, error)

	// SupportsAction returns true if this processor can handle the given action type.
	SupportsAction(actionType string) bool
}

// CompositeActionProcessor combines multiple ActionProcessors into one.
// It routes actions to the appropriate processor based on action type.
type CompositeActionProcessor struct {
	processors []ActionProcessor
}

// NewCompositeActionProcessor creates a new composite processor.
func NewCompositeActionProcessor(processors ...ActionProcessor) *CompositeActionProcessor {
	return &CompositeActionProcessor{processors: processors}
}

// ProcessAction routes the action to the appropriate processor.
func (c *CompositeActionProcessor) ProcessAction(ctx context.Context, accountID string, actionType string, params map[string]any) (map[string]any, error) {
	for _, p := range c.processors {
		if p.SupportsAction(actionType) {
			return p.ProcessAction(ctx, accountID, actionType, params)
		}
	}
	return nil, ErrUnsupportedAction
}

// SupportsAction returns true if any processor supports the action type.
func (c *CompositeActionProcessor) SupportsAction(actionType string) bool {
	for _, p := range c.processors {
		if p.SupportsAction(actionType) {
			return true
		}
	}
	return false
}

// AddProcessor adds a processor to the composite.
func (c *CompositeActionProcessor) AddProcessor(p ActionProcessor) {
	c.processors = append(c.processors, p)
}

// ErrUnsupportedAction is returned when no processor supports the action type.
var ErrUnsupportedAction = &errUnsupportedActionType{}

type errUnsupportedActionType struct{}

func (*errUnsupportedActionType) Error() string { return "unsupported action type" }

// GasBankActionProcessor handles GasBank-related actions.
// This is an example of how to implement ActionProcessor for a specific service.
type GasBankActionProcessor struct {
	gasBank GasBankAdapter
}

// GasBankAdapter defines the interface for GasBank operations needed by Functions.
// This allows Functions to use GasBank without importing the gasbank package directly.
type GasBankAdapter interface {
	EnsureAccount(ctx context.Context, accountID, wallet string) (GasBankAccountInfo, error)
	GetAccount(ctx context.Context, id string) (GasBankAccountInfo, error)
	Withdraw(ctx context.Context, accountID, gasAccountID string, amount float64, toAddress string) (GasBankAccountInfo, GasBankTransaction, error)
	ListTransactionsFiltered(ctx context.Context, gasAccountID, txType, status string, limit int) ([]GasBankTransaction, error)
}

// GasBankAccountInfo represents gas bank account information.
type GasBankAccountInfo struct {
	ID            string
	AccountID     string
	WalletAddress string
	Balance       float64
	Available     float64
	Pending       float64
}

// GasBankTransaction represents a gas bank transaction.
type GasBankTransaction struct {
	ID        string
	AccountID string
	Type      string
	Amount    float64
	Status    string
}

// NewGasBankActionProcessor creates a new GasBank action processor.
func NewGasBankActionProcessor(gasBank GasBankAdapter) *GasBankActionProcessor {
	return &GasBankActionProcessor{gasBank: gasBank}
}

// SupportsAction returns true for GasBank action types.
func (p *GasBankActionProcessor) SupportsAction(actionType string) bool {
	switch actionType {
	case ActionTypeGasBankEnsureAccount, ActionTypeGasBankWithdraw, ActionTypeGasBankBalance, ActionTypeGasBankListTx:
		return true
	}
	return false
}

// ProcessAction handles GasBank actions.
func (p *GasBankActionProcessor) ProcessAction(ctx context.Context, accountID string, actionType string, params map[string]any) (map[string]any, error) {
	if p.gasBank == nil {
		return nil, errDependencyUnavailable
	}

	switch actionType {
	case ActionTypeGasBankEnsureAccount:
		return p.handleEnsure(ctx, accountID, params)
	case ActionTypeGasBankWithdraw:
		return p.handleWithdraw(ctx, accountID, params)
	case ActionTypeGasBankBalance:
		return p.handleBalance(ctx, accountID, params)
	case ActionTypeGasBankListTx:
		return p.handleListTransactions(ctx, accountID, params)
	default:
		return nil, ErrUnsupportedAction
	}
}

func (p *GasBankActionProcessor) handleEnsure(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	wallet := stringParam(params, "wallet", "")
	acct, err := p.gasBank.EnsureAccount(ctx, accountID, wallet)
	if err != nil {
		return nil, err
	}
	return map[string]any{"account": structToMap(acct)}, nil
}

func (p *GasBankActionProcessor) handleWithdraw(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	gasAccountID := stringParam(params, "gasAccountId", "")
	wallet := stringParam(params, "wallet", "")
	if gasAccountID == "" && wallet == "" {
		return nil, errMissingParam("gasAccountId or wallet")
	}

	if gasAccountID == "" {
		ensured, err := p.gasBank.EnsureAccount(ctx, accountID, wallet)
		if err != nil {
			return nil, err
		}
		gasAccountID = ensured.ID
	}

	amount, err := floatParam(params, "amount")
	if err != nil || amount <= 0 {
		return nil, errInvalidParam("amount", "must be positive")
	}

	toAddress := stringParam(params, "to", "")
	updated, tx, err := p.gasBank.Withdraw(ctx, accountID, gasAccountID, amount, toAddress)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"account":     structToMap(updated),
		"transaction": structToMap(tx),
	}, nil
}

func (p *GasBankActionProcessor) handleBalance(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	gasAccountID := stringParam(params, "gasAccountId", "")
	wallet := stringParam(params, "wallet", "")
	if gasAccountID == "" && wallet == "" {
		return nil, errMissingParam("gasAccountId or wallet")
	}

	var acct GasBankAccountInfo
	var err error
	if gasAccountID != "" {
		acct, err = p.gasBank.GetAccount(ctx, gasAccountID)
	} else {
		acct, err = p.gasBank.EnsureAccount(ctx, accountID, wallet)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"account": structToMap(acct)}, nil
}

func (p *GasBankActionProcessor) handleListTransactions(ctx context.Context, accountID string, params map[string]any) (map[string]any, error) {
	gasAccountID := stringParam(params, "gasAccountId", "")
	wallet := stringParam(params, "wallet", "")
	if gasAccountID == "" && wallet == "" {
		return nil, errMissingParam("gasAccountId or wallet")
	}

	var acct GasBankAccountInfo
	var err error
	if gasAccountID != "" {
		acct, err = p.gasBank.GetAccount(ctx, gasAccountID)
	} else {
		acct, err = p.gasBank.EnsureAccount(ctx, accountID, wallet)
	}
	if err != nil {
		return nil, err
	}

	status := stringParam(params, "status", "")
	txType := stringParam(params, "type", "")
	limit := intParam(params, "limit", 100)

	txs, err := p.gasBank.ListTransactionsFiltered(ctx, acct.ID, txType, status, limit)
	if err != nil {
		return nil, err
	}

	serialized := make([]map[string]any, len(txs))
	for i, tx := range txs {
		serialized[i] = structToMap(tx)
	}
	return map[string]any{"transactions": serialized}, nil
}

// Helper error types
type errMissingParamType struct{ param string }

func errMissingParam(param string) error { return errMissingParamType{param} }
func (e errMissingParamType) Error() string {
	return "missing required parameter: " + e.param
}

type errInvalidParamType struct{ param, reason string }

func errInvalidParam(param, reason string) error { return errInvalidParamType{param, reason} }
func (e errInvalidParamType) Error() string {
	return "invalid parameter " + e.param + ": " + e.reason
}
