# Changelog

All notabla changes to HarnessLab are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.2.0-alpha] - 2026-08-16

Trustworthy benchmark milestone: real success criteria, sandboxed execution,
cost accounting, environment validation, and a public 10-task benchmark.

### Added

- **Success criteria** in HarnessSpec: `success.require_verification_pass`
  (default true) and `success.require_workspace_change` (default false) so
  text-only hallucinated answers can no longer fake a PASS
- Structured `VerificationResult` (per-command results, exit codes, clipped
  output, best-effort test pass/fail counts) persisted on run records
- **Sandbox** (`sandbox.type: none | process | docker | bwrap`) for agent
  shell commands and verification:
  - `process`: cwd isolation, secret env scrubbing, per-command timeouts,
    command allow/deny lists
  - `docker`: container execution with the workspace mounted at `/workspace`
  - `bwrap`: bubblewrap sandbox on Linux
  - Reliable whole-tree kills on Windows via job objects (KILL_ON_JOB_CLOSE)
  - `sandbox.NewExecTool` 鈥?sandbox-routed `exec_command` tool used instead
    of the host-exec toolset when a sandbox is configured
- `tools.filesystem` group: framework file read/write/replace/search tools
  scoped to the workspace, so coding agents can edit files cross-platform
- **Cost model**: `pricing` section in HarnessSpec (provider 鈫?model 鈫?  input/output per million USD, `*` wildcard supported); `Run.Metrics.CostUSD`
  is computed when pricing is configured and flows into reports
- **Environment validation**: run records capture the toolchain environment;
  `harness reproduce --env-mode warn|strict|ignore` compares recorded vs
  current OS/arch/Go/git/HarnessLab/tRPC-Agent-Go and reports drift
  (`environment drift detected`; strict aborts on mismatch)
- **Public benchmark v1** (`benchmarks/`): 10 tasks against the seeded
  `bench/tasks-v1` commit of the HarnessLab repo 鈥?4 implementation
  regressions (each caught by a failing test), 2 doc typos, 2 add-a-test,
  2 add-a-feature 鈥?plus a base harness with pricing and a variant matrix
- Benchmark `Matrix` gains a `tools_filesystem` dimension
- Bench progress lines now show job errors
- `workspace.Diff.Untracked` 鈥?new files created by the agent now count as a
  workspace change (previously only `git diff` tracked changes)
- Benchmark reports: `Ver` and `Chg` columns + per-run verification flags
- Benchmark tasks can override the harness `success` criteria
- Optimize: new `no_change` failure pattern ("repository task without
  workspace changes 鈥?likely hallucinated output")
- CLI version is overridable via `-ldflags -X .../cli.version=...`

### Fixed

- Windows verification commands containing quotes were mangled by Go's
  `cmd /c` argument quoting (e.g. `findstr "pattern" file` failed to match).
  Commands now run through a temporary batch file, parsed exactly as typed.
- Mojibake corruption of non-ASCII characters introduced during the module
  path rename (em-dashes, arrows, section signs restored).
- Concurrent benchmark runs no longer race on the shared git mirror
  (creation is guarded by a lock file; workers wait for the lock to release)

## [v0.1.0-alpha.1] - 2026-08-16

First frozen baseline: the full Build 鈫?Trace 鈫?Replay 鈫?Diff 鈫?Benchmark 鈫?Reproduce 鈫?Optimize loop implemented on top of tRPC-Agent-Go.

### Added

- `harness init` 鈥?generate a `.harness/` directory with a default `harness.yaml`
- `harness run "<task>"` 鈥?run an agent with a harness; stream progress, verify,
  and persist run record + JSONL trace (records tool & model calls for replay)
- `harness trace <run-id>` 鈥?render a run's trajectory
- `harness runs` 鈥?list recorded runs
- `harness replay <run-id>` 鈥?offline replay from the recorded replay store
  (strict; `--fallback` for live-on-miss)
- `harness diff <run-a> <run-b>` 鈥?metrics table, aligned trajectory, and
  first-divergence detection (LCS alignment)
- `harness bench <tasks> [--matrix m.yaml]` 鈥?tasks x harness variants on a
  bounded worker pool with token/time budget control, retry waves, and JSON
  reports
- `harness snapshot` / `harness export <run-id>` / `harness reproduce <run-id|bundle>`
  鈥?harness.lock, reproducible `.harness` bundles, and offline reproduction
- `harness optimize [--report bench.json]` 鈥?failure analysis, candidate
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
