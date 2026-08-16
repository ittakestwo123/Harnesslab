package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ittakestwo123/Harnesslab/internal/benchmark"
	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
)

// newBenchCmd runs a benchmark: tasks x harness variants on a worker pool.
func newBenchCmd() *cobra.Command {
	var (
		config      string
		harnessDir  string
		matrixFile  string
		storeDriver string
		parallel    int
		maxTokens   int64
		timeout     time.Duration
		retry       int
		dryRun      bool
		quiet       bool
	)
	cmd := &cobra.Command{
		Use:   "bench <tasks>",
		Short: "Benchmark tasks across harness variants",
		Long: "Run every task in <tasks> (a directory of task yamls or a single " +
			"file) under every harness variant of the matrix, on a bounded worker " +
			"pool with token/time budget control.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if harnessDir == "" {
				if config != "" {
					harnessDir = filepath.Dir(config)
				} else {
					harnessDir = ".harness"
				}
			}
			if config == "" {
				config = filepath.Join(harnessDir, "harness.yaml")
			}
			base, err := spec.Load(config)
			if err != nil {
				return err
			}

			matrix := benchmark.Matrix{}
			if matrixFile != "" {
				data, err := os.ReadFile(matrixFile)
				if err != nil {
					return fmt.Errorf("bench: matrix %s: %w", matrixFile, err)
				}
				if err := yaml.Unmarshal(data, &matrix); err != nil {
					return fmt.Errorf("bench: parse matrix: %w", err)
				}
			}

			variants, err := matrix.Variants(base)
			if err != nil {
				return err
			}
			tasks, err := benchmark.LoadTasks(args[0])
			if err != nil {
				return err
			}

			var jobs []benchmark.Job
			for _, t := range tasks {
				for _, v := range variants {
					jobs = append(jobs, benchmark.Job{
						ID:      t.ID + "/" + v.Name,
						Task:    t,
						Variant: v,
					})
				}
			}

			fmt.Printf("Benchmark: %d tasks x %d variants = %d jobs\n\n", len(tasks), len(variants), len(jobs))
			if dryRun {
				fmt.Println("Tasks:")
				for _, t := range tasks {
					fmt.Printf("  - %s (%s)\n", t.ID, t.Repo)
				}
				fmt.Println("Variants:")
				for _, v := range variants {
					fmt.Printf("  - %s\n", v.Name)
				}
				return nil
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			sched := benchmark.NewScheduler(benchmark.Options{
				HarnessDir:  harnessDir,
				StoreDriver: storeDriver,
				Parallel:    parallel,
				MaxTokens:   maxTokens,
				Timeout:     timeout,
				Retry:       retry,
			})

			var done atomic.Int64
			report, err := sched.Run(ctx, jobs, func(o benchmark.Outcome) {
				n := done.Add(1)
				if quiet {
					return
				}
				taskID, variant := "", ""
				if o.Job != nil {
					taskID, variant = o.Job.Task.ID, o.Job.Variant.Name
				}
				status := string(o.Status)
				if o.Error != nil {
					status = "error"
				}
				toks := o.Metrics.InputTokens + o.Metrics.OutputTokens
				fmt.Printf("[%d/%d] task=%-14s variant=%-32s run=%-14s %-7s tokens=%d\n",
					n, len(jobs), taskID, variant, o.RunID, status, toks)
			})
			if err != nil {
				return err
			}

			fmt.Println()
			fmt.Println(report.RenderTable())

			reportPath := filepath.Join(harnessDir, "bench", report.ID+".json")
			if err := report.WriteJSON(reportPath); err != nil {
				return err
			}
			fmt.Printf("\nReport: %s\n", reportPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&config, "config", "", "base harness.yaml (default <harness-dir>/harness.yaml)")
	cmd.Flags().StringVar(&harnessDir, "harness-dir", "", "harness directory (default .harness)")
	cmd.Flags().StringVar(&matrixFile, "matrix", "", "matrix yaml defining harness variants")
	cmd.Flags().StringVar(&storeDriver, "store", "json", "run store backend: json or sqlite")
	cmd.Flags().IntVar(&parallel, "parallel", 2, "max concurrent agent runs")
	cmd.Flags().Int64Var(&maxTokens, "max-tokens", 0, "stop after this many cumulative tokens (0 = unlimited)")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "overall benchmark timeout (0 = none)")
	cmd.Flags().IntVar(&retry, "retry", 0, "retry scheduler-level failures this many extra times")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the job matrix without running")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "only print the summary table")
	return cmd
}
