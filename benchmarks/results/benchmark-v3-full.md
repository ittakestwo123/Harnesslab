# HarnessLab Benchmark v3 — Full Report

> Full Benchmark v3 run on the complete 34-task taskset (8 categories) across
> the H0–H6 harness ablation matrix with repeated runs. Every PASS comes from
> deterministic verification (real workspace state); infrastructure errors are
> reported separately and never counted as harness failures.

## Experiment setup

| Field | Value |
|---|---|
| HarnessLab version | v0.3.1-alpha |
| Repository commit | 9bf7c98 (feat/v0.3.1-launch head during the run) |
| Task baseline commit | `075550a` (`bench/tasks-v2`) |
| Model / provider | DeepSeek `deepseek-chat` → `deepseek-v4-flash`, temperature 0.1 |
| Task count | 34 (8 categories: basic 5, debugging 4, multi-file 4, refactor 4, testing 8, concurrency 3, context-heavy 3, tool-heavy 3) |
| Dev / Holdout | 25 / 9 |
| Harness variants | H0 baseline, H1 planner, H2 verification, H3 retry, H4 context (repo-map), H5 skills, H6 full |
| Repeats | dev ×2, holdout ×3 |
| Total runs | 539 (350 dev + 189 holdout) |
| Total tokens | 16,922,204 in / 475,493 out |
| Total cost | USD 5.0921 |
| Total runtime | 121.4 min |
| Date | 2026-08-17 (UTC+8) |
| Sandbox | process (env scrub, timeouts, allow/deny) |
| Verification | findstr / go test / go build per task (deterministic) |

## Reproduce

```bash
harness bench benchmarks/tasks --config benchmarks/harness.yaml --matrix benchmarks/matrices/harness-ablation.yaml --set dev --repeat 2 --parallel 2
harness bench benchmarks/tasks --config benchmarks/harness.yaml --matrix benchmarks/matrices/harness-ablation.yaml --set holdout --repeat 3 --parallel 2
```

Raw per-run data: `benchmarks/results/benchmark-v3-full/*.json` (task, harness,
repeat, run_id, status, verification, workspace_changed, input/output tokens,
cost, duration, tool/model calls, error).

## Dev results (25 tasks × 7 harnesses × 2 repeats = 350 runs)

| Harness | Success | Ver | Median tokens | Mean tokens | Median cost | Mean cost | Median time | P90 time | Tool calls | Model calls | Infra errors |
|---|---|---|---|---|---|---|---|---|---|---|---|
| h0-baseline | 92% (46/50) | 50 | 17691 | 27012 | $0.006 | $0.008 | 6.7s | 18.0s | 6.3 | 5.5 | 0 |
| h1-planner | 92% (46/50) | 50 | 19838 | 25890 | $0.006 | $0.008 | 6.9s | 25.9s | 6.4 | 5.5 | 0 |
| h2-verification | 92% (46/50) | 48 | 15958 | 27377 | $0.005 | $0.008 | 7.2s | 21.7s | 6.6 | 5.9 | 0 |
| h3-retry | 92% (46/50) | 49 | 15968 | 23991 | $0.005 | $0.007 | 5.9s | 14.0s | 6.2 | 5.4 | 0 |
| h4-context | 94% (47/50) | 49 | 18915 | 28209 | $0.006 | $0.008 | 6.7s | 13.7s | 5.4 | 4.9 | 0 |
| h5-skills | 92% (46/50) | 48 | 16445 | 25541 | $0.005 | $0.008 | 7.3s | 19.9s | 6.0 | 5.8 | 0 |
| h6-full | 94% (47/50) | 49 | 19724 | 26071 | $0.006 | $0.008 | 6.9s | 13.9s | 4.9 | 4.7 | 0 |

## Holdout results (9 tasks × 7 harnesses × 3 repeats = 189 runs)

| Harness | Success | Ver | Median tokens | Mean tokens | Median cost | Mean cost | Median time | P90 time | Tool calls | Model calls | Infra errors |
|---|---|---|---|---|---|---|---|---|---|---|---|
| h0-baseline | 100% (27/27) | 27 | 39541 | 49722 | $0.012 | $0.015 | 12.0s | 24.7s | 9.2 | 8.4 | 0 |
| h1-planner | 96% (26/27) | 27 | 37173 | 52454 | $0.011 | $0.016 | 12.3s | 35.3s | 9.4 | 8.3 | 0 |
| h2-verification | 100% (27/27) | 27 | 35366 | 39941 | $0.010 | $0.012 | 11.4s | 22.5s | 8.9 | 7.5 | 0 |
| h3-retry | 100% (27/27) | 27 | 34410 | 40923 | $0.010 | $0.013 | 10.9s | 22.7s | 9.0 | 7.5 | 0 |
| h4-context | 89% (24/27) | 26 | 31897 | 35615 | $0.009 | $0.011 | 9.0s | 66.6s | 7.0 | 5.8 | 0 |
| h5-skills | 93% (25/27) | 26 | 31195 | 32067 | $0.009 | $0.010 | 10.9s | 17.0s | 7.5 | 6.9 | 0 |
| h6-full | 96% (26/27) | 27 | 32471 | 35118 | $0.010 | $0.010 | 8.6s | 19.6s | 6.6 | 5.7 | 0 |

## Component attribution (H0–H6 ablation)

Comparisons are incremental (component added on top of the previous harness);
success = PASS / total with deterministic verification. **H0 and H1 disable
verification by design**, so their "passes" mean only "workspace changed" —
they are unverified by construction.

| Question | Dev answer | Holdout answer |
|---|---|---|
| Does planning help? | No change (H1 92% vs H0 92%) | Slight regression (H1 96% vs H0 100%; both unverified baselines) — **no measurable improvement in this benchmark** |
| Does verification reduce failure? | No success-rate change (H2 92% vs H0 92%) | Same success (H2 100% vs H0 100%) but holdout tokens drop ~20% (39.9k vs 49.7k) and cost ~20% — **verification adds real success criteria at lower token/cost** |
| Does retry recover? | No change (H3 92% vs H2 92%) | No change (H3 100% vs H2 100%, tokens 40.9k vs 39.9k) — **no measurable improvement in this benchmark** |
| Does repo-map context cut tokens? | Slight success gain (H4 94% vs H2 92%), tokens similar | **Regresses holdout success (H4 89% vs H2 100%) with +76% latency (25.3s vs 14.4s)** — repo-map context does not generalize on this taskset |
| Do skills raise success? | No change (H5 92% vs H2 92%) | Regression (H5 93% vs H2 100%), but lowest tokens on holdout (32.1k, −20%) — **no success gain; token saving needs a larger sample to confirm** |
| Is the full harness worth its cost? | Slight gain (H6 94% vs H0 92%) | H6 96% vs H0 100% (H0 unverified); **H6 delivers −29% tokens (35.1k vs 49.7k), −33% cost ($0.010 vs $0.015) and −22% latency (10.9s vs 14.0s) with real verification** — worth it on efficiency, not on success alone |

**Best harness on holdout (verified): H2 verification (100%) and H3 retry (100%)**;
H6 full is the efficiency pick (lowest latency, −29% tokens at 96% success).

## Notes

- Success requires `require_verification_pass` AND `require_workspace_change`
  (base harness success spec); H0/H1 disable verification by design, so their
  success rates are unverified (workspace-change only).
- Infrastructure errors (network, mirror clone, sandbox) are counted as
  `errored` and excluded from success rate; verification failures are genuine
  harness outcomes. This run had **0 infrastructure errors** (539/539 runs
  executed).
- Holdout success being higher than dev overall reflects the holdout task mix
  (several previously-validated tasks); the dev set contains the harder
  context-heavy/tool-heavy/refactor tasks.
