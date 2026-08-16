// Package reproduce implements Phase-3 reproducibility: harness lock files,
// environment capture, and reproducible run bundles (export/import).
package reproduce

import (
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
)

// Environment captures the toolchain a run was produced with.
type Environment struct {
	GoVersion   string `json:"go_version" yaml:"go_version"`
	GOOS        string `json:"goos" yaml:"goos"`
	GOARCH      string `json:"goarch" yaml:"goarch"`
	GitVersion  string `json:"git_version,omitempty" yaml:"git_version,omitempty"`
	TRPCAgentGo string `json:"trpc_agent_go,omitempty" yaml:"trpc_agent_go,omitempty"`
	HarnessLab  string `json:"harnesslab,omitempty" yaml:"harnesslab,omitempty"`
}

// Capture snapshots the current environment.
func Capture() Environment {
	env := Environment{
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
	}
	if out, err := exec.Command("git", "--version").Output(); err == nil {
		env.GitVersion = strings.TrimSpace(string(out))
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		env.HarnessLab = bi.Main.Version
		for _, m := range bi.Deps {
			if m.Path == "trpc.group/trpc-go/trpc-agent-go" {
				env.TRPCAgentGo = m.Version
				break
			}
		}
	}
	return env
}

// Lock is the harness.lock: a frozen description of the harness a run was
// executed under, plus the environment.
type Lock struct {
	Version      string                `yaml:"version"`
	Name         string                `yaml:"name"`
	Runtime      spec.RuntimeSpec      `yaml:"runtime"`
	Model        spec.ModelSpec        `yaml:"model"`
	Planning     spec.PlanningSpec     `yaml:"planning"`
	Tools        spec.ToolsSpec        `yaml:"tools"`
	Verification spec.VerificationSpec `yaml:"verification"`
	Budget       spec.BudgetSpec       `yaml:"budget"`
	Environment  Environment           `yaml:"environment"`
}

// GenerateLock renders a harness.lock for the given spec.
func GenerateLock(s *spec.HarnessSpec) ([]byte, error) {
	lock := Lock{
		Version:      s.Version,
		Name:         s.Name,
		Runtime:      s.Runtime,
		Model:        s.Model,
		Planning:     s.Planning,
		Tools:        s.Tools,
		Verification: s.Verification,
		Budget:       s.Budget,
		Environment:  Capture(),
	}
	return yaml.Marshal(&lock)
}
