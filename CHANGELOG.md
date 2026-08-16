# Changelog

All notable changes to HarnessLab are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Demo GIF** (`docs/demo.gif`): 12-second animated terminal demo rendered
  from real CLI output (run → runs → replay → diff), plus a static preview.
- **Community launch materials** (`docs/announcement.md`): HN / Reddit / X /
  知乎 / 掘金 copy around the "Same model. Same task. Same repository.
  Different harness." narrative, with real benchmark and replay evidence.
- **README**: demo GIF, installation quick-start (Release binaries or
  `go build`), go-version/release badges.

### Fixed

- **Offline replay hash mismatches**: the canonicalizer now normalizes nested
  JSON strings (tool results embedded in message content), and recorded tool
  outputs are normalized (`$WORKSPACE`) at store time — so every later model
  call whose context carries a prior tool result replays cleanly in a fresh
  worktree (full offline replay: 4 model + 3 tool calls served in ~16ms).
- **Replay verification**: `harness replay` / `reproduce` re-apply the
  recorded workspace patch before verification, so a successfully recorded run
  replays as PASSED (previously verification ran against the untouched fresh
  worktree and reported failed).

## [v0.3.0-alpha] - 2026-08-16

Benchmark v3 milestone: a larger, categorized, statistically rigorous public
benchmark plus open-source UX (templates, binary releases).

### Added

- **LLM Harness Optimizer**: `harness optimize --llm` generates full harness
  candidates from the failure analysis via the configured LLM
  (DeepSeek/OpenAI), validates them against the real spec schema (hallucinated
  fields are skipped), and writes them to `.harness/candidates/candidate-XXX.yaml`
  with metadata (parent, reason, expected_effect) — never over `harness.yaml`.
  `harness optimize --evaluate` runs the dev benchmark (baseline + candidates),
  selects the Pareto front (pass up, tokens/cost down), validates the selected
  candidates on the holdout set and **rejects Dev-only wins** (dev improves but
  holdout regresses). Evaluation reuses the bench scheduler and dev/holdout
  tasksets.
- **Codex CLI runtime adapter** (`runtime.type: codex`): drives a locally
  installed `codex` binary via `codex exec --json` (prompt on stdin, JSONL
  event stream parsed into the same `HarnessEvent` dialect). Configured via
  `runtime.codex` (binary, model, sandbox, ask_for_approval, extra_args, env).
  HarnessLab core is now proven runtime-agnostic: same workspace, trace,
  verification, cost and benchmark pipeline on tRPC-Agent-Go and Codex CLI.
  Offline replay remains a trpc-runtime feature.
- Shared runner/normalizer pipeline (`trpc.RunFrameworkAgent`) so every
  runtime adapter emits the same normalized events and metrics.

### Fixed

- Codex runtime defaults (binary/sandbox/approval) are only filled for
  `runtime.type: codex`, so marshaled trpc harnesses no longer carry a
  `runtime.codex` block.

## [v0.3.0-alpha] - 2026-08-16

Benchmark v3 milestone: a larger, categorized, statistically rigorous public
benchmark plus open-source UX (templates, binary releases).

### Added

- **Task taxonomy**: benchmark tasks carry `category` (basic, debugging,
  multi-file, refactor, testing, concurrency, context-heavy, tool-heavy) and
  `set` (dev, holdout); task loading recurses into category directories
- **Dev / holdout split**: `benchmarks/tasksets/dev.yaml` + `holdout.yaml`;
  `harness bench --set dev|holdout` and `--taskset <file>` select subsets so
  the future optimizer optimizes on dev and validates on holdout
- **Repeated runs**: `harness bench --repeat N` produces N independent run
  records per task × harness (never overwritten)
- **Statistical reports**: per-variant and per-task mean, median, stddev,
  P50, P90 and a 95% confidence interval of the mean for tokens, cost and
  latency; raw per-run data (incl. CostUSD) preserved in the JSON report
- **Harness ablation matrix**: `benchmarks/harnesses/h0-baseline..h6-full`
  add planner, verification, retry, repo-map context and skills one at a
  time; `benchmarks/matrices/harness-ablation.yaml` runs all seven
- **Adaptive context**: `context.strategy: none | repo-map` injects an
  auto-generated repository structure summary into the agent instruction
- **Skills**: `skills.enabled` + `skills.list` inject named working
  procedures as instruction sections (extension point for tool-backed skills)
- **Benchmark v3 taskset**: 30+ real coding tasks across 8 categories against
  the `bench/tasks-v2` seed (`075550a`)
- **Open source UX**: issue templates (bug report / feature request / config),
  pull request template, GoReleaser binary releases (linux/darwin amd64+arm64,
  windows amd64) via a tag-triggered release workflow

### Fixed

- Run metrics now carry the real `CostUSD` (previously 0 in reports because
  `finish()` stamped the record but not the returned metrics copy)
- Benchmark task verification no longer clobbers a harness that deliberately
  disables verification (ablation baseline H0)

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
  - `sandbox.NewExecTool` — sandbox-routed `exec_command` tool used instead
    of the host-exec toolset when a sandbox is configured
- `tools.filesystem` group: framework file read/write/replace/search tools
  scoped to the workspace, so coding agents can edit files cross-platform
- **Cost model**: `pricing` section in HarnessSpec (provider → model →
  input/output per million USD, `*` wildcard supported); `Run.Metrics.CostUSD`
  is computed when pricing is configured and flows into reports
- **Environment validation**: run records capture the toolchain environment;
  `harness reproduce --env-mode warn|strict|ignore` compares recorded vs
  current OS/arch/Go/git/HarnessLab/tRPC-Agent-Go and reports drift
  (`environment drift detected`; strict aborts on mismatch)
- **Public benchmark v1** (`benchmarks/`): 10 tasks against the seeded
  `bench/tasks-v1` commit of the HarnessLab repo — 4 implementation
  regressions (each caught by a failing test), 2 doc typos, 2 add-a-test,
  2 add-a-feature — plus a base harness with pricing and a variant matrix
- Benchmark `Matrix` gains a `tools_filesystem` dimension
- Bench progress lines now show job errors
- `workspace.Diff.Untracked` — new files created by the agent now count as a
  workspace change (previously only `git diff` tracked changes)
- Benchmark reports: `Ver` and `Chg` columns + per-run verification flags
- Benchmark tasks can override the harness `success` criteria
- Optimize: new `no_change` failure pattern ("repository task without
  workspace changes — likely hallucinated output")
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
