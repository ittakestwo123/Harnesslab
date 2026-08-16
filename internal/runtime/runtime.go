// Package runtime defines the runtime-agnostic interface between HarnessLab
// and an agent runtime (tRPC-Agent-Go for now, Codex/Claude/OpenCode later).
// HarnessLab core must never depend on a specific runtime's internal types;
// the TRPC adapter implements this interface.
package runtime

import (
	"context"
	"time"

	"github.com/ittakestwo123/Harnesslab/internal/replay"
)

// Runtime executes an agent task and emits a normalized stream of RunEvents.
type Runtime interface {
	// Run starts a single agent run. The returned channel is closed when the
	// run finishes (or the context is cancelled). The first event is always
	// run_start and the last event is always run_end.
	Run(ctx context.Context, req RunRequest) (<-chan RunEvent, error)
}

// RunRequest describes one agent run.
type RunRequest struct {
	// RunID identifies the run within HarnessLab.
	RunID string

	// Task is the user prompt / task description.
	Task string

	// UserID is the harness user owning the run.
	UserID string

	// SessionID isolates session state; HarnessLab uses one session per run.
	SessionID string

	// WorkspaceRoot is the directory the agent operates in (may be empty).
	WorkspaceRoot string

	// Replay, when non-nil, enables tool/model replay for this run.
	Replay *ReplayConfig
}

// ReplayMode controls how replay treats recorded vs missing entries.
type ReplayMode string

const (
	// ReplayRecord records tool/model calls into the replay store.
	ReplayRecord ReplayMode = "record"
	// ReplayStrict replays recorded calls and fails on a miss (offline).
	ReplayStrict ReplayMode = "strict"
	// ReplayFallback replays recorded calls and falls back to live calls on a miss.
	ReplayFallback ReplayMode = "fallback"
)

// ReplayConfig configures replay for one run.
type ReplayConfig struct {
	// Mode selects record/strict/fallback behavior.
	Mode ReplayMode
	// Store is the replay store consulted and/or written.
	Store replay.Store
	// Canonicalizer normalizes inputs before hashing.
	Canonicalizer *replay.Canonicalizer
	// ReplayModel enables model-call replay (in addition to tool replay).
	ReplayModel bool
}

// EventType enumerates normalized run events.
type EventType string

const (
	EventRunStart   EventType = "run_start"
	EventRunEnd     EventType = "run_end"
	EventModelStart EventType = "model_start"
	EventModelEnd   EventType = "model_end"
	EventToolStart  EventType = "tool_start"
	EventToolEnd    EventType = "tool_end"
	EventError      EventType = "error"
)

// ModelEvent carries model-call details for a model_start/model_end event.
type ModelEvent struct {
	Model     string `json:"model,omitempty"`
	TokensIn  int    `json:"tokens_in,omitempty"`
	TokensOut int    `json:"tokens_out,omitempty"`
	Content   string `json:"content,omitempty"`
}

// ToolEvent carries tool-call details for a tool_start/tool_end event.
type ToolEvent struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
	Result    string `json:"result,omitempty"`
	ExitCode  *int   `json:"exit_code,omitempty"`
}

// ErrorEvent carries failure details for an error event.
type ErrorEvent struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
}

// RunEvent is one normalized event in a run's trajectory.
type RunEvent struct {
	ID         string      `json:"id"`
	RunID      string      `json:"run_id"`
	ParentID   string      `json:"parent_id,omitempty"`
	Type       EventType   `json:"type"`
	Timestamp  time.Time   `json:"timestamp"`
	DurationMS int64       `json:"duration_ms,omitempty"`
	Step       int         `json:"step,omitempty"`
	Model      *ModelEvent `json:"model,omitempty"`
	Tool       *ToolEvent  `json:"tool,omitempty"`
	Error      *ErrorEvent `json:"error,omitempty"`
}
