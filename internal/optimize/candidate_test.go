package optimize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
)

func testSpec(t *testing.T, name string) *spec.HarnessSpec {
	t.Helper()
	s, err := spec.Parse([]byte("version: harnesslab/v1\nname: " + name + "\nmodel:\n  model: gpt-5\n"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestWriteLoadCandidateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := testSpec(t, "candidate-001")
	s.Planning.Strategy = spec.PlanningTodo
	md := Metadata{
		Parent:         "baseline",
		Reason:         []string{"no_tools", "late verification"},
		ExpectedEffect: Effect{Success: "increase", Tokens: "neutral"},
	}
	file, err := WriteCandidate(dir, "001", s, md)
	if err != nil {
		t.Fatalf("WriteCandidate: %v", err)
	}
	if file != "candidate-001.yaml" {
		t.Fatalf("file = %q", file)
	}

	// A candidate file must also parse as a plain HarnessSpec (metadata block
	// ignored by spec.Parse).
	data, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := spec.Parse(data)
	if err != nil {
		t.Fatalf("candidate file is not a valid harness spec: %v", err)
	}
	if parsed.Planning.Strategy != "todo" {
		t.Fatalf("planning lost in round trip: %+v", parsed.Planning)
	}

	cands, err := LoadCandidates(dir)
	if err != nil {
		t.Fatalf("LoadCandidates: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("candidates = %d, want 1", len(cands))
	}
	if cands[0].Metadata.Parent != "baseline" || len(cands[0].Metadata.Reason) != 2 {
		t.Fatalf("metadata = %+v", cands[0].Metadata)
	}
	if cands[0].Metadata.ExpectedEffect.Success != "increase" {
		t.Fatalf("effect = %+v", cands[0].Metadata.ExpectedEffect)
	}
	if cands[0].Spec.Name != "candidate-001" || cands[0].Spec.Planning.Strategy != "todo" {
		t.Fatalf("spec = %+v", cands[0].Spec)
	}
}

func TestLoadCandidatesSkipsInvalid(t *testing.T) {
	dir := t.TempDir()
	s := testSpec(t, "ok")
	if _, err := WriteCandidate(dir, "ok", s, Metadata{}); err != nil {
		t.Fatal(err)
	}
	bad := "metadata:\n  parent: x\nversion: harnesslab/v1\nname: bad\nmodel:\n  model: gpt-5\nsandbox:\n  type: nope\n"
	if err := os.WriteFile(filepath.Join(dir, "candidate-bad.yaml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	cands, err := LoadCandidates(dir)
	if err == nil {
		t.Fatal("expected an error mentioning the skipped candidate")
	}
	if !strings.Contains(err.Error(), "candidate-bad.yaml") {
		t.Fatalf("error should name the bad file: %v", err)
	}
	if len(cands) != 1 || cands[0].File != "candidate-ok.yaml" {
		t.Fatalf("valid candidate lost: %+v", cands)
	}
}

func TestLoadCandidatesMissingDir(t *testing.T) {
	cands, err := LoadCandidates(filepath.Join(t.TempDir(), "nope"))
	if err != nil || cands != nil {
		t.Fatalf("missing dir should yield nil,nil: %v %v", cands, err)
	}
}
