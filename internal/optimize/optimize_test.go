package optimize

import (
	"testing"

	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
	"github.com/ittakestwo123/Harnesslab/internal/store"
)

func TestAnalyzePatterns(t *testing.T) {
	runs := []*store.Run{
		{ID: "r1", Metrics: store.Metrics{ToolCalls: 0}},
		{ID: "r2", Metrics: store.Metrics{ToolCalls: 3, InputTokens: 20000}},
		{ID: "r3", Metrics: store.Metrics{ToolCalls: 2}},
	}
	traces := map[string][]hlruntime.RunEvent{
		"r2": {}, // high tokens detected from metrics
		"r3": {
			{Type: hlruntime.EventToolStart, Tool: &hlruntime.ToolEvent{Name: "exec_command", Arguments: `{"command":"go test"}`}},
			{Type: hlruntime.EventToolEnd, Tool: &hlruntime.ToolEvent{Result: `{"exit_code":1}`}},
			{Type: hlruntime.EventToolStart, Tool: &hlruntime.ToolEvent{Name: "exec_command", Arguments: `{"command":"go test"}`}},
			{Type: hlruntime.EventToolEnd, Tool: &hlruntime.ToolEvent{Result: `{"exit_code":1}`}},
			{Type: hlruntime.EventError, Error: &hlruntime.ErrorEvent{Message: "boom"}},
		},
	}
	a := Analyze(runs, traces)
	got := map[string]int{}
	for _, p := range a.Patterns {
		got[p.ID] = p.Count
	}
	if got["no_tools"] != 1 {
		t.Errorf("no_tools = %d, want 1", got["no_tools"])
	}
	if got["high_tokens"] != 1 {
		t.Errorf("high_tokens = %d, want 1", got["high_tokens"])
	}
	if got["repeated_tool"] != 1 {
		t.Errorf("repeated_tool = %d, want 1", got["repeated_tool"])
	}
	if got["tool_failure"] != 1 {
		t.Errorf("tool_failure = %d, want 1", got["tool_failure"])
	}
	if got["trajectory_error"] != 1 {
		t.Errorf("trajectory_error = %d, want 1", got["trajectory_error"])
	}
	if len(a.Candidates) == 0 {
		t.Error("expected candidates")
	}
}

func TestParetoFront(t *testing.T) {
	points := []Point{
		{Variant: "a", Pass: 1.0, Tokens: 1000, DurationMS: 100},
		{Variant: "b", Pass: 0.5, Tokens: 500, DurationMS: 50},   // dominated by a? pass lower, tokens lower... not dominated by a (a has higher tokens). dominated by none?
		{Variant: "c", Pass: 1.0, Tokens: 2000, DurationMS: 200}, // dominated by a
		{Variant: "d", Pass: 1.0, Tokens: 800, DurationMS: 90},   // dominated by a? a: tokens 1000 > 800 鈫?a does NOT dominate d; d: tokens 800 < 1000, pass equal, dur 90 < 100 鈫?d dominates a!
	}
	front := Front(points)
	names := map[string]bool{}
	for _, p := range front {
		names[p.Variant] = true
	}
	// a is dominated by d (same pass, lower tokens and duration); c is
	// dominated by both a and d. b survives because no point matches its
	// pass while being cheaper.
	if names["a"] {
		t.Error("a should be dominated by d")
	}
	if names["c"] {
		t.Error("c should be dominated")
	}
	if !names["b"] {
		t.Error("b should be on the front")
	}
	if !names["d"] {
		t.Error("d should be on the front")
	}
	if len(front) != 2 {
		t.Fatalf("front = %v, want 2 points", front)
	}
}
