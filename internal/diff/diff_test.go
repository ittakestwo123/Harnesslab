package diff

import (
	"testing"

	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
)

func modelStep(content string) Step {
	return Step{Type: StepModel, Name: "gpt-5", Content: content}
}

func toolStep(name, args string) Step {
	return Step{Type: StepTool, Name: name, Args: args, Result: "ok"}
}

func TestBuildSteps(t *testing.T) {
	events := []hlruntime.RunEvent{
		{Type: hlruntime.EventModelStart},
		{Type: hlruntime.EventModelEnd, Model: &hlruntime.ModelEvent{Model: "gpt-5", Content: "hi", TokensIn: 10, TokensOut: 2}},
		{Type: hlruntime.EventToolStart, Tool: &hlruntime.ToolEvent{Name: "exec_command", Arguments: `{"cmd":"ls"}`}},
		{Type: hlruntime.EventToolEnd, Tool: &hlruntime.ToolEvent{Result: "file.txt"}},
	}
	steps := BuildSteps(events)
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(steps))
	}
	if steps[0].Type != StepModel || steps[0].Content != "hi" {
		t.Fatalf("step0 = %+v", steps[0])
	}
	if steps[1].Type != StepTool || steps[1].Name != "exec_command" || steps[1].Result != "file.txt" {
		t.Fatalf("step1 = %+v", steps[1])
	}
}

func TestCompareIdentical(t *testing.T) {
	a := []Step{modelStep("hello"), toolStep("exec_command", `{"cmd":"ls"}`), modelStep("done")}
	b := []Step{modelStep("hello"), toolStep("exec_command", `{"cmd":"ls"}`), modelStep("done")}
	d := Compare(a, b)
	if !d.Identical || d.FirstDivergence != nil {
		t.Fatalf("identical = %v, divergence = %+v", d.Identical, d.FirstDivergence)
	}
	if len(d.Lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(d.Lines))
	}
}

func TestCompareSingleDivergence(t *testing.T) {
	a := []Step{modelStep("hello"), toolStep("exec_command", `{"cmd":"ls"}`), modelStep("done")}
	b := []Step{modelStep("hello"), toolStep("exec_command", `{"cmd":"dir"}`), modelStep("done")}
	d := Compare(a, b)
	if d.Identical || d.FirstDivergence == nil {
		t.Fatalf("identical = %v, divergence = %+v", d.Identical, d.FirstDivergence)
	}
	if d.FirstDivergence.StepA != 2 || d.FirstDivergence.StepB != 2 {
		t.Fatalf("divergence at A%d/B%d, want 2/2", d.FirstDivergence.StepA, d.FirstDivergence.StepB)
	}
}

func TestCompareInsertion(t *testing.T) {
	// Run B inserted an extra tool call before the final model step.
	a := []Step{modelStep("start"), toolStep("exec_command", `{"cmd":"ls"}`), modelStep("done")}
	b := []Step{modelStep("start"), toolStep("exec_command", `{"cmd":"ls"}`), toolStep("exec_command", `{"cmd":"dir"}`), modelStep("done")}
	d := Compare(a, b)
	if d.FirstDivergence == nil {
		t.Fatal("expected divergence")
	}
	if d.FirstDivergence.StepB != 3 || d.FirstDivergence.StepA != 3 {
		t.Fatalf("divergence at A%d/B%d, want 3/3", d.FirstDivergence.StepA, d.FirstDivergence.StepB)
	}
}

func TestCompareModelContentDivergence(t *testing.T) {
	a := []Step{modelStep("answer is 4"), modelStep("final")}
	b := []Step{modelStep("answer is 5"), modelStep("final")}
	d := Compare(a, b)
	if d.FirstDivergence == nil || d.FirstDivergence.StepA != 1 {
		t.Fatalf("divergence = %+v, want at step 1", d.FirstDivergence)
	}
}
