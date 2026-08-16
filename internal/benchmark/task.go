// Package benchmark implements Phase-2 benchmarking: benchmark tasks, harness
// matrices, a worker-pool scheduler with budget control, and reports.
package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
)

// Task is one benchmark task: a repository, a prompt, and optional
// verification/success criteria that override the harness defaults.
type Task struct {
	ID           string                `yaml:"id"`
	Repo         string                `yaml:"repo"`
	Commit       string                `yaml:"commit"`
	Prompt       string                `yaml:"prompt"`
	Verification spec.VerificationSpec `yaml:"verification"`
	// Success overrides the harness success criteria (e.g. requiring the
	// workspace to change) when set.
	Success spec.SuccessSpec `yaml:"success"`
}

// Validate checks required fields.
func (t *Task) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("task.id is required")
	}
	if t.Prompt == "" {
		return fmt.Errorf("task %s: prompt is required", t.ID)
	}
	return nil
}

// LoadTasks loads a single task file or every *.yaml in a directory.
func LoadTasks(path string) ([]*Task, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("benchmark: tasks %s: %w", path, err)
	}
	if !info.IsDir() {
		t, err := loadTaskFile(path)
		if err != nil {
			return nil, err
		}
		return []*Task{t}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("benchmark: read dir %s: %w", path, err)
	}
	var tasks []*Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		t, err := loadTaskFile(filepath.Join(path, e.Name()))
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("benchmark: no task yaml files found in %s", path)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	return tasks, nil
}

func loadTaskFile(path string) (*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("benchmark: read %s: %w", path, err)
	}
	var t Task
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("benchmark: parse %s: %w", path, err)
	}
	if err := t.Validate(); err != nil {
		return nil, fmt.Errorf("benchmark: %s: %w", path, err)
	}
	return &t, nil
}
