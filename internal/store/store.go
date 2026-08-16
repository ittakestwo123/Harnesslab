// Package store defines the Run Store — the durable record of every harness
// run. Stage 1 ships a JSON-file implementation; SQLite/Postgres come later.
package store

import (
	"context"
	"time"
)

// Status is the terminal or interim state of a run.
type Status string

const (
	StatusRunning   Status = "running"
	StatusPassed    Status = "passed"
	StatusFailed    Status = "failed"
	StatusError     Status = "error"
	StatusCancelled Status = "cancelled"
)

// Metrics aggregates the observable cost of a run.
type Metrics struct {
	Success      bool
	InputTokens  int64
	OutputTokens int64
	ToolCalls    int
	ModelCalls   int
	CostUSD      float64
	DurationMS   int64
}

// Run is the basic entity of HarnessLab.
type Run struct {
	ID             string    `json:"id"`
	Task           string    `json:"task"`
	HarnessVersion string    `json:"harness_version"`
	HarnessName    string    `json:"harness_name"`
	Repository     string    `json:"repository,omitempty"`
	Commit         string    `json:"commit,omitempty"`
	Workspace      string    `json:"workspace,omitempty"`
	TracePath      string    `json:"trace_path,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at,omitempty"`
	Status         Status    `json:"status"`
	Metrics        Metrics   `json:"metrics"`
	// SpecYAML is the harness spec this run executed, persisted for
	// reproduction/export fidelity.
	SpecYAML string `json:"spec_yaml,omitempty"`
	// WorkspacePatch is the working-tree diff produced by the run.
	WorkspacePatch string `json:"workspace_patch,omitempty"`
}

// Store persists runs.
type Store interface {
	CreateRun(ctx context.Context, run *Run) error
	UpdateRun(ctx context.Context, run *Run) error
	GetRun(ctx context.Context, id string) (*Run, error)
	ListRuns(ctx context.Context) ([]*Run, error)
}

// EventSink is implemented by stores that additionally persist raw run events
// (payload is the JSON-encoded normalized event).
type EventSink interface {
	AppendEvent(ctx context.Context, runID, parentID, evType string, ts time.Time, payload []byte) error
}
