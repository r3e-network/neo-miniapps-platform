package function

import "time"

// Definition describes a user-provided function that can be executed by the
// runtime.
type Definition struct {
	ID          string
	AccountID   string
	Name        string
	Description string
	Source      string
	Secrets     []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
