package jsonstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ittakestwo123/Harnesslab/internal/store"
)

func TestStoreCRUD(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	run := &store.Run{
		ID:        "run-1",
		Task:      "fix bug",
		Status:    store.StatusRunning,
		StartedAt: time.Now(),
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := s.GetRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.ID != "run-1" || got.Status != store.StatusRunning {
		t.Fatalf("got %+v", got)
	}

	got.Status = store.StatusPassed
	got.Metrics = store.Metrics{ModelCalls: 3, ToolCalls: 5}
	if err := s.UpdateRun(ctx, got); err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}
	again, _ := s.GetRun(ctx, "run-1")
	if again.Status != store.StatusPassed || again.Metrics.ModelCalls != 3 {
		t.Fatalf("after update: %+v", again)
	}

	runs, err := s.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != "run-1" {
		t.Fatalf("ListRuns = %+v", runs)
	}
}
