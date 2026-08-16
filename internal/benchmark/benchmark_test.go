package benchmark

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	"github.com/ittakestwo123/Harnesslab/internal/store"
)

func baseSpec() *spec.HarnessSpec {
	s, err := spec.Parse([]byte(spec.DefaultTemplate))
	if err != nil {
		panic(err)
	}
	return s
}

func TestMatrixNoDims(t *testing.T) {
	m := Matrix{}
	v, err := m.Variants(baseSpec())
	if err != nil {
		t.Fatalf("Variants: %v", err)
	}
	if len(v) != 1 || v[0].Name != "base" {
		t.Fatalf("variants = %+v, want [base]", v)
	}
}

func TestMatrixCartesian(t *testing.T) {
	m := Matrix{
		Planning:   []string{"none", "todo"},
		ToolsShell: []bool{true, false},
	}
	v, err := m.Variants(baseSpec())
	if err != nil {
		t.Fatalf("Variants: %v", err)
	}
	if len(v) != 4 {
		t.Fatalf("variants = %d, want 4", len(v))
	}
	names := map[string]bool{}
	for _, x := range v {
		names[x.Name] = true
	}
	for _, want := range []string{
		"planning=none+tools_shell=true",
		"planning=none+tools_shell=false",
		"planning=todo+tools_shell=true",
		"planning=todo+tools_shell=false",
	} {
		if !names[want] {
			t.Fatalf("missing variant %q in %v", want, names)
		}
	}
	// Overrides applied?
	for _, x := range v {
		if x.Name == "planning=todo+tools_shell=false" {
			if x.Spec.Planning.Strategy != "todo" || x.Spec.Tools.Shell {
				t.Fatalf("variant spec not applied: %+v", x.Spec)
			}
		}
	}
}

func TestMatrixVerificationNoneClearsCommands(t *testing.T) {
	m := Matrix{Verification: []string{"final", "none"}}
	v, err := m.Variants(baseSpec())
	if err != nil {
		t.Fatalf("Variants: %v", err)
	}
	for _, x := range v {
		if x.Name == "verification=none" && len(x.Spec.Verification.Commands) != 0 {
			t.Fatalf("verification=none should clear commands, got %v", x.Spec.Verification.Commands)
		}
		if x.Name == "verification=final" && len(x.Spec.Verification.Commands) != 2 {
			t.Fatalf("verification=final should keep commands, got %v", x.Spec.Verification.Commands)
		}
	}
}

func TestLoadTasksDir(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.yaml", "id: task-a\nrepo: https://example.com/x.git\nprompt: fix it\n")
	write("b.yaml", "id: task-b\nprompt: do it\nverification:\n  commands:\n    - go test ./...\n")
	write("skip.txt", "not a task")

	tasks, err := LoadTasks(dir)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(tasks))
	}
	if tasks[0].ID != "task-a" || tasks[0].Repo == "" {
		t.Fatalf("task-a = %+v", tasks[0])
	}
	if len(tasks[1].Verification.Commands) != 1 {
		t.Fatalf("task-b verification = %+v", tasks[1].Verification)
	}
}

func TestReportAggregation(t *testing.T) {
	r := &Report{}
	r.add(Outcome{
		Job:     &Job{Task: &Task{ID: "t1"}, Variant: Variant{Name: "base"}},
		RunID:   "run-1",
		Status:  store.StatusPassed,
		Metrics: store.Metrics{InputTokens: 100, OutputTokens: 50, ModelCalls: 2, ToolCalls: 1, DurationMS: 3000},
	})
	r.add(Outcome{
		Job:     &Job{Task: &Task{ID: "t2"}, Variant: Variant{Name: "base"}},
		RunID:   "run-2",
		Status:  store.StatusFailed,
		Metrics: store.Metrics{InputTokens: 200, OutputTokens: 60, ModelCalls: 3, ToolCalls: 2, DurationMS: 4000},
	})
	r.add(Outcome{
		Job:   &Job{Task: &Task{ID: "t3"}, Variant: Variant{Name: "planning=todo"}},
		Error: errors.New("build failed"),
	})

	if len(r.Variants) != 2 {
		t.Fatalf("variants = %d, want 2", len(r.Variants))
	}
	base := r.variant("base")
	if base.Total != 2 || base.Passed != 1 || base.Failed != 1 {
		t.Fatalf("base = %+v", base)
	}
	if base.InputTokens != 300 || base.OutputTokens != 110 || base.ModelCalls != 5 || base.ToolCalls != 3 {
		t.Fatalf("base aggregates = %+v", base)
	}
	todo := r.variant("planning=todo")
	if todo.Total != 1 || todo.Errored != 1 {
		t.Fatalf("todo = %+v", todo)
	}
}

func TestRenderTable(t *testing.T) {
	r := &Report{ID: "bench-1", StartedAt: time.Now()}
	r.add(Outcome{Job: &Job{Task: &Task{ID: "t"}, Variant: Variant{Name: "base"}},
		Status: store.StatusPassed, Metrics: store.Metrics{InputTokens: 12345, DurationMS: 1000}})
	table := r.RenderTable()
	if len(table) == 0 {
		t.Fatal("empty table")
	}
	if len(r.Variants) != 1 {
		t.Fatalf("variants = %d", len(r.Variants))
	}
}
