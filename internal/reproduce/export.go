package reproduce

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
	"github.com/ittakestwo123/Harnesslab/internal/store"
)

// Manifest describes a reproducible run bundle.
type Manifest struct {
	RunID      string    `json:"run_id"`
	Task       string    `json:"task"`
	Repo       string    `json:"repo,omitempty"`
	Commit     string    `json:"commit,omitempty"`
	Harness    string    `json:"harness_name"`
	Status     string    `json:"status"`
	ExportedAt time.Time `json:"exported_at"`
	Files      []string  `json:"files"`
}

// ExportOptions configures a bundle export.
type ExportOptions struct {
	// HarnessDir locates traces/replay for the run.
	HarnessDir string
	// Run is the run record to export.
	Run *store.Run
	// OutPath is the destination .harness zip file.
	OutPath string
}

// Export writes a reproducible bundle for one run as a zip file:
// manifest.json, harness.yaml, harness.lock, trace.jsonl, metrics.json,
// git.patch, environment.json and replay/<run-id>/entries.jsonl.
func Export(opts ExportOptions) error {
	if opts.Run == nil {
		return fmt.Errorf("reproduce: nil run")
	}
	run := opts.Run

	specYAML := run.SpecYAML
	if specYAML == "" {
		data, err := os.ReadFile(filepath.Join(opts.HarnessDir, "harness.yaml"))
		if err == nil {
			specYAML = string(data)
		}
	}
	var s *spec.HarnessSpec
	if specYAML != "" {
		s, _ = spec.Parse([]byte(specYAML))
	}

	lockData, _ := GenerateLock(s)
	envData, _ := json.MarshalIndent(Capture(), "", "  ")
	metricsData, _ := json.MarshalIndent(run.Metrics, "", "  ")

	entries := map[string][]byte{
		"harness.yaml":     []byte(specYAML),
		"harness.lock":     lockData,
		"metrics.json":     metricsData,
		"git.patch":        []byte(run.WorkspacePatch),
		"environment.json": envData,
	}

	if data, err := readFile(filepath.Join(opts.HarnessDir, "traces", run.ID+".jsonl")); err == nil {
		entries["trace.jsonl"] = data
	}
	if data, err := readFile(filepath.Join(opts.HarnessDir, "replay", run.ID, "entries.jsonl")); err == nil {
		entries[filepath.Join("replay", run.ID, "entries.jsonl")] = data
	}

	manifest := Manifest{
		RunID:      run.ID,
		Task:       run.Task,
		Repo:       run.Repository,
		Commit:     run.Commit,
		Harness:    run.HarnessName,
		Status:     string(run.Status),
		ExportedAt: time.Now(),
	}
	for name := range entries {
		manifest.Files = append(manifest.Files, name)
	}
	manifestData, _ := json.MarshalIndent(&manifest, "", "  ")
	entries["manifest.json"] = manifestData

	return writeZip(opts.OutPath, entries)
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func writeZip(path string, entries map[string][]byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("reproduce: create %s: %w", path, err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("reproduce: zip entry %s: %w", name, err)
		}
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("reproduce: zip write %s: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("reproduce: zip close: %w", err)
	}
	return nil
}

// Extract unzips a bundle into dir.
func Extract(bundlePath, dir string) error {
	zr, err := zip.OpenReader(bundlePath)
	if err != nil {
		return fmt.Errorf("reproduce: open bundle %s: %w", bundlePath, err)
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		target := filepath.Join(dir, zf.Name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// LoadManifest reads manifest.json from an extracted bundle dir.
func LoadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
