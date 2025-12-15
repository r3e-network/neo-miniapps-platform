// Package service provides common service infrastructure.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	gasclient "github.com/R3E-Network/service_layer/services/gasaccounting/client"
)

// GasAccountingAdapter provides GAS accounting integration for services.
// Services use this to reserve, consume, and release GAS for operations.
type GasAccountingAdapter struct {
	client    *gasclient.Client
	serviceID string
	logError  func(ctx context.Context, msg string, err error) // Optional error logger
}

// NewGasAccountingAdapter creates a new GasAccounting adapter.
func NewGasAccountingAdapter(baseURL, serviceID string) (*GasAccountingAdapter, error) {
	if strings.TrimSpace(baseURL) == "" {
		return &GasAccountingAdapter{client: nil, serviceID: serviceID}, nil
	}

	client, err := gasclient.New(gasclient.Config{
		BaseURL:   baseURL,
		ServiceID: serviceID,
	})
	if err != nil {
		return nil, err
	}

	return &GasAccountingAdapter{
		client:    client,
		serviceID: serviceID,
	}, nil
}

// WithErrorLogger sets an error logging callback for non-fatal errors.
func (a *GasAccountingAdapter) WithErrorLogger(fn func(ctx context.Context, msg string, err error)) *GasAccountingAdapter {
	a.logError = fn
	return a
}

// CheckBalance verifies a user has sufficient balance for an operation.
func (a *GasAccountingAdapter) CheckBalance(ctx context.Context, userID int64, requiredAmount int64) error {
	if a.client == nil {
		return nil // GasAccounting not configured, skip check
	}

	balance, err := a.client.GetBalance(ctx, userID)
	if err != nil {
		return fmt.Errorf("get balance: %w", err)
	}

	if balance.AvailableBalance < requiredAmount {
		return fmt.Errorf("insufficient balance: have %d, need %d", balance.AvailableBalance, requiredAmount)
	}

	return nil
}

// Reserve reserves GAS for a pending operation.
// Returns reservation ID that must be released when operation completes.
func (a *GasAccountingAdapter) Reserve(ctx context.Context, userID int64, amount int64, requestID string, ttl time.Duration) (string, error) {
	if a.client == nil {
		return "", nil // GasAccounting not configured
	}

	if ttl == 0 {
		ttl = 10 * time.Minute
	}

	resp, err := a.client.Reserve(ctx, userID, amount, requestID, ttl)
	if err != nil {
		return "", fmt.Errorf("reserve gas: %w", err)
	}

	return resp.ReservationID, nil
}

// Release releases a reservation, optionally consuming the GAS.
func (a *GasAccountingAdapter) Release(ctx context.Context, reservationID string, consume bool, actualAmount int64) error {
	if a.client == nil || reservationID == "" {
		return nil // GasAccounting not configured or no reservation
	}

	_, err := a.client.Release(ctx, reservationID, consume, actualAmount)
	if err != nil {
		return fmt.Errorf("release reservation: %w", err)
	}

	return nil
}

// Consume directly consumes GAS without reservation (for simple operations).
func (a *GasAccountingAdapter) Consume(ctx context.Context, userID int64, amount int64, requestID, description string) error {
	if a.client == nil {
		return nil // GasAccounting not configured
	}

	_, err := a.client.Consume(ctx, userID, amount, requestID, description)
	if err != nil {
		return fmt.Errorf("consume gas: %w", err)
	}

	return nil
}

// GetBalance returns a user's current balance.
func (a *GasAccountingAdapter) GetBalance(ctx context.Context, userID int64) (available, reserved int64, err error) {
	if a.client == nil {
		return 0, 0, nil // GasAccounting not configured
	}

	balance, err := a.client.GetBalance(ctx, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("get balance: %w", err)
	}

	return balance.AvailableBalance, balance.ReservedBalance, nil
}

// =============================================================================
// Operation Helper
// =============================================================================

// WithGasReservation executes an operation with GAS reservation.
// It reserves GAS before the operation and releases/consumes after.
func (a *GasAccountingAdapter) WithGasReservation(
	ctx context.Context,
	userID int64,
	estimatedCost int64,
	requestID string,
	ttl time.Duration,
	operation func() (actualCost int64, err error),
) error {
	// Reserve GAS
	reservationID, err := a.Reserve(ctx, userID, estimatedCost, requestID, ttl)
	if err != nil {
		return err
	}

	// Execute operation
	actualCost, opErr := operation()

	// Release reservation
	if opErr != nil {
		// Operation failed, release without consuming
		a.Release(ctx, reservationID, false, 0)
		return opErr
	}

	// Operation succeeded, consume actual cost
	if err := a.Release(ctx, reservationID, true, actualCost); err != nil {
		// Log but don't fail the operation (operation already succeeded)
		if a.logError != nil {
			a.logError(ctx, "failed to release gas reservation after successful operation", err)
		}
	}

	return nil
}
