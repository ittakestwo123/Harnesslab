package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ittakestwo123/Harnesslab/internal/harness/builder"
	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	"github.com/ittakestwo123/Harnesslab/internal/reproduce"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
)

// newSnapshotCmd writes .harness/harness.lock for the current harness.
func newSnapshotCmd() *cobra.Command {
	var harnessDir, config string
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Write harness.lock for the current harness",
		RunE: func(cmd *cobra.Command, args []string) error {
			if harnessDir == "" {
				harnessDir = ".harness"
			}
			if config == "" {
				config = filepath.Join(harnessDir, "harness.yaml")
			}
			s, err := spec.Load(config)
			if err != nil {
				return err
			}
			lock, err := reproduce.GenerateLock(s)
			if err != nil {
				return err
			}
			path := filepath.Join(harnessDir, "harness.lock")
			if err := os.WriteFile(path, lock, 0o644); err != nil {
				return err
			}
			fmt.Printf("harness.lock written to %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&harnessDir, "harness-dir", "", "harness directory (default .harness)")
	cmd.Flags().StringVar(&config, "config", "", "harness.yaml path (default <harness-dir>/harness.yaml)")
	return cmd
}

// newExportCmd writes a reproducible bundle for one run.
func newExportCmd() *cobra.Command {
	var harnessDir, storeDriver, out string
	cmd := &cobra.Command{
		Use:   "export <run-id>",
		Short: "Export a run as a reproducible .harness bundle",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if harnessDir == "" {
				harnessDir = ".harness"
			}
			st, err := openStore(harnessDir, storeDriver)
			if err != nil {
				return err
			}
			run, err := st.GetRun(context.Background(), args[0])
			if err != nil {
				return fmt.Errorf("export: run %s: %w", args[0], err)
			}
			if out == "" {
				out = args[0] + ".harness"
			}
			if err := reproduce.Export(reproduce.ExportOptions{
				HarnessDir: harnessDir,
				Run:        run,
				OutPath:    out,
			}); err != nil {
				return err
			}
			fmt.Printf("exported %s -> %s\n", args[0], out)
			return nil
		},
	}
	cmd.Flags().StringVar(&harnessDir, "harness-dir", "", "harness directory (default .harness)")
	cmd.Flags().StringVar(&storeDriver, "store", "json", "run store backend: json or sqlite")
	cmd.Flags().StringVar(&out, "out", "", "output bundle path (default <run-id>.harness)")
	return cmd
}

// newReproduceCmd re-runs a run from its recorded spec and replay store.
// The argument is a run id (from the store) or a .harness bundle file.
func newReproduceCmd() *cobra.Command {
	var (
		harnessDir  string
		storeDriver string
		keepBundle  bool
		quiet       bool
		envMode     string
	)
	cmd := &cobra.Command{
		Use:   "reproduce <run-id | bundle.harness>",
		Short: "Re-run a recorded run using its recorded harness spec and replay store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if harnessDir == "" {
				harnessDir = ".harness"
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			arg := args[0]
			var (
				specYAML    string
				task        string
				repo        string
				commit      string
				replayID    string
				bundleDir   string
				recordedEnv string
			)

			if isBundle(arg) {
				tmp, err := os.MkdirTemp("", "harness-reproduce-*")
				if err != nil {
					return err
				}
				if !keepBundle {
					defer os.RemoveAll(tmp)
				} else {
					fmt.Printf("bundle extracted to %s\n", tmp)
				}
				if err := reproduce.Extract(arg, tmp); err != nil {
					return err
				}
				m, err := reproduce.LoadManifest(tmp)
				if err != nil {
					return fmt.Errorf("reproduce: manifest: %w", err)
				}
				bundleDir = tmp
				task, repo, commit, replayID = m.Task, m.Repo, m.Commit, m.RunID
				harnessDir = tmp
				if data, err := os.ReadFile(filepath.Join(tmp, "environment.json")); err == nil {
					recordedEnv = string(data)
				}
			} else {
				st, err := openStore(harnessDir, storeDriver)
				if err != nil {
					return err
				}
				run, err := st.GetRun(ctx, arg)
				if err != nil {
					return fmt.Errorf("reproduce: run %s: %w", arg, err)
				}
				specYAML, task, repo, commit, replayID = run.SpecYAML, run.Task, run.Repository, run.Commit, run.ID
				recordedEnv = run.Environment
			}

			// Resolve the harness spec: prefer the recorded one.
			s, err := spec.Parse([]byte(specYAML))
			if err != nil || s == nil {
				s, err = spec.Load(filepath.Join(harnessDir, "harness.yaml"))
				if err != nil {
					return fmt.Errorf("reproduce: no recorded spec and no harness.yaml: %w", err)
				}
			}

			if err := checkEnvironment(recordedEnv, envMode); err != nil {
				return err
			}

			fmt.Printf("Reproduce: %s\n", replayID)
			fmt.Printf("Task       %s\n", task)
			if repo != "" {
				fmt.Printf("Repo       %s @ %s\n", repo, commit)
			}
			fmt.Println()

			h, err := builder.Build(ctx, s, builder.Options{
				HarnessDir:  harnessDir,
				Repo:        repo,
				Commit:      commit,
				UserID:      "reproduce",
				ReplayFrom:  replayID,
				ReplayModel: true,
			})
			if err != nil {
				return err
			}
			defer h.Close()

			var onEvent func(hlruntime.RunEvent) = nil
			if !quiet {
				onEvent = printEvent
			}
			result, err := h.Run(ctx, task, onEvent)
			if err != nil {
				return err
			}
			printSummary(result)
			_ = bundleDir
			return nil
		},
	}
	cmd.Flags().StringVar(&harnessDir, "harness-dir", "", "harness directory (default .harness)")
	cmd.Flags().StringVar(&storeDriver, "store", "json", "run store backend: json or sqlite")
	cmd.Flags().BoolVar(&keepBundle, "keep-bundle", false, "keep the extracted bundle directory")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "only print the summary")
	cmd.Flags().StringVar(&envMode, "env-mode", "warn", "environment drift handling: warn | strict | ignore")
	return cmd
}

// checkEnvironment validates the recorded environment against the current
// one according to envMode.
func checkEnvironment(recordedEnv, mode string) error {
	if mode == "ignore" {
		return nil
	}
	recorded, err := reproduce.EnvFromJSON(recordedEnv)
	if err != nil {
		return fmt.Errorf("reproduce: recorded environment: %w", err)
	}
	current := reproduce.Capture()
	diffs := reproduce.CompareEnv(recorded, current)
	if len(diffs) == 0 {
		return nil
	}
	mismatches := 0
	fmt.Println("Environment check:")
	for _, d := range diffs {
		mark := "MATCH"
		if !d.Match {
			mark = "MISMATCH"
			mismatches++
		}
		fmt.Printf("  %-14s %-8s recorded=%q current=%q\n", d.Key, mark, d.Recorded, d.Current)
	}
	if mismatches > 0 {
		if reproduce.ParseEnvMode(mode) == reproduce.EnvStrict {
			return fmt.Errorf("reproduce: environment drift detected (%d mismatches)", mismatches)
		}
		fmt.Printf("environment drift detected (%d mismatches)\n", mismatches)
	}
	return nil
}

func isBundle(arg string) bool {
	return filepath.Ext(arg) == ".harness"
}
