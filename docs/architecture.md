# Architecture

HarnessLab is **not** another agent framework. It is a laboratory for the layer
around the model — the **harness**:

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
- **Optimizable** — failure-pattern analysis, LLM candidate generation, Pareto
  selection and holdout validation
- **Verifiable** — `success.require_verification_pass` +
  `success.require_workspace_change` stop text-only hallucinated answers from
  faking a PASS
- **Sandboxable** — agent shell commands and verification run through a
  configurable sandbox (`none` / `process` / `docker` / `bwrap`) with env
  scrubbing, timeouts, and command allow/deny policies

## Pipeline

```text
HarnessSpec (harness.yaml)
    → Harness Builder        (workspace + runtime + recorder + store)
    → Runtime Interface      (runtime-agnostic)
    → Runtime Adapter        (tRPC-Agent-Go runner / Codex CLI, event
                              normalization, model wrapper + tool callbacks
                              for replay)
    → HarnessEvent stream
    → Trace (JSONL) / Run Store (JSON|SQLite) / Replay Store
    → Diff / Benchmark / Reproduce / Optimize
```

## HarnessSpec

`harness.yaml` declaratively describes one complete harness:

```yaml
version: harnesslab/v1
name: golang-coding-default
runtime:
  type: trpc                 # trpc | codex (see docs/runtimes.md)
agent:
  type: coding
model:
  provider: deepseek         # deepseek | openai
  model: deepseek-chat
  temperature: 0.1
planning:
  strategy: todo             # none | todo
context:
  strategy: none             # none | repo-map (auto repo structure summary)
skills:
  enabled: false
  list: []                   # working procedures injected as instructions
tools:
  filesystem: true
  shell: true
  git: false
  search: false
verification:
  strategy: final            # none | final | incremental | test-first
  commands:
    - go test ./...
    - go vet ./...
success:
  require_verification_pass: true
  require_workspace_change: false
retry:
  max_model_errors: 3
  max_tool_errors: 3
sandbox:
  type: process              # none | process | docker | bwrap
  timeout: 30s
budget:
  max_tokens: 120000
  max_cost_usd: 5
  timeout: 20m
pricing:                     # USD per million tokens, "*" = provider default
  deepseek:
    deepseek-chat:
      input_per_million: 0.27
      output_per_million: 1.10
```

## Cost model

`Run.Metrics.CostUSD` is computed from token usage whenever `pricing` is set:

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

## Development notes

- Verification commands run in the workspace root when `--repo` is given,
  otherwise in the current directory.
- The shell tool group uses the framework's host-exec toolset when no sandbox
  is configured; with a non-`none` sandbox it uses a sandbox-routed
  `exec_command` tool instead.
- Replay hashes are computed over the full model request (messages + tool
  declarations + config); runs must be replayed with the same harness spec.
  Absolute workspace paths are canonicalized to `$WORKSPACE`, including inside
  nested JSON (tool results embedded in message content).
- The benchmark layer depends only on HarnessLab-owned abstractions (Runtime
  interface, HarnessEvent, Run, Metrics) — never on a specific runtime's
  internals — so adding a runtime is one new adapter plus one case in the
  builder factory.
