package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ittakestwo123/Harnesslab/internal/benchmark"
	"github.com/ittakestwo123/Harnesslab/internal/optimize"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
	"github.com/ittakestwo123/Harnesslab/internal/store"
)

// newOptimizeCmd analyzes run trajectories for failure patterns, suggests
// harness candidates, and (with --report) computes the Pareto front of a
// benchmark.
func newOptimizeCmd() *cobra.Command {
	var (
		harnessDir  string
		storeDriver string
		reportFile  string
		minRuns     int
	)
	cmd := &cobra.Command{
		Use:   "optimize",
		Short: "Analyze runs for failure patterns and suggest harness candidates",
		RunE: func(cmd *cobra.Command, args []string) error {
			if harnessDir == "" {
				harnessDir = ".harness"
			}
			ctx := context.Background()

			var runs []*store.Run
			var points []optimize.Point

			if reportFile != "" {
				data, err := os.ReadFile(reportFile)
				if err != nil {
					return fmt.Errorf("optimize: report %s: %w", reportFile, err)
				}
				var rep benchmark.Report
				if err := json.Unmarshal(data, &rep); err != nil {
					return fmt.Errorf("optimize: parse report: %w", err)
				}
				for _, v := range rep.Variants {
					if v.Total == 0 {
						continue
					}
					points = append(points, optimize.Point{
						Variant:    v.Variant,
						Pass:       float64(v.Passed) / float64(v.Total),
						Tokens:     avg(v.InputTokens+v.OutputTokens, v.Total),
						DurationMS: avg(v.DurationMS, v.Total),
						ModelCalls: v.ModelCalls / max(v.Total, 1),
						ToolCalls:  v.ToolCalls / max(v.Total, 1),
					})
					for _, r := range v.Runs {
						if r.RunID == "" {
							continue
						}
						runs = append(runs, &store.Run{ID: r.RunID, Task: r.TaskID, Metrics: store.Metrics{
							ToolCalls:   r.ToolCalls,
							ModelCalls:  r.ModelCalls,
							InputTokens: r.Tokens,
						}})
					}
				}
				fmt.Printf("Optimize: benchmark %s, %d runs\n\n", rep.ID, len(runs))
			} else {
				st, err := openStore(harnessDir, storeDriver)
				if err != nil {
					return err
				}
				all, err := st.ListRuns(ctx)
				if err != nil {
					return err
				}
				runs = all
				fmt.Printf("Optimize: analyzing %d runs in store\n\n", len(runs))
			}

			if len(runs) < minRuns {
				return fmt.Errorf("optimize: only %d runs available, need at least %d (use --min-runs or a bench --report)", len(runs), minRuns)
			}

			// Load traces for every run (missing traces are skipped).
			traces := map[string][]hlruntime.RunEvent{}
			for _, r := range runs {
				if evs, err := loadTrace(harnessDir, r.ID); err == nil {
					traces[r.ID] = evs
				}
			}

			a := optimize.Analyze(runs, traces)
			printAnalysis(a)

			if len(points) > 0 {
				front := optimize.Front(points)
				printPareto(front, points)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&harnessDir, "harness-dir", "", "harness directory (default .harness)")
	cmd.Flags().StringVar(&storeDriver, "store", "json", "run store backend: json or sqlite")
	cmd.Flags().StringVar(&reportFile, "report", "", "benchmark report json to analyze and compute the Pareto front from")
	cmd.Flags().IntVar(&minRuns, "min-runs", 1, "minimum number of runs to analyze")
	return cmd
}

func printAnalysis(a *optimize.Analysis) {
	fmt.Println("Failure patterns:")
	if len(a.Patterns) == 0 {
		fmt.Println("  none detected")
	} else {
		for _, p := range a.Patterns {
			fmt.Printf("  %d/%d  %s\n", p.Count, a.Runs, p.Label)
		}
	}
	fmt.Println()
	fmt.Println("Suggested harness changes:")
	if len(a.Candidates) == 0 {
		fmt.Println("  none")
	} else {
		for _, c := range a.Candidates {
			fmt.Printf("  - %s\n", c)
		}
	}
}

func printPareto(front, all []optimize.Point) {
	fmt.Println()
	fmt.Println("Pareto front (pass up, tokens/duration down):")
	fmt.Printf("%-36s %-8s %-12s %-10s\n", "Variant", "Pass", "Tokens", "Time")
	dominated := map[string]bool{}
	for _, a := range all {
		for _, b := range all {
			if a.Variant == b.Variant {
				continue
			}
			if dominatesPoint(b, a) {
				dominated[a.Variant] = true
			}
		}
	}
	for _, p := range front {
		mark := " "
		if dominated[p.Variant] {
			mark = "*"
		}
		fmt.Printf("%-36s %-8s %-12s %-10s%s\n", p.Variant, fmt.Sprintf("%.0f%%", p.Pass*100), comma(p.Tokens), dur(p.DurationMS), mark)
	}
	if len(dominated) > 0 {
		fmt.Println("\n* dominated variant shown for reference")
	}
}

func dominatesPoint(a, b optimize.Point) bool {
	better := a.Pass > b.Pass || a.Tokens < b.Tokens || a.DurationMS < b.DurationMS
	noWorse := a.Pass >= b.Pass && a.Tokens <= b.Tokens && a.DurationMS <= b.DurationMS
	return better && noWorse
}

func avg(total int64, n int) int64 {
	if n <= 0 {
		return 0
	}
	return total / int64(n)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
