package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ittakestwo123/Harnesslab/internal/store"
)

// RunSummary is one job's result within a variant group.
type RunSummary struct {
	RunID      string       `json:"run_id"`
	TaskID     string       `json:"task_id"`
	Variant    string       `json:"variant"`
	Status     store.Status `json:"status"`
	Tokens     int64        `json:"tokens"`
	ModelCalls int          `json:"model_calls"`
	ToolCalls  int          `json:"tool_calls"`
	DurationMS int64        `json:"duration_ms"`
	// VerificationPassed reports whether verification commands passed.
	VerificationPassed bool `json:"verification_passed"`
	// WorkspaceChanged reports whether the agent modified the workspace.
	WorkspaceChanged bool   `json:"workspace_changed"`
	Error            string `json:"error,omitempty"`
}

// VariantResult aggregates outcomes for one harness variant.
type VariantResult struct {
	Variant      string `json:"variant"`
	Total        int    `json:"total"`
	Passed       int    `json:"passed"`
	Failed       int    `json:"failed"`
	Errored      int    `json:"errored"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	ModelCalls   int    `json:"model_calls"`
	ToolCalls    int    `json:"tool_calls"`
	DurationMS   int64  `json:"duration_ms"`
	// VerPassed counts runs whose verification passed.
	VerPassed int `json:"verification_passed"`
	// ChgCount counts runs that modified the workspace.
	ChgCount int          `json:"workspace_changed"`
	Runs     []RunSummary `json:"runs"`
}

// Report is the aggregated result of a benchmark.
type Report struct {
	ID         string           `json:"id"`
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt time.Time        `json:"finished_at"`
	Variants   []*VariantResult `json:"variants"`
}

func newBenchID() string {
	return "bench-" + uuid.NewString()[:8]
}

// add folds one outcome into the report.
func (r *Report) add(o Outcome) {
	name := "base"
	if o.Job != nil {
		name = o.Job.Variant.Name
	}
	vr := r.variant(name)
	vr.Total++
	if o.Error != nil {
		vr.Errored++
	} else if o.Status == store.StatusPassed {
		vr.Passed++
	} else {
		vr.Failed++
	}
	vr.InputTokens += o.Metrics.InputTokens
	vr.OutputTokens += o.Metrics.OutputTokens
	vr.ModelCalls += o.Metrics.ModelCalls
	vr.ToolCalls += o.Metrics.ToolCalls
	vr.DurationMS += o.Metrics.DurationMS
	if o.Metrics.VerificationPassed {
		vr.VerPassed++
	}
	if o.Metrics.WorkspaceChanged {
		vr.ChgCount++
	}

	summary := RunSummary{
		Variant:            name,
		Status:             o.Status,
		Tokens:             o.Metrics.InputTokens + o.Metrics.OutputTokens,
		ModelCalls:         o.Metrics.ModelCalls,
		ToolCalls:          o.Metrics.ToolCalls,
		DurationMS:         o.Metrics.DurationMS,
		VerificationPassed: o.Metrics.VerificationPassed,
		WorkspaceChanged:   o.Metrics.WorkspaceChanged,
	}
	if o.Job != nil && o.Job.Task != nil {
		summary.TaskID = o.Job.Task.ID
	}
	if o.RunID != "" {
		summary.RunID = o.RunID
	}
	if o.Error != nil {
		summary.Error = o.Error.Error()
	}
	vr.Runs = append(vr.Runs, summary)
}

func (r *Report) variant(name string) *VariantResult {
	for _, v := range r.Variants {
		if v.Variant == name {
			return v
		}
	}
	v := &VariantResult{Variant: name}
	r.Variants = append(r.Variants, v)
	return v
}

// WriteJSON persists the report to path.
func (r *Report) WriteJSON(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("benchmark: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("benchmark: encode report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("benchmark: write report: %w", err)
	}
	return nil
}

// RenderTable prints the per-variant summary table. Ver and Chg columns show
// how many runs passed verification and modified the workspace — the two
// correctness signals that separate a real PASS from a hallucinated answer.
func (r *Report) RenderTable() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-32s %-8s %-5s %-4s %-10s %-6s %-5s %-10s\n",
		"Harness", "Pass", "Total", "Ver", "Tokens", "Models", "Chg", "Time"))
	for _, v := range r.Variants {
		pass := fmt.Sprintf("%d/%d", v.Passed, v.Total)
		if v.Errored > 0 {
			pass = fmt.Sprintf("%d/%d+%de", v.Passed, v.Total, v.Errored)
		}
		ver := "-"
		if v.Total > 0 {
			ver = fmt.Sprintf("%d", v.VerPassed)
		}
		chg := "-"
		if v.Total > 0 {
			chg = fmt.Sprintf("%d", v.ChgCount)
		}
		b.WriteString(fmt.Sprintf("%-32s %-8s %-5d %-4s %-10s %-6d %-5s %-10s\n",
			v.Variant,
			pass,
			v.Total,
			ver,
			comma(v.InputTokens+v.OutputTokens),
			v.ModelCalls,
			chg,
			(time.Duration(v.DurationMS) * time.Millisecond).Round(time.Millisecond),
		))
	}
	return b.String()
}

func comma(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}
