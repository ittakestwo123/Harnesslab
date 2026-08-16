# HarnessLab Public Benchmark Report

- **Harness**: HarnessLab `v0.2.0-alpha` (CLI `harness version 0.2.0-alpha`)
- **Model**: DeepSeek `deepseek-chat` (resolves to `deepseek-v4-flash`), temperature 0.1
- **Repository**: [github.com/ittakestwo123/Harnesslab](https://github.com/ittakestwo123/Harnesslab)
- **Seed commit**: `075550a` (`bench/tasks-v2`) - full tree incl. `internal/harness` and `cmd/`, four intentional regressions + two doc typos
- **Taskset**: 10 tasks x 4 variants = 40 runs (`benchmarks/tasks/`, `benchmarks/matrix.yaml`)
- **Runtime**: process sandbox; verification via `findstr` + `go test`; `success.require_verification_pass=true`, `success.require_workspace_change=true`
- **Date**: 2026-08-16 (UTC+8)

> 6 runs (add-spec-test x3, add-cost-test x3) initially errored on a transient mirror-clone network failure (SSL_ERROR_SYSCALL) and were re-run; this report merges the 34 first-wave passes with the 6 re-run results. `fix(workspace): retry mirror clone` now prevents this. One re-run (add-cost-test, todo+shell=false) failed because the agent wrote a test with `int` arguments where `Calculate` expects `int64` and did not fix the compile error.

## Summary

| Variant | Pass | Total | Ver | Chg | Input tokens | Output tokens | Cost USD | Time |
|---|---|---|---|---|---|---|---|---|
| planning=none+tools_shell=false | 10/10 | 10 | 10 | 10 | 361510 | 9740 | 0.1057 | 95s |
| planning=none+tools_shell=true | 10/10 | 10 | 10 | 10 | 309331 | 7095 | 0.0894 | 97.4s |
| planning=todo+tools_shell=false | 9/10 | 10 | 9 | 10 | 347102 | 9808 | 0.1019 | 94.4s |
| planning=todo+tools_shell=true | 10/10 | 10 | 10 | 10 | 288757 | 6813 | 0.0836 | 94s |
| **Total** | **39/40** | 40 | 39 | 40 | 1306700 | 33456 | 0.3806 | 380.9s |

Ver = verification passed, Chg = workspace changed.

## Per-task results

| Task | none/shell=false | none/shell=true | todo/shell=false | todo/shell=true |
|---|---|---|---|---|
| fix-diff-tools | PASS | PASS | PASS | PASS |
| fix-workspace-changed | PASS | PASS | PASS | PASS |
| fix-replay-hash | PASS | PASS | PASS | PASS |
| fix-sandbox-deny | PASS | PASS | PASS | PASS |
| fix-readme-typo | PASS | PASS | PASS | PASS |
| fix-changelog-typo | PASS | PASS | PASS | PASS |
| add-spec-test | PASS | PASS | PASS | PASS |
| add-cost-test | PASS | PASS | FAIL | PASS |
| add-version-package | PASS | PASS | PASS | PASS |
| add-sum-tokens-helper | PASS | PASS | PASS | PASS |

## Cost

- Total input tokens: 1306700
- Total output tokens: 33456
- Total cost: USD 0.3806 (0.0095 per run)

## Methodology

1. The seed commit `075550a` is the full HarnessLab repository with four intentional regressions (diff tool-name pairing, workspace change detection, replay canonicalizer hash, sandbox deny matching) and two documentation typos; each task asks the agent to fix or extend exactly one thing, verified by real `go test` commands (plus `findstr` for text tasks).
2. Four harness variants are swept: planning strategy `none` vs `todo`, and shell tool availability `true` vs `false` (`benchmarks/matrix.yaml`).
3. Runs execute in an isolated git worktree under a process sandbox; success requires the verification commands to pass **and** the workspace to be changed.
4. Run traces, metrics and cost are stored in the harness run store; this report is generated from `bench-*.json` plus run-store records (`metrics.CostUSD`).

## Artifacts

- `bench-31400b43.json` + `bench-3ffe144a.json` - raw bench reports
- `store/run-*.json` - per-run records incl. `metrics.CostUSD`
- `traces/run-*.jsonl` - full agent traces (replayable via `harness replay`)
