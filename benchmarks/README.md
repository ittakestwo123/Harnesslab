# HarnessLab Benchmark v1

A public, reproducible coding-agent benchmark: **same model, same repository,
same tasks — different harnesses**. Every task is verified by real repository
state (tests pass, workspace changed), so hallucinated answers cannot fake a
PASS.

## Reproducibility

- **Task repository**: https://github.com/ittakestwo123/Harnesslab
- **Task baseline commit**: `a774e51` (branch `bench/tasks-v1`)
- That commit intentionally seeds 4 implementation regressions (each caught by
  a failing test) plus 2 documentation typos; the benchmark tasks ask the agent
  to fix them.
- The harness spec, matrix and every task are committed here, so anyone can
  re-run the whole benchmark and compare results.

## Running

```bash
# Build the CLI
go build -o harness ./cmd/harness

# Configure the model provider (spec: provider: deepseek)
export DEEPSEEK_API_KEY=sk-...

# Run the full benchmark (10 tasks x 4 variants = 40 runs)
harness bench benchmarks/tasks \
  --config benchmarks/harness.yaml \
  --matrix benchmarks/matrix.yaml \
  --parallel 2

# Run a subset (a single task file also works)
harness bench benchmarks/tasks/01-fix-diff-tools.yaml \
  --config benchmarks/harness.yaml \
  --matrix benchmarks/matrix.yaml
```

## Tasks

| # | Task | Verification |
| --- | --- | --- |
| 01 | Fix regression in `diff.BuildSteps` (tool names lost) | `go test ./internal/diff/...` |
| 02 | Fix regression in `workspace.Diff.Changed` (stat/untracked ignored) | `go test ./internal/workspace/...` |
| 03 | Fix regression in `replay.Canonicalizer.Hash` (name dropped) | `go test ./internal/replay/...` |
| 04 | Fix regression in `sandbox.matchDenied` (prefix-only match) | `go test ./internal/sandbox/...` |
| 05 | Fix README typo ("harnEas engineering") | `findstr "harness engineering" README.md` |
| 06 | Fix CHANGELOG typo ("Notabla changes") | `findstr "All notable changes" CHANGELOG.md` |
| 07 | Add `TestParseMinimal` to spec tests | `findstr` + `go test ./internal/harness/spec/...` |
| 08 | Add `TestCalculateZero` to cost tests | `findstr` + `go test ./internal/cost/...` |
| 09 | Add `internal/version` with `func Version() string` | `findstr` + `go build ./...` |
| 10 | Add `SumTokens` helper to internal/benchmark | `findstr` + `go build` + `go test` |

## Notes

- Verification commands are shell-specific (the examples use Windows `cmd`
  syntax; adapt `findstr`/paths for other shells).
- The default harness runs the agent with **filesystem + shell tools** inside
  a `process` sandbox; verification also runs sandboxed.
- `success.require_workspace_change` guarantees the agent actually modified
  the repository.
