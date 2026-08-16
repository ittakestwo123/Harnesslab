package trpc

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
	"trpc.group/trpc-go/trpc-agent-go/tool"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	"github.com/ittakestwo123/Harnesslab/internal/replay"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
)

// Runtime adapts a tRPC-Agent-Go runner to the HarnessLab runtime interface.
// One Runtime instance is built per HarnessSpec.
type Runtime struct {
	spec  *spec.HarnessSpec
	tools []tool.Tool
}

// New creates a TRPC runtime for the given harness spec and tool set.
func New(s *spec.HarnessSpec, tools []tool.Tool) (*Runtime, error) {
	if s.Runtime.Type != spec.RuntimeTRPC {
		return nil, fmt.Errorf("trpc runtime: unsupported runtime type %q", s.Runtime.Type)
	}
	return &Runtime{spec: s, tools: tools}, nil
}

// Run executes the task once against the tRPC runner and returns a normalized
// event stream (run_start ... run_end).
func (r *Runtime) Run(ctx context.Context, req hlruntime.RunRequest) (<-chan hlruntime.RunEvent, error) {
	m, err := newModel(r.spec.Model)
	if err != nil {
		return nil, err
	}

	if req.UserID == "" {
		req.UserID = "harness"
	}
	if req.SessionID == "" {
		req.SessionID = req.RunID
	}

	gen := model.GenerationConfig{Stream: true}
	if r.spec.Model.Temperature != nil {
		gen.Temperature = r.spec.Model.Temperature
	}

	// Replay wiring: wrap the model for model replay and attach tool
	// callbacks for tool replay/recording.
	var canon *replay.Canonicalizer
	if cfg := req.Replay; cfg != nil && cfg.Store != nil && cfg.Mode != "" {
		canon = cfg.Canonicalizer
		if canon == nil {
			canon = &replay.Canonicalizer{WorkspaceRoot: req.WorkspaceRoot, TempDir: os.TempDir()}
		}
		if cfg.ReplayModel {
			m = newReplayModel(m, cfg, canon)
		}
	}

	opts := []llmagent.Option{
		llmagent.WithModel(m),
		llmagent.WithInstruction(buildInstruction(r.spec, req.WorkspaceRoot)),
		llmagent.WithGenerationConfig(gen),
	}
	if len(r.tools) > 0 {
		opts = append(opts, llmagent.WithTools(r.tools))
	}
	if cbs := buildReplayToolCallbacks(req.Replay, canon); cbs != nil {
		opts = append(opts, llmagent.WithToolCallbacks(cbs))
	}
	ag := llmagent.New(r.spec.Agent.Name, opts...)

	rr := runner.NewRunner(r.spec.Name, ag,
		runner.WithSessionService(inmemory.NewSessionService()),
	)

	out := make(chan hlruntime.RunEvent, 128)
	go func() {
		defer close(out)
		started := time.Now()
		norm := newNormalizer(req.RunID)
		step := 1

		// Budget timeout bounds the whole run; it must live inside the
		// goroutine so it is not cancelled when Run returns.
		runCtx := ctx
		if d, err := r.spec.Timeout(); err == nil && d > 0 {
			var cancel context.CancelFunc
			runCtx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}

		out <- runEvent(req.RunID, hlruntime.EventRunStart, step)

		evCh, err := rr.Run(runCtx, req.UserID, req.SessionID,
			model.NewUserMessage(req.Task),
			agent.WithRequestID(req.RunID),
		)
		if err != nil {
			step++
			out <- runEvent(req.RunID, hlruntime.EventError, step)
			step++
			out <- runEvent(req.RunID, hlruntime.EventRunEnd, step)
			return
		}

		for ev := range evCh {
			for _, he := range norm.normalize(ev) {
				out <- he
			}
		}

		step = norm.step + 1
		end := runEvent(req.RunID, hlruntime.EventRunEnd, step)
		end.DurationMS = time.Since(started).Milliseconds()
		out <- end
	}()
	return out, nil
}

// newModel maps a ModelSpec to a framework model instance.
func newModel(m spec.ModelSpec) (model.Model, error) {
	switch m.Provider {
	case "openai":
		return openai.New(m.Name), nil
	case "deepseek":
		return openai.New(m.Name, openai.WithVariant(openai.VariantDeepSeek)), nil
	default:
		return nil, fmt.Errorf("trpc runtime: unsupported model provider %q (supported: openai, deepseek)", m.Provider)
	}
}

// buildInstruction compiles the harness's planning/verification/tool settings
// into the agent's system instruction. workspaceRoot enables adaptive-context
// strategies (e.g. repo-map) that need to inspect the repository.
func buildInstruction(s *spec.HarnessSpec, workspaceRoot string) string {
	var b strings.Builder
	b.WriteString("You are a coding agent working inside a repository workspace.")
	if s.Agent.Instruction != "" {
		b.WriteString("\n\n" + s.Agent.Instruction)
	}
	switch s.Planning.Strategy {
	case spec.PlanningTodo:
		b.WriteString("\n\nPlanning: Before making changes, create a short TODO list of steps and work through them one at a time.")
	}
	if s.Context.Strategy == spec.ContextRepoMap {
		if m := repoMap(workspaceRoot); m != "" {
			b.WriteString("\n\nRepository map (auto-generated; read files for details):\n" + m)
		}
	}
	if s.Skills.Enabled && len(s.Skills.List) > 0 {
		b.WriteString("\n\nSkills (follow these working procedures):")
		for _, skill := range s.Skills.List {
			b.WriteString("\n- " + strings.TrimSpace(skill))
		}
	}
	if len(s.Verification.Commands) > 0 {
		b.WriteString("\n\nVerification: You must run the following commands to verify your work and iterate until they pass:")
		for _, c := range s.Verification.Commands {
			b.WriteString("\n  - " + c)
		}
	}
	if s.Tools.Shell {
		b.WriteString("\n\nUse the exec_command tool to run shell commands such as builds and tests.")
	}
	return b.String()
}

// repoMap builds a compact, deterministic repository structure summary from a
// shallow file walk. It returns "" when root is unavailable. This is the
// smallest useful adaptive-context strategy: it shows the agent where things
// live without burning tokens on full file contents.
func repoMap(root string) string {
	if root == "" {
		return ""
	}
	const maxEntries = 200
	type entry struct {
		path string
		size int64
	}
	var entries []entry
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == ".harness" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		entries = append(entries, entry{path: filepath.ToSlash(rel), size: info.Size()})
		return nil
	})
	if len(entries) > maxEntries {
		// Keep the layout readable: truncate to the first maxEntries files.
		entries = entries[:maxEntries]
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s (%d B)\n", e.path, e.size)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// runEvent builds a plain run event with a fresh id and step.
func runEvent(runID string, t hlruntime.EventType, step int) hlruntime.RunEvent {
	return hlruntime.RunEvent{
		ID:        fmt.Sprintf("%s-%05d", runID, step),
		RunID:     runID,
		Type:      t,
		Timestamp: time.Now(),
		Step:      step,
	}
}
