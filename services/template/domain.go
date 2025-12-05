// Package template provides domain types for the template service.
package template

import "time"

// =============================================================================
// Domain Types
// =============================================================================

// Item represents a domain entity.
type Item struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
	Status      ItemStatus     `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ItemStatus represents the status of an item.
type ItemStatus string

const (
	ItemStatusActive   ItemStatus = "active"
	ItemStatusInactive ItemStatus = "inactive"
	ItemStatusDeleted  ItemStatus = "deleted"
)

// SetTimestamps sets creation and update timestamps.
func (i *Item) SetTimestamps() {
	now := time.Now()
	if i.CreatedAt.IsZero() {
		i.CreatedAt = now
	}
	i.UpdatedAt = now
}

// =============================================================================
// Request/Response Types
// =============================================================================

// CreateItemRequest represents a request to create an item.
type CreateItemRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}

// UpdateItemRequest represents a request to update an item.
type UpdateItemRequest struct {
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
	Status      *ItemStatus    `json:"status,omitempty"`
}

// ListItemsRequest represents a request to list items.
type ListItemsRequest struct {
	Status *ItemStatus `json:"status,omitempty"`
	Limit  int         `json:"limit,omitempty"`
	Offset int         `json:"offset,omitempty"`
}

// ListItemsResponse represents a response containing items.
type ListItemsResponse struct {
	Items      []*Item `json:"items"`
	TotalCount int     `json:"total_count"`
	HasMore    bool    `json:"has_more"`
}

// =============================================================================
// Statistics
// =============================================================================

// Stats represents service statistics.
type Stats struct {
	TotalItems   int       `json:"total_items"`
	ActiveItems  int       `json:"active_items"`
	ProcessedOps int64     `json:"processed_ops"`
	GeneratedAt  time.Time `json:"generated_at"`
}
