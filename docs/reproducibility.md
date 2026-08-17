# Reproducibility

Every run is reproducible as a `.harness` bundle.

## Snapshot / Export / Reproduce

```bash
harness snapshot                       # write .harness/harness.lock (toolchain fingerprint)
harness export <run-id>                # bundle: manifest + harness.yaml + harness.lock +
                                       #         trace + metrics + git.patch +
                                       #         environment.json + replay store
harness reproduce <run-id | bundle.harness> [--env-mode warn|strict|ignore]
```

`harness reproduce` re-runs the recorded run from its recorded spec and replay
store, and compares the recorded toolchain environment against the current
one:

```text
Environment check:
  OS             MATCH    recorded="windows" current="windows"
  Go             MISMATCH recorded="go1.23.0" current="go1.26.5"
  ...
environment drift detected (1 mismatches)
```

- `strict` aborts on any mismatch
- `warn` (default) continues and reports drift
- `ignore` skips the environment check

Recorded environment fields: OS, arch, Go version, git version, HarnessLab
version, tRPC-Agent-Go version.

## Offline replay

`harness replay <run-id>` reproduces the trajectory offline (no API calls) and
re-applies the recorded workspace patch before verification — see
[docs/replay.md](replay.md).

## Reproduce details

- Reproducing from a bundle re-creates the workspace from the recorded repo +
  commit and applies the bundled `git.patch`.
- Verification and workspace-change reflect the recorded outcome, so a
  successfully recorded run reproduces as PASSED.
