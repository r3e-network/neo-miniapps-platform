package vrf

import appvrf "github.com/R3E-Network/service_layer/internal/app/domain/vrf"

type (
	Key           = appvrf.Key
	Request       = appvrf.Request
	KeyStatus     = appvrf.KeyStatus
	RequestStatus = appvrf.RequestStatus
)

const (
	KeyStatusInactive        = appvrf.KeyStatusInactive
	KeyStatusPendingApproval = appvrf.KeyStatusPendingApproval
	KeyStatusActive          = appvrf.KeyStatusActive
	KeyStatusRevoked         = appvrf.KeyStatusRevoked

	RequestStatusPending   = appvrf.RequestStatusPending
	RequestStatusFulfilled = appvrf.RequestStatusFulfilled
	RequestStatusFailed    = appvrf.RequestStatusFailed
)
