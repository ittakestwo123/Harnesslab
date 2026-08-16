package spec

import (
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	s, err := Parse([]byte(DefaultTemplate))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Version != VersionV1 {
		t.Errorf("version = %q, want %q", s.Version, VersionV1)
	}
	if s.Name != "golang-coding-default" {
		t.Errorf("name = %q", s.Name)
	}
	if s.Runtime.Type != RuntimeTRPC {
		t.Errorf("runtime type = %q", s.Runtime.Type)
	}
	if len(s.Verification.Commands) != 2 {
		t.Errorf("verification commands = %v", s.Verification.Commands)
	}
	if !s.Success.RequireVerification() || s.Success.RequireChange() {
		t.Errorf("success defaults = ver:%v chg:%v, want true/false",
			s.Success.RequireVerification(), s.Success.RequireChange())
	}
	if d, err := s.Timeout(); err != nil || d <= 0 {
		t.Errorf("timeout = %v, err = %v", d, err)
	}
}

func TestSuccessSpecSemantics(t *testing.T) {
	// Defaults: verification required, change not required.
	var s SuccessSpec
	if !s.RequireVerification() {
		t.Error("default should require verification")
	}
	if s.RequireChange() {
		t.Error("default should not require workspace change")
	}
	if s.IsSet() {
		t.Error("unset success spec should report IsSet=false")
	}

	// Explicit values.
	s = SuccessSpec{RequireVerificationPass: boolPtr(false), RequireWorkspaceChange: boolPtr(true)}
	if s.RequireVerification() || !s.RequireChange() || !s.IsSet() {
		t.Errorf("explicit spec = ver:%v chg:%v set:%v", s.RequireVerification(), s.RequireChange(), s.IsSet())
	}
}

func boolPtr(b bool) *bool { return &b }

func TestValidateDefaults(t *testing.T) {
	s, err := Parse([]byte("version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Runtime.Type != RuntimeTRPC {
		t.Errorf("runtime default = %q", s.Runtime.Type)
	}
	if s.Planning.Strategy != PlanningNone {
		t.Errorf("planning default = %q", s.Planning.Strategy)
	}
	if s.Verification.Strategy != VerificationFinal {
		t.Errorf("verification default = %q", s.Verification.Strategy)
	}
	if s.Agent.Type != AgentCoding {
		t.Errorf("agent default = %q", s.Agent.Type)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"bad version", "version: v0\nname: x", "unsupported spec version"},
		{"missing name", "version: harnesslab/v1", "name is required"},
		{"bad runtime", "version: harnesslab/v1\nname: x\nruntime:\n  type: python", "unsupported runtime type"},
		{"missing model", "version: harnesslab/v1\nname: x\nmodel:\n  provider: openai", "model.model is required"},
		{"bad planning", "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\nplanning:\n  strategy: magic", "unsupported planning strategy"},
		{"bad verification", "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\nverification:\n  strategy: magic", "unsupported verification strategy"},
		{"bad timeout", "version: harnesslab/v1\nname: x\nmodel:\n  model: gpt-5\nbudget:\n  timeout: 10parsecs", "not a valid duration"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.yaml))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}
