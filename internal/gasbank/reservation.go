package gasbank

import (
	"context"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/google/uuid"
)

// =============================================================================
// Reservation Operations
// =============================================================================

// Reserve reserves funds for a pending service operation.
func (m *Manager) Reserve(ctx context.Context, userID, serviceID, referenceID string, amount int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	account, err := m.db.GetGasBankAccount(ctx, userID)
	if err != nil {
		return "", err
	}

	available := account.Balance - account.Reserved
	if amount > available {
		return "", fmt.Errorf("insufficient balance: available %d, required %d", available, amount)
	}

	reservation := &Reservation{
		ID:          uuid.New().String(),
		UserID:      userID,
		ServiceID:   serviceID,
		ReferenceID: referenceID,
		Amount:      amount,
		Status:      ReservationPending,
		CreatedAt:   time.Now(),
	}

	newReserved := account.Reserved + amount
	if err := m.db.UpdateGasBankBalance(ctx, userID, account.Balance, newReserved); err != nil {
		return "", err
	}

	m.reservations[reservation.ID] = reservation
	return reservation.ID, nil
}

// Release releases a reservation back to the user.
func (m *Manager) Release(ctx context.Context, userID, reservationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	reservation, ok := m.reservations[reservationID]
	if !ok {
		return nil // Idempotent: treat as already released
	}

	delete(m.reservations, reservationID)

	if reservation.UserID != userID {
		return database.ErrUnauthorized
	}

	if reservation.Status != ReservationPending {
		return fmt.Errorf("reservation already %s", reservation.Status)
	}

	account, err := m.db.GetGasBankAccount(ctx, userID)
	if err != nil {
		return err
	}

	newReserved := account.Reserved - reservation.Amount
	if newReserved < 0 {
		newReserved = 0
	}

	if err := m.db.UpdateGasBankBalance(ctx, userID, account.Balance, newReserved); err != nil {
		return err
	}

	reservation.Status = ReservationReleased
	return nil
}

// Consume consumes a reservation (service completed successfully).
func (m *Manager) Consume(ctx context.Context, userID, reservationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	reservation, ok := m.reservations[reservationID]
	if !ok {
		return database.NewNotFoundError("reservation", reservationID)
	}

	if reservation.UserID != userID {
		return database.ErrUnauthorized
	}

	if reservation.Status != ReservationPending {
		return fmt.Errorf("reservation already %s", reservation.Status)
	}

	account, err := m.db.GetGasBankAccount(ctx, userID)
	if err != nil {
		return err
	}

	newBalance := account.Balance - reservation.Amount
	newReserved := account.Reserved - reservation.Amount
	if newReserved < 0 {
		newReserved = 0
	}

	if err := m.db.UpdateGasBankBalance(ctx, userID, newBalance, newReserved); err != nil {
		return err
	}

	if err := m.db.CreateGasBankTransaction(ctx, &database.GasBankTransaction{
		ID:           uuid.New().String(),
		AccountID:    account.ID,
		TxType:       TxTypeServiceFee,
		Amount:       -reservation.Amount,
		BalanceAfter: newBalance,
		ReferenceID:  reservation.ReferenceID,
		Status:       "completed",
		CreatedAt:    time.Now(),
	}); err != nil {
		return err
	}

	reservation.Status = ReservationConsumed
	reservation.ConsumedAt = time.Now()
	delete(m.reservations, reservationID)

	return nil
}
