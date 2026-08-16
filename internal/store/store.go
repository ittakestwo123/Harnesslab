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
	// VerificationPassed reports whether the verification commands passed.
	VerificationPassed bool
	// WorkspaceChanged reports whether the agent modified the workspace.
	WorkspaceChanged bool
}

// CommandResult is the outcome of one verification command.
type CommandResult struct {
	Command  string `json:"command"`
	Passed   bool   `json:"passed"`
	ExitCode int    `json:"exit_code,omitempty"`
	// Output is the clipped command output (stdout+stderr).
	Output string `json:"output,omitempty"`
}

// VerificationResult captures the post-run verification outcome.
type VerificationResult struct {
	// Passed is true when all verification commands passed.
	Passed bool `json:"passed"`
	// Commands lists the per-command results.
	Commands []CommandResult `json:"commands,omitempty"`
	// WorkspaceChanged reports whether the agent modified the workspace.
	WorkspaceChanged bool `json:"workspace_changed"`
	// TestsPassed/Failed are best-effort counts parsed from test output.
	TestsPassed int   `json:"tests_passed,omitempty"`
	TestsFailed int   `json:"tests_failed,omitempty"`
	DurationMS  int64 `json:"duration_ms,omitempty"`
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
	// Verification is the structured post-run verification outcome.
	Verification VerificationResult `json:"verification,omitempty"`
	// Environment is the JSON-encoded toolchain environment the run was
	// produced with (used for strict env validation on reproduction).
	Environment string `json:"environment,omitempty"`
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
