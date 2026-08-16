package optimize

import (
	"context"
	"strings"
	"testing"
)

type stubLLM struct {
	out string
	err error
}

func (s *stubLLM) Complete(_ context.Context, system, user string) (string, error) {
	return s.out, s.err
}

const validCandidatesYAML = `candidates:
  - name: candidate-001
    metadata:
      parent: baseline
      reason:
        - no_tools
      expected_effect:
        success: increase
        tokens: neutral
    harness:
      version: harnesslab/v1
      name: candidate-001
      model:
        model: gpt-5
      planning:
        strategy: todo
  - name: candidate-002
    metadata:
      parent: baseline
      reason:
        - high_tokens
      expected_effect:
        success: neutral
        tokens: decrease
    harness:
      version: harnesslab/v1
      name: candidate-002
      model:
        model: gpt-5
      context:
        strategy: repo-map
`

func TestGenerateCandidatesValid(t *testing.T) {
	base := testSpec(t, "base")
	a := &Analysis{Runs: 10, Patterns: []Pattern{{ID: "no_tools", Count: 6}}, Candidates: []string{"add tools"}}
	gens, err := GenerateCandidates(context.Background(), &stubLLM{out: validCandidatesYAML}, base, a, 2)
	if err != nil {
		t.Fatalf("GenerateCandidates: %v", err)
	}
	if len(gens) != 2 {
		t.Fatalf("candidates = %d, want 2", len(gens))
	}
	if gens[0].File != "candidate-candidate-001.yaml" {
		t.Fatalf("file = %q", gens[0].File)
	}
	if gens[0].Spec.Planning.Strategy != "todo" {
		t.Fatalf("candidate-001 planning = %+v", gens[0].Spec.Planning)
	}
	if gens[1].Spec.Context.Strategy != "repo-map" {
		t.Fatalf("candidate-002 context = %+v", gens[1].Spec.Context)
	}
	if gens[0].Metadata.ExpectedEffect.Success != "increase" {
		t.Fatalf("metadata = %+v", gens[0].Metadata)
	}
}

func TestGenerateCandidatesStripsFence(t *testing.T) {
	base := testSpec(t, "base")
	fenced := "```yaml\n" + validCandidatesYAML + "```\n"
	gens, err := GenerateCandidates(context.Background(), &stubLLM{out: fenced}, base, &Analysis{}, 2)
	if err != nil {
		t.Fatalf("fenced output rejected: %v", err)
	}
	if len(gens) != 2 {
		t.Fatalf("candidates = %d", len(gens))
	}
}

func TestGenerateCandidatesSkipsInvalid(t *testing.T) {
	base := testSpec(t, "base")
	yaml := `candidates:
  - name: bad
    metadata: {parent: baseline}
    harness:
      version: harnesslab/v1
      name: bad
      model:
        model: gpt-5
      sandbox:
        type: nope
  - name: good
    metadata: {parent: baseline}
    harness:
      version: harnesslab/v1
      name: good
      model:
        model: gpt-5
`
	gens, err := GenerateCandidates(context.Background(), &stubLLM{out: yaml}, base, &Analysis{}, 2)
	if err == nil {
		t.Fatal("expected a skip warning")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Fatalf("error should mention the invalid candidate: %v", err)
	}
	if len(gens) != 1 || gens[0].Spec.Name != "good" {
		t.Fatalf("valid candidate lost: %+v", gens)
	}
}

func TestGenerateCandidatesErrors(t *testing.T) {
	base := testSpec(t, "base")
	if _, err := GenerateCandidates(context.Background(), &stubLLM{err: context.DeadlineExceeded}, base, &Analysis{}, 2); err == nil {
		t.Fatal("llm error not propagated")
	}
	if _, err := GenerateCandidates(context.Background(), &stubLLM{out: "not yaml at all"}, base, &Analysis{}, 2); err == nil {
		t.Fatal("unparseable output accepted")
	}
	if _, err := GenerateCandidates(context.Background(), &stubLLM{out: "candidates: []"}, base, &Analysis{}, 2); err == nil {
		t.Fatal("empty candidate list accepted")
	}
}

func TestSanitizeName(t *testing.T) {
	if got := sanitizeName("candidate 001!"); got != "candidate-001" {
		t.Fatalf("sanitize = %q, want candidate-001", got)
	}
	if got := sanitizeName("candidate-001"); got != "candidate-001" {
		t.Fatalf("sanitize = %q, want candidate-001", got)
	}
	if got := sanitizeName(""); got != "" {
		t.Fatalf("sanitize empty = %q", got)
	}
}
