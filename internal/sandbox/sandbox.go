// Package sandbox isolates agent command execution. Backends range from
// direct host execution (none) over process-level hardening (process: cwd
// isolation, env scrubbing, timeouts, command allow/deny lists) to real
// container isolation (docker, bwrap).
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Command is a command to execute in the sandbox.
type Command struct {
	// Dir is the host working directory for the command.
	Dir string
	// Command is the shell command line.
	Command string
	// Timeout bounds the execution (0 = use the sandbox default).
	Timeout time.Duration
}

// Result is the outcome of a sandboxed command.
type Result struct {
	// ExitCode is the process exit code (-1 when not a process failure).
	ExitCode int
	// Output is the combined stdout+stderr.
	Output string
	// Err is the execution error, if any.
	Err error
}

// Sandbox executes commands with isolation and policy.
type Sandbox interface {
	Run(ctx context.Context, cmd Command) Result
	// Close releases sandbox resources (containers, temp dirs).
	Close() error
}

// Spec configures a sandbox backend.
type Spec struct {
	// Type is none | process | docker | bwrap.
	Type string
	// Network maps to backend policy: restricted | allowlist | none.
	Network string
	// Image is the container image for the docker backend.
	Image string
	// Timeout bounds each command (0 = none).
	Timeout time.Duration
	// AllowedCommands restricts process-backend commands (prefix match).
	AllowedCommands []string
	// DeniedCommands rejects process-backend commands containing these.
	DeniedCommands []string
}

// New builds a sandbox for the given spec.
func New(spec Spec) (Sandbox, error) {
	switch spec.Type {
	case "", "none":
		return &noneSandbox{timeout: spec.Timeout}, nil
	case "process":
		return &processSandbox{spec: spec}, nil
	case "docker":
		return &dockerSandbox{spec: spec}, nil
	case "bwrap":
		if runtime.GOOS == "windows" {
			return nil, fmt.Errorf("sandbox: bwrap is not supported on windows")
		}
		return &bwrapSandbox{spec: spec}, nil
	default:
		return nil, fmt.Errorf("sandbox: unsupported type %q", spec.Type)
	}
}

// --- shared host execution ---

// execShell runs a shell command on the host, killing the whole process
// tree when the context is cancelled (per-command timeouts included).
func execShell(ctx context.Context, dir, cmd string) Result {
	return runCommandTree(ctx, dir, cmd, nil)
}

// execShellEnv runs a shell command with an explicit environment
// (nil inherits the parent environment).
func execShellEnv(ctx context.Context, dir, cmd string, env []string) Result {
	return runCommandTree(ctx, dir, cmd, env)
}

// execShellTimeout runs a shell command with a per-command timeout.
func execShellTimeout(ctx context.Context, dir, cmd string, timeout time.Duration) Result {
	return execShellTimeoutEnv(ctx, dir, cmd, nil, timeout)
}

// execShellTimeoutEnv runs a shell command with an explicit environment and
// a per-command timeout.
func execShellTimeoutEnv(ctx context.Context, dir, cmd string, env []string, timeout time.Duration) Result {
	if timeout <= 0 {
		return execShellEnv(ctx, dir, cmd, env)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return execShellEnv(ctx, dir, cmd, env)
}

// runWithTreeKill starts a command and reaps it, killing the whole process
// tree when ctx is done. Platform-specific implementations live in
// exec_windows.go / exec_unix.go.
func runWithTreeKill(ctx context.Context, dir, cmd string) Result {
	return runCommandTree(ctx, dir, cmd, nil)
}

// waitFor reaps c, capturing output into buf. When ctx is done, kill is
// invoked first so the whole process tree dies and the pipes close, then the
// process is reaped.
func waitFor(ctx context.Context, c *exec.Cmd, buf *bytes.Buffer, kill func()) Result {
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case err := <-done:
		return Result{ExitCode: exitCode(err), Output: buf.String(), Err: err}
	case <-ctx.Done():
		if kill != nil {
			kill()
		}
		<-done
		return Result{ExitCode: -1, Output: buf.String(), Err: ctx.Err()}
	}
}

func writeBatch(cmd string) (string, error) {
	f, err := os.CreateTemp("", "harness-sandbox-*.bat")
	if err != nil {
		return "", fmt.Errorf("sandbox: create batch: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString("@echo off\r\n" + cmd + "\r\n"); err != nil {
		return "", fmt.Errorf("sandbox: write batch: %w", err)
	}
	return f.Name(), nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// scrubEnv returns a copy of the environment without known secrets
// (API keys, tokens). Used by the process backend as defense in depth so a
// sandboxed command cannot read the harness's credentials.
func scrubEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		key, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(key)
		if strings.Contains(upper, "API_KEY") ||
			strings.Contains(upper, "TOKEN") ||
			strings.Contains(upper, "SECRET") ||
			strings.Contains(upper, "PASSWORD") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
