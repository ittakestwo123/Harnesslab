package optimize

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
)

// Candidate is one LLM-generated (or hand-written) harness variant with the
// metadata explaining why it was proposed. Candidates are full harness
// specs stored under .harness/candidates/candidate-XXX.yaml and are NEVER
// written over the user's harness.yaml.
type Candidate struct {
	// File is the candidate file name (candidate-001.yaml).
	File string `json:"file"`
	// Metadata explains the candidate's parent, reasons and expected effect.
	Metadata Metadata `json:"metadata"`
	// Spec is the full validated harness spec of the candidate.
	Spec *spec.HarnessSpec `json:"-"`
}

// Metadata records why a candidate was proposed and what it expects to move.
type Metadata struct {
	// Parent is the harness this candidate derives from (e.g. "baseline").
	Parent string `json:"parent" yaml:"parent"`
	// Reason lists the failure-pattern ids or observations behind the change.
	Reason []string `json:"reason" yaml:"reason"`
	// ExpectedEffect is the intended direction of change.
	ExpectedEffect Effect `json:"expected_effect" yaml:"expected_effect"`
}

// Effect describes the expected direction of a metric.
type Effect struct {
	// Success: increase | neutral | decrease.
	Success string `json:"success" yaml:"success"`
	// Tokens: increase | neutral | decrease.
	Tokens string `json:"tokens" yaml:"tokens"`
}

// candidateFile is the on-disk layout: a metadata block followed by an
// inline HarnessSpec. spec.Parse ignores the metadata block; candidate
// readers extract it separately.
type candidateFile struct {
	Metadata         Metadata `yaml:"metadata"`
	spec.HarnessSpec `yaml:",inline"`
}

// WriteCandidate persists a candidate as candidate-<name>.yaml in dir,
// returning the file name. The harness spec is emitted inline after the
// metadata block so the file is simultaneously a valid harness.yaml (spec
// parsing ignores metadata) and self-describing.
func WriteCandidate(dir, name string, s *spec.HarnessSpec, md Metadata) (string, error) {
	cf := candidateFile{Metadata: md, HarnessSpec: *s}
	data, err := yaml.Marshal(&cf)
	if err != nil {
		return "", fmt.Errorf("optimize: marshal candidate: %w", err)
	}
	file := filepath.Join(dir, "candidate-"+name+".yaml")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("optimize: mkdir candidates: %w", err)
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		return "", fmt.Errorf("optimize: write candidate %s: %w", file, err)
	}
	return filepath.Base(file), nil
}

// LoadCandidates reads every candidate-*.yaml in dir, validating each spec
// and skipping unparseable files (returning them as errors so the caller can
// warn without aborting).
func LoadCandidates(dir string) ([]*Candidate, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("optimize: read candidates dir: %w", err)
	}
	var out []*Candidate
	var errs []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "candidate-") || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		var cf candidateFile
		if err := yaml.Unmarshal(data, &cf); err != nil {
			errs = append(errs, fmt.Sprintf("%s: parse: %v", e.Name(), err))
			continue
		}
		// Re-parse the spec portion for full validation (defaults filled).
		body, err := yaml.Marshal(cf.HarnessSpec)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: remarshal: %v", e.Name(), err))
			continue
		}
		s, err := spec.Parse(body)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: spec: %v", e.Name(), err))
			continue
		}
		out = append(out, &Candidate{File: e.Name(), Metadata: cf.Metadata, Spec: s})
	}
	if len(errs) > 0 {
		return out, fmt.Errorf("optimize: %d candidate(s) skipped: %s", len(errs), strings.Join(errs, "; "))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out, nil
}
