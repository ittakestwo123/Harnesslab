package sqlitestore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ittakestwo123/Harnesslab/internal/store"
)

func TestSQLiteStoreCRUD(t *testing.T) {
	path := filepath.Join(t.TempDir(), "harness.db")
	s, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()
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
	got.Metrics = store.Metrics{ModelCalls: 3, ToolCalls: 5, InputTokens: 100}
	if err := s.UpdateRun(ctx, got); err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}
	again, _ := s.GetRun(ctx, "run-1")
	if again.Status != store.StatusPassed || again.Metrics.ModelCalls != 3 || again.Metrics.InputTokens != 100 {
		t.Fatalf("after update: %+v", again)
	}

	runs, err := s.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != "run-1" {
		t.Fatalf("ListRuns = %+v", runs)
	}

	// Event sink.
	if err := s.AppendEvent(ctx, "run-1", "", "model_end", time.Now(), []byte(`{"x":1}`)); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
}
