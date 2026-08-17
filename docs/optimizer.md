# LLM Harness Optimizer

The optimizer turns benchmark results into better harnesses. The loop follows
the dev → select → holdout discipline:

```text
Current Harness ──► Dev Benchmark ──► Failure Analyzer ──► LLM Candidate Generator
                                                              │
                                                    ┌─────────┼─────────┐
                                                    ▼         ▼         ▼
                                               candidate-001..003 (yaml + metadata)
                                                              │
                                                              ▼
                                                   Dev Benchmark (candidates)
                                                              │
                                                              ▼
                                                       Pareto Selection
                                                              │
                                                              ▼
                                                    Holdout Validation
                                                              │
                                                     Dev-only win? ──► REJECT
                                                              │
                                                              ▼
                                                       Recommended Harness
```

## Usage

```bash
# 1. Generate candidates from the failure analysis (needs DEEPSEEK_API_KEY /
#    OPENAI_API_KEY for the model named in your harness)
harness optimize --llm --candidates 3 \
  --report .harness/bench/bench-<id>.json \
  --config benchmarks/harness.yaml --harness-dir .harness

# 2. Evaluate: dev bench -> Pareto -> holdout gate -> recommendation
harness optimize --evaluate \
  --config benchmarks/harness.yaml --harness-dir .harness \
  --tasks benchmarks/tasks --tasksets benchmarks/tasksets \
  --repeat 1 --parallel 2
```

## Guarantees

- **Candidates never overwrite your harness** — they are full
  `harnesslab/v1` specs written to `.harness/candidates/candidate-XXX.yaml`.
- **No holdout leakage** — candidate generation reads only the dev failure
  analysis; holdout is used only for the final validation gate.
- **Dev-only wins are rejected** — a candidate that improves (or ties) the dev
  success rate but regresses the holdout success rate below baseline is
  **REJECTED**; only candidates that hold on holdout are recommended.
- **Auditable** — every candidate carries `metadata` (parent, reason,
  expected_effect) and, after evaluation, its dev/holdout results and final
  decision.

## Candidate example

```yaml
metadata:
  parent: baseline
  reason:
    - input tokens above threshold (context explosion)
  expected_effect:
    success: neutral
    tokens: decrease
version: harnesslab/v1
name: candidate-001
...
```

## Failure analysis

`harness optimize` (without flags) analyzes runs and their traces for known
failure patterns — no tool calls, high token usage, no workspace change,
repeated tool calls, tool failures, trajectory errors — and suggests
rule-based harness changes.
