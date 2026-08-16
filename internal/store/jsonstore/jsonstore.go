// Package jsonstore implements store.Store with one JSON file per run.
// It is the Stage-1 storage backend; a SQLite backend can replace it without
// touching callers.
package jsonstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ittakestwo123/Harnesslab/internal/store"
)

// Store persists runs as <dir>/<run-id>.json.
type Store struct {
	dir string
}

// New creates a JSON store rooted at dir.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("jsonstore: mkdir %s: %w", dir, err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// CreateRun writes a new run file.
func (s *Store) CreateRun(ctx context.Context, run *store.Run) error {
	return s.write(run)
}

// UpdateRun overwrites the run file.
func (s *Store) UpdateRun(ctx context.Context, run *store.Run) error {
	return s.write(run)
}

// GetRun reads one run.
func (s *Store) GetRun(ctx context.Context, id string) (*store.Run, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, fmt.Errorf("jsonstore: read run %s: %w", id, err)
	}
	var run store.Run
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("jsonstore: decode run %s: %w", id, err)
	}
	return &run, nil
}

// ListRuns returns all runs ordered by start time (newest first).
func (s *Store) ListRuns(ctx context.Context) ([]*store.Run, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("jsonstore: list: %w", err)
	}
	var runs []*store.Run
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		run, err := s.GetRun(ctx, id)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].StartedAt.After(runs[j].StartedAt) })
	return runs, nil
}

func (s *Store) write(run *store.Run) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("jsonstore: encode run: %w", err)
	}
	if err := os.WriteFile(s.path(run.ID), data, 0o644); err != nil {
		return fmt.Errorf("jsonstore: write run %s: %w", run.ID, err)
	}
	return nil
}
