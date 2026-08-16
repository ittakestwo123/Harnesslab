package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ittakestwo123/Harnesslab/internal/diff"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
	"github.com/ittakestwo123/Harnesslab/internal/store"
)

// newDiffCmd compares two runs: metrics, trajectory and first divergence.
func newDiffCmd() *cobra.Command {
	var (
		harnessDir  string
		storeDriver string
		full        bool
	)
	cmd := &cobra.Command{
		Use:   "diff <run-a> <run-b>",
		Short: "Compare two runs: metrics, trajectory and first divergence",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if harnessDir == "" {
				harnessDir = ".harness"
			}
			st, err := openStore(harnessDir, storeDriver)
			if err != nil {
				return err
			}
			runA, err := st.GetRun(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("diff: run %s: %w", args[0], err)
			}
			runB, err := st.GetRun(cmd.Context(), args[1])
			if err != nil {
				return fmt.Errorf("diff: run %s: %w", args[1], err)
			}

			evA, err := loadTrace(harnessDir, args[0])
			if err != nil {
				return err
			}
			evB, err := loadTrace(harnessDir, args[1])
			if err != nil {
				return err
			}

			d := diff.Compare(diff.BuildSteps(evA), diff.BuildSteps(evB))

			printMetricTable(runA, runB, d)
			printTrajectory(d, full)
			return nil
		},
	}
	cmd.Flags().StringVar(&harnessDir, "harness-dir", "", "harness directory (default .harness)")
	cmd.Flags().StringVar(&storeDriver, "store", "json", "run store backend: json or sqlite")
	cmd.Flags().BoolVar(&full, "full", false, "print the full aligned trajectory (not just the head)")
	return cmd
}

func printMetricTable(a, b *store.Run, d *diff.Diff) {
	fmt.Printf("Harness Diff: %s vs %s\n\n", a.ID, b.ID)
	fmt.Printf("%-20s %-18s %-18s\n", "", "Run A ("+a.ID+")", "Run B ("+b.ID+")")
	fmt.Printf("%-20s %-18s %-18s\n", "Status", string(a.Status), string(b.Status))
	fmt.Printf("%-20s %-18s %-18s\n", "Tokens in", commaI64(a.Metrics.InputTokens), commaI64(b.Metrics.InputTokens))
	fmt.Printf("%-20s %-18s %-18s\n", "Tokens out", commaI64(a.Metrics.OutputTokens), commaI64(b.Metrics.OutputTokens))
	fmt.Printf("%-20s %-18d %-18d\n", "Tool Calls", a.Metrics.ToolCalls, b.Metrics.ToolCalls)
	fmt.Printf("%-20s %-18d %-18d\n", "Model Calls", a.Metrics.ModelCalls, b.Metrics.ModelCalls)
	fmt.Printf("%-20s %-18s %-18s\n", "Duration", dur(a.Metrics.DurationMS), dur(b.Metrics.DurationMS))
	fmt.Println()
}

func printTrajectory(d *diff.Diff, full bool) {
	limit := len(d.Lines)
	if !full && limit > 14 {
		limit = 12
	}
	fmt.Printf("Trajectory: %d steps vs %d steps\n\n", len(d.StepsA), len(d.StepsB))
	fmt.Printf("%-4s %-6s %s\n", "Step", "Mark", "Event")
	for i, ln := range d.Lines {
		if i >= limit {
			fmt.Printf("... (%d more lines, use --full)\n", len(d.Lines)-limit)
			break
		}
		step := ln.StepA
		if ln.StepB > step {
			step = ln.StepB
		}
		mark := "== "
		desc := describeStep(ln.A)
		if ln.Divergence {
			mark = "!! "
		} else if ln.A == nil {
			mark = "B> "
		} else if ln.B == nil {
			mark = "A> "
		}
		if ln.Divergence && ln.B != nil {
			fmt.Printf("%-4d %-6s %s\n", step, mark, desc)
			fmt.Printf("      %-6s %s\n", "", "B: "+describeStep(ln.B))
			continue
		}
		fmt.Printf("%-4d %-6s %s\n", step, mark, desc)
	}
	if d.FirstDivergence != nil && full {
		// Show what each run did after the divergence.
		for i := d.DivergenceStepA; i < len(d.StepsA); i++ {
			fmt.Printf("%-4d %-6s %s\n", i+1, "A> ", describeStep(&d.StepsA[i]))
		}
		for i := d.DivergenceStepB; i < len(d.StepsB); i++ {
			fmt.Printf("%-4d %-6s %s\n", i+1, "B> ", describeStep(&d.StepsB[i]))
		}
	}
	fmt.Println()

	if d.FirstDivergence != nil {
		div := d.FirstDivergence
		fmt.Printf("First divergence at A step %d / B step %d:\n", div.StepA, div.StepB)
		if div.A != nil {
			fmt.Printf("  A: %s\n", describeStep(div.A))
		} else {
			fmt.Printf("  A: (end of trajectory)\n")
		}
		if div.B != nil {
			fmt.Printf("  B: %s\n", describeStep(div.B))
		} else {
			fmt.Printf("  B: (end of trajectory)\n")
		}
	} else {
		fmt.Printf("First divergence: none — identical trajectories\n")
	}
}

func describeStep(s *diff.Step) string {
	if s == nil {
		return "-"
	}
	switch s.Type {
	case diff.StepModel:
		extra := ""
		if s.TokensIn > 0 || s.TokensOut > 0 {
			extra = fmt.Sprintf(" tokens %d->%d", s.TokensIn, s.TokensOut)
		}
		content := ""
		if s.Content != "" {
			content = " " + clipArgs(s.Content)
		}
		return fmt.Sprintf("model %s%s%s", s.Name, extra, content)
	case diff.StepTool:
		args := ""
		if s.Args != "" {
			args = " " + clipArgs(s.Args)
		}
		result := ""
		if s.Result != "" {
			result = " -> " + clipArgs(s.Result)
		}
		return fmt.Sprintf("tool %s%s%s", s.Name, args, result)
	default:
		return fmt.Sprintf("error %s", clipArgs(s.Result))
	}
}

// loadTrace reads a run's JSONL trace into normalized events.
func loadTrace(harnessDir, runID string) ([]hlruntime.RunEvent, error) {
	path := filepath.Join(harnessDir, "traces", runID+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("diff: trace %s: %w", runID, err)
	}
	defer f.Close()
	var events []hlruntime.RunEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var ev hlruntime.RunEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func commaI64(n int64) string {
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

func dur(ms int64) string {
	return (time.Duration(ms) * time.Millisecond).Round(time.Millisecond).String()
}
