package diff

import (
	"testing"

	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
)

// TestBuildStepsMultipleTools pins correct tool pairing when several tools
// are called in one trajectory.
func TestBuildStepsMultipleTools(t *testing.T) {
	events := []hlruntime.RunEvent{
		{Type: hlruntime.EventToolStart, Tool: &hlruntime.ToolEvent{Name: "ls", Arguments: `{"command":"ls"}`}},
		{Type: hlruntime.EventToolEnd, Tool: &hlruntime.ToolEvent{Result: "a.txt"}},
		{Type: hlruntime.EventToolStart, Tool: &hlruntime.ToolEvent{Name: "cat", Arguments: `{"command":"cat a.txt"}`}},
		{Type: hlruntime.EventToolEnd, Tool: &hlruntime.ToolEvent{Result: "hello"}},
	}
	steps := BuildSteps(events)
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2 (one per tool call): %+v", len(steps), steps)
	}
	if steps[0].Name != "ls" || steps[1].Name != "cat" {
		t.Fatalf("unexpected tool steps: %+v", steps)
	}
	if steps[1].Result != "hello" {
		t.Fatalf("second tool result should belong to cat, got %+v", steps[1])
	}
}