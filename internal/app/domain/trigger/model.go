package trigger

import "time"

// Type represents the supported trigger categories.
type Type string

const (
	TypeCron    Type = "cron"
	TypeEvent   Type = "event"
	TypeWebhook Type = "webhook"
)

// Trigger binds a runtime rule to a function invocation.
type Trigger struct {
	ID         string
	AccountID  string
	FunctionID string
	Type       Type
	Rule       string
	Config     map[string]string
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
