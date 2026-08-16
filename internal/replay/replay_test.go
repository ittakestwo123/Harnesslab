package replay

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCanonicalizerStable(t *testing.T) {
	c := &Canonicalizer{WorkspaceRoot: "/home/user/proj", TempDir: "/tmp"}
	in1 := []byte(`{"cmd":"ls","dir":"/home/user/proj/src","tmp":"/tmp/x"}`)
	in2 := []byte(`{"tmp":"/tmp/x","dir":"/home/user/proj/src","cmd":"ls"}`) // different key order

	h1, err := c.Hash(KindTool, "shell", in1)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	h2, err := c.Hash(KindTool, "shell", in2)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("hash differs for semantically equal inputs:\n%s\n%s", h1, h2)
	}

	norm, _ := c.Normalize(in1)
	got := string(norm)
	if !contains(got, "$WORKSPACE") || !contains(got, "$TMP") {
		t.Fatalf("normalized = %s, want path placeholders", got)
	}
}

func TestCanonicalizerKindNameSeparated(t *testing.T) {
	c := &Canonicalizer{}
	// (kind, name) must be part of the hash domain.
	a, _ := c.Hash(KindTool, "shell", []byte(`{}`))
	b, _ := c.Hash(KindModel, "shell", []byte(`{}`))
	if a == b {
		t.Fatalf("hashes should differ across kinds")
	}
}

func TestCanonicalizerNonJSON(t *testing.T) {
	c := &Canonicalizer{WorkspaceRoot: "/ws"}
	h, err := c.Hash(KindTool, "x", []byte("run in /ws now"))
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if h == "" {
		t.Fatal("empty hash")
	}
}

func TestJSONStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r", "entries.jsonl")
	s, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	if _, ok, err := s.Lookup(ctx, KindTool, "missing"); err != nil || ok {
		t.Fatalf("lookup missing: ok=%v err=%v", ok, err)
	}

	in := []byte(`{"a":1}`)
	out := []byte(`{"result":42}`)
	if err := s.Put(ctx, Entry{Kind: KindTool, InputHash: "h1", Input: in, Output: out}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, ok, err := s.Lookup(ctx, KindTool, "h1")
	if err != nil || !ok {
		t.Fatalf("lookup h1: ok=%v err=%v", ok, err)
	}
	if string(got) != string(out) {
		t.Fatalf("got %s, want %s", got, out)
	}

	// Reopen and read again (persistence).
	s.Close()
	s2, err := NewJSONStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	got, ok, err = s2.Lookup(ctx, KindTool, "h1")
	if err != nil || !ok || string(got) != string(out) {
		t.Fatalf("reopened lookup: ok=%v err=%v got=%s", ok, err, got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
