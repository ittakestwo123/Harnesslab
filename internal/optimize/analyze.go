// Package optimize implements Phase-4 harness optimization: trajectory
// failure analysis, rule-based candidate generation, and multi-objective
// Pareto front computation over benchmark variants.
package optimize

import (
	"sort"
	"strings"

	"github.com/ittakestwo123/Harnesslab/internal/diff"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
	"github.com/ittakestwo123/Harnesslab/internal/store"
)

// highTokenThreshold flags runs whose input tokens suggest context explosion.
const highTokenThreshold = 10000

// Pattern is one detected failure pattern and how often it occurred.
type Pattern struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Count int      `json:"count"`
	Runs  []string `json:"runs,omitempty"`
}

// Analysis is the aggregate failure analysis of a set of runs.
type Analysis struct {
	Runs       int       `json:"runs"`
	Patterns   []Pattern `json:"patterns"`
	Candidates []string  `json:"candidates"`
}

// Analyze inspects runs and their traces for known failure patterns and maps
// them to harness candidate changes.
func Analyze(runs []*store.Run, traces map[string][]hlruntime.RunEvent) *Analysis {
	a := &Analysis{Runs: len(runs)}
	seen := map[string]*Pattern{}

	note := func(id, label string, runID string) {
		p, ok := seen[id]
		if !ok {
			p = &Pattern{ID: id, Label: label}
			seen[id] = p
		}
		p.Count++
		p.Runs = append(p.Runs, runID)
	}

	for _, r := range runs {
		events := traces[r.ID]
		steps := diff.BuildSteps(events)

		if r.Metrics.ToolCalls == 0 {
			note("no_tools", "no tool calls — agent answered from knowledge", r.ID)
		}
		if r.Metrics.InputTokens > highTokenThreshold {
			note("high_tokens", "input tokens above threshold (context explosion)", r.ID)
		}
		if r.Repository != "" && !r.Metrics.WorkspaceChanged {
			note("no_change", "repository task without workspace changes — likely hallucinated output", r.ID)
		}
		if hasRepeatedTool(steps) {
			note("repeated_tool", "identical tool call repeated in the same trajectory", r.ID)
		}
		if hasToolFailure(steps) {
			note("tool_failure", "tool result reports a failing command (exit_code 1)", r.ID)
		}
		if hasErrorStep(steps) {
			note("trajectory_error", "trajectory contains an error step", r.ID)
		}
	}

	for _, p := range seen {
		a.Patterns = append(a.Patterns, *p)
	}
	sort.Slice(a.Patterns, func(i, j int) bool { return a.Patterns[i].Count > a.Patterns[j].Count })

	for _, p := range a.Patterns {
		if c := candidateFor(p.ID); c != "" {
			a.Candidates = append(a.Candidates, c)
		}
	}
	return a
}

func hasRepeatedTool(steps []diff.Step) bool {
	counts := map[string]int{}
	for _, s := range steps {
		if s.Type != diff.StepTool {
			continue
		}
		key := s.Name + "\x00" + s.Args
		counts[key]++
		if counts[key] >= 2 {
			return true
		}
	}
	return false
}

func hasToolFailure(steps []diff.Step) bool {
	for _, s := range steps {
		if s.Type == diff.StepTool && strings.Contains(s.Result, `"exit_code":1`) {
			return true
		}
	}
	return false
}

func hasErrorStep(steps []diff.Step) bool {
	for _, s := range steps {
		if s.Type == diff.StepError {
			return true
		}
	}
	return false
}

func candidateFor(id string) string {
	switch id {
	case "no_tools":
		return "Consider adding shell/filesystem tools or stronger tool-use instructions if the task requires repository access."
	case "no_change":
		return "Require workspace changes (success.require_workspace_change) so text-only hallucinated answers fail the run."
	case "repeated_tool":
		return "Add a duplicate-command guard (skip identical tool calls with unchanged args) to the harness."
	case "high_tokens":
		return "Enable summary/adaptive context or reduce injected context to contain token usage."
	case "tool_failure":
		return "Add verification-after-edit guidance or a command retry policy for failing commands."
	case "trajectory_error":
		return "Increase retry limits (spec.retry) or add a fallback model (model failover)."
	default:
		return ""
	}
}
