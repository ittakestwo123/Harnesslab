package trpc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
)

func parseSpec(t *testing.T, yaml string) *spec.HarnessSpec {
	t.Helper()
	s, err := spec.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	return s
}

func TestBuildInstructionBase(t *testing.T) {
	s := parseSpec(t, "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\n")
	instr := buildInstruction(s, "")
	if !strings.Contains(instr, "coding agent") {
		t.Fatalf("missing base instruction: %q", instr)
	}
	if strings.Contains(instr, "Planning:") || strings.Contains(instr, "Skills") || strings.Contains(instr, "Repository map") {
		t.Fatalf("unexpected sections in base instruction: %q", instr)
	}
}

func TestBuildInstructionPlanningAndVerification(t *testing.T) {
	s := parseSpec(t, "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\nplanning:\n  strategy: todo\nverification:\n  commands:\n    - go test ./...\n")
	instr := buildInstruction(s, "")
	if !strings.Contains(instr, "Planning:") || !strings.Contains(instr, "go test ./...") {
		t.Fatalf("missing planning/verification sections: %q", instr)
	}
}

func TestBuildInstructionRepoMap(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "a", "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := parseSpec(t, "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\ncontext:\n  strategy: repo-map\n")
	instr := buildInstruction(s, root)
	if !strings.Contains(instr, "Repository map") {
		t.Fatalf("missing repository map section: %q", instr)
	}
	if !strings.Contains(instr, "internal/a/a.go") {
		t.Fatalf("repo map missing file: %q", instr)
	}
	// .git should never appear.
	if strings.Contains(instr, ".git") {
		t.Fatalf("repo map leaked .git: %q", instr)
	}
}

func TestBuildInstructionRepoMapNoRoot(t *testing.T) {
	s := parseSpec(t, "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\ncontext:\n  strategy: repo-map\n")
	instr := buildInstruction(s, "")
	if strings.Contains(instr, "Repository map") {
		t.Fatalf("repo map rendered without root: %q", instr)
	}
}

func TestBuildInstructionSkills(t *testing.T) {
	s := parseSpec(t, "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\nskills:\n  enabled: true\n  list:\n    - \"debug: run the failing test, read the error, then bisect\"\n    - \"verify: always run go test after editing\"\n")
	instr := buildInstruction(s, "")
	if !strings.Contains(instr, "Skills (follow these working procedures):") {
		t.Fatalf("missing skills section: %q", instr)
	}
	if !strings.Contains(instr, "debug: run the failing test") || !strings.Contains(instr, "always run go test") {
		t.Fatalf("missing skill entries: %q", instr)
	}
}

func TestBuildInstructionSkillsDisabled(t *testing.T) {
	s := parseSpec(t, "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\nskills:\n  enabled: false\n  list:\n    - \"debug: x\"\n")
	instr := buildInstruction(s, "")
	if strings.Contains(instr, "Skills") {
		t.Fatalf("disabled skills rendered: %q", instr)
	}
}
