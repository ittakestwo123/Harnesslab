package trpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/tool"

	"github.com/ittakestwo123/Harnesslab/internal/replay"
	hlruntime "github.com/ittakestwo123/Harnesslab/internal/runtime"
)

// memStore is a minimal in-memory replay store for tests.
type memStore struct {
	entries map[string]replay.Entry
}

func newMemStore() *memStore {
	return &memStore{entries: map[string]replay.Entry{}}
}

func (m *memStore) Lookup(_ context.Context, kind replay.Kind, hash string) (json.RawMessage, bool, error) {
	e, ok := m.entries[string(kind)+"\x00"+hash]
	if !ok {
		return nil, false, nil
	}
	return e.Output, true, nil
}

func (m *memStore) Put(_ context.Context, e replay.Entry) error {
	m.entries[string(e.Kind)+"\x00"+e.InputHash] = e
	return nil
}

// TestReplayStoresNormalizedToolResult pins that recorded tool results have
// absolute workspace paths replaced by $WORKSPACE, so a replay in a fresh
// worktree hashes identically for every later model call.
func TestReplayStoresNormalizedToolResult(t *testing.T) {
	store := newMemStore()
	canon := &replay.Canonicalizer{WorkspaceRoot: `C:\ws\worktrees\run-abc`}
	cfg := &hlruntime.ReplayConfig{Mode: hlruntime.ReplayRecord, Store: store, Canonicalizer: canon}
	cbs := buildReplayToolCallbacks(cfg, canon)
	if cbs == nil {
		t.Fatal("nil callbacks")
	}

	args := &tool.AfterToolArgs{
		ToolName:  "read_file",
		Arguments: []byte(`{"file_name":"README.md"}`),
		Result: map[string]any{
			"base_directory": `C:\ws\worktrees\run-abc`,
			"contents":       "# Demo",
			"file_name":      "README.md",
		},
	}
	if _, err := cbs.RunAfterTool(context.Background(), args); err != nil {
		t.Fatalf("RunAfterTool: %v", err)
	}

	// One entry stored, output normalized.
	if len(store.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(store.entries))
	}
	for _, e := range store.entries {
		if !strings.Contains(string(e.Output), "$WORKSPACE") {
			t.Fatalf("stored output not normalized: %s", e.Output)
		}
		if strings.Contains(string(e.Output), "run-abc") {
			t.Fatalf("stored output leaks the absolute workspace path: %s", e.Output)
		}
	}
}

// TestReplayServesNormalizedToolResult pins the replay side: a strict lookup
// returns the stored (normalized) output, which a fresh-worktree canonicalizer
// leaves untouched.
func TestReplayServesNormalizedToolResult(t *testing.T) {
	store := newMemStore()
	recordCanon := &replay.Canonicalizer{WorkspaceRoot: `C:\ws\worktrees\run-abc`}
	recordCfg := &hlruntime.ReplayConfig{Mode: hlruntime.ReplayRecord, Store: store, Canonicalizer: recordCanon}
	cbs := buildReplayToolCallbacks(recordCfg, recordCanon)
	if _, err := cbs.RunAfterTool(context.Background(), &tool.AfterToolArgs{
		ToolName:  "read_file",
		Arguments: []byte(`{"file_name":"README.md"}`),
		Result:    map[string]any{"base_directory": `C:\ws\worktrees\run-abc`, "contents": "hi"},
	}); err != nil {
		t.Fatal(err)
	}

	// Replay in a NEW worktree: the canonicalizer root differs, but the stored
	// output is already normalized so the hash still matches.
	replayCanon := &replay.Canonicalizer{WorkspaceRoot: `C:\ws\worktrees\run-xyz`}
	replayCfg := &hlruntime.ReplayConfig{Mode: hlruntime.ReplayStrict, Store: store, Canonicalizer: replayCanon}
	replayCbs := buildReplayToolCallbacks(replayCfg, replayCanon)
	res, err := replayCbs.RunBeforeTool(context.Background(), &tool.BeforeToolArgs{
		ToolName:  "read_file",
		Arguments: []byte(`{"file_name":"README.md"}`),
	})
	if err != nil {
		t.Fatalf("strict replay miss: %v", err)
	}
	if res == nil || res.CustomResult == nil {
		t.Fatalf("no custom result served: %+v", res)
	}
}
