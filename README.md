# HarnessLab

**Build. Trace. Replay. Benchmark. Evolve.**

An open-source Go platform for **AI agent harness engineering**, built on top of
[tRPC-Agent-Go](https://github.com/trpc-group/trpc-agent-go).

> **Same model. Same task. Same repository. Different harness.**
>
> HarnessLab makes the difference measurable: it turns the hard-coded parts of
> a coding agent — prompt, context, planning, tools, verification, retry,
> budget — into a configurable, observable, reproducible, comparable and
> optimizable **harness**.

[![CI](https://github.com/ittakestwo123/Harnesslab/actions/workflows/ci.yml/badge.svg)](https://github.com/ittakestwo123/Harnesslab/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-green.svg)](LICENSE)

---

## Demo 1 — Offline Replay

Replay a coding-agent run **without calling the model API**:

```text
Live Agent Run      2.6s      (real DeepSeek model + exec_command tool)
Offline Replay      16ms
External Model Calls  0
External Tool Calls   0
Tokens              6498/167  (identical to the live run)
```

```bash
harness run --repo https://github.com/octocat/Hello-World.git "list the files"
harness replay <run-id>          # offline, no API key needed
```

## Demo 2 — Harness Diff

Same model, same task, same repository — different harness:

```text
Run A (with shell tool)          Run B (without tools)
Tokens in    6,498               38
Tool Calls   3                   0
Model Calls  4                   1

First divergence at step 1:
  A: model ... (then executes ls / dir / type README)
  B: model ... <function_results>  ← hallucinated a file listing
```

```bash
harness diff <run-a> <run-b>
```

## Demo 3 — Verified Benchmark

With `success.require_workspace_change` and a real verification command, a
harness that only "answers" can no longer fake a PASS:

```text
Harness                          Pass  Total Ver  Tokens     Chg   Time
planning=none+tools_shell=false  0/2   2     0    294        0     2.2s
planning=none+tools_shell=true   2/2   2     2    1,263,751  2     4m0.6s
```

The no-tools variant "passed" at 52 tokens in earlier demos by hallucinating;
with verification enabled it correctly fails (no workspace change, no
verification pass). The `Ver`/`Chg` columns separate a real PASS from a
text-only answer.

```bash
harness bench ./tasks --matrix matrix.yaml --parallel 2
harness optimize --report .harness/bench/bench-<id>.json
```

---

## What HarnessLab does

HarnessLab is **not** another agent framework. It is a laboratory for the
layer around the model — the **harness**:

```text
                     LLM
                      │
                      ▼
           ┌─────────────────────┐
           │       Harness       │   Context · Planning · Memory · Skills
           │                     │   Tools · Retry · Verification · Sandbox
           │                     │   Compaction · Policies · Budget
           └──────────┬──────────┘
                      ▼
                  Repository
```

It records the full agent trajectory and makes it:

- **Replayable** — offline replay from recorded tool/model results
- **Comparable** — trajectory diff with first-divergence detection
- **Benchmarkable** — task x harness-variant matrices on a worker pool
- **Reproducible** — `.harness` bundles (spec + trace + replay store + env)
- **Optimizable** — failure-pattern analysis and Pareto fronts
- **Verifiable** — `success.require_verification_pass` + `success.require_workspace_change`
  stop text-only hallucinated answers from faking a PASS
- **Sandboxable** — agent shell commands and verification run through a
  configurable sandbox (`none` / `process` / `docker` / `bwrap`) with env
  scrubbing, timeouts, and command allow/deny policies

## Sandbox

```yaml
sandbox:
  type: process        # none | process | docker | bwrap
  timeout: 30s         # per-command timeout (process backend)
  denied_commands:     # process backend: reject commands containing these
    - "rm -rf"
    - "format c:"
  # allowed_commands:  # process backend: restrict to this prefix allowlist
  #   - go
  #   - echo
```

- `none` — direct host execution (default)
- `process` — cwd isolation, **secret env scrubbing** (API keys/tokens are not
  visible to sandboxed commands), per-command timeouts, allow/deny lists
- `docker` — commands run inside a container with the workspace mounted at
  `/workspace` (`sandbox.image`, network policy)
- `bwrap` — bubblewrap sandbox on Linux (workspace rw, system dirs ro,
  network disabled unless allowed)

When a non-`none` sandbox is configured, the agent's `exec_command` tool and
the verification commands both route through it.

## Costs

```yaml
pricing:
  deepseek:
    deepseek-chat:
      input_per_million: 0.27
      output_per_million: 1.10
  openai:
    "*":
      input_per_million: 1.25
      output_per_million: 10.0
```

When `pricing` is set, every run's `CostUSD` is computed from its token usage
and shown in reports (`$0.0016`–`$0.033` for the benchmark demo runs).

## Reproducibility & environment validation

`harness reproduce <run-id | bundle.harness> [--env-mode warn|strict|ignore]`
compares the recorded toolchain environment (OS, arch, Go, git, HarnessLab,
tRPC-Agent-Go versions) against the current one:

```text
Environment check:
  OS             MATCH    recorded="windows" current="windows"
  Go             MISMATCH recorded="go1.23.0" current="go1.26.5"
  ...
environment drift detected (1 mismatches)
```

`strict` aborts the reproduction on any mismatch; `warn` (default) continues.

## Public benchmark

`benchmarks/` contains a reproducible coding-agent benchmark: 10 tasks
against the seeded `bench/tasks-v2` commit `075550a` of this repository
(4 real code regressions caught by failing tests, 2 typos, 2 add-a-test,
2 add-a-feature), a base harness with DeepSeek pricing, and a 2×2 variant
matrix. Every task is verified by real repository state:

```bash
harness bench benchmarks/tasks --config benchmarks/harness.yaml --matrix benchmarks/matrix.yaml
```

Latest 40-run result (2026-08-16, DeepSeek `deepseek-chat`):
**39/40 passes, USD 0.38 total** — see
[benchmarks/results/README.md](benchmarks/results/README.md).

See [benchmarks/README.md](benchmarks/README.md).

## Status

`v0.2.0-alpha` — the full loop **Build → Trace → Replay → Diff → Benchmark
→ Reproduce → Optimize** is implemented and verified end-to-end with a real
DeepSeek model, with sandboxed execution, cost accounting, environment
validation and a public 10-task benchmark. See [CHANGELOG.md](CHANGELOG.md)
and the runnable demos in [docs/demo.md](docs/demo.md).

## Quick start

```bash
# 1. Build
go build ./cmd/harness

# 2. Configure your LLM
export OPENAI_API_KEY=sk-...        # OpenAI-compatible
export DEEPSEEK_API_KEY=sk-...      # or DeepSeek (spec: provider: deepseek)

# 3. Initialize a harness
harness init
# -> .harness/harness.yaml  (edit it: model, tools, verification commands...)

# 4. Run a task against a repository
harness run --repo https://github.com/example/project "fix the failing parser tests"
```

Example output:

```
Run: run-8f3a2b1c
Model       gpt-5 (openai)
Harness     golang-coding-default
Workspace   .harness/workspaces/worktrees/run-8f3a2b1c

01:53:26  RUN START
01:53:28  MODEL gpt-5
01:53:31  TOOL exec_command {"cmd":"go test ./..."}
01:53:33  TOOL exec_command done
01:53:35  MODEL gpt-5 tokens 3812 -> 884
01:53:36  RUN END

Status      passed
Tokens      3,812 in / 884 out
Tool Calls  1
Model Calls 2
Duration    10.2s
Trace:
.harness/traces/run-8f3a2b1c.jsonl
```

## CLI reference

| Command | Description |
| --- | --- |
| `harness init` | Generate a `.harness/` directory with a default `harness.yaml` |
| `harness run "<task>"` | Run an agent with a harness, stream progress, verify, persist |
| `harness trace <run-id>` | Render a run's trajectory |
| `harness runs` | List recorded runs |
| `harness replay <run-id>` | Offline replay from the recorded replay store |
| `harness diff <run-a> <run-b>` | Metrics table + aligned trajectory + first divergence |
| `harness bench <tasks>` | Benchmark tasks x harness variants (worker pool, budgets) |
| `harness snapshot` | Write `.harness/harness.lock` for the current harness |
| `harness export <run-id>` | Export a reproducible `.harness` bundle |
| `harness reproduce <run-id\|bundle>` | Re-run offline from recorded spec + replay store |
| `harness optimize` | Failure analysis, candidate changes, Pareto front |

## Architecture

```text
HarnessSpec (harness.yaml)
    → Harness Builder        (workspace + runtime + recorder + store)
    → Runtime Interface      (runtime-agnostic)
    → TRPC Runtime Adapter   (tRPC-Agent-Go runner, event normalization,
                              model wrapper + tool callbacks for replay)
    → HarnessEvent stream
    → Trace (JSONL) / Run Store (JSON|SQLite) / Replay Store
    → Diff / Benchmark / Reproduce / Optimize
```

## Development notes

- Verification commands run in the workspace root when `--repo` is given,
  otherwise in the current directory.
- The shell tool group uses the framework's host-exec toolset when no sandbox
  is configured; with a non-`none` sandbox it uses a sandbox-routed
  `exec_command` tool instead.
- Replay hashes are computed over the full model request (messages + tool
  declarations + config); runs must be replayed with the same harness spec.
- See [CONTRIBUTING.md](CONTRIBUTING.md) for how to contribute.

## License

Apache-2.0. See [LICENSE](LICENSE).
