# HarnessLab Demos

This file documents five runnable demos with **real outputs** captured on
Windows with DeepSeek. Every command below works against a local checkout;
expected output is shown after `$`.

## 0. Setup

```bash
go build -o harness ./cmd/harness
export DEEPSEEK_API_KEY=sk-...     # Windows: $env:DEEPSEEK_API_KEY = "sk-..."
harness init                        # -> .harness/harness.yaml (edit the model)
```

## 1. Offline Replay

Replay a coding-agent run **without calling the model API**:

```bash
harness run --repo https://github.com/octocat/Hello-World.git \
  "List the files in this repository using the exec_command tool, then summarize in one sentence what this project is about."
harness replay <run-id>    # offline; a bogus API key works
```

```
$ harness replay run-700289a4 -q
Status      passed
Tokens      6498 in / 167 out     # identical to the live run
Tool Calls  3
Model Calls 4
Duration    16ms                  # vs 2.6s live; 0 external calls
```

## 2. Trajectory Diff

Same model, same task, same repository — different harness:

```bash
harness diff <run-with-tools> <run-without-tools> --full
```

```
First divergence at A step 1 / B step 1:
  A: model ... (then executes ls / dir / type README)
  B: model ... <function_results>   ← hallucinated a file listing
```

## 3. Verified Benchmark (no fake PASS)

`success.require_workspace_change` + a real verification command turn
"answered from knowledge" into a FAIL:

```bash
harness bench ./tasks --config harness.yaml --matrix matrix.yaml
```

```
Harness                          Pass  Total Ver  Tokens     Chg   Time
planning=none+tools_shell=false  0/2   2     0    294        0     2.2s
planning=none+tools_shell=true   2/2   2     2    1,263,751  2     4m0.6s
```

## 4. Sandboxed Execution

```yaml
sandbox:
  type: process
  timeout: 30s
  denied_commands:
    - "rm -rf"
```

Agent shell commands and verification both route through the sandbox, which
also scrubs API keys/tokens from the command environment:

```bash
harness run --config sandbox-harness.yaml --repo ... "append 'sandboxed' to README"
```

```
Verification PASS in 122ms
Workspace   changed
```

A denied verification command is rejected by policy (exit -1 → run FAILs).

## 5. Reproduce with Environment Validation

```bash
harness snapshot
harness export <run-id>
harness reproduce <run-id>.harness --env-mode strict
```

```
Environment check:
  OS             MATCH    recorded="windows" current="windows"
  Go             MATCH    recorded="go1.26.5" current="go1.26.5"
  TRPCAgentGo    MATCH    recorded="v1.11.1-...-6dc730ebfacd" current="v1.11.1-...-6dc730ebfacd"
```

If the toolchain drifts, `--env-mode strict` aborts:

```
  Go             MISMATCH recorded="go1.23.0" current="go1.26.5"
environment drift detected (1 mismatches)
```

## 6. Public Benchmark

See [benchmarks/README.md](../benchmarks/README.md) — 10 tasks against the
seeded `bench/tasks-v1` commit, with costs:

```bash
harness bench benchmarks/tasks --config benchmarks/harness.yaml --matrix benchmarks/matrix.yaml
```

```
run=run-9a2911be status=passed tokens=89956 cost=$0.0259
run=run-b9d502ac status=passed tokens=111861 cost=$0.0330
```
