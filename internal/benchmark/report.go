package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ittakestwo123/Harnesslab/internal/store"
)

// RunSummary is one job's result within a variant group.
type RunSummary struct {
	RunID        string       `json:"run_id"`
	TaskID       string       `json:"task_id"`
	Variant      string       `json:"variant"`
	Repeat       int          `json:"repeat,omitempty"`
	Status       store.Status `json:"status"`
	Tokens       int64        `json:"tokens"`
	InputTokens  int64        `json:"input_tokens,omitempty"`
	OutputTokens int64        `json:"output_tokens,omitempty"`
	ModelCalls   int          `json:"model_calls"`
	ToolCalls    int          `json:"tool_calls"`
	DurationMS   int64        `json:"duration_ms"`
	CostUSD      float64      `json:"cost_usd,omitempty"`
	// VerificationPassed reports whether verification commands passed.
	VerificationPassed bool `json:"verification_passed"`
	// WorkspaceChanged reports whether the agent modified the workspace.
	WorkspaceChanged bool   `json:"workspace_changed"`
	Error            string `json:"error,omitempty"`
}

// TaskResult aggregates the repeated runs of one task under one variant.
type TaskResult struct {
	TaskID string `json:"task_id"`
	Passed int    `json:"passed"`
	Total  int    `json:"total"`
	Stats  Stats  `json:"stats"`
}

// VariantResult aggregates outcomes for one harness variant.
type VariantResult struct {
	Variant      string  `json:"variant"`
	Total        int     `json:"total"`
	Passed       int     `json:"passed"`
	Failed       int     `json:"failed"`
	Errored      int     `json:"errored"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	ModelCalls   int     `json:"model_calls"`
	ToolCalls    int     `json:"tool_calls"`
	DurationMS   int64   `json:"duration_ms"`
	CostUSD      float64 `json:"cost_usd"`
	// VerPassed counts runs whose verification passed.
	VerPassed int `json:"verification_passed"`
	// ChgCount counts runs that modified the workspace.
	ChgCount int `json:"workspace_changed"`
	// Stats summarizes tokens/cost/latency over the runs (mean/median/...).
	Stats Stats `json:"stats"`
	// ByTask breaks the variant down per task (with repeats).
	ByTask []TaskResult `json:"by_task"`
	Runs   []RunSummary `json:"runs"`
}

// Report is the aggregated result of a benchmark.
type Report struct {
	ID         string           `json:"id"`
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt time.Time        `json:"finished_at"`
	Variants   []*VariantResult `json:"variants"`
}

// Finalize sorts variants and computes statistical aggregates (mean/median/
// stddev/P50/P90/CI95) and per-task breakdowns from the raw runs.
func (r *Report) Finalize() {
	for _, v := range r.Variants {
		v.ByTask = buildByTask(v.Runs)
		v.Stats = statsOf(v.Runs)
	}
	sort.Slice(r.Variants, func(i, j int) bool { return r.Variants[i].Variant < r.Variants[j].Variant })
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
	vr.CostUSD += o.Metrics.CostUSD
	if o.Metrics.VerificationPassed {
		vr.VerPassed++
	}
	if o.Metrics.WorkspaceChanged {
		vr.ChgCount++
	}

	repeat := 0
	if o.Job != nil {
		repeat = o.Job.Repeat
	}
	summary := RunSummary{
		Variant:            name,
		Repeat:             repeat,
		Status:             o.Status,
		Tokens:             o.Metrics.InputTokens + o.Metrics.OutputTokens,
		InputTokens:        o.Metrics.InputTokens,
		OutputTokens:       o.Metrics.OutputTokens,
		ModelCalls:         o.Metrics.ModelCalls,
		ToolCalls:          o.Metrics.ToolCalls,
		DurationMS:         o.Metrics.DurationMS,
		CostUSD:            o.Metrics.CostUSD,
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

// statsOf computes token/cost/latency stats over runs that actually executed
// (those with a run id), so scheduler-level errors are not counted as zeros.
func statsOf(runs []RunSummary) Stats {
	var in, out, cost, lat []float64
	for _, r := range runs {
		if r.RunID == "" {
			continue
		}
		in = append(in, float64(r.InputTokens))
		out = append(out, float64(r.OutputTokens))
		cost = append(cost, r.CostUSD)
		lat = append(lat, float64(r.DurationMS))
	}
	return Stats{
		InputTokens:  computeStat(in),
		OutputTokens: computeStat(out),
		CostUSD:      computeStat(cost),
		LatencyMS:    computeStat(lat),
	}
}

// buildByTask groups runs per task and computes per-task stats.
func buildByTask(runs []RunSummary) []TaskResult {
	byID := map[string]*TaskResult{}
	var order []string
	for _, r := range runs {
		if r.TaskID == "" {
			continue
		}
		tr, ok := byID[r.TaskID]
		if !ok {
			tr = &TaskResult{TaskID: r.TaskID}
			byID[r.TaskID] = tr
			order = append(order, r.TaskID)
		}
		tr.Total++
		if r.Status == store.StatusPassed {
			tr.Passed++
		}
	}
	out := make([]TaskResult, 0, len(order))
	for _, id := range order {
		tr := byID[id]
		// Per-task stats over that task's runs.
		var in, cost, lat []float64
		for _, r := range runs {
			if r.TaskID != id || r.RunID == "" {
				continue
			}
			in = append(in, float64(r.InputTokens))
			cost = append(cost, r.CostUSD)
			lat = append(lat, float64(r.DurationMS))
		}
		tr.Stats = Stats{
			InputTokens: computeStat(in),
			CostUSD:     computeStat(cost),
			LatencyMS:   computeStat(lat),
		}
		out = append(out, *tr)
	}
	return out
}

// RenderStats prints a statistical summary table per variant: success rate
// and mean/median/P50/P90 of tokens, cost and latency.
func (r *Report) RenderStats() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-28s %-8s %-11s %-11s %-11s %-11s %-11s %-13s %-13s\n",
		"Harness", "Success", "Tokens mean", "Tokens P50", "Tokens P90", "Cost mean", "Lat mean", "Lat P90", "CI95(tok)"))
	for _, v := range r.Variants {
		rate := "-"
		if v.Total > 0 {
			rate = fmt.Sprintf("%.0f%%", 100*float64(v.Passed)/float64(v.Total))
		}
		b.WriteString(fmt.Sprintf("%-28s %-8s %-11s %-11s %-11s %-11s %-11s %-13s %-13s\n",
			v.Variant,
			rate,
			f2(v.Stats.InputTokens.Mean),
			f2(v.Stats.InputTokens.P50),
			f2(v.Stats.InputTokens.P90),
			f2(v.Stats.CostUSD.Mean),
			f2(v.Stats.LatencyMS.Mean),
			f2(v.Stats.LatencyMS.P90),
			fmt.Sprintf("%s..%s", f2(v.Stats.InputTokens.CI95Lo), f2(v.Stats.InputTokens.CI95Hi)),
		))
	}
	return b.String()
}

func f2(v float64) string {
	if v == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", v)
}
