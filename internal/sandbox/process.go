package sandbox

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// noneSandbox executes directly on the host (default, no isolation).
type noneSandbox struct {
	timeout time.Duration
}

func (s *noneSandbox) Run(ctx context.Context, cmd Command) Result {
	return execShellTimeout(ctx, cmd.Dir, cmd.Command, s.timeout)
}

func (s *noneSandbox) Close() error { return nil }

// processSandbox hardens host execution: cwd isolation, secret env
// scrubbing, per-command timeouts and command allow/deny lists.
type processSandbox struct {
	spec Spec
}

func (s *processSandbox) Run(ctx context.Context, cmd Command) Result {
	if cmd.Command == "" {
		return Result{ExitCode: -1, Err: fmt.Errorf("sandbox: empty command")}
	}
	if denied := matchDenied(cmd.Command, s.spec.DeniedCommands); denied != "" {
		return Result{ExitCode: -1, Err: fmt.Errorf("sandbox: command denied by policy (%q matches %q)", cmd.Command, denied)}
	}
	if len(s.spec.AllowedCommands) > 0 && !matchAllowed(cmd.Command, s.spec.AllowedCommands) {
		return Result{ExitCode: -1, Err: fmt.Errorf("sandbox: command not in allowlist")}
	}

	timeout := s.spec.Timeout
	if cmd.Timeout > timeout {
		timeout = cmd.Timeout
	}
	// Run with a scrubbed environment so the sandboxed command cannot read
	// the harness's API keys or tokens.
	res := execShellTimeoutEnv(ctx, cmd.Dir, cmd.Command, scrubEnv(), timeout)
	res.Output = redactSecrets(res.Output)
	// A command killed by the timeout must never report success.
	if ctx.Err() != nil {
		res.ExitCode = -1
	}
	return res
}

func (s *processSandbox) Close() error { return nil }

func matchDenied(cmd string, denied []string) string {
	for _, d := range denied {
		// BUG(seed): only prefix matches are rejected; denied commands
		// embedded in the middle of a command slip through.
		if d != "" && strings.HasPrefix(cmd, d) {
			return d
		}
	}
	return ""
}

func matchAllowed(cmd string, allowed []string) bool {
	for _, a := range allowed {
		if a != "" && strings.HasPrefix(cmd, a) {
			return true
		}
	}
	return false
}

// redactSecrets masks known secret-shaped substrings in output.
func redactSecrets(s string) string {
	out := s
	for _, marker := range []string{"sk-", "ghp_", "github_pat_", "xoxb-"} {
		out = redactMarker(out, marker)
	}
	return out
}

// redactMarker masks the token after marker until a non-token character.
func redactMarker(s, marker string) string {
	var b strings.Builder
	for {
		idx := strings.Index(s, marker)
		if idx < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:idx])
		b.WriteString(marker)
		rest := s[idx+len(marker):]
		end := 0
		for end < len(rest) && isTokenChar(rest[end]) {
			end++
		}
		b.WriteString("***")
		s = rest[end:]
	}
}

func isTokenChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}
