// Package builder assembles a complete Harness from a HarnessSpec: it creates
// the workspace, builds the runtime, attaches the recorder and the run store,
// executes the run, verifies the result and records metrics.
package builder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	"trpc.group/trpc-go/trpc-agent-go/log"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/hostexec"

	"github.com/ittakestwo123/Harnesslab/internal/cost"
	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	"github.com/ittakestwo123/Harnesslab/internal/replay"
	"github.com/ittakestwo123/Harnesslab/internal/reproduce"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
	codexrt "github.com/ittakestwo123/Harnesslab/internal/runtime/codex"
	"github.com/ittakestwo123/Harnesslab/internal/runtime/trpc"
	"github.com/ittakestwo123/Harnesslab/internal/sandbox"
	"github.com/ittakestwo123/Harnesslab/internal/store"
	"github.com/ittakestwo123/Harnesslab/internal/store/jsonstore"
	"github.com/ittakestwo123/Harnesslab/internal/store/sqlitestore"
	"github.com/ittakestwo123/Harnesslab/internal/trace/recorder"
	"github.com/ittakestwo123/Harnesslab/internal/workspace"
	"github.com/ittakestwo123/Harnesslab/internal/workspace/git"
	"trpc.group/trpc-go/trpc-agent-go/tool/file"
)

// Options configures a single harness build.
type Options struct {
	// HarnessDir is the .harness directory holding store/traces/workspaces.
	HarnessDir string
	// Repo is the git URL/local path to build a worktree workspace from.
	Repo string
	// Commit pins the workspace revision (empty = default branch HEAD).
	Commit string
	// UserID owns the run.
	UserID string
	// KeepWorkspace keeps the workspace after the run instead of destroying it.
	KeepWorkspace bool
	// ReplayFrom, when set, replays tool/model calls recorded by the given
	// run instead of calling the live tool/model (offline replay).
	ReplayFrom string
	// ReplayFallback allows live calls when a replay lookup misses.
	ReplayFallback bool
	// ReplayModel enables model-call replay in addition to tool replay.
	ReplayModel bool
	// ReplayPatch, when set, is applied to the (fresh) workspace before
	// verification during offline replay, so the recorded workspace changes
	// are re-materialized and verification reflects the recorded outcome.
	ReplayPatch string
	// StoreDriver selects the run store backend: "json" (default) or "sqlite".
	StoreDriver string
}

// Harness is a fully assembled, runnable harness for one run.
type Harness struct {
	Spec      *spec.HarnessSpec
	RunID     string
	UserID    string
	Workspace *workspace.Instance
	Runtime   hlruntime.Runtime
	Store     store.Store
	Recorder  recorder.Recorder
	TracePath string
	RunRecord *store.Run
	Replay    *hlruntime.ReplayConfig

	workspaceMgr workspace.Workspace
	sandbox      sandbox.Sandbox
	cost         *cost.Calculator
	replayPatch  string
}

// Result is the outcome of one harness run.
type Result struct {
	Run           *store.Run
	Metrics       store.Metrics
	TracePath     string
	WorkspaceDiff *workspace.Diff
	Verification  *store.VerificationResult
}

// Build assembles a Harness: workspace -> tools -> runtime -> recorder -> store.
func Build(ctx context.Context, s *spec.HarnessSpec, opts Options) (*Harness, error) {
	if opts.HarnessDir == "" {
		opts.HarnessDir = ".harness"
	}
	id := "run-" + uuid.NewString()[:8]

	st, err := openStore(opts)
	if err != nil {
		return nil, err
	}

	var ws *workspace.Instance
	var wsMgr workspace.Workspace
	if opts.Repo != "" || opts.Commit != "" {
		wsMgr = git.New(filepath.Join(opts.HarnessDir, "workspaces"))
		ws, err = wsMgr.Create(ctx, id, workspace.Spec{Repo: opts.Repo, Commit: opts.Commit})
		if err != nil {
			return nil, err
		}
	}

	sb, err := newSandbox(s)
	if err != nil {
		return nil, err
	}

	tools, err := buildTools(ctx, s, ws, sb)
	if err != nil {
		return nil, err
	}

	rt, err := newRuntime(s, tools)
	if err != nil {
		return nil, err
	}

	tracePath := filepath.Join(opts.HarnessDir, "traces", id+".jsonl")
	rec, err := recorder.NewJSONL(tracePath)
	if err != nil {
		return nil, err
	}

	wsRoot := ""
	if ws != nil {
		wsRoot = ws.Root
	}
	run := &store.Run{
		ID:             id,
		HarnessVersion: s.Version,
		HarnessName:    s.Name,
		Repository:     opts.Repo,
		Commit:         opts.Commit,
		Workspace:      wsRoot,
		TracePath:      tracePath,
		StartedAt:      time.Now(),
		Status:         store.StatusRunning,
	}
	if y, err := yaml.Marshal(s); err == nil {
		run.SpecYAML = string(y)
	}
	if envJSON, err := json.Marshal(reproduce.Capture()); err == nil {
		run.Environment = string(envJSON)
	}
	if err := st.CreateRun(ctx, run); err != nil {
		return nil, err
	}

	replayCfg, err := buildReplay(ctx, opts, id, wsRoot)
	if err != nil {
		return nil, err
	}

	return &Harness{
		Spec:         s,
		RunID:        id,
		UserID:       opts.UserID,
		Workspace:    ws,
		workspaceMgr: wsMgr,
		Runtime:      rt,
		Store:        st,
		Recorder:     rec,
		TracePath:    tracePath,
		RunRecord:    run,
		Replay:       replayCfg,
		sandbox:      sb,
		cost:         cost.New(s.Pricing),
		replayPatch:  opts.ReplayPatch,
	}, nil
}

// newRuntime builds the runtime adapter selected by the harness spec. The
// Runtime interface is the only boundary HarnessLab core depends on; adding a
// runtime is a new adapter plus a case here.
func newRuntime(s *spec.HarnessSpec, tools []tool.Tool) (hlruntime.Runtime, error) {
	switch s.Runtime.Type {
	case "", spec.RuntimeTRPC:
		return trpc.New(s, tools)
	case spec.RuntimeCodex:
		return codexrt.New(s)
	default:
		return nil, fmt.Errorf("builder: unsupported runtime type %q", s.Runtime.Type)
	}
}

// applyWorkspacePatch applies a unified git diff to the workspace root,
// re-materializing recorded file changes on a fresh worktree.
func applyWorkspacePatch(ctx context.Context, root, patch string) error {
	if strings.TrimSpace(patch) == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "apply", "--whitespace=nowarn", "-")
	cmd.Stdin = strings.NewReader(patch)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git apply: %w: %s", err, stderr.String())
	}
	return nil
}

// newSandbox builds the sandbox configured by the spec.
func newSandbox(s *spec.HarnessSpec) (sandbox.Sandbox, error) {
	timeout, err := s.Sandbox.CommandTimeout()
	if err != nil {
		return nil, err
	}
	return sandbox.New(sandbox.Spec{
		Type:            s.Sandbox.Type,
		Network:         s.Sandbox.Network,
		Image:           s.Sandbox.Image,
		Timeout:         timeout,
		AllowedCommands: s.Sandbox.AllowedCommands,
		DeniedCommands:  s.Sandbox.DeniedCommands,
	})
}

// openStore opens the run store backend selected by Options.StoreDriver.
func openStore(opts Options) (store.Store, error) {
	dir := filepath.Join(opts.HarnessDir, "store")
	switch opts.StoreDriver {
	case "", "json":
		return jsonstore.New(dir)
	case "sqlite":
		return sqlitestore.New(filepath.Join(dir, "harness.db"))
	default:
		return nil, fmt.Errorf("builder: unsupported store driver %q (supported: json, sqlite)", opts.StoreDriver)
	}
}

// buildReplay opens the replay store for this run: record mode by default,
// or strict/fallback replay from another run's store.
func buildReplay(ctx context.Context, opts Options, id, wsRoot string) (*hlruntime.ReplayConfig, error) {
	if opts.ReplayFrom != "" {
		path := filepath.Join(opts.HarnessDir, "replay", opts.ReplayFrom, "entries.jsonl")
		store, err := replay.NewJSONStore(path)
		if err != nil {
			return nil, err
		}
		mode := hlruntime.ReplayStrict
		if opts.ReplayFallback {
			mode = hlruntime.ReplayFallback
		}
		return &hlruntime.ReplayConfig{
			Mode:          mode,
			Store:         store,
			Canonicalizer: &replay.Canonicalizer{WorkspaceRoot: wsRoot, TempDir: os.TempDir()},
			ReplayModel:   opts.ReplayModel,
		}, nil
	}
	path := filepath.Join(opts.HarnessDir, "replay", id, "entries.jsonl")
	store, err := replay.NewJSONStore(path)
	if err != nil {
		return nil, err
	}
	return &hlruntime.ReplayConfig{
		Mode:          hlruntime.ReplayRecord,
		Store:         store,
		Canonicalizer: &replay.Canonicalizer{WorkspaceRoot: wsRoot, TempDir: os.TempDir()},
		ReplayModel:   opts.ReplayModel,
	}, nil
}

// Run executes the task, records the trace and metrics, verifies the result
// and persists the final run record. onEvent, when non-nil, receives every
// normalized event as it is produced (used by the CLI for live progress).
func (h *Harness) Run(ctx context.Context, task string, onEvent func(hlruntime.RunEvent)) (*Result, error) {
	h.RunRecord.Task = task
	if err := h.Store.UpdateRun(ctx, h.RunRecord); err != nil {
		return nil, err
	}

	userID := h.UserID
	if userID == "" {
		userID = "harness"
	}
	req := hlruntime.RunRequest{
		RunID:         h.RunID,
		Task:          task,
		UserID:        userID,
		SessionID:     h.RunID,
		WorkspaceRoot: h.workspaceRoot(),
		Replay:        h.Replay,
	}

	evCh, err := h.Runtime.Run(ctx, req)
	if err != nil {
		h.finish(ctx, store.StatusError, store.Metrics{}, nil, nil)
		return nil, err
	}

	var metrics store.Metrics
	agentErrored := false
	started := time.Now()
	for ev := range evCh {
		if onEvent != nil {
			onEvent(ev)
		}
		if err := h.Recorder.Record(ctx, ev); err != nil {
			log.Errorf("builder: record event: %v", err)
		}
		if sink, ok := h.Store.(store.EventSink); ok {
			payload, _ := json.Marshal(ev)
			if err := sink.AppendEvent(ctx, ev.RunID, ev.ParentID, string(ev.Type), ev.Timestamp, payload); err != nil {
				log.Errorf("builder: sink event: %v", err)
			}
		}
		switch ev.Type {
		case hlruntime.EventModelEnd:
			metrics.ModelCalls++
			if ev.Model != nil {
				metrics.InputTokens += int64(ev.Model.TokensIn)
				metrics.OutputTokens += int64(ev.Model.TokensOut)
			}
		case hlruntime.EventToolStart:
			metrics.ToolCalls++
		case hlruntime.EventError:
			agentErrored = true
		}
	}
	metrics.DurationMS = time.Since(started).Milliseconds()

	// Offline replay: re-materialize the recorded workspace changes on the
	// fresh worktree so verification reflects the recorded outcome.
	if h.replayPatch != "" && h.Workspace != nil {
		if err := applyWorkspacePatch(ctx, h.Workspace.Root, h.replayPatch); err != nil {
			log.Warnf("builder: apply replay patch: %v", err)
		}
	}

	// Verification is a first-class citizen of a harness: it must actually
	// pass (no fake PASS from a text-only answer), and for coding tasks the
	// workspace must have been modified.
	ver := h.verifyResult(ctx, h.Spec.Verification)

	var diff *workspace.Diff
	workspaceChanged := false
	if h.Workspace != nil && h.workspaceMgr != nil {
		if d, err := h.workspaceMgr.Diff(ctx, h.Workspace); err == nil {
			diff = d
			workspaceChanged = d.Changed()
		}
	}
	if ver != nil {
		ver.WorkspaceChanged = workspaceChanged
	}
	metrics.VerificationPassed = ver == nil || ver.Passed
	metrics.WorkspaceChanged = workspaceChanged

	status := computeStatus(agentErrored, ver, workspaceChanged,
		h.Spec.Success.RequireVerification(), h.Spec.Success.RequireChange(),
		ctx.Err() != nil)
	metrics.Success = status == store.StatusPassed

	h.finish(ctx, status, metrics, diff, ver)
	metrics = h.RunRecord.Metrics // finish() stamps CostUSD into the record
	return &Result{
		Run:           h.RunRecord,
		Metrics:       metrics,
		TracePath:     h.TracePath,
		WorkspaceDiff: diff,
		Verification:  ver,
	}, nil
}

// computeStatus applies the harness success rules. Verification failures only
// fail the run when requireVerification is set (default true); an unmodified
// workspace only fails the run when requireChange is set (default false).
func computeStatus(agentErrored bool, ver *store.VerificationResult, workspaceChanged bool,
	requireVerification, requireChange, cancelled bool) store.Status {
	if cancelled {
		return store.StatusCancelled
	}
	if agentErrored {
		return store.StatusFailed
	}
	if requireVerification && ver != nil && !ver.Passed {
		return store.StatusFailed
	}
	if requireChange && !workspaceChanged {
		return store.StatusFailed
	}
	return store.StatusPassed
}

// Close releases harness resources (sandbox + workspace + recorder).
func (h *Harness) Close() error {
	var errs []error
	if h.sandbox != nil {
		if err := h.sandbox.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if h.Recorder != nil {
		if err := h.Recorder.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if h.Workspace != nil && h.workspaceMgr != nil {
		if err := h.workspaceMgr.Destroy(context.Background(), h.Workspace); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (h *Harness) workspaceRoot() string {
	if h.Workspace != nil {
		return h.Workspace.Root
	}
	return "."
}

func (h *Harness) finish(ctx context.Context, status store.Status, metrics store.Metrics, diff *workspace.Diff, ver *store.VerificationResult) {
	h.RunRecord.Status = status
	h.RunRecord.Metrics = metrics
	h.RunRecord.FinishedAt = time.Now()
	if h.cost != nil {
		h.RunRecord.Metrics.CostUSD = h.cost.Calculate(
			h.Spec.Model.Provider, h.Spec.Model.Name,
			metrics.InputTokens, metrics.OutputTokens,
		)
	}
	if diff != nil {
		h.RunRecord.WorkspacePatch = diff.Patch
	}
	if ver != nil {
		h.RunRecord.Verification = *ver
	}
	if err := h.Store.UpdateRun(ctx, h.RunRecord); err != nil {
		log.Errorf("builder: persist run %s: %v", h.RunRecord.ID, err)
	}
}

// verifyResult runs the harness's verification commands once after the agent
// finishes (strategy: final) and returns the structured outcome. Commands run
// through the configured sandbox. It returns nil when no verification is
// configured.
func (h *Harness) verifyResult(ctx context.Context, v spec.VerificationSpec) *store.VerificationResult {
	if v.Strategy == spec.VerificationNone || len(v.Commands) == 0 {
		return nil
	}
	ver := &store.VerificationResult{Passed: true}
	started := time.Now()
	for _, cmd := range v.Commands {
		res := h.sandbox.Run(ctx, sandbox.Command{Dir: h.workspaceRoot(), Command: cmd})
		passed := res.Err == nil
		ver.Commands = append(ver.Commands, store.CommandResult{
			Command:  cmd,
			Passed:   passed,
			ExitCode: res.ExitCode,
			Output:   clip([]byte(res.Output)),
		})
		if !passed {
			ver.Passed = false
		}
		p, f := countTests(res.Output)
		ver.TestsPassed += p
		ver.TestsFailed += f
	}
	ver.DurationMS = time.Since(started).Milliseconds()
	return ver
}

// buildTools wires tool groups from the spec. The shell group uses the
// framework's hostexec toolset rooted at the workspace when no sandbox is
// configured, or a sandbox-routed exec tool otherwise; the filesystem group
// wires the framework's file read/write/replace/search tools, scoped to the
// workspace.
func buildTools(ctx context.Context, s *spec.HarnessSpec, ws *workspace.Instance, sb sandbox.Sandbox) ([]tool.Tool, error) {
	baseDir := "."
	if ws != nil {
		baseDir = ws.Root
	}
	var tools []tool.Tool
	if s.Tools.Filesystem {
		fs, err := file.NewToolSet(
			file.WithBaseDir(baseDir),
			file.WithReadFileEnabled(true),
			file.WithReadMultipleFilesEnabled(true),
			file.WithListFileEnabled(true),
			file.WithSearchFileEnabled(true),
			file.WithSearchContentEnabled(true),
			file.WithReplaceContentEnabled(true),
			file.WithSaveFileEnabled(true),
		)
		if err != nil {
			return nil, fmt.Errorf("builder: file toolset: %w", err)
		}
		tools = append(tools, fs.Tools(ctx)...)
	}
	if !s.Tools.Shell {
		return tools, nil
	}
	if s.Sandbox.SandboxEnabled() {
		return append(tools, sandbox.NewExecTool(sb, baseDir)), nil
	}
	ts, err := hostexec.NewToolSet(hostexec.WithBaseDir(baseDir))
	if err != nil {
		return nil, fmt.Errorf("builder: hostexec toolset: %w", err)
	}
	return append(tools, ts.Tools(ctx)...), nil
}

// countTests best-effort parses "go test -v" output for pass/fail counts.
func countTests(output string) (passed, failed int) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "--- PASS:"):
			passed++
		case strings.HasPrefix(line, "--- FAIL:"):
			failed++
		}
	}
	return passed, failed
}

func clip(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 600 {
		return s[:600] + "..."
	}
	return s
}
