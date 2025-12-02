package lottery

import "context"

// Store defines the persistence interface for lottery data.
type Store interface {
	// Round operations
	CreateRound(ctx context.Context, round Round) (Round, error)
	GetRound(ctx context.Context, roundID string) (Round, error)
	GetActiveRound(ctx context.Context, accountID string) (Round, error)
	UpdateRound(ctx context.Context, round Round) (Round, error)
	ListRounds(ctx context.Context, accountID string, limit int) ([]Round, error)

	// Ticket operations
	CreateTicket(ctx context.Context, ticket Ticket) (Ticket, error)
	GetTicket(ctx context.Context, ticketID string) (Ticket, error)
	UpdateTicket(ctx context.Context, ticket Ticket) (Ticket, error)
	ListTickets(ctx context.Context, roundID string, limit int) ([]Ticket, error)
	ListTicketsByAccount(ctx context.Context, accountID string, limit int) ([]Ticket, error)
	ListTicketsByRoundAndAccount(ctx context.Context, roundID, accountID string) ([]Ticket, error)

	// Config operations
	CreateConfig(ctx context.Context, config LotteryConfig) (LotteryConfig, error)
	GetConfig(ctx context.Context, configID string) (LotteryConfig, error)
	GetConfigByAccount(ctx context.Context, accountID string) (LotteryConfig, error)
	UpdateConfig(ctx context.Context, config LotteryConfig) (LotteryConfig, error)

	// Stats operations
	GetStats(ctx context.Context, accountID string) (LotteryStats, error)

	// Winner operations
	GetWinnersByRound(ctx context.Context, roundID string) ([]Winner, error)
}


// VRFService provides verifiable random number generation.
type VRFService interface {
	// CreateRequest creates a VRF randomness request.
	CreateRequest(ctx context.Context, accountID, keyID, consumer, seed string, metadata map[string]string) (VRFRequest, error)
	// GetRequest retrieves a VRF request by ID.
	GetRequest(ctx context.Context, accountID, requestID string) (VRFRequest, error)
}

// VRFRequest represents a VRF randomness request (simplified).
type VRFRequest struct {
	ID        string
	AccountID string
	KeyID     string
	Consumer  string
	Seed      string
	Status    string
	Result    string
	Error     string
}

// AutomationService provides scheduled job execution.
type AutomationService interface {
	// CreateJob creates an automation job.
	CreateJob(ctx context.Context, accountID, functionID, name, schedule, description string) (AutomationJob, error)
	// GetJob retrieves an automation job by ID.
	GetJob(ctx context.Context, jobID string) (AutomationJob, error)
	// SetEnabled enables or disables a job.
	SetEnabled(ctx context.Context, jobID string, enabled bool) (AutomationJob, error)
}

// AutomationJob represents an automation job (simplified).
type AutomationJob struct {
	ID         string
	AccountID  string
	FunctionID string
	Name       string
	Schedule   string
	Enabled    bool
}
