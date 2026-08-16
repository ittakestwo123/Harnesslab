# Benchmark v3 — Statistical Report Example

A demonstration of the Benchmark v3 statistical reporting: **4 tasks x 7
harnesses (H0..H6 ablation) x 2 repetitions = 56 real runs** on DeepSeek
`deepseek-chat` (2026-08-16).

- Tasks: `fix-readme-typo` (basic), `t-cost-wildcard` (testing),
  `concurrency-cost-counter` (concurrency), `tool-git-find-seed` (tool-heavy)
- Command:
  `harness bench <tasks> --config benchmarks/harness.yaml --matrix benchmarks/matrices/harness-ablation.yaml --repeat 2`
- Raw report: `bench-v3-example-stats.json` (per-run records preserved:
  run id, repeat, tokens, cost, latency, verification, workspace change)

## Per-variant table

| Harness | Success | Tokens mean | Tokens P50 | Tokens P90 | Cost mean | Lat mean | Lat P90 | CI95(tokens) |
|---|---|---|---|---|---|---|---|---|
| h0-baseline | 100% | 14,378.9 | 13,367.0 | 18,529.0 | $0.0044 | 9,911.5 | 26,426.0 | 11,437.5..17,320.2 |
| h1-planner | 100% | 13,762.4 | 13,408.0 | 20,203.0 | $0.0041 | 5,952.8 | 10,938.0 | 10,728.7..16,796.0 |
| h2-verification | 100% | 14,935.6 | 13,585.0 | 21,666.0 | $0.0044 | 5,786.1 | 9,211.0 | 12,698.1..17,173.2 |
| h3-retry | 100% | 13,650.1 | 13,585.0 | 15,968.0 | $0.0040 | 5,515.6 | 8,333.0 | 12,414.6..14,885.6 |
| h4-context | 100% | 27,866.2 | 15,888.0 | 105,060.0 | $0.0080 | 8,305.0 | 28,445.0 | 6,104.1..49,628.4 |
| h5-skills | 100% | 14,248.0 | 13,794.0 | 16,182.0 | $0.0042 | 5,752.9 | 7,561.0 | 13,066.1..15,429.9 |
| h6-full | 100% | 16,124.9 | 16,279.0 | 19,235.0 | $0.0049 | 4,954.6 | 6,313.0 | 14,365.5..17,884.3 |

## Reading the example

- All four tasks are easy enough that every harness passes, so success rate
  does not discriminate here — the point of this example is the **statistical
  machinery**: per-variant mean / median / stddev / P50 / P90 / CI95 over the
  two repetitions, with raw per-run data intact in the JSON.
- `h4-context` (repo-map) shows a wide P90/CI and ~2x mean tokens — the
  injected repository map costs context on easy tasks. Whether it pays off on
  context-heavy tasks is exactly what a larger `--repeat N` run over the full
  34-task dev set would answer.
- Costs are real (DeepSeek pricing from the harness files): ~$0.004/run.

## Why this matters

Repetition + statistics turn "this harness passed 9/10" into
"this harness differs from baseline by X% with confidence interval Y",
which is the prerequisite for the LLM Harness Optimizer (dev optimization →
candidate selection → holdout validation).
