package reproduce

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	"github.com/ittakestwo123/Harnesslab/internal/store"
)

func TestGenerateLock(t *testing.T) {
	s, err := spec.Parse([]byte(spec.DefaultTemplate))
	if err != nil {
		t.Fatal(err)
	}
	data, err := GenerateLock(s)
	if err != nil {
		t.Fatalf("GenerateLock: %v", err)
	}
	text := string(data)
	for _, want := range []string{"harnesslab/v1", "golang-coding-default", "openai", "go_version"} {
		if !strings.Contains(text, want) {
			t.Fatalf("lock missing %q:\n%s", want, text)
		}
	}
}

func TestExportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// Fake harness dir with trace + replay.
	tracePath := filepath.Join(dir, "traces", "run-1.jsonl")
	if err := os.MkdirAll(filepath.Dir(tracePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracePath, []byte("{\"type\":\"run_start\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	replayPath := filepath.Join(dir, "replay", "run-1", "entries.jsonl")
	if err := os.MkdirAll(filepath.Dir(replayPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replayPath, []byte("{\"kind\":\"tool\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, _ := spec.Parse([]byte(spec.DefaultTemplate))
	sy, _ := marshalSpec(s)
	run := &store.Run{
		ID: "run-1", Task: "fix bug", Repository: "https://example.com/x.git",
		Commit: "abc123", Status: store.StatusPassed, SpecYAML: sy,
		Metrics: store.Metrics{InputTokens: 100},
	}

	bundle := filepath.Join(dir, "run-1.harness")
	if err := Export(ExportOptions{HarnessDir: dir, Run: run, OutPath: bundle}); err != nil {
		t.Fatalf("Export: %v", err)
	}

	extractDir := filepath.Join(dir, "extracted")
	if err := Extract(bundle, extractDir); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	m, err := LoadManifest(extractDir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.RunID != "run-1" || m.Repo != "https://example.com/x.git" {
		t.Fatalf("manifest = %+v", m)
	}
	for _, f := range []string{"harness.yaml", "harness.lock", "trace.jsonl", "metrics.json", "git.patch", "environment.json", filepath.Join("replay", "run-1", "entries.jsonl")} {
		if _, err := os.Stat(filepath.Join(extractDir, f)); err != nil {
			t.Fatalf("bundle missing %s: %v", f, err)
		}
	}
}

func marshalSpec(s *spec.HarnessSpec) (string, error) {
	data, err := yaml.Marshal(s)
	return string(data), err
}
