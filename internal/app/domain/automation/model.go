package automation

import "time"

// Job represents a scheduled automation task tied to an account and function.
type Job struct {
	ID          string
	AccountID   string
	FunctionID  string
	Name        string
	Description string
	Schedule    string
	Enabled     bool
	LastRun     time.Time
	NextRun     time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
