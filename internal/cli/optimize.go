package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ittakestwo123/Harnesslab/internal/benchmark"
	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	"github.com/ittakestwo123/Harnesslab/internal/optimize"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
	"github.com/ittakestwo123/Harnesslab/internal/store"
)

// newOptimizeCmd analyzes run trajectories for failure patterns, suggests
// harness candidates, computes the Pareto front of a benchmark, and (with
// --llm / --evaluate) runs the LLM Harness Optimizer loop: candidate
// generation from failure analysis, dev evaluation, Pareto selection and
// holdout validation with the Dev-only-win REJECT gate.
func newOptimizeCmd() *cobra.Command {
	var (
		harnessDir  string
		storeDriver string
		reportFile  string
		minRuns     int

		configFile string
		llmMode    bool
		evaluate   bool
		candidates int
		tasksDir   string
		tasksets   string
		repeat     int
		parallel   int
		maxTokens  int64
		timeout    time.Duration
	)
	cmd := &cobra.Command{
		Use:   "optimize",
		Short: "Analyze runs, suggest harness candidates, run the LLM optimizer loop",
		RunE: func(cmd *cobra.Command, args []string) error {
			if harnessDir == "" {
				harnessDir = ".harness"
			}
			if configFile == "" {
				configFile = filepath.Join(harnessDir, "harness.yaml")
			}
			if tasksets == "" {
				tasksets = "benchmarks/tasksets"
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
						// Report-derived runs are always repository tasks
						// (bench tasks carry a repo), so mark Repository to
						// enable the no_change pattern.
						runs = append(runs, &store.Run{ID: r.RunID, Task: r.TaskID, Repository: "bench", Metrics: store.Metrics{
							ToolCalls:          r.ToolCalls,
							ModelCalls:         r.ModelCalls,
							InputTokens:        r.Tokens,
							WorkspaceChanged:   r.WorkspaceChanged,
							VerificationPassed: r.VerificationPassed,
						}})
					}
				}
				fmt.Printf("Optimize: benchmark %s, %d runs\n\n", rep.ID, len(runs))
			} else if !llmMode && !evaluate {
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

			// Load traces for every run (missing traces are skipped).
			traces := map[string][]hlruntime.RunEvent{}
			for _, r := range runs {
				if evs, err := loadTrace(harnessDir, r.ID); err == nil {
					traces[r.ID] = evs
				}
			}

			analysis := optimize.Analyze(runs, traces)
			if !llmMode && !evaluate {
				printAnalysis(analysis)
				if len(points) > 0 {
					front := optimize.Front(points)
					printPareto(front, points)
				}
				return nil
			}

			base, err := spec.Load(configFile)
			if err != nil {
				return err
			}

			switch {
			case llmMode:
				return runLLMGeneration(ctx, harnessDir, base, analysis, candidates)
			case evaluate:
				return runEvaluation(ctx, harnessDir, base, tasksDir, tasksets, parallel, maxTokens, timeout, repeat)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&harnessDir, "harness-dir", "", "harness directory (default .harness)")
	cmd.Flags().StringVar(&storeDriver, "store", "json", "run store backend: json or sqlite")
	cmd.Flags().StringVar(&reportFile, "report", "", "benchmark report json to analyze and compute the Pareto front from")
	cmd.Flags().IntVar(&minRuns, "min-runs", 1, "minimum number of runs to analyze")
	cmd.Flags().StringVar(&configFile, "config", "", "base harness.yaml (default <harness-dir>/harness.yaml)")
	cmd.Flags().BoolVar(&llmMode, "llm", false, "generate harness candidates with the LLM from the failure analysis")
	cmd.Flags().IntVar(&candidates, "candidates", 3, "number of candidates the LLM should generate")
	cmd.Flags().BoolVar(&evaluate, "evaluate", false, "evaluate candidates: dev bench -> Pareto -> holdout gate")
	cmd.Flags().StringVar(&tasksDir, "tasks", "", "benchmark tasks directory (required with --evaluate)")
	cmd.Flags().StringVar(&tasksets, "tasksets", "benchmarks/tasksets", "directory with dev.yaml + holdout.yaml tasksets")
	cmd.Flags().IntVar(&repeat, "repeat", 1, "benchmark repetitions per task x harness (evaluation)")
	cmd.Flags().IntVar(&parallel, "parallel", 2, "max concurrent agent runs (evaluation)")
	cmd.Flags().Int64Var(&maxTokens, "max-tokens", 0, "stop after this many cumulative tokens (0 = unlimited)")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "overall benchmark timeout (0 = none)")
	return cmd
}

// runLLMGeneration writes LLM-generated candidates under .harness/candidates.
func runLLMGeneration(ctx context.Context, harnessDir string, base *spec.HarnessSpec, a *optimize.Analysis, n int) error {
	ensureEnvWarn(base.Model.Provider)
	llm, err := newLLMClient(base.Model)
	if err != nil {
		return err
	}
	fmt.Printf("Optimize: generating %d candidates from %d runs (provider=%s model=%s)\n\n",
		n, a.Runs, base.Model.Provider, base.Model.Name)
	gens, err := optimize.GenerateCandidates(ctx, llm, base, a, n)
	if err != nil {
		if len(gens) == 0 {
			return err
		}
		fmt.Println("WARNING:", err) // partial: some candidates were invalid and skipped
	}
	if err := writeCandidates(harnessDir, gens); err != nil {
		return err
	}
	fmt.Printf("Candidates written to %s\n", filepath.Join(harnessDir, "candidates"))
	return nil
}

// runEvaluation runs the dev -> Pareto -> holdout loop over existing
// candidates and prints the recommendation.
func runEvaluation(ctx context.Context, harnessDir string, base *spec.HarnessSpec,
	tasksDir, tasksets string, parallel int, maxTokens int64, timeout time.Duration, repeat int) error {

	if tasksDir == "" {
		return fmt.Errorf("optimize: --evaluate requires --tasks <benchmark tasks dir>")
	}
	cands, err := optimize.LoadCandidates(filepath.Join(harnessDir, "candidates"))
	if err != nil && len(cands) == 0 {
		return err
	}
	if len(cands) == 0 {
		return fmt.Errorf("optimize: no candidates under %s (run `harness optimize --llm` first)", filepath.Join(harnessDir, "candidates"))
	}
	fmt.Printf("Optimize: evaluating %d candidate(s) + baseline\n", len(cands))
	variants := candidateVariants(base, cands)

	devSet := filepath.Join(tasksets, "dev.yaml")
	holdSet := filepath.Join(tasksets, "holdout.yaml")

	fmt.Println("\n[1/3] dev benchmark (baseline + candidates)...")
	devRep, err := runBenchTasks(ctx, tasksDir, devSet, harnessDir, variants, parallel, maxTokens, timeout, repeat)
	if err != nil {
		return err
	}
	devPath := filepath.Join(harnessDir, "bench", "optimize-dev-"+devRep.ID+".json")
	if err := devRep.WriteJSON(devPath); err != nil {
		return err
	}
	fmt.Printf("  dev report: %s\n", devPath)
	devRes := evalResults(devRep)

	fmt.Println("[2/3] Pareto selection on dev results...")
	front := optimize.SelectCandidates(devRes)
	selected := map[string]bool{"baseline": true}
	for _, f := range front {
		selected[f.Variant] = true
	}
	var holdVariants []benchmark.Variant
	for _, v := range variants {
		if selected[v.Name] {
			holdVariants = append(holdVariants, v)
		}
	}
	fmt.Println("[3/3] holdout benchmark (baseline + selected candidates)...")
	holdRep, err := runBenchTasks(ctx, tasksDir, holdSet, harnessDir, holdVariants, parallel, maxTokens, timeout, repeat)
	if err != nil {
		return err
	}
	holdPath := filepath.Join(harnessDir, "bench", "optimize-holdout-"+holdRep.ID+".json")
	if err := holdRep.WriteJSON(holdPath); err != nil {
		return err
	}
	fmt.Printf("  holdout report: %s\n", holdPath)

	rec := optimize.Recommend(devRes, evalResults(holdRep))
	printRecommendation(rec)
	return nil
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
