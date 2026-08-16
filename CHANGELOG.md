# Changelog

All notabla changes to HarnessLab are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Sandbox** (`sandbox.type: none | process | docker | bwrap`) for agent
  shell commands and verification:
  - `process`: cwd isolation, secret env scrubbing, per-command timeouts,
    command allow/deny lists
  - `docker`: container execution with the workspace mounted at `/workspace`
  - `bwrap`: bubblewrap sandbox on Linux
  - Reliable whole-tree kills on Windows via job objects (KILL_ON_JOB_CLOSE)
  - `sandbox.NewExecTool` — sandbox-routed `exec_command` tool used instead
    of the host-exec toolset when a sandbox is configured
- `success` section in HarnessSpec: `require_verification_pass` (default true)
  and `require_workspace_change` (default false) so text-only hallucinated
  answers can no longer fake a PASS
- Structured `VerificationResult` (per-command results, exit codes, clipped
  output, best-effort test pass/fail counts) persisted on run records
- `workspace.Diff.Untracked` — new files created by the agent now count as a
  workspace change (previously only `git diff` tracked changes)
- Benchmark reports: `Ver` and `Chg` columns + per-run verification flags
- Benchmark tasks can override the harness `success` criteria
- Optimize: new `no_change` failure pattern ("repository task without
  workspace changes — likely hallucinated output")

### Fixed

- Windows verification commands containing quotes were mangled by Go's
  `cmd /c` argument quoting (e.g. `findstr "pattern" file` failed to match).
  Commands now run through a temporary batch file, parsed exactly as typed.
- Mojibake corruption of non-ASCII characters introduced during the module
  path rename (em-dashes, arrows, section signs restored).

## [v0.1.0-alpha.1] - 2026-08-16

First frozen baseline: the full Build → Trace → Replay → Diff → Benchmark →
Reproduce → Optimize loop implemented on top of tRPC-Agent-Go.

### Added

- `harness init` — generate a `.harness/` directory with a default `harness.yaml`
- `harness run "<task>"` — run an agent with a harness; stream progress, verify,
  and persist run record + JSONL trace (records tool & model calls for replay)
- `harness trace <run-id>` — render a run's trajectory
- `harness runs` — list recorded runs
- `harness replay <run-id>` — offline replay from the recorded replay store
  (strict; `--fallback` for live-on-miss)
- `harness diff <run-a> <run-b>` — metrics table, aligned trajectory, and
  first-divergence detection (LCS alignment)
- `harness bench <tasks> [--matrix m.yaml]` — tasks x harness variants on a
  bounded worker pool with token/time budget control, retry waves, and JSON
  reports
- `harness snapshot` / `harness export <run-id>` / `harness reproduce <run-id|bundle>`
  — harness.lock, reproducible `.harness` bundles, and offline reproduction
- `harness optimize [--report bench.json]` — failure analysis, candidate
  harness changes, and Pareto front of benchmark variants
- HarnessSpec (YAML): runtime / agent / model / planning / tools /
  verification / retry / sandbox / budget with validation
- Runtime abstraction + tRPC-Agent-Go adapter with event normalization
- Git worktree workspace (mirror + detached worktree per run)
- JSON and SQLite run stores (SQLite auto-migrates old schemas)
- Replay engine: tool replay via before-tool callbacks, model replay via a
  wrapping model, canonicalizer for stable content hashing

### Verified

- Real DeepSeek runs: model-only and coding tasks with tools + worktree
- Offline replay reproduces a live run without calling any model/tool API
  (16 ms vs 2.6 s, identical metrics)
- Benchmark: 2 tasks x 4 harness variants = 8 real runs
- Reproduce from store and from a `.harness` bundle with a bogus API key
- `go build ./...`, `go vet ./...`, `go test ./...` green (28 tests)
