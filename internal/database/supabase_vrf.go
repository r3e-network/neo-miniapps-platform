package database

import (
	"context"
	"encoding/json"
	"fmt"
)

// =============================================================================
// VRF Request Operations
// =============================================================================

// CreateVRFRequest inserts a VRF request.
func (r *Repository) CreateVRFRequest(ctx context.Context, req *VRFRequestRecord) error {
	data, err := r.client.request(ctx, "POST", "vrf_requests", req, "")
	if err != nil {
		return err
	}
	var rows []VRFRequestRecord
	if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
		*req = rows[0]
	}
	return nil
}

// UpdateVRFRequest updates an existing VRF request.
func (r *Repository) UpdateVRFRequest(ctx context.Context, req *VRFRequestRecord) error {
	query := fmt.Sprintf("request_id=eq.%s", req.RequestID)
	_, err := r.client.request(ctx, "PATCH", "vrf_requests", req, query)
	return err
}

// GetVRFRequest fetches a VRF request by request_id.
func (r *Repository) GetVRFRequest(ctx context.Context, requestID string) (*VRFRequestRecord, error) {
	data, err := r.client.request(ctx, "GET", "vrf_requests", nil, "request_id=eq."+requestID+"&limit=1")
	if err != nil {
		return nil, err
	}
	var rows []VRFRequestRecord
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("vrf request not found")
	}
	return &rows[0], nil
}

// ListVRFRequestsByStatus lists VRF requests by status.
func (r *Repository) ListVRFRequestsByStatus(ctx context.Context, status string) ([]VRFRequestRecord, error) {
	data, err := r.client.request(ctx, "GET", "vrf_requests", nil, "status=eq."+status)
	if err != nil {
		return nil, err
	}
	var rows []VRFRequestRecord
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
