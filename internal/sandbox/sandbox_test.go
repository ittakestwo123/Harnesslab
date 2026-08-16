package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNoneSandboxRuns(t *testing.T) {
	sb, err := New(Spec{Type: "none"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer sb.Close()
	res := sb.Run(context.Background(), Command{Command: "echo hello"})
	if res.Err != nil || res.ExitCode != 0 || !strings.Contains(res.Output, "hello") {
		t.Fatalf("result = %+v", res)
	}
}

func TestProcessSandboxPolicy(t *testing.T) {
	sb, err := New(Spec{Type: "process", DeniedCommands: []string{"rm -rf", "sudo"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer sb.Close()
	ctx := context.Background()

	// Denied command rejected before execution.
	res := sb.Run(ctx, Command{Command: "rm -rf /"})
	if res.Err == nil || !strings.Contains(res.Err.Error(), "denied") {
		t.Fatalf("denied command should be rejected, got %+v", res)
	}

	// Allowed command runs.
	res = sb.Run(ctx, Command{Command: "echo ok"})
	if res.Err != nil || res.ExitCode != 0 {
		t.Fatalf("allowed command failed: %+v", res)
	}

	// Allowlist mode rejects anything not listed.
	sb2, _ := New(Spec{Type: "process", AllowedCommands: []string{"go", "echo"}})
	defer sb2.Close()
	if res := sb2.Run(ctx, Command{Command: "curl evil.example.com"}); res.Err == nil {
		t.Fatal("allowlist should reject curl")
	}
	if res := sb2.Run(ctx, Command{Command: "echo hi"}); res.Err != nil {
		t.Fatalf("allowlist should accept echo: %+v", res)
	}
}

func TestProcessSandboxTimeout(t *testing.T) {
	sb, _ := New(Spec{Type: "process", Timeout: 200 * time.Millisecond})
	defer sb.Close()
	start := time.Now()
	res := sb.Run(context.Background(), Command{Command: longSleepCommand()})
	if time.Since(start) > 3*time.Second {
		t.Fatal("timeout did not bound the command")
	}
	if res.ExitCode == 0 {
		t.Fatalf("timed-out command should not exit 0, got %+v", res)
	}
}

func longSleepCommand() string {
	if runtime.GOOS == "windows" {
		return "ping -n 5 127.0.0.1 > nul"
	}
	return "sleep 5"
}

func TestScrubEnv(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-secret123")
	t.Setenv("MY_TOKEN", "tok123")
	t.Setenv("KEEP_ME", "value")
	env := scrubEnv()
	for _, kv := range env {
		key := strings.SplitN(kv, "=", 2)[0]
		if strings.Contains(strings.ToUpper(key), "API_KEY") ||
			strings.Contains(strings.ToUpper(key), "TOKEN") {
			t.Fatalf("secret env leaked: %s", key)
		}
	}
}

func TestRedactSecrets(t *testing.T) {
	got := redactSecrets("key=sk-abcdef123456 and ghp_xYz")
	if strings.Contains(got, "sk-abcdef123456") || strings.Contains(got, "ghp_xYz") {
		t.Fatalf("secret not redacted: %s", got)
	}
	if !strings.Contains(got, "sk-***") {
		t.Fatalf("expected sk-*** marker in %s", got)
	}
}

func TestProcessSandboxEnvScrubbed(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-supersecret")
	sb, err := New(Spec{Type: "process"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer sb.Close()
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo %DEEPSEEK_API_KEY%"
	} else {
		cmd = "echo $DEEPSEEK_API_KEY"
	}
	res := sb.Run(context.Background(), Command{Command: cmd})
	if strings.Contains(res.Output, "sk-supersecret") {
		t.Fatalf("secret leaked into sandbox output: %q", res.Output)
	}
}

// TestExecShellQuoted verifies the Windows batch-file path parses embedded
// quotes exactly as typed (regression: Go exec mangled them via cmd /c).
func TestExecShellQuoted(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific quoting regression")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(file, []byte("hello needle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := execShell(context.Background(), dir, `findstr "needle" sample.txt`)
	if res.Err != nil || res.ExitCode != 0 {
		t.Fatalf("findstr should match: %+v", res)
	}
}
