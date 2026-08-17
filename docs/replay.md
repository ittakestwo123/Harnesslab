# Offline Replay

Replay a coding-agent run **without calling the model API**.

```text
Live Agent Run      ~4s     (real DeepSeek model + tools)
Offline Replay      16ms    (all model + tool calls served from the store)
External Calls      0
Verification        PASS    (recorded workspace changes re-applied)
Tokens              identical to the live run
```

## How it works

Every run records into a replay store:

- **Tool calls** — via `tool.Callbacks` (`AfterTool` records, `BeforeTool`
  replays with a `CustomResult` that skips the real tool).
- **Model calls** — via a model wrapper that aggregates the streaming response
  and stores it; replay serves the stored response instead of calling the API.

Each entry is keyed by a canonical content hash. The `Canonicalizer`:

- sorts map keys for stable JSON serialization,
- replaces absolute workspace/temp paths with `$WORKSPACE` / `$TMP` — including
  **inside nested JSON** (tool results embedded as message content), so a
  replay in a fresh worktree hashes identically,
- hashes `kind + name + normalized input`.

Offline replay re-applies the recorded **workspace patch** before verification,
so a successfully recorded run replays as PASSED with its file changes
re-materialized.

## Usage

```bash
harness replay <run-id>                 # strict (offline), no API key needed
harness replay <run-id> --fallback      # live calls on a replay miss
harness run "<task>" --replay <run-id>  # run a new task replaying a recorded run
```

## Notes

- Replay requires the same harness spec that recorded the run (hashes include
  the model request: messages + tool declarations + config).
- Replay is a trpc-runtime feature; codex runs are traced but not
  offline-replayable.
