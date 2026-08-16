// Package cli implements the harness CLI (Stage 1: init + run).
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ittakestwo123/Harnesslab/internal/harness/builder"
	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
)

// NewRootCmd builds the harness command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "harness",
		Short:        "HarnessLab 鈥?build, trace, replay, benchmark and evolve AI agent harnesses",
		Version:      "0.1.0",
		SilenceUsage: true,
	}
	root.AddCommand(newInitCmd(), newRunCmd(), newTraceCmd(), newRunsCmd(), newReplayCmd(), newDiffCmd(), newBenchCmd(), newSnapshotCmd(), newExportCmd(), newReproduceCmd(), newOptimizeCmd())
	return root
}

func newInitCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a .harness directory with a default harness.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				dir = ".harness"
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			for _, sub := range []string{"skills", "policies", "evals", "tasks", "traces", "store", "workspaces"} {
				if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
					return err
				}
			}
			cfg := filepath.Join(dir, "harness.yaml")
			if _, err := os.Stat(cfg); os.IsNotExist(err) {
				if err := os.WriteFile(cfg, []byte(spec.DefaultTemplate), 0o644); err != nil {
					return err
				}
			}
			fmt.Printf("initialized harness in %s\n", dir)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "harness directory (default .harness)")
	return cmd
}

func newRunCmd() *cobra.Command {
	var (
		config         string
		harnessDir     string
		repo           string
		commit         string
		userID         string
		keepWorkspace  bool
		quiet          bool
		replayFrom     string
		replayFallback bool
		replayModel    bool
		storeDriver    string
	)
	cmd := &cobra.Command{
		Use:   "run [task]",
		Short: "Run an agent task with a harness",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task := strings.Join(args, " ")
			if harnessDir == "" {
				if config != "" {
					// Default the harness directory to the config file's dir.
					harnessDir = filepath.Dir(config)
				} else {
					harnessDir = ".harness"
				}
			}
			if config == "" {
				config = filepath.Join(harnessDir, "harness.yaml")
			}
			s, err := spec.Load(config)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			h, err := builder.Build(ctx, s, builder.Options{
				HarnessDir:     harnessDir,
				Repo:           repo,
				Commit:         commit,
				UserID:         userID,
				KeepWorkspace:  keepWorkspace,
				ReplayFrom:     replayFrom,
				ReplayFallback: replayFallback,
				ReplayModel:    replayModel,
				StoreDriver:    storeDriver,
			})
			if err != nil {
				return err
			}
			defer h.Close()

			fmt.Printf("Run: %s\n", h.RunID)
			fmt.Printf("Model       %s (%s)\n", s.Model.Name, s.Model.Provider)
			fmt.Printf("Harness     %s\n", s.Name)
			if replayFrom != "" {
				fmt.Printf("Replay      from %s\n", replayFrom)
			}
			if h.Workspace != nil {
				fmt.Printf("Workspace   %s\n", h.Workspace.Root)
			}
			fmt.Println()

			var onEvent func(hlruntime.RunEvent)
			if !quiet {
				onEvent = printEvent
			}
			result, err := h.Run(ctx, task, onEvent)
			if err != nil {
				return err
			}
			printSummary(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&config, "config", "", "harness.yaml path (default .harness/harness.yaml)")
	cmd.Flags().StringVar(&harnessDir, "harness-dir", "", "harness directory (default .harness)")
	cmd.Flags().StringVar(&repo, "repo", "", "repository URL or local path for a git worktree workspace")
	cmd.Flags().StringVar(&commit, "commit", "", "commit to check out in the workspace")
	cmd.Flags().StringVar(&userID, "user", "harness", "user id owning the run")
	cmd.Flags().BoolVar(&keepWorkspace, "keep-workspace", false, "keep the workspace after the run")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "only print the summary, not the live event stream")
	cmd.Flags().StringVar(&replayFrom, "replay", "", "replay tool calls recorded by the given run (offline)")
	cmd.Flags().BoolVar(&replayFallback, "replay-fallback", false, "allow live calls when a replay lookup misses (with --replay)")
	cmd.Flags().BoolVar(&replayModel, "replay-model", true, "also replay model calls (with --replay); records model calls by default so runs can be replayed offline")
	cmd.Flags().StringVar(&storeDriver, "store", "json", "run store backend: json or sqlite")
	return cmd
}

func printEvent(ev hlruntime.RunEvent) {
	ts := ev.Timestamp.Format("15:04:05")
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
		fmt.Printf("%s  MODEL %s tokens %d -> %d\n", ts, name, tin, tout)
	case hlruntime.EventToolStart:
		name, args := "", ""
		if ev.Tool != nil {
			name, args = ev.Tool.Name, ev.Tool.Arguments
		}
		fmt.Printf("%s  TOOL %s %s\n", ts, name, clipArgs(args))
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
		fmt.Printf("%s  ERROR %s\n", ts, msg)
	}
}

func printSummary(r *builder.Result) {
	fmt.Println()
	fmt.Printf("Status      %s\n", r.Run.Status)
	fmt.Printf("Tokens      %d in / %d out\n", r.Metrics.InputTokens, r.Metrics.OutputTokens)
	fmt.Printf("Tool Calls  %d\n", r.Metrics.ToolCalls)
	fmt.Printf("Model Calls %d\n", r.Metrics.ModelCalls)
	fmt.Printf("Duration    %s\n", (time.Duration(r.Metrics.DurationMS) * time.Millisecond).Round(time.Millisecond))
	fmt.Printf("Trace:\n%s\n", r.TracePath)
	if r.VerificationErr != nil {
		fmt.Printf("Verification: %v\n", r.VerificationErr)
	}
	if r.WorkspaceDiff != nil && strings.TrimSpace(r.WorkspaceDiff.Stat) != "" {
		fmt.Printf("Workspace diff:\n%s\n", r.WorkspaceDiff.Stat)
	}
}

func clipArgs(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}
