package cost

import (
	"testing"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
)

func TestCalculate(t *testing.T) {
	calc := New(spec.PricingSpec{
		"deepseek": {
			"deepseek-chat": {InputPerMillion: 0.27, OutputPerMillion: 1.10},
			"*":             {InputPerMillion: 0.5, OutputPerMillion: 2.0},
		},
	})
	// 1,000,000 in + 1,000,000 out at exact model.
	got := calc.Calculate("deepseek", "deepseek-chat", 1_000_000, 1_000_000)
	if got < 1.36 || got > 1.38 {
		t.Fatalf("exact model cost = %v, want ~1.37", got)
	}
	// Wildcard fallback for an unknown model.
	got = calc.Calculate("deepseek", "some-other", 1_000_000, 500_000)
	if got < 1.49 || got > 1.51 {
		t.Fatalf("wildcard cost = %v, want ~1.50", got)
	}
	// Unknown provider -> 0.
	if got := calc.Calculate("openai", "gpt-5", 1_000_000, 1_000_000); got != 0 {
		t.Fatalf("unknown provider cost = %v, want 0", got)
	}
}

func TestCalculateNoPricing(t *testing.T) {
	if got := New(nil).Calculate("deepseek", "deepseek-chat", 1_000_000, 1_000_000); got != 0 {
		t.Fatalf("no pricing cost = %v, want 0", got)
	}
}

func TestModelPriceFallback(t *testing.T) {
	p := spec.PricingSpec{"openai": {"*": {InputPerMillion: 1}}}
	if _, ok := p.ModelPrice("openai", "gpt-5"); !ok {
		t.Fatal("expected wildcard fallback")
	}
	if _, ok := p.ModelPrice("anthropic", "claude"); ok {
		t.Fatal("unknown provider should not resolve")
	}
}
