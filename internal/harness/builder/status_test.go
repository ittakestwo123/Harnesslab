package builder

import (
	"testing"

	"github.com/ittakestwo123/Harnesslab/internal/store"
)

func TestComputeStatus(t *testing.T) {
	passed := &store.VerificationResult{Passed: true}
	failed := &store.VerificationResult{Passed: false}

	cases := []struct {
		name                string
		agentErrored        bool
		ver                 *store.VerificationResult
		workspaceChanged    bool
		requireVerification bool
		requireChange       bool
		cancelled           bool
		want                store.Status
	}{
		{"all good", false, passed, true, true, true, false, store.StatusPassed},
		{"agent errored", true, passed, true, true, true, false, store.StatusFailed},
		{"verification failed required", false, failed, true, true, false, false, store.StatusFailed},
		{"verification failed not required", false, failed, true, false, false, false, store.StatusPassed},
		{"no workspace change required", false, passed, false, true, true, false, store.StatusFailed},
		{"no workspace change not required", false, passed, false, true, false, false, store.StatusPassed},
		{"cancelled", false, passed, true, true, true, true, store.StatusCancelled},
		{"no verification configured", false, nil, true, true, true, false, store.StatusPassed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeStatus(tc.agentErrored, tc.ver, tc.workspaceChanged,
				tc.requireVerification, tc.requireChange, tc.cancelled)
			if got != tc.want {
				t.Fatalf("computeStatus = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestCountTests(t *testing.T) {
	out := `=== RUN   TestA
--- PASS: TestA (0.00s)
=== RUN   TestB
--- FAIL: TestB (0.01s)
=== RUN   TestC
--- FAIL: TestC (0.00s)
--- FAIL: TestC (0.00s)
PASS`
	p, f := countTests(out)
	if p != 1 || f != 3 {
		t.Fatalf("countTests = %d/%d, want 1/3", p, f)
	}
	p, f = countTests("no tests here")
	if p != 0 || f != 0 {
		t.Fatalf("countTests empty = %d/%d, want 0/0", p, f)
	}
}

func TestWorkspaceDiffChanged(t *testing.T) {
	// Covered via workspace.Diff.Changed in the workspace package; here we
	// only pin the builder's success-rule wiring through computeStatus.
	if computeStatus(false, &store.VerificationResult{Passed: true}, false, true, true, false) != store.StatusFailed {
		t.Fatal("require_workspace_change must fail an unchanged workspace")
	}
}
