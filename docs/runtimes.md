# Runtimes

HarnessLab core is runtime-agnostic behind the `hlruntime.Runtime` interface —
the same workspace, trace, verification, cost and benchmark pipeline runs on
every adapter.

```
HarnessLab Core
    │
Runtime Interface
    │
┌─────────┴─────────┐
│                   │
▼                   ▼
TRPCRuntime      CodexRuntime
```

## tRPC-Agent-Go (`runtime.type: trpc`, default)

Full agent loop with framework tools, planning, skills and **offline replay**.
The adapter drives the tRPC-Agent-Go runner and normalizes its event stream
into HarnessLab's `HarnessEvent` dialect.

## Codex CLI (`runtime.type: codex`)

Drives a locally installed `codex` binary via `codex exec --json` (prompt on
stdin, JSONL event stream parsed into the same `HarnessEvent` dialect). The
harness instruction is prepended to the prompt (codex has no separate system
channel); codex brings its own tools and sandbox:

```yaml
runtime:
  type: codex
  codex:
    binary: codex                 # default; or an absolute path
    model: gpt-5.1-codex          # optional --model override
    sandbox: workspace-write      # read-only | workspace-write | danger-full-access
    ask_for_approval: never       # never | on-request | on-failure
    env:
      - CODEX_HOME=/path/to/codex-home   # optional isolated codex config
```

## Notes

- With two runtimes behind one interface, HarnessLab is a **Universal Agent
  Harness Engineering Platform**, not a tRPC-Agent-Go tool.
- Offline replay is a trpc-runtime feature; codex runs are fully traced but
  not offline-replayable (the CLI is the runtime).
- The benchmark layer never depends on runtime internals, so a third runtime
  (Claude Code, OpenCode, …) is one adapter + one builder case away.
