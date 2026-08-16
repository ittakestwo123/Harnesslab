package optimize

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
)

// LLMClient is the minimal model surface the optimizer needs; tests stub it.
type LLMClient interface {
	// Complete returns the model's completion for one request.
	Complete(ctx context.Context, system, user string) (string, error)
}

// generatePrompt is the JSON/YAML contract the LLM must fill: a list of
// candidate harness specs with metadata.
type generatePrompt struct {
	Candidates []llmCandidate `yaml:"candidates"`
}

// llmCandidate is one entry of the LLM response.
type llmCandidate struct {
	Name     string         `yaml:"name"`
	Metadata Metadata       `yaml:"metadata"`
	Harness  map[string]any `yaml:"harness"`
}

// GenerateCandidates asks the LLM to produce n harness candidates that
// target the failure patterns in a, derived from base. Each candidate is a
// full harness spec plus metadata. Unparseable or invalid candidates are
// skipped (with a non-fatal error summary).
func GenerateCandidates(ctx context.Context, llm LLMClient, base *spec.HarnessSpec, a *Analysis, n int) ([]*Candidate, error) {
	if n <= 0 {
		n = 3
	}
	baseYAML, err := yaml.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("optimize: marshal base harness: %w", err)
	}

	var patterns []string
	for _, p := range a.Patterns {
		patterns = append(patterns, fmt.Sprintf("- %s (%d/%d runs)", p.Label, p.Count, a.Runs))
	}
	if len(patterns) == 0 {
		patterns = append(patterns, "- none detected")
	}

	system := `You are an expert harness engineer for AI coding agents. A "harness" is the
declarative configuration around an LLM: planning strategy, injected context,
skills, tools, verification, retry, sandbox and budget. You will be given the
current harness spec and the failure analysis of a benchmark run.

Your job: propose ` + fmt.Sprintf("%d", n) + ` DIFFERENT harness candidates that
plausibly improve the observed failures. Each candidate must be a COMPLETE
valid harnesslab/v1 harness spec (same schema as the base, same
model/pricing/sandbox/budget unless there is a strong reason to change them)
that changes at most a few harness strategy fields.

ALLOWED ENUM VALUES (use only these, never invent new values):
- runtime.type: trpc | codex
- planning.strategy: none | todo
- context.strategy: none | repo-map
- skills.enabled: true | false
- verification.strategy: none | final | incremental | test-first
- sandbox.type: none | process | docker | bwrap
- success.require_verification_pass / require_workspace_change: true | false

Respond with ONLY a YAML document in exactly this shape:

candidates:
  - name: candidate-001
    metadata:
      parent: baseline
      reason:
        - <failure pattern id or short observation>
      expected_effect:
        success: increase        # increase | neutral | decrease
        tokens: neutral          # increase | neutral | decrease
    harness:
      version: harnesslab/v1
      name: candidate-001
      <full harness fields>

Do not wrap the YAML in markdown fences. Do not include explanations outside
the YAML. Every candidate must differ from the base harness and from the other
candidates. Prefer small, targeted changes; each change should map to at least
one observed failure pattern.`

	user := fmt.Sprintf(`BASE HARNESS (harnesslab/v1):
%s

FAILURE ANALYSIS (%d runs):
%s

RULE-BASED SUGGESTIONS (optional hints):
%s

Generate %d candidates now.`, string(baseYAML), a.Runs, strings.Join(patterns, "\n"),
		strings.Join(a.Candidates, "\n"), n)

	out, err := llm.Complete(ctx, system, user)
	if err != nil {
		return nil, fmt.Errorf("optimize: llm: %w", err)
	}

	var prompt generatePrompt
	if err := yaml.Unmarshal([]byte(stripFence(out)), &prompt); err != nil {
		return nil, fmt.Errorf("optimize: parse llm candidates: %w", err)
	}
	if len(prompt.Candidates) == 0 {
		return nil, errors.New("optimize: llm returned no candidates")
	}

	var candidates []*Candidate
	var skipped []string
	seen := map[string]bool{}
	for _, c := range prompt.Candidates {
		name := c.Name
		if name == "" {
			name = fmt.Sprintf("candidate-%03d", len(candidates)+1)
		}
		name = sanitizeName(name)
		if seen[name] {
			skipped = append(skipped, name+": duplicate name")
			continue
		}
		seen[name] = true
		if len(c.Harness) == 0 {
			skipped = append(skipped, name+": empty harness")
			continue
		}
		body, err := yaml.Marshal(c.Harness)
		if err != nil {
			skipped = append(skipped, name+": remarshal")
			continue
		}
		s, err := spec.Parse(body)
		if err != nil {
			skipped = append(skipped, name+": spec: "+err.Error())
			continue
		}
		if s.Name == "" {
			s.Name = name
		}
		candidates = append(candidates, &Candidate{File: "candidate-" + name + ".yaml", Metadata: c.Metadata, Spec: s})
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("optimize: no valid candidates (%d skipped: %s)", len(skipped), strings.Join(skipped, "; "))
	}
	if len(skipped) > 0 {
		return candidates, fmt.Errorf("optimize: %d candidate(s) skipped: %s", len(skipped), strings.Join(skipped, "; "))
	}
	return candidates, nil
}

// stripFence removes a single ```yaml ... ``` fence if the model wrapped its
// output despite the instructions.
func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
