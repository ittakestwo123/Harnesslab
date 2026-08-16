// Package recorder persists normalized run events. Stage 1 writes JSONL
// traces; SQLite/ClickHouse/object-storage recorders can implement the same
// interface later.
package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/ittakestwo123/Harnesslab/internal/runtime"
)

// Recorder appends run events to a destination.
type Recorder interface {
	Record(ctx context.Context, ev runtime.RunEvent) error
	Flush(ctx context.Context) error
	Close() error
}

// JSONL writes one JSON object per line to a trace file.
type JSONL struct {
	mu  sync.Mutex
	f   *os.File
	enc *json.Encoder
}

// NewJSONL creates a JSONL recorder at path.
func NewJSONL(path string) (*JSONL, error) {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return nil, fmt.Errorf("recorder: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("recorder: open %s: %w", path, err)
	}
	return &JSONL{f: f, enc: json.NewEncoder(f)}, nil
}

// Record appends one event as a JSON line.
func (r *JSONL) Record(ctx context.Context, ev runtime.RunEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enc.Encode(ev)
}

// Flush syncs the underlying file.
func (r *JSONL) Flush(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.f.Sync()
}

// Close closes the underlying file.
func (r *JSONL) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.f.Close()
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
