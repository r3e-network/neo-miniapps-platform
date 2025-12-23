// Package neosimulation provides simulation service for automated transaction testing.
package neosimulation

import "time"

// StartSimulationRequest is the request to start simulation.
type StartSimulationRequest struct {
	MiniApps      []string `json:"mini_apps,omitempty"`       // Override configured MiniApps
	MinIntervalMS int      `json:"min_interval_ms,omitempty"` // Override min interval
	MaxIntervalMS int      `json:"max_interval_ms,omitempty"` // Override max interval
}

// StartSimulationResponse is the response for starting simulation.
type StartSimulationResponse struct {
	Success  bool     `json:"success"`
	Message  string   `json:"message,omitempty"`
	MiniApps []string `json:"mini_apps"`
	Running  bool     `json:"running"`
}

// StopSimulationRequest is the request to stop simulation.
type StopSimulationRequest struct{}

// StopSimulationResponse is the response for stopping simulation.
type StopSimulationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Running bool   `json:"running"`
}

// SimulationStatus represents the current simulation status.
type SimulationStatus struct {
	Running       bool              `json:"running"`
	MiniApps      []string          `json:"mini_apps"`
	MinIntervalMS int               `json:"min_interval_ms"`
	MaxIntervalMS int               `json:"max_interval_ms"`
	TxCounts      map[string]int64  `json:"tx_counts"`       // Per-app transaction counts
	LastTxTimes   map[string]string `json:"last_tx_times"`   // Per-app last transaction time
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	Uptime        string            `json:"uptime,omitempty"`
}

// SimulationTx represents a simulated transaction record.
type SimulationTx struct {
	ID             int64     `json:"id"`
	AppID          string    `json:"app_id"`
	AccountAddress string    `json:"account_address"`
	TxType         string    `json:"tx_type"`
	Amount         int64     `json:"amount,omitempty"`
	Status         string    `json:"status"`
	TxHash         string    `json:"tx_hash,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
