# Contributing to HarnessLab

Thanks for your interest in contributing! HarnessLab is an open-source Go
platform for AI agent harness engineering, built on top of
[tRPC-Agent-Go](https://github.com/trpc-group/trpc-agent-go).

## Ways to contribute

- Report bugs or suggest features via [GitHub Issues](https://github.com/ittakestwo123/Harnesslab/issues)
- Improve documentation and examples
- Submit pull requests (bug fixes, features, tests, benchmarks)

## Development setup

```bash
git clone https://github.com/ittakestwo123/Harnesslab.git
cd Harnesslab
go build ./...
go vet ./...
go test ./...
```

### Dependency note

HarnessLab currently pins a specific version of `trpc.group/trpc-go/trpc-agent-go`
in `go.mod` (no local `replace`). Do not add a `replace` directive unless you
are working on the runtime adapter against a local clone; keep it out of
merged commits.

## Coding conventions

- Run `gofmt` before committing.
- Follow [Effective Go](https://go.dev/doc/effective_go) and the
  [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).
- Every new Go file must include the Apache-2.0 license header:

```go
// Copyright 2026 HarnessLab contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// ...
```

- Every exported symbol needs a meaningful Go doc comment describing its
  caller-visible contract.
- Preserve backward compatibility: exported APIs, defaults, serialization,
  persistence schemas and protocol behavior are long-lived commitments.
- Keep changes minimal and focused; avoid unrelated refactoring.
- Tests must cover public behavior and meaningful boundary conditions.

## Testing

```bash
go test ./...
```

Run targeted tests while iterating, and the full suite before delivering.
Tests must not require API credentials or network access.

## Commit style

- Use conventional commits, e.g. `feat:`, `fix:`, `docs:`, `refactor:`, `ci:`.
- Keep the first line under 72 characters.
- Reference the issue or PR number when relevant.

## Pull request checklist

- [ ] `gofmt` clean
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes
- [ ] New public API documented
- [ ] CHANGELOG updated (when user-visible behavior changes)
