# Benchmark

The public benchmark (`benchmarks/`) is a reproducible coding-agent benchmark:
**same model, same repository, same tasks — different harnesses**. Every task
is verified by real repository state, so hallucinated answers cannot fake a
PASS.

## TaskSet

**34 real coding tasks in 8 categories** against the seeded
`bench/tasks-v2` commit `075550a` of this repository:

| Category | Count | Kind of work |
|---|---|---|
| basic | 5 | docs typos & small additions |
| debugging | 4 | fix the seeded regressions (real failing tests) |
| multi-file | 4 | cross-file moves with callers updated |
| refactor | 4 | rename/extract keeping tests green |
| testing | 8 | add deterministic passing tests |
| concurrency | 3 | thread-safe code + concurrent tests |
| context-heavy | 3 | must read several files before acting |
| tool-heavy | 3 | must use the shell to discover facts |

The seed commit `075550a` intentionally contains 4 implementation regressions
(each caught by a failing test) plus 2 documentation typos.

## Dev / Holdout

Tasks are partitioned into `dev` (25) and `holdout` (9)
(`benchmarks/tasksets/dev.yaml`, `holdout.yaml`). Harness optimization may only
use `dev`; the `holdout` set is reserved for final generalization validation.

## Harness ablation (H0–H6)

`benchmarks/harnesses/h0-baseline.yaml` … `h6-full.yaml` add one harness
component at a time:

| Harness | Components |
|---|---|
| H0 baseline | bare agent (filesystem + shell tools, no planning/verification/…) |
| H1 planner | + TODO planning |
| H2 verification | + final verification |
| H3 retry | + model/tool retries |
| H4 context | + repo-map adaptive context |
| H5 skills | + working-procedure skills |
| H6 full | everything combined |

This lets the benchmark answer: does planning help? does verification reduce
failures? does retry recover? does repo-map context cut tokens? do skills raise
success? is the full harness worth its extra cost?

## Statistical reporting

`harness bench --repeat N` produces N independent run records per
task × harness (never overwritten). Reports include per-variant and per-task
**mean / median / stddev / P50 / P90 and a 95% confidence interval** for
tokens, cost and latency, while the raw per-run data stays in the JSON report.

## Running

```bash
# Classic matrix: 34 tasks x 4 variants
harness bench benchmarks/tasks \
  --config benchmarks/harness.yaml \
  --matrix benchmarks/matrix.yaml --parallel 2

# Ablation: every task x H0..H6
harness bench benchmarks/tasks \
  --config benchmarks/harness.yaml \
  --matrix benchmarks/matrices/harness-ablation.yaml --parallel 2

# Dev set, 2 repetitions (statistical power)
harness bench benchmarks/tasks \
  --config benchmarks/harness.yaml \
  --matrix benchmarks/matrices/harness-ablation.yaml \
  --set dev --repeat 2 --parallel 2

# Holdout evaluation
harness bench benchmarks/tasks \
  --config benchmarks/harness.yaml \
  --matrix benchmarks/matrices/harness-ablation.yaml \
  --set holdout --parallel 2
```

## Deterministic verification

Any PASS must come from real workspace state:

- `go test` / `go build` for code tasks
- `findstr` / file assertions for text tasks
- `success.require_verification_pass` + `success.require_workspace_change`

Invariant: **hallucination != pass** — a model claiming "fixed it" without the
workspace state passing is not a PASS.

## Results

- v0.3 full benchmark: see `benchmarks/results/benchmark-v3-full.md`
- v0.2 historical 40-run report: `benchmarks/results/README.md`
- v0.3 statistical example (56 runs): `benchmarks/results/benchmark-v3-example.md`

## Methodology

The full methodology, error classification and reproducibility instructions
are recorded with each published report under `benchmarks/results/`.
