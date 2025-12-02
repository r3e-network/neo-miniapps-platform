// Package lottery provides a Mega Millions style lottery service.
package lottery

import "time"

// RoundStatus represents the lifecycle state of a lottery round.
type RoundStatus string

const (
	RoundStatusActive    RoundStatus = "active"
	RoundStatusDrawing   RoundStatus = "drawing"
	RoundStatusCompleted RoundStatus = "completed"
	RoundStatusCancelled RoundStatus = "cancelled"
)

// Round represents a lottery drawing round.
type Round struct {
	ID             string            `json:"id"`
	AccountID      string            `json:"account_id"`       // Lottery operator account
	RoundNumber    int64             `json:"round_number"`     // Sequential round number
	Status         RoundStatus       `json:"status"`           // Current round status
	PrizePool      int64             `json:"prize_pool"`       // Total prize pool in smallest unit
	TicketCount    int64             `json:"ticket_count"`     // Number of tickets sold
	TicketPrice    int64             `json:"ticket_price"`     // Price per ticket
	WinningNumbers []int             `json:"winning_numbers"`  // 5 main winning numbers (1-70)
	MegaNumber     int               `json:"mega_number"`      // Mega ball number (1-25)
	VRFRequestID   string            `json:"vrf_request_id"`   // VRF request for randomness
	AutomationJob  string            `json:"automation_job"`   // Automation job ID for draws
	DrawSchedule   string            `json:"draw_schedule"`    // Cron schedule for draws
	StartedAt      time.Time         `json:"started_at"`       // Round start time
	DrawAt         time.Time         `json:"draw_at"`          // Scheduled draw time
	CompletedAt    time.Time         `json:"completed_at"`     // Round completion time
	JackpotWinners int               `json:"jackpot_winners"`  // Count of jackpot winners
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// Ticket represents a lottery ticket purchase.
type Ticket struct {
	ID           string            `json:"id"`
	AccountID    string            `json:"account_id"`    // Ticket owner
	RoundID      string            `json:"round_id"`      // Associated round
	Numbers      []int             `json:"numbers"`       // 5 chosen numbers (1-70)
	MegaNumber   int               `json:"mega_number"`   // Chosen mega number (1-25)
	QuickPick    bool              `json:"quick_pick"`    // Was this auto-generated?
	PurchaseTx   string            `json:"purchase_tx"`   // On-chain transaction hash
	Claimed      bool              `json:"claimed"`       // Has prize been claimed?
	PrizeTier    int               `json:"prize_tier"`    // Winning tier (0=no win, 1=jackpot, etc.)
	PrizeAmount  int64             `json:"prize_amount"`  // Prize amount won
	ClaimTx      string            `json:"claim_tx"`      // Prize claim transaction hash
	Metadata     map[string]string `json:"metadata,omitempty"`
	PurchasedAt  time.Time         `json:"purchased_at"`
	ClaimedAt    time.Time         `json:"claimed_at"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// PrizeTier defines the prize tiers and their match requirements.
type PrizeTier struct {
	Tier           int    `json:"tier"`
	Name           string `json:"name"`
	MainMatches    int    `json:"main_matches"`    // Number of main numbers matched
	MegaMatch      bool   `json:"mega_match"`      // Whether mega number matched
	PrizeType      string `json:"prize_type"`      // "percentage" or "fixed"
	PrizeValue     int64  `json:"prize_value"`     // Percentage of pool or fixed amount
	EstimatedPrize int64  `json:"estimated_prize"` // Estimated prize amount
}

// LotteryConfig holds configuration for a lottery instance.
type LotteryConfig struct {
	ID                string            `json:"id"`
	AccountID         string            `json:"account_id"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	TicketPrice       int64             `json:"ticket_price"`        // Price per ticket
	MainNumberMin     int               `json:"main_number_min"`     // Min main number (1)
	MainNumberMax     int               `json:"main_number_max"`     // Max main number (70)
	MainNumberCount   int               `json:"main_number_count"`   // Count of main numbers (5)
	MegaNumberMin     int               `json:"mega_number_min"`     // Min mega number (1)
	MegaNumberMax     int               `json:"mega_number_max"`     // Max mega number (25)
	DrawSchedule      string            `json:"draw_schedule"`       // Cron expression
	MinTicketsForDraw int               `json:"min_tickets_for_draw"`
	VRFKeyID          string            `json:"vrf_key_id"`          // VRF key for randomness
	ContractHash      string            `json:"contract_hash"`       // On-chain contract
	Enabled           bool              `json:"enabled"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// DrawResult contains the result of a lottery draw.
type DrawResult struct {
	RoundID        string    `json:"round_id"`
	WinningNumbers []int     `json:"winning_numbers"`
	MegaNumber     int       `json:"mega_number"`
	VRFProof       string    `json:"vrf_proof"`
	VRFOutput      string    `json:"vrf_output"`
	TotalPrizePool int64     `json:"total_prize_pool"`
	Winners        []Winner  `json:"winners"`
	DrawnAt        time.Time `json:"drawn_at"`
}

// Winner represents a winning ticket.
type Winner struct {
	TicketID    string `json:"ticket_id"`
	AccountID   string `json:"account_id"`
	PrizeTier   int    `json:"prize_tier"`
	PrizeAmount int64  `json:"prize_amount"`
	MainMatches int    `json:"main_matches"`
	MegaMatch   bool   `json:"mega_match"`
}

// LotteryStats provides statistics for a lottery.
type LotteryStats struct {
	TotalRounds       int64 `json:"total_rounds"`
	TotalTicketsSold  int64 `json:"total_tickets_sold"`
	TotalPrizesPaid   int64 `json:"total_prizes_paid"`
	CurrentJackpot    int64 `json:"current_jackpot"`
	LargestJackpot    int64 `json:"largest_jackpot"`
	ActiveRoundID     string `json:"active_round_id"`
	NextDrawAt        time.Time `json:"next_draw_at"`
}

// Default prize tiers (Mega Millions style).
var DefaultPrizeTiers = []PrizeTier{
	{Tier: 1, Name: "Jackpot", MainMatches: 5, MegaMatch: true, PrizeType: "percentage", PrizeValue: 50},
	{Tier: 2, Name: "Match 5", MainMatches: 5, MegaMatch: false, PrizeType: "percentage", PrizeValue: 15},
	{Tier: 3, Name: "Match 4+Mega", MainMatches: 4, MegaMatch: true, PrizeType: "percentage", PrizeValue: 10},
	{Tier: 4, Name: "Match 4", MainMatches: 4, MegaMatch: false, PrizeType: "fixed", PrizeValue: 50_000_000},
	{Tier: 5, Name: "Match 3+Mega", MainMatches: 3, MegaMatch: true, PrizeType: "fixed", PrizeValue: 20_000_000},
	{Tier: 6, Name: "Match 3", MainMatches: 3, MegaMatch: false, PrizeType: "fixed", PrizeValue: 10_000_000},
	{Tier: 7, Name: "Match 2+Mega", MainMatches: 2, MegaMatch: true, PrizeType: "fixed", PrizeValue: 5_000_000},
	{Tier: 8, Name: "Match 1+Mega", MainMatches: 1, MegaMatch: true, PrizeType: "fixed", PrizeValue: 2_000_000},
	{Tier: 9, Name: "Match Mega", MainMatches: 0, MegaMatch: true, PrizeType: "fixed", PrizeValue: 1_000_000},
}

// Default configuration values.
const (
	DefaultTicketPrice       = 10_000_000 // 0.1 GAS
	DefaultMainNumberMin     = 1
	DefaultMainNumberMax     = 70
	DefaultMainNumberCount   = 5
	DefaultMegaNumberMin     = 1
	DefaultMegaNumberMax     = 25
	DefaultMinTicketsForDraw = 10
	DefaultDrawSchedule      = "0 0 * * 3,6" // Wednesday and Saturday at midnight
)
