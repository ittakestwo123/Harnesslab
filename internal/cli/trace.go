package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ittakestwo123/Harnesslab/internal/harness/builder"
	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
	"github.com/ittakestwo123/Harnesslab/internal/store"
	"github.com/ittakestwo123/Harnesslab/internal/store/jsonstore"
	"github.com/ittakestwo123/Harnesslab/internal/store/sqlitestore"
)

// openStore opens the run store backend selected by driver.
func openStore(harnessDir, driver string) (store.Store, error) {
	dir := filepath.Join(harnessDir, "store")
	switch driver {
	case "", "json":
		return jsonstore.New(dir)
	case "sqlite":
		return sqlitestore.New(filepath.Join(dir, "harness.db"))
	default:
		return nil, fmt.Errorf("unsupported store driver %q (supported: json, sqlite)", driver)
	}
}

// newTraceCmd renders the JSONL trace of a run (§18 trace view).
func newTraceCmd() *cobra.Command {
	var harnessDir string
	cmd := &cobra.Command{
		Use:   "trace <run-id>",
		Short: "Render the trace of a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if harnessDir == "" {
				harnessDir = ".harness"
			}
			path := filepath.Join(harnessDir, "traces", args[0]+".jsonl")
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("trace: %w", err)
			}
			defer f.Close()

			fmt.Printf("Run %s\n\n", args[0])
			var start time.Time
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 1024*1024), 1024*1024)
			for sc.Scan() {
				var ev hlruntime.RunEvent
				if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
					continue
				}
				if start.IsZero() {
					start = ev.Timestamp
				}
				printTraceLine(ev, start)
			}
			return sc.Err()
		},
	}
	cmd.Flags().StringVar(&harnessDir, "harness-dir", "", "harness directory (default .harness)")
	return cmd
}

// newRunsCmd lists recorded runs from the run store.
func newRunsCmd() *cobra.Command {
	var harnessDir, storeDriver string
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List recorded runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if harnessDir == "" {
				harnessDir = ".harness"
			}
			st, err := openStore(harnessDir, storeDriver)
			if err != nil {
				return err
			}
			runs, err := st.ListRuns(context.Background())
			if err != nil {
				return err
			}
			if len(runs) == 0 {
				fmt.Println("no runs recorded yet")
				return nil
			}
			fmt.Printf("%-14s %-9s %-8s %-12s %-10s %s\n", "RUN", "STATUS", "MODELS", "TOKENS", "TOOLS", "TASK")
			for _, r := range runs {
				fmt.Printf("%-14s %-9s %-8d %-12d %-10d %s\n",
					r.ID, r.Status, r.Metrics.ModelCalls,
					r.Metrics.InputTokens+r.Metrics.OutputTokens,
					r.Metrics.ToolCalls, clipTask(r.Task))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&harnessDir, "harness-dir", "", "harness directory (default .harness)")
	cmd.Flags().StringVar(&storeDriver, "store", "json", "run store backend: json or sqlite")
	return cmd
}

// newReplayCmd re-runs a recorded run offline from its replay store.
func newReplayCmd() *cobra.Command {
	var (
		harnessDir    string
		storeDriver   string
		fallback      bool
		keepWorkspace bool
		quiet         bool
	)
	cmd := &cobra.Command{
		Use:   "replay <run-id>",
		Short: "Re-run a recorded run offline using its replay store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]
			if harnessDir == "" {
				harnessDir = ".harness"
			}
			st, err := openStore(harnessDir, storeDriver)
			if err != nil {
				return err
			}
			rec, err := st.GetRun(context.Background(), runID)
			if err != nil {
				return fmt.Errorf("replay: run %s: %w", runID, err)
			}
			s, err := spec.Load(filepath.Join(harnessDir, "harness.yaml"))
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			mode := "strict (offline)"
			if fallback {
				mode = "fallback (live on miss)"
			}
			h, err := builder.Build(ctx, s, builder.Options{
				HarnessDir:     harnessDir,
				Repo:           rec.Repository,
				Commit:         rec.Commit,
				UserID:         "harness",
				KeepWorkspace:  keepWorkspace,
				ReplayFrom:     runID,
				ReplayFallback: fallback,
				ReplayModel:    true,
			})
			if err != nil {
				return err
			}
			defer h.Close()

			fmt.Printf("Replay: %s -> %s\n", runID, h.RunID)
			fmt.Printf("Mode        %s\n", mode)
			fmt.Printf("Task        %s\n", rec.Task)
			fmt.Println()

			var onEvent func(hlruntime.RunEvent)
			if !quiet {
				onEvent = printEvent
			}
			result, err := h.Run(ctx, rec.Task, onEvent)
			if err != nil {
				return err
			}
			printSummary(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&harnessDir, "harness-dir", "", "harness directory (default .harness)")
	cmd.Flags().StringVar(&storeDriver, "store", "json", "run store backend: json or sqlite")
	cmd.Flags().BoolVar(&fallback, "fallback", false, "allow live calls when a replay lookup misses")
	cmd.Flags().BoolVar(&keepWorkspace, "keep-workspace", false, "keep the workspace after the run")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "only print the summary, not the live event stream")
	return cmd
}

func printTraceLine(ev hlruntime.RunEvent, start time.Time) {
	rel := ev.Timestamp.Sub(start).Round(100 * time.Millisecond)
	ts := fmt.Sprintf("%02d:%02d", int(rel.Minutes()), int(rel.Seconds())%60)
	switch ev.Type {
	case hlruntime.EventRunStart:
		fmt.Printf("%s  RUN START\n", ts)
	case hlruntime.EventRunEnd:
		fmt.Printf("%s  RUN END\n", ts)
	case hlruntime.EventModelStart:
		name := ""
		if ev.Model != nil {
			name = ev.Model.Model
		}
		fmt.Printf("%s  MODEL %s\n", ts, name)
	case hlruntime.EventModelEnd:
		name, tin, tout := "", 0, 0
		if ev.Model != nil {
			name, tin, tout = ev.Model.Model, ev.Model.TokensIn, ev.Model.TokensOut
		}
		fmt.Printf("%s  MODEL %s\n", ts, name)
		if tin > 0 || tout > 0 {
			fmt.Printf("       tokens %d -> %d\n", tin, tout)
		}
		if ev.Model != nil && ev.Model.Content != "" {
			fmt.Printf("       %s\n", clipArgs(ev.Model.Content))
		}
	case hlruntime.EventToolStart:
		name, args := "", ""
		if ev.Tool != nil {
			name, args = ev.Tool.Name, ev.Tool.Arguments
		}
		fmt.Printf("%s  TOOL %s\n", ts, name)
		if args != "" {
			fmt.Printf("       %s\n", clipArgs(args))
		}
	case hlruntime.EventToolEnd:
		name := ""
		if ev.Tool != nil {
			name = ev.Tool.Name
		}
		fmt.Printf("%s  TOOL %s done\n", ts, name)
	case hlruntime.EventError:
		msg := ""
		if ev.Error != nil {
			msg = ev.Error.Message
		}
		fmt.Printf("%s  ERROR %s\n", ts, clipArgs(msg))
	}
}

func clipTask(s string) string {
	if len(s) > 40 {
		return s[:40] + "..."
	}
	return s
}
