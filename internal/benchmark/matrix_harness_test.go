package benchmark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatrixHarnessVariants(t *testing.T) {
	dir := t.TempDir()
	h0 := filepath.Join(dir, "h0-baseline.yaml")
	h1 := filepath.Join(dir, "h1-planner.yaml")
	write := func(p, content string) {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(h0, "version: harnesslab/v1\nname: h0\nmodel:\n  provider: deepseek\n  model: deepseek-chat\n")
	write(h1, "version: harnesslab/v1\nname: h1\nmodel:\n  provider: deepseek\n  model: deepseek-chat\nplanning:\n  strategy: todo\n")

	m := Matrix{Harness: []string{"h0-baseline.yaml", "h1-planner.yaml"}}
	variants, err := m.HarnessVariants(dir)
	if err != nil {
		t.Fatalf("HarnessVariants: %v", err)
	}
	if len(variants) != 2 {
		t.Fatalf("variants = %d, want 2", len(variants))
	}
	if variants[0].Name != "h0-baseline" || variants[0].Spec.Name != "h0" {
		t.Fatalf("h0 = %+v", variants[0])
	}
	if variants[1].Name != "h1-planner" || variants[1].Spec.Planning.Strategy != "todo" {
		t.Fatalf("h1 = %+v", variants[1])
	}
}

func TestMatrixHarnessVariantsMissingFile(t *testing.T) {
	m := Matrix{Harness: []string{"does-not-exist.yaml"}}
	if _, err := m.HarnessVariants(t.TempDir()); err == nil {
		t.Fatal("missing harness file accepted")
	}
}
