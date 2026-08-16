// Package codex implements the Codex CLI runtime adapter: a HarnessLab
// Runtime that drives a locally installed `codex` binary via
// `codex exec --json` and normalizes its JSONL event stream into the same
// HarnessEvent dialect as the tRPC runtime. It reuses the framework's
// agent/codex implementation for CLI invocation and transcript parsing, and
// the shared runner/normalizer pipeline exported by the trpc adapter
// (trpc.RunFrameworkAgent).
//
// This adapter proves that HarnessLab Core is runtime-agnostic: the same
// workspace, trace, verification, cost and benchmark pipeline runs against
// a completely different agent runtime.
//
// Differences vs the trpc runtime:
//   - Tools: the codex CLI brings its own tools; the spec `tools:` section is
//     ignored (codex sandbox/approval are configured under runtime.codex).
//   - Offline replay: not supported for codex runs (the CLI is the runtime);
//     traces are still recorded and replayable as JSONL.
//   - The harness instruction (planning/verification/context/skills) is
//     prepended to the prompt because codex has no separate system channel.
package codex

import (
	"context"
	"fmt"

	"trpc.group/trpc-go/trpc-agent-go/agent/codex"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
	trpcrt "github.com/ittakestwo123/Harnesslab/internal/runtime/trpc"
)

// Runtime adapts the Codex CLI to the HarnessLab runtime interface.
type Runtime struct {
	spec *spec.HarnessSpec
}

// New creates a Codex CLI runtime for the given harness spec.
func New(s *spec.HarnessSpec) (*Runtime, error) {
	if s.Runtime.Type != spec.RuntimeCodex {
		return nil, fmt.Errorf("codex runtime: unsupported runtime type %q", s.Runtime.Type)
	}
	return &Runtime{spec: s}, nil
}

// Run executes the task once via `codex exec --json` and returns a normalized
// event stream (run_start ... run_end).
func (r *Runtime) Run(ctx context.Context, req hlruntime.RunRequest) (<-chan hlruntime.RunEvent, error) {
	cfg := r.spec.Runtime.Codex
	bin := cfg.Binary
	if bin == "" {
		bin = "codex"
	}

	opts := []codex.Option{
		codex.WithBin(bin),
		// One fresh `codex exec` per run: no thread resume across runs.
		codex.WithResumeEnabled(false),
		codex.WithGlobalArgs("--sandbox", cfg.Sandbox),
		codex.WithGlobalArgs("--ask-for-approval", cfg.AskForApproval),
		codex.WithWorkDir(req.WorkspaceRoot),
		// The harness instruction becomes a prompt prefix (codex has no
		// separate system-message channel).
		codex.WithMessageBuilder(func(_ context.Context, args *codex.MessageBuilderArgs) (string, error) {
			return trpcrt.BuildInstruction(r.spec, req.WorkspaceRoot) + "\n\n" + args.Message.Content, nil
		}),
	}
	if cfg.Model != "" {
		opts = append(opts, codex.WithExtraArgs("--model", cfg.Model))
	}
	if len(cfg.ExtraArgs) > 0 {
		opts = append(opts, codex.WithExtraArgs(cfg.ExtraArgs...))
	}
	if len(cfg.Env) > 0 {
		opts = append(opts, codex.WithEnv(cfg.Env...))
	}

	ag, err := codex.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("codex runtime: %w", err)
	}

	return trpcrt.RunFrameworkAgent(ctx, r.spec, ag, req)
}
