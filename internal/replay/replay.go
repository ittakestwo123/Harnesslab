// Package replay implements the Stage-3 replay capability: it records tool
// calls and model calls of a live run and replays them without touching the
// external world. A canonicalizer normalizes non-deterministic input (paths,
// ordering) so recorded entries can be looked up by content hash.
package replay

import (
	"context"
	"encoding/json"
	"time"
)

// Kind identifies what an entry records.
type Kind string

const (
	// KindTool records a tool call (input = canonicalized args, output = result).
	KindTool Kind = "tool"
	// KindModel records a model call (input = canonicalized request, output = response).
	KindModel Kind = "model"
)

// Entry is one recorded replay unit.
type Entry struct {
	Kind      Kind            `json:"kind"`
	InputHash string          `json:"input_hash"`
	Input     json.RawMessage `json:"input"`
	Output    json.RawMessage `json:"output"`
	CreatedAt time.Time       `json:"created_at"`
}

// Store persists and looks up replay entries.
type Store interface {
	// Lookup returns the recorded output for (kind, hash), or nil, false when missing.
	Lookup(ctx context.Context, kind Kind, hash string) (json.RawMessage, bool, error)
	// Put stores an entry.
	Put(ctx context.Context, e Entry) error
}
