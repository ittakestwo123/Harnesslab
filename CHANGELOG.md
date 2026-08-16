# Changelog

All notable changes to HarnessLab are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
