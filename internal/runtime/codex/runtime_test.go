package codex

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
)

// fakeCodexBin is a throwaway codex CLI compiled once in TestMain. It ignores
// all CLI args, drains the prompt from stdin and emits a fixed JSONL
// transcript chosen by HL_FAKE_CODEX_MODE, mirroring the real `codex exec
// --json` contract.
var fakeCodexBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fakecodex-*")
	if err != nil {
		os.Exit(1)
	}
	src := `package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	_, _ = io.Copy(io.Discard, os.Stdin)
	switch os.Getenv("HL_FAKE_CODEX_MODE") {
	case "fail":
		fmt.Println(` + "`" + `{"type":"turn.failed","error":{"message":"codex exploded","code":"boom"}}` + "`" + `)
		os.Exit(1)
	case "timeout":
		fmt.Println(` + "`" + `{"type":"thread.started","thread_id":"t-slow"}` + "`" + `)
		_ = os.Stdout.Sync()
		time.Sleep(30 * time.Second)
		os.Exit(0)
	default:
		fmt.Println(` + "`" + `{"type":"thread.started","thread_id":"t-1"}` + "`" + `)
		fmt.Println(` + "`" + `{"type":"item.started","item":{"id":"item_0","type":"command_execution","command":"go test ./...","status":"in_progress"}}` + "`" + `)
		fmt.Println(` + "`" + `{"type":"item.completed","item":{"id":"item_0","type":"command_execution","command":"go test ./...","aggregated_output":"ok","exit_code":0,"status":"completed"}}` + "`" + `)
		fmt.Println(` + "`" + `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"all tests pass"}}` + "`" + `)
		fmt.Println(` + "`" + `{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":3,"reasoning_output_tokens":1}}` + "`" + `)
		os.Exit(0)
	}
}
`
	mainGo := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainGo, []byte(src), 0o644); err != nil {
		os.Exit(1)
	}
	fakeCodexBin = filepath.Join(dir, "codex-fake.exe")
	build := exec.Command("go", "build", "-o", fakeCodexBin, mainGo)
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		_, _ = os.Stderr.Write(out)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// ---- helpers ----

func codexSpec(t *testing.T, mode string) *spec.HarnessSpec {
	t.Helper()
	s, err := spec.Parse([]byte("version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\nruntime:\n  type: codex\n  codex:\n    binary: " + fakeCodexBin + "\n    env:\n      - HL_FAKE_CODEX_MODE=" + mode + "\n"))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	return s
}

func collect(t *testing.T, rt hlruntime.Runtime, task string, timeout time.Duration) []hlruntime.RunEvent {
	t.Helper()
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	ch, err := rt.Run(ctx, hlruntime.RunRequest{
		RunID:         "run-codex-1",
		Task:          task,
		WorkspaceRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var events []hlruntime.RunEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func types(events []hlruntime.RunEvent) []string {
	var out []string
	for _, e := range events {
		out = append(out, string(e.Type))
	}
	return out
}

// ---- tests ----

func TestCodexRuntimeSuccess(t *testing.T) {
	rt, err := New(codexSpec(t, "success"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	events := collect(t, rt, "run the tests", 60*time.Second)

	got := types(events)
	if len(got) < 3 || got[0] != "run_start" || got[len(got)-1] != "run_end" {
		t.Fatalf("event sequence = %v, want [run_start ... run_end]", got)
	}

	var toolSeen, modelSeen bool
	var tokensIn, tokensOut int
	for _, e := range events {
		if e.Type == hlruntime.EventToolStart && e.Tool != nil && e.Tool.Name == "command_execution" {
			toolSeen = true
		}
		if e.Type == hlruntime.EventModelEnd && e.Model != nil {
			modelSeen = true
			if e.Model.TokensIn > 0 {
				tokensIn = e.Model.TokensIn
			}
			if e.Model.TokensOut > 0 {
				tokensOut = e.Model.TokensOut
			}
		}
	}
	if !toolSeen {
		t.Fatal("expected a command_execution tool event from the codex transcript")
	}
	if !modelSeen {
		t.Fatal("expected model events from the codex turn")
	}
	// Usage from turn.completed: 10 in / 3 out.
	if tokensIn != 10 {
		t.Fatalf("tokens in = %d, want 10", tokensIn)
	}
	if tokensOut != 3 {
		t.Fatalf("tokens out = %d, want 3", tokensOut)
	}
}

func TestCodexRuntimeFailure(t *testing.T) {
	rt, err := New(codexSpec(t, "fail"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	events := collect(t, rt, "break something", 60*time.Second)

	got := types(events)
	if got[0] != "run_start" || got[len(got)-1] != "run_end" {
		t.Fatalf("event sequence = %v", got)
	}
	hasError := false
	for _, e := range events {
		if e.Type == hlruntime.EventError {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Fatalf("expected an error event for a failing codex run, got %v", got)
	}
}

func TestCodexRuntimeTimeout(t *testing.T) {
	s := codexSpec(t, "timeout")
	s.Budget.Timeout = "500ms"
	rt, err := New(s)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	start := time.Now()
	events := collect(t, rt, "hang", 10*time.Second)
	elapsed := time.Since(start)

	if elapsed > 8*time.Second {
		t.Fatalf("run did not respect the budget timeout: %v", elapsed)
	}
	got := types(events)
	if got[0] != "run_start" || got[len(got)-1] != "run_end" {
		t.Fatalf("event sequence = %v", got)
	}
}

func TestCodexRuntimeRejectsWrongType(t *testing.T) {
	s := codexSpec(t, "success")
	s.Runtime.Type = "trpc"
	if _, err := New(s); err == nil {
		t.Fatal("codex.New accepted a non-codex runtime type")
	}
}

func TestCodexSpecValidates(t *testing.T) {
	bad := "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\nruntime:\n  type: codex\n  codex:\n    sandbox: nope\n"
	if _, err := spec.Parse([]byte(bad)); err == nil {
		t.Fatal("invalid codex sandbox accepted")
	}
	badApproval := "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\nruntime:\n  type: codex\n  codex:\n    ask_for_approval: sometimes\n"
	if _, err := spec.Parse([]byte(badApproval)); err == nil {
		t.Fatal("invalid codex approval mode accepted")
	}
	good := "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\nruntime:\n  type: codex\n"
	s, err := spec.Parse([]byte(good))
	if err != nil {
		t.Fatalf("valid codex spec rejected: %v", err)
	}
	if s.Runtime.Codex.Sandbox != spec.CodexSandboxReadOnly {
		t.Fatalf("default codex sandbox = %q, want read-only", s.Runtime.Codex.Sandbox)
	}
	if s.Runtime.Codex.AskForApproval != spec.CodexApprovalNever {
		t.Fatalf("default approval = %q, want never", s.Runtime.Codex.AskForApproval)
	}
	if s.Runtime.Codex.Binary != "codex" {
		t.Fatalf("default binary = %q, want codex", s.Runtime.Codex.Binary)
	}
	if !strings.Contains(good, "runtime") {
		t.Fatal("sanity")
	}
}
