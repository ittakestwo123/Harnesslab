// Package benchmark implements Phase-2 benchmarking: benchmark tasks, harness
// matrices, a worker-pool scheduler with budget control, and reports.
package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
)

// Task categories. Benchmarks group tasks by the kind of coding work they
// require, so reports can show per-category discriminative power.
const (
	CategoryBasic        = "basic"
	CategoryDebugging    = "debugging"
	CategoryMultiFile    = "multi-file"
	CategoryRefactor     = "refactor"
	CategoryTesting      = "testing"
	CategoryConcurrency  = "concurrency"
	CategoryContextHeavy = "context-heavy"
	CategoryToolHeavy    = "tool-heavy"
)

// ValidCategories lists the accepted task categories.
var ValidCategories = []string{
	CategoryBasic, CategoryDebugging, CategoryMultiFile, CategoryRefactor,
	CategoryTesting, CategoryConcurrency, CategoryContextHeavy, CategoryToolHeavy,
}

// Task sets partition the taskset for optimizer development.
const (
	SetDev     = "dev"
	SetHoldout = "holdout"
)

// Task is one benchmark task: a repository, a prompt, and optional
// verification/success criteria that override the harness defaults.
type Task struct {
	ID           string                `yaml:"id"`
	Category     string                `yaml:"category,omitempty"`
	Set          string                `yaml:"set,omitempty"`
	Repo         string                `yaml:"repo"`
	Commit       string                `yaml:"commit"`
	Prompt       string                `yaml:"prompt"`
	Verification spec.VerificationSpec `yaml:"verification"`
	// Success overrides the harness success criteria (e.g. requiring the
	// workspace to change) when set.
	Success spec.SuccessSpec `yaml:"success"`
}

// Validate checks required fields and known enum values.
func (t *Task) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("task.id is required")
	}
	if t.Prompt == "" {
		return fmt.Errorf("task %s: prompt is required", t.ID)
	}
	if t.Category != "" && !slices.Contains(ValidCategories, t.Category) {
		return fmt.Errorf("task %s: unsupported category %q (supported: %s)",
			t.ID, t.Category, strings.Join(ValidCategories, ", "))
	}
	if t.Set != "" && t.Set != SetDev && t.Set != SetHoldout {
		return fmt.Errorf("task %s: unsupported set %q (supported: dev, holdout)", t.ID, t.Set)
	}
	return nil
}

// LoadTasks loads a single task file or every *.yaml under a directory
// (recursively, so category subdirectories like tasks/debugging/ work).
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
	var tasks []*Task
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".yaml") && !strings.HasSuffix(d.Name(), ".yml") {
			return nil
		}
		t, err := loadTaskFile(p)
		if err != nil {
			return err
		}
		tasks = append(tasks, t)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("benchmark: walk %s: %w", path, err)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("benchmark: no task yaml files found in %s", path)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	return tasks, nil
}

// TaskSet is an explicit list of task ids (tasksets/dev.yaml, holdout.yaml).
type TaskSet struct {
	ID    string   `yaml:"id"`
	Name  string   `yaml:"name"`
	Tasks []string `yaml:"tasks"`
}

// LoadTaskSet loads a taskset file (a YAML list of task ids).
func LoadTaskSet(path string) (*TaskSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("benchmark: taskset %s: %w", path, err)
	}
	var ts TaskSet
	if err := yaml.Unmarshal(data, &ts); err != nil {
		return nil, fmt.Errorf("benchmark: parse taskset %s: %w", path, err)
	}
	if len(ts.Tasks) == 0 {
		return nil, fmt.Errorf("benchmark: taskset %s has no tasks", path)
	}
	return &ts, nil
}

// Contains reports whether id is in the taskset.
func (ts *TaskSet) Contains(id string) bool {
	for _, t := range ts.Tasks {
		if t == id {
			return true
		}
	}
	return false
}

// FilterBySet keeps tasks whose Set field matches (dev or holdout). An empty
// set returns all tasks unchanged.
func FilterBySet(tasks []*Task, set string) []*Task {
	if set == "" {
		return tasks
	}
	var out []*Task
	for _, t := range tasks {
		if t.Set == set {
			out = append(out, t)
		}
	}
	return out
}

// FilterByTaskSet keeps tasks listed in the taskset. When taskset is nil all
// tasks are kept.
func FilterByTaskSet(tasks []*Task, ts *TaskSet) []*Task {
	if ts == nil {
		return tasks
	}
	var out []*Task
	for _, t := range tasks {
		if ts.Contains(t.ID) {
			out = append(out, t)
		}
	}
	return out
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
