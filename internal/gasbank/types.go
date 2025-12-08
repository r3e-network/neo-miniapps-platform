package gasbank

import "time"

const (
	// Transaction types
	TxTypeDeposit       = "deposit"
	TxTypeWithdraw      = "withdraw"
	TxTypeServiceFee    = "service_fee"
	TxTypeRefund        = "refund"
	TxTypeSponsor       = "sponsor"        // Sponsor payment for contract/user
	TxTypeSponsorCredit = "sponsor_credit" // Credit received from sponsor

	// Reservation status
	ReservationPending  = "pending"
	ReservationConsumed = "consumed"
	ReservationReleased = "released"
)

// ServiceFees defines the fee for each service (in GAS smallest unit, 1e-8 GAS).
var ServiceFees = map[string]int64{
	"vrf":          100000,  // 0.001 GAS per VRF request
	"automation":   50000,   // 0.0005 GAS per trigger execution
	"datafeeds":    10000,   // 0.0001 GAS per price query
	"mixer":        5000000, // 0.05 GAS base fee (+ 0.5% of amount)
	"confidential": 100000,  // 0.001 GAS per compute job
}

// Reservation represents a fee reservation for a pending operation.
type Reservation struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	ServiceID   string    `json:"service_id"`
	ReferenceID string    `json:"reference_id"`
	Amount      int64     `json:"amount"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	ConsumedAt  time.Time `json:"consumed_at,omitempty"`
}

// GetServiceFee returns the fee for a service.
func GetServiceFee(serviceID string) int64 {
	if fee, ok := ServiceFees[serviceID]; ok {
		return fee
	}
	return 0
}
