// Package spec defines the HarnessSpec — the declarative description of a
// complete agent harness (runtime, agent, model, context, tools, verification,
// retry, sandbox and budget). It is the single most important abstraction of
// HarnessLab: a HarnessSpec plus a repository commit should fully determine
// how an agent run is built and executed.
package spec

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// VersionV1 is the current HarnessSpec schema version.
	VersionV1 = "harnesslab/v1"

	// RuntimeTRPC is the tRPC-Agent-Go runtime type.
	RuntimeTRPC = "trpc"

	// RuntimeCodex is the Codex CLI runtime type: a locally installed
	// `codex` binary driven via `codex exec --json`.
	RuntimeCodex = "codex"

	// Codex sandbox modes (codex CLI --sandbox).
	CodexSandboxReadOnly         = "read-only"
	CodexSandboxWorkspaceWrite   = "workspace-write"
	CodexSandboxDangerFullAccess = "danger-full-access"

	// Codex approval modes (codex CLI --ask-for-approval).
	CodexApprovalNever     = "never"
	CodexApprovalOnRequest = "on-request"
	CodexApprovalOnFailure = "on-failure"

	// AgentCoding marks a coding agent harness.
	AgentCoding = "coding"

	// Verification strategies.
	VerificationNone        = "none"
	VerificationFinal       = "final"
	VerificationIncremental = "incremental"
	VerificationTestFirst   = "test-first"

	// Planning strategies.
	PlanningNone = "none"
	PlanningTodo = "todo"

	// Context strategies.
	ContextNone    = "none"
	ContextRepoMap = "repo-map"
)

// HarnessSpec describes one complete harness. It maps 1:1 to a harness.yaml.
type HarnessSpec struct {
	Version      string           `yaml:"version"`
	Name         string           `yaml:"name"`
	Runtime      RuntimeSpec      `yaml:"runtime"`
	Agent        AgentSpec        `yaml:"agent"`
	Model        ModelSpec        `yaml:"model"`
	Planning     PlanningSpec     `yaml:"planning"`
	Context      ContextSpec      `yaml:"context,omitempty"`
	Skills       SkillsSpec       `yaml:"skills,omitempty"`
	Tools        ToolsSpec        `yaml:"tools"`
	Verification VerificationSpec `yaml:"verification"`
	Success      SuccessSpec      `yaml:"success"`
	Retry        RetrySpec        `yaml:"retry"`
	Sandbox      SandboxSpec      `yaml:"sandbox"`
	Budget       BudgetSpec       `yaml:"budget"`
	Pricing      PricingSpec      `yaml:"pricing"`
}

// RuntimeSpec selects the agent runtime and its version.
type RuntimeSpec struct {
	Type    string    `yaml:"type"`
	Version string    `yaml:"version,omitempty"`
	Codex   CodexSpec `yaml:"codex,omitempty"`
}

// CodexSpec configures the Codex CLI runtime (`runtime.type: codex`).
// The runtime drives a locally installed `codex` binary via
// `codex exec --json`; the prompt is written to stdin and stdout is parsed
// as a JSONL event stream by the runtime adapter.
type CodexSpec struct {
	// Binary is the codex executable path (default "codex").
	Binary string `yaml:"binary,omitempty"`
	// Model overrides the codex model (passed as --model to codex exec).
	Model string `yaml:"model,omitempty"`
	// Sandbox is the codex CLI sandbox mode: read-only (default) |
	// workspace-write | danger-full-access.
	Sandbox string `yaml:"sandbox,omitempty"`
	// AskForApproval controls codex approval prompts: never (default) |
	// on-request | on-failure.
	AskForApproval string `yaml:"ask_for_approval,omitempty"`
	// ExtraArgs are appended after `exec` (before --json), e.g. --full-auto.
	ExtraArgs []string `yaml:"extra_args,omitempty"`
	// Env adds KEY=VALUE variables to the codex process environment (e.g.
	// CODEX_HOME=... to select an isolated codex home/profile).
	Env []string `yaml:"env,omitempty"`
}

// AgentSpec selects the agent flavour and optional custom instruction.
type AgentSpec struct {
	Type        string `yaml:"type"`
	Name        string `yaml:"name,omitempty"`
	Instruction string `yaml:"instruction,omitempty"`
}

// ModelSpec selects the LLM provider and model.
type ModelSpec struct {
	Provider    string   `yaml:"provider"`
	Name        string   `yaml:"model"`
	Temperature *float64 `yaml:"temperature,omitempty"`
}

// PlanningSpec controls the planning strategy.
type PlanningSpec struct {
	Strategy string `yaml:"strategy"`
}

// ContextSpec controls how much repository context the agent receives.
// Strategy "none" (default) gives no extra context; "repo-map" injects a
// generated repository structure summary into the system instruction.
// This is the smallest useful adaptive-context strategy; richer strategies
// (retrieval, token-budgeted compaction) can be added behind the same field.
type ContextSpec struct {
	Strategy string `yaml:"strategy"`
}

// SkillsSpec enables static skills: named, reusable working procedures
// injected into the system instruction. This is a clean interface with a
// minimal strategy (instruction sections); tool-backed skills can be layered
// on later without changing the schema.
type SkillsSpec struct {
	// Enabled turns the skills section on.
	Enabled bool `yaml:"enabled"`
	// List is an ordered list of skill definitions, each rendered as a
	// titled instruction section, e.g. "debug: run the failing test first,
	// read the error, then bisect".
	List []string `yaml:"list,omitempty"`
}

// ToolsSpec toggles which built-in tool groups the agent gets.
type ToolsSpec struct {
	Filesystem bool `yaml:"filesystem"`
	Shell      bool `yaml:"shell"`
	Git        bool `yaml:"git"`
	Search     bool `yaml:"search"`
}

// VerificationSpec defines how a run is verified after the agent finishes.
type VerificationSpec struct {
	Strategy        string   `yaml:"strategy"`
	Commands        []string `yaml:"commands"`
	RequireCleanGit bool     `yaml:"require_clean_git,omitempty"`
}

// SuccessSpec defines what counts as a successful run. It exists so that
// "answered from knowledge" runs cannot fake a PASS: a coding task must
// actually pass its verification commands and (optionally) modify the
// workspace.
type SuccessSpec struct {
	// RequireVerificationPass requires every verification command to pass.
	// nil means true (the default).
	RequireVerificationPass *bool `yaml:"require_verification_pass,omitempty"`
	// RequireWorkspaceChange requires the agent to have modified the
	// workspace (non-empty git diff or new untracked files). nil means
	// false (the default).
	RequireWorkspaceChange *bool `yaml:"require_workspace_change,omitempty"`
}

// RequireVerification returns whether verification must pass (default true).
func (s *SuccessSpec) RequireVerification() bool {
	return s == nil || s.RequireVerificationPass == nil || *s.RequireVerificationPass
}

// RequireChange returns whether the workspace must have been modified
// (default false).
func (s *SuccessSpec) RequireChange() bool {
	return s != nil && s.RequireWorkspaceChange != nil && *s.RequireWorkspaceChange
}

// IsSet reports whether any success criterion is explicitly configured.
func (s *SuccessSpec) IsSet() bool {
	return s != nil && (s.RequireVerificationPass != nil || s.RequireWorkspaceChange != nil)
}

// RetrySpec bounds model and tool error retries.
type RetrySpec struct {
	MaxModelErrors int `yaml:"max_model_errors"`
	MaxToolErrors  int `yaml:"max_tool_errors"`
}

// Sandbox types.
const (
	SandboxNone    = "none"
	SandboxProcess = "process"
	SandboxDocker  = "docker"
	SandboxBwrap   = "bwrap"
)

// SandboxSpec describes the sandbox the agent runs in. `none` executes on
// the host (default); `process` adds cwd isolation, env scrubbing, timeouts
// and command allow/deny lists; `docker` and `bwrap` provide real container
// isolation.
type SandboxSpec struct {
	// Type is none | process | docker | bwrap.
	Type string `yaml:"type"`
	// Network maps to backend policy: restricted | allowlist | none.
	Network string `yaml:"network,omitempty"`
	// Image is the container image for the docker backend.
	Image string `yaml:"image,omitempty"`
	// Timeout bounds a single sandboxed command (e.g. "30s").
	Timeout string `yaml:"timeout,omitempty"`
	// AllowedCommands, when non-empty, restricts process-backend commands to
	// this allowlist (matched by prefix).
	AllowedCommands []string `yaml:"allowed_commands,omitempty"`
	// DeniedCommands rejects process-backend commands containing any of
	// these substrings.
	DeniedCommands []string `yaml:"denied_commands,omitempty"`
}

// CommandTimeout parses the per-command timeout (0 when unset).
func (s *SandboxSpec) CommandTimeout() (time.Duration, error) {
	if s.Timeout == "" {
		return 0, nil
	}
	return time.ParseDuration(s.Timeout)
}

// SandboxEnabled reports whether a non-none sandbox is configured.
func (s *SandboxSpec) SandboxEnabled() bool {
	return s.Type != "" && s.Type != SandboxNone
}

// BudgetSpec bounds a single run's cost and duration.
type BudgetSpec struct {
	MaxTokens  int     `yaml:"max_tokens"`
	MaxCostUSD float64 `yaml:"max_cost_usd"`
	Timeout    string  `yaml:"timeout"`
}

// ModelPricing is the USD price per million tokens for one model.
type ModelPricing struct {
	InputPerMillion  float64 `yaml:"input_per_million"`
	OutputPerMillion float64 `yaml:"output_per_million"`
}

// PricingSpec maps provider -> model -> price. The model key may be "*" for
// a provider-wide default. Costs are only computed when pricing is set.
type PricingSpec map[string]map[string]ModelPricing

// ModelPrice resolves the price for (provider, model), falling back to the
// provider-wide "*" entry.
func (p PricingSpec) ModelPrice(provider, model string) (ModelPricing, bool) {
	if p == nil {
		return ModelPricing{}, false
	}
	byModel, ok := p[provider]
	if !ok {
		return ModelPricing{}, false
	}
	if mp, ok := byModel[model]; ok {
		return mp, true
	}
	mp, ok := byModel["*"]
	return mp, ok
}

// Load reads, parses and validates a HarnessSpec from a YAML file.
func Load(path string) (*HarnessSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("spec: read %s: %w", path, err)
	}
	s, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("spec: parse %s: %w", path, err)
	}
	return s, nil
}

// Parse parses and validates a HarnessSpec from raw YAML bytes.
func Parse(data []byte) (*HarnessSpec, error) {
	var s HarnessSpec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// Validate checks required fields and known enum values.
func (s *HarnessSpec) Validate() error {
	if s.Version != VersionV1 {
		return fmt.Errorf("unsupported spec version %q (want %q)", s.Version, VersionV1)
	}
	if s.Name == "" {
		return errors.New("spec.name is required")
	}
	if s.Runtime.Type == "" {
		s.Runtime.Type = RuntimeTRPC
	}
	switch s.Runtime.Type {
	case RuntimeTRPC, RuntimeCodex:
	default:
		return fmt.Errorf("unsupported runtime type %q (supported: trpc, codex)", s.Runtime.Type)
	}
	if s.Runtime.Type == RuntimeCodex {
		switch s.Runtime.Codex.Sandbox {
		case "":
			s.Runtime.Codex.Sandbox = CodexSandboxReadOnly
		case CodexSandboxReadOnly, CodexSandboxWorkspaceWrite, CodexSandboxDangerFullAccess:
		default:
			return fmt.Errorf("unsupported codex sandbox mode %q (supported: read-only, workspace-write, danger-full-access)", s.Runtime.Codex.Sandbox)
		}
		switch s.Runtime.Codex.AskForApproval {
		case "":
			s.Runtime.Codex.AskForApproval = CodexApprovalNever
		case CodexApprovalNever, CodexApprovalOnRequest, CodexApprovalOnFailure:
		default:
			return fmt.Errorf("unsupported codex approval mode %q (supported: never, on-request, on-failure)", s.Runtime.Codex.AskForApproval)
		}
		if s.Runtime.Codex.Binary == "" {
			s.Runtime.Codex.Binary = "codex"
		}
	}
	if s.Agent.Type == "" {
		s.Agent.Type = AgentCoding
	}
	if s.Agent.Type != AgentCoding {
		return fmt.Errorf("unsupported agent type %q (want %q)", s.Agent.Type, AgentCoding)
	}
	if s.Agent.Name == "" {
		s.Agent.Name = s.Name
	}
	if s.Model.Name == "" {
		return errors.New("spec.model.model is required (e.g. gpt-5 or deepseek-chat)")
	}
	if s.Model.Provider == "" {
		s.Model.Provider = "openai"
	}
	switch s.Planning.Strategy {
	case "", PlanningNone:
		s.Planning.Strategy = PlanningNone
	case PlanningTodo:
	default:
		return fmt.Errorf("unsupported planning strategy %q", s.Planning.Strategy)
	}
	switch s.Context.Strategy {
	case "":
		s.Context.Strategy = ContextNone
	case ContextNone, ContextRepoMap:
	default:
		return fmt.Errorf("unsupported context strategy %q (supported: none, repo-map)", s.Context.Strategy)
	}
	switch s.Verification.Strategy {
	case "":
		s.Verification.Strategy = VerificationFinal
	case VerificationNone, VerificationFinal, VerificationIncremental, VerificationTestFirst:
	default:
		return fmt.Errorf("unsupported verification strategy %q", s.Verification.Strategy)
	}
	switch s.Sandbox.Type {
	case "":
		s.Sandbox.Type = SandboxNone
	case SandboxNone, SandboxProcess, SandboxDocker, SandboxBwrap:
	default:
		return fmt.Errorf("unsupported sandbox type %q (supported: none, process, docker, bwrap)", s.Sandbox.Type)
	}
	switch s.Sandbox.Network {
	case "", "restricted", "allowlist", "none":
	default:
		return fmt.Errorf("unsupported sandbox network mode %q", s.Sandbox.Network)
	}
	if s.Sandbox.Timeout != "" {
		if _, err := time.ParseDuration(s.Sandbox.Timeout); err != nil {
			return fmt.Errorf("spec.sandbox.timeout %q is not a valid duration: %w", s.Sandbox.Timeout, err)
		}
	}
	if s.Sandbox.Type == SandboxDocker && s.Sandbox.Image == "" {
		s.Sandbox.Image = "alpine:latest"
	}
	if s.Budget.Timeout != "" {
		if _, err := time.ParseDuration(s.Budget.Timeout); err != nil {
			return fmt.Errorf("spec.budget.timeout %q is not a valid duration: %w", s.Budget.Timeout, err)
		}
	}
	for provider, models := range s.Pricing {
		for model, mp := range models {
			if mp.InputPerMillion < 0 || mp.OutputPerMillion < 0 {
				return fmt.Errorf("spec.pricing.%s.%s: prices must be >= 0", provider, model)
			}
		}
	}
	return nil
}

// Timeout returns the budget timeout as a duration, or 0 when unset.
func (s *HarnessSpec) Timeout() (time.Duration, error) {
	if s.Budget.Timeout == "" {
		return 0, nil
	}
	return time.ParseDuration(s.Budget.Timeout)
}

// ToolsEnabled reports whether at least one tool group is enabled.
func (s *HarnessSpec) ToolsEnabled() bool {
	return s.Tools.Filesystem || s.Tools.Shell || s.Tools.Git || s.Tools.Search
}
