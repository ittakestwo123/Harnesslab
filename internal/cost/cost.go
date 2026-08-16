// Package cost computes LLM call costs from token usage and configured
// pricing (USD per million tokens).
package cost

import "github.com/ittakestwo123/Harnesslab/internal/harness/spec"

// Calculator computes costs from a pricing table.
type Calculator struct {
	pricing spec.PricingSpec
}

// New creates a calculator. A nil pricing table yields zero costs.
func New(pricing spec.PricingSpec) *Calculator {
	return &Calculator{pricing: pricing}
}

// Calculate returns the USD cost of a call with the given token counts.
func (c *Calculator) Calculate(provider, model string, inputTokens, outputTokens int64) float64 {
	if c == nil || len(c.pricing) == 0 {
		return 0
	}
	mp, ok := c.pricing.ModelPrice(provider, model)
	if !ok {
		return 0
	}
	return float64(inputTokens)/1e6*mp.InputPerMillion +
		float64(outputTokens)/1e6*mp.OutputPerMillion
}
