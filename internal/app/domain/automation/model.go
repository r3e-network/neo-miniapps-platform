package automation

import "time"

// JobStatus represents the lifecycle state of an automation job.
// Maps to contract status: 0=active, 1=completed, 2=paused.
type JobStatus string

const (
	JobStatusActive    JobStatus = "active"    // Contract: 0
	JobStatusCompleted JobStatus = "completed" // Contract: 1 (MaxRuns reached)
	JobStatusPaused    JobStatus = "paused"    // Contract: 2
)

// Job represents a scheduled automation task tied to an account and function.
// Aligned with AutomationScheduler.cs contract Job struct.
type Job struct {
	ID          string
	AccountID   string
	FunctionID  string // Maps to contract ServiceId
	Name        string
	Description string
	Schedule    string    // Maps to contract Spec (cron expression)
	Status      JobStatus // Maps to contract Status byte
	Enabled     bool      // Derived: Status == JobStatusActive
	RunCount    int       // Maps to contract Runs - current execution count
	MaxRuns     int       // Maps to contract MaxRuns - 0 means unlimited
	LastRun     time.Time
	NextRun     time.Time // Maps to contract NextRun (BigInteger timestamp)
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsCompleted returns true if the job has reached its max runs limit.
func (j Job) IsCompleted() bool {
	return j.MaxRuns > 0 && j.RunCount >= j.MaxRuns
}
