# Sandbox

Agent shell commands and verification commands can run inside a configurable
sandbox:

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

## Backends

- `none` — direct host execution (default)
- `process` — cwd isolation, **secret env scrubbing** (API keys/tokens are not
  visible to sandboxed commands), per-command timeouts, allow/deny lists.
  On Windows, commands run through a temp batch file and are killed as a whole
  process tree via a Job Object (KILL_ON_JOB_CLOSE).
- `docker` — commands run inside a container with the workspace mounted at
  `/workspace` (`sandbox.image`, network policy).
- `bwrap` — bubblewrap sandbox on Linux (workspace rw, system dirs ro,
  network disabled unless allowed).

## Routing

When a non-`none` sandbox is configured, both the agent's `exec_command` tool
and the verification commands route through it.

Note: for the codex runtime, the codex CLI brings its own sandbox, configured
under `runtime.codex.sandbox` (see docs/runtimes.md).
