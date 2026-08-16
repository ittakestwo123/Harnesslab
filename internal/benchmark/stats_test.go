package benchmark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ittakestwo123/Harnesslab/internal/store"
)

func TestComputeStat(t *testing.T) {
	s := computeStat([]float64{1, 2, 3, 4, 5})
	if s.Count != 5 {
		t.Fatalf("count = %d, want 5", s.Count)
	}
	if s.Mean != 3 || s.Median != 3 || s.P50 != 3 {
		t.Fatalf("mean/median/p50 = %v/%v/%v, want 3/3/3", s.Mean, s.Median, s.P50)
	}
	// Sample stddev of {1,2,3,4,5} = sqrt(2.5) ~ 1.581
	if s.StdDev < 1.58 || s.StdDev > 1.59 {
		t.Fatalf("stddev = %v, want ~1.581", s.StdDev)
	}
	// Nearest-rank P90 of 5 values = index ceil(0.9*5)-1 = ceil(4.5)-1 = 4 -> value 5.
	if s.P90 != 5 {
		t.Fatalf("p90 = %v, want 5", s.P90)
	}
	// CI95 ~ 3 +/- 1.96*1.581/sqrt(5) = 3 +/- 1.386
	if s.CI95Lo > 1.62 || s.CI95Lo < 1.60 || s.CI95Hi > 4.40 || s.CI95Hi < 4.38 {
		t.Fatalf("ci95 = [%v, %v], want ~[1.614, 4.386]", s.CI95Lo, s.CI95Hi)
	}
}

func TestComputeStatEmpty(t *testing.T) {
	s := computeStat(nil)
	if s.Count != 0 || s.Mean != 0 {
		t.Fatalf("empty stat = %+v", s)
	}
}

func TestComputeStatPercentileNearestRank(t *testing.T) {
	// 4 values: P50 -> ceil(0.5*4)-1 = 1 -> sorted[1].
	s := computeStat([]float64{10, 20, 30, 40})
	if s.Median != 20 || s.P50 != 20 {
		t.Fatalf("median/p50 = %v/%v, want 20/20 (nearest-rank)", s.Median, s.P50)
	}
	if s.P90 != 40 {
		t.Fatalf("p90 = %v, want 40", s.P90)
	}
}

func TestStatsOfSkipsErroredRuns(t *testing.T) {
	runs := []RunSummary{
		{RunID: "run-1", InputTokens: 100, OutputTokens: 10, CostUSD: 0.01, DurationMS: 1000, Status: store.StatusPassed},
		{RunID: "run-2", InputTokens: 200, OutputTokens: 20, CostUSD: 0.02, DurationMS: 2000, Status: store.StatusFailed},
		{RunID: "", InputTokens: 0, CostUSD: 0, DurationMS: 0}, // scheduler error, no run
	}
	st := statsOf(runs)
	if st.InputTokens.Count != 2 {
		t.Fatalf("input count = %d, want 2 (errored run excluded)", st.InputTokens.Count)
	}
	if st.InputTokens.Mean != 150 {
		t.Fatalf("input mean = %v, want 150", st.InputTokens.Mean)
	}
	if st.CostUSD.Count != 2 || st.CostUSD.Mean != 0.015 {
		t.Fatalf("cost stats = %+v, want count 2 mean 0.015", st.CostUSD)
	}
}

func TestBuildByTask(t *testing.T) {
	runs := []RunSummary{
		{TaskID: "t1", RunID: "r1", Status: store.StatusPassed, InputTokens: 100, CostUSD: 0.01, DurationMS: 1000},
		{TaskID: "t1", RunID: "r2", Status: store.StatusFailed, InputTokens: 300, CostUSD: 0.03, DurationMS: 3000},
		{TaskID: "t2", RunID: "r3", Status: store.StatusPassed, InputTokens: 50, CostUSD: 0.005, DurationMS: 500},
	}
	by := buildByTask(runs)
	if len(by) != 2 {
		t.Fatalf("byTask = %d, want 2", len(by))
	}
	if by[0].TaskID != "t1" || by[0].Total != 2 || by[0].Passed != 1 {
		t.Fatalf("t1 = %+v", by[0])
	}
	if by[0].Stats.InputTokens.Mean != 200 {
		t.Fatalf("t1 input mean = %v, want 200", by[0].Stats.InputTokens.Mean)
	}
	if by[1].TaskID != "t2" || by[1].Total != 1 || by[1].Passed != 1 {
		t.Fatalf("t2 = %+v", by[1])
	}
}

func TestReportFinalizeComputesStats(t *testing.T) {
	r := &Report{}
	for i := 0; i < 3; i++ {
		r.add(Outcome{
			Job:     &Job{Task: &Task{ID: "t1"}, Variant: Variant{Name: "base"}, Repeat: i},
			RunID:   "run-" + string(rune('a'+i)),
			Status:  store.StatusPassed,
			Metrics: store.Metrics{InputTokens: int64(100 * (i + 1)), OutputTokens: 10, CostUSD: 0.01 * float64(i+1), DurationMS: int64(1000 * (i + 1)), VerificationPassed: true, WorkspaceChanged: true},
		})
	}
	r.Finalize()
	v := r.Variants[0]
	if v.Stats.InputTokens.Count != 3 || v.Stats.InputTokens.Mean != 200 {
		t.Fatalf("variant stats = %+v", v.Stats)
	}
	if len(v.ByTask) != 1 || v.ByTask[0].Total != 3 || v.ByTask[0].Passed != 3 {
		t.Fatalf("byTask = %+v", v.ByTask)
	}
	// Repeat recorded per run.
	if v.Runs[2].Repeat != 2 {
		t.Fatalf("run repeat = %d, want 2", v.Runs[2].Repeat)
	}
	if v.CostUSD != 0.06 {
		t.Fatalf("variant cost = %v, want 0.06", v.CostUSD)
	}
	if table := r.RenderStats(); len(table) == 0 {
		t.Fatal("empty stats table")
	}
}

func TestLoadTasksRecursiveCategories(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "debugging")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, content string) {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "a.yaml"), "id: task-a\ncategory: basic\nset: dev\nprompt: fix it\n")
	write(filepath.Join(sub, "b.yaml"), "id: task-b\ncategory: debugging\nset: holdout\nprompt: debug it\n")

	tasks, err := LoadTasks(dir)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("tasks = %d, want 2 (recursive)", len(tasks))
	}
	if tasks[0].ID != "task-a" || tasks[0].Category != "basic" || tasks[0].Set != "dev" {
		t.Fatalf("task-a = %+v", tasks[0])
	}
	if tasks[1].ID != "task-b" || tasks[1].Category != "debugging" || tasks[1].Set != "holdout" {
		t.Fatalf("task-b = %+v", tasks[1])
	}

	dev := FilterBySet(tasks, "dev")
	if len(dev) != 1 || dev[0].ID != "task-a" {
		t.Fatalf("dev = %+v", dev)
	}
	holdout := FilterBySet(tasks, "holdout")
	if len(holdout) != 1 || holdout[0].ID != "task-b" {
		t.Fatalf("holdout = %+v", holdout)
	}
}

func TestTaskValidateCategoryAndSet(t *testing.T) {
	if err := (&Task{ID: "x", Prompt: "p", Category: "nope"}).Validate(); err == nil {
		t.Fatal("invalid category accepted")
	}
	if err := (&Task{ID: "x", Prompt: "p", Set: "nope"}).Validate(); err == nil {
		t.Fatal("invalid set accepted")
	}
	if err := (&Task{ID: "x", Prompt: "p", Category: "multi-file", Set: "dev"}).Validate(); err != nil {
		t.Fatalf("valid task rejected: %v", err)
	}
}

func TestLoadTaskSetAndFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dev.yaml")
	content := "id: dev\nname: dev set\ntasks:\n  - task-a\n  - task-c\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ts, err := LoadTaskSet(path)
	if err != nil {
		t.Fatalf("LoadTaskSet: %v", err)
	}
	if !ts.Contains("task-a") || ts.Contains("task-b") {
		t.Fatalf("taskset = %+v", ts)
	}
	tasks := []*Task{{ID: "task-a"}, {ID: "task-b"}, {ID: "task-c"}}
	filtered := FilterByTaskSet(tasks, ts)
	if len(filtered) != 2 || filtered[0].ID != "task-a" || filtered[1].ID != "task-c" {
		t.Fatalf("filtered = %+v", filtered)
	}
	if len(FilterByTaskSet(tasks, nil)) != 3 {
		t.Fatal("nil taskset should keep all")
	}
}
