package benchmark

import (
	"math"
	"sort"
)

// Stat summarizes one metric over a set of runs.
type Stat struct {
	Count  int     `json:"count"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	StdDev float64 `json:"stddev"`
	P50    float64 `json:"p50"`
	P90    float64 `json:"p90"`
	// CI95Lo/CI95Hi bound the 95% confidence interval of the mean using the
	// normal approximation mean +/- 1.96 * stddev / sqrt(n). For n < 2 the
	// interval collapses to the mean.
	CI95Lo float64 `json:"ci95_lo"`
	CI95Hi float64 `json:"ci95_hi"`
}

// Stats groups the statistical summaries the benchmark reports per variant
// and per task x variant.
type Stats struct {
	InputTokens  Stat `json:"input_tokens"`
	OutputTokens Stat `json:"output_tokens"`
	CostUSD      Stat `json:"cost_usd"`
	LatencyMS    Stat `json:"latency_ms"`
}

// computeStat summarizes values. Empty input yields a zero Stat.
func computeStat(values []float64) Stat {
	if len(values) == 0 {
		return Stat{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	n := len(sorted)
	var sum float64
	for _, v := range sorted {
		sum += v
	}
	mean := sum / float64(n)
	var sq float64
	for _, v := range sorted {
		d := v - mean
		sq += d * d
	}
	stddev := 0.0
	if n > 1 {
		stddev = math.Sqrt(sq / float64(n-1)) // sample standard deviation
	}
	ci := 0.0
	if n > 1 {
		ci = 1.96 * stddev / math.Sqrt(float64(n))
	}
	return Stat{
		Count:  n,
		Mean:   round(mean, 3),
		Median: round(percentile(sorted, 50), 3),
		StdDev: round(stddev, 3),
		P50:    round(percentile(sorted, 50), 3),
		P90:    round(percentile(sorted, 90), 3),
		CI95Lo: round(mean-ci, 3),
		CI95Hi: round(mean+ci, 3),
	}
}

// percentile returns the nearest-rank percentile of a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func round(v float64, places int) float64 {
	pow := math.Pow(10, float64(places))
	return math.Round(v*pow) / pow
}
