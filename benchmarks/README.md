# HarnessLab Benchmark v3

A public, reproducible coding-agent benchmark: **same model, same repository,
same tasks — different harnesses**. Every task is verified by real repository
state (tests pass, workspace changed), so hallucinated answers cannot fake a
PASS.

## Reproducibility

- **Task repository**: https://github.com/ittakestwo123/Harnesslab
- **Task baseline commit**: `075550a` (branch `bench/tasks-v2`)
- The baseline commit intentionally seeds 4 implementation regressions (each
  caught by a failing test) plus 2 documentation typos; the benchmark tasks
  ask the agent to fix, extend or refactor the repository.
- The harness spec, matrices, harness files and every task are committed here,
  so anyone can re-run the whole benchmark and compare results.

## Layout

```text
benchmarks/
├── harness.yaml                 base harness (deepseek + process sandbox)
├── matrix.yaml                  2x2 variant matrix (planning x shell tools)
├── harnesses/                   full harness files for the ablation matrix
│   ├── h0-baseline.yaml         bare agent (no planning/verification/...)
│   ├── h1-planner.yaml          + TODO planning
│   ├── h2-verification.yaml     + final verification
│   ├── h3-retry.yaml            + model/tool retries
│   ├── h4-context.yaml          + repo-map adaptive context
│   ├── h5-skills.yaml           + working-procedure skills
│   └── h6-full.yaml             everything combined
├── matrices/
│   └── harness-ablation.yaml    the seven harnesses above
├── tasks/
│   ├── basic/                   docs & small additions
│   ├── debugging/               fix the seeded regressions
│   ├── multi-file/              cross-file moves with callers updated
│   ├── refactor/                rename/extract keeping tests green
│   ├── testing/                 add deterministic passing tests
│   ├── concurrency/             thread-safe code + concurrent tests
│   ├── context-heavy/           must read several files before acting
│   └── tool-heavy/              must use the shell to discover facts
├── tasksets/
│   ├── dev.yaml                 optimizer-visible tasks
│   └── holdout.yaml             held-out tasks for final evaluation
└── results/                     published reports
```

## Methodology

1. **Deterministic verification** — every PASS comes from real workspace state:
   `go test` for code tasks, `findstr` file assertions for text tasks. An agent
   that only produces a textual answer cannot pass
   (`success.require_verification_pass` + `success.require_workspace_change`).
2. **Dev / holdout split** — each task is labelled `dev` or `holdout`
   (`benchmarks/tasksets/*.yaml`). Harness optimization may only use `dev`;
   final evaluation must use `holdout`.
3. **Repeated runs** — agent behaviour is stochastic; `--repeat N` produces N
   independent run records per task x harness (never overwritten), so reports
   carry mean / median / stddev / P50 / P90 and a 95% confidence interval of
   the mean for tokens, cost and latency.
4. **Ablation** — the harness matrix adds one component at a time
   (H0..H6), so the benchmark can answer: does planning help? does
   verification reduce failures? does retry recover? does repo-map context cut
   tokens? do skills raise success? is the full harness worth its extra cost?
5. **Raw data preserved** — the JSON report keeps every per-run record
   (run id, repeat, tokens, cost, latency, verification, workspace change);
   traces are replayable offline via `harness replay <run-id>`.

## Running

```bash
# Build the CLI
go build -o harness ./cmd/harness

# Configure the model provider (spec: provider: deepseek)
export DEEPSEEK_API_KEY=sk-...

# Classic matrix: 10+ tasks x 4 variants
harness bench benchmarks/tasks \
  --config benchmarks/harness.yaml \
  --matrix benchmarks/matrix.yaml \
  --parallel 2

# Harness ablation: every task x H0..H6
harness bench benchmarks/tasks \
  --config benchmarks/harness.yaml \
  --matrix benchmarks/matrices/harness-ablation.yaml \
  --parallel 2

# Dev set only, 2 repetitions per cell (statistical power)
harness bench benchmarks/tasks \
  --config benchmarks/harness.yaml \
  --matrix benchmarks/matrices/harness-ablation.yaml \
  --set dev --repeat 2 --parallel 2

# Holdout evaluation
harness bench benchmarks/tasks \
  --config benchmarks/harness.yaml \
  --matrix benchmarks/matrices/harness-ablation.yaml \
  --set holdout --parallel 2

# Inspect the job matrix without running
harness bench benchmarks/tasks \
  --config benchmarks/harness.yaml \
  --matrix benchmarks/matrix.yaml --dry-run
```

## Tasks

<!-- The task table is generated from benchmarks/tasks; see results/ for the
     latest published run. -->

## Notes

- Verification commands are shell-specific (the examples use Windows `cmd`
  syntax; adapt `findstr`/paths for other shells).
- The default harness runs the agent with **filesystem + shell tools** inside
  a `process` sandbox; verification also runs sandboxed.
- Benchmark runs clone the task repository once into a local mirror (with
  retry) and execute each job in an isolated git worktree.
