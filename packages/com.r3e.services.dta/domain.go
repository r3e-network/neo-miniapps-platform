package dta

import "time"

// ProductStatus enumerates DTA product states.
type ProductStatus string

const (
	ProductStatusInactive  ProductStatus = "inactive"
	ProductStatusActive    ProductStatus = "active"
	ProductStatusSuspended ProductStatus = "suspended"
)

// Product describes a subscription/redemption product.
type Product struct {
	ID              string            `json:"id"`
	AccountID       string            `json:"account_id"`
	Name            string            `json:"name"`
	Symbol          string            `json:"symbol"`
	Type            string            `json:"type"`
	Status          ProductStatus     `json:"status"`
	SettlementTerms string            `json:"settlement_terms"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// OrderType enumerates order types.
type OrderType string

const (
	OrderTypeSubscription OrderType = "subscription"
	OrderTypeRedemption   OrderType = "redemption"
)

// OrderStatus enumerates lifecycle states.
type OrderStatus string

const (
	OrderStatusPending  OrderStatus = "pending"
	OrderStatusApproved OrderStatus = "approved"
	OrderStatusSettled  OrderStatus = "settled"
	OrderStatusRejected OrderStatus = "rejected"
	OrderStatusCanceled OrderStatus = "canceled"
)

// Order represents a subscription/redemption request.
type Order struct {
	ID        string            `json:"id"`
	AccountID string            `json:"account_id"`
	ProductID string            `json:"product_id"`
	Type      OrderType         `json:"type"`
	Amount    string            `json:"amount"`
	Wallet    string            `json:"wallet_address"`
	Status    OrderStatus       `json:"status"`
	Error     string            `json:"error"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}
