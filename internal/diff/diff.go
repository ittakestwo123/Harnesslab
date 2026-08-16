// Package diff implements Stage-4 trajectory comparison: it converts two
// runs' event streams into comparable steps, aligns them, and locates the
// first divergence — the earliest point where the two trajectories differ.
package diff

import (
	"crypto/sha256"
	"encoding/hex"

	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
)

// StepType enumerates trajectory step kinds.
type StepType string

const (
	StepModel StepType = "model"
	StepTool  StepType = "tool"
	StepError StepType = "error"
)

// Step is one comparable trajectory step.
type Step struct {
	Type      StepType
	Name      string // tool name or model name
	Args      string // tool arguments (JSON)
	Result    string // tool result snippet
	Content   string // model content snippet
	TokensIn  int
	TokensOut int

	key string
}

func (s *Step) keyOf() string {
	if s.key != "" {
		return s.key
	}
	switch s.Type {
	case StepTool:
		s.key = "tool:" + s.Name + ":" + hashText(s.Args)
	case StepModel:
		s.key = "model:" + hashText(s.Content)
	default:
		s.key = "error:" + hashText(s.Result)
	}
	return s.key
}

func hashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// Line is one aligned position of the two trajectories.
type Line struct {
	StepA int // 1-based index in A, 0 when absent
	StepB int // 1-based index in B, 0 when absent
	A     *Step
	B     *Step
	// Divergence marks the first position where the trajectories differ.
	Divergence bool
}

// Divergence describes the first place where the trajectories differ.
type Divergence struct {
	StepA, StepB int // 1-based step indices (0 = the run ended here)
	A, B         *Step
}

// Diff is the result of comparing two trajectories.
type Diff struct {
	StepsA, StepsB  []Step
	Lines           []Line
	FirstDivergence *Divergence
	Identical       bool
	DivergenceStepA int // 1-based step in A at divergence (0 = none)
	DivergenceStepB int
}

// BuildSteps converts a normalized event stream into comparable steps.
// model_end events become model steps; tool_start+tool_end pairs become
// tool steps; error events become error steps.
func BuildSteps(events []hlruntime.RunEvent) []Step {
	var steps []Step
	var pendingTool *Step
	for _, ev := range events {
		switch ev.Type {
		case hlruntime.EventModelEnd:
			s := Step{Type: StepModel}
			if ev.Model != nil {
				s.Name = ev.Model.Model
				s.Content = ev.Model.Content
				s.TokensIn = ev.Model.TokensIn
				s.TokensOut = ev.Model.TokensOut
			}
			steps = append(steps, s)
		case hlruntime.EventToolStart:
			// BUG(seed): the pending tool step is never captured, so
			// tool_end events produce standalone steps without a name.
		case hlruntime.EventToolEnd:
			if pendingTool == nil {
				s := Step{Type: StepTool}
				if ev.Tool != nil {
					s.Name = ev.Tool.Name
					s.Result = ev.Tool.Result
				}
				steps = append(steps, s)
				continue
			}
			if ev.Tool != nil {
				pendingTool.Result = ev.Tool.Result
			}
			steps = append(steps, *pendingTool)
			// BUG(seed): pendingTool is not reset, so a second tool_end
			// appends the first tool's step again.
		case hlruntime.EventError:
			s := Step{Type: StepError}
			if ev.Error != nil {
				s.Result = ev.Error.Message
			}
			steps = append(steps, s)
		}
	}
	return steps
}

// Compare aligns two step sequences and locates the first divergence.
func Compare(a, b []Step) *Diff {
	d := &Diff{StepsA: a, StepsB: b}

	// LCS table over step keys.
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1].keyOf() == b[j-1].keyOf() {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	markDivergence := func(stepA int, aStep *Step, stepB int, bStep *Step) {
		if d.FirstDivergence != nil {
			return
		}
		d.FirstDivergence = &Divergence{StepA: stepA, StepB: stepB, A: aStep, B: bStep}
		d.DivergenceStepA = stepA
		d.DivergenceStepB = stepB
	}

	// Walk the table from the front; the first frontier where keys differ
	// (or one side ends) is the first divergence.
	i, j := 0, 0
	for i < n && j < m {
		if a[i].keyOf() == b[j].keyOf() {
			d.Lines = append(d.Lines, Line{StepA: i + 1, StepB: j + 1, A: &a[i], B: &b[j]})
			i++
			j++
			continue
		}
		// Divergence: advance the side that the LCS prefers.
		if dp[i+1][j] >= dp[i][j+1] {
			d.Lines = append(d.Lines, Line{StepA: i + 1, StepB: j + 1, A: &a[i], B: &b[j], Divergence: true})
			markDivergence(i+1, &a[i], j+1, &b[j])
			i++
		} else {
			d.Lines = append(d.Lines, Line{StepA: i + 1, StepB: j + 1, A: &a[i], B: &b[j], Divergence: true})
			markDivergence(i+1, &a[i], j+1, &b[j])
			j++
		}
		break
	}

	// Common prefix fully matched; trailing steps on either side are the
	// divergence (one run did more work).
	if d.FirstDivergence == nil {
		first := true
		for ; i < n; i++ {
			d.Lines = append(d.Lines, Line{StepA: i + 1, A: &a[i], Divergence: first})
			if first {
				markDivergence(i+1, &a[i], j+1, nil)
			}
			first = false
		}
		for ; j < m; j++ {
			d.Lines = append(d.Lines, Line{StepB: j + 1, B: &b[j], Divergence: first})
			if first {
				markDivergence(i+1, nil, j+1, &b[j])
			}
			first = false
		}
		if i == n && j == m && len(a) == len(b) {
			d.Identical = true
		}
	}
	return d
}
