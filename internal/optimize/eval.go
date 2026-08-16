package optimize

import "sort"

// EvalResult is the aggregated outcome of one harness on one task set
// (dev or holdout).
type EvalResult struct {
	Variant string  `json:"variant"`
	Pass    float64 `json:"pass"`   // 0..1
	Tokens  int64   `json:"tokens"` // mean per run
	Cost    float64 `json:"cost"`   // mean per run
	Runs    int     `json:"runs"`
}

// SelectCandidates returns the non-dominated subset of results, where
// dominance maximizes Pass and minimizes Tokens and Cost. The front is
// sorted by Pass desc, then Tokens asc.
func SelectCandidates(results []EvalResult) []EvalResult {
	var front []EvalResult
	for i, a := range results {
		dominated := false
		for j, b := range results {
			if i == j {
				continue
			}
			if evalDominates(b, a) {
				dominated = true
				break
			}
		}
		if !dominated {
			front = append(front, a)
		}
	}
	sort.Slice(front, func(i, j int) bool {
		if front[i].Pass != front[j].Pass {
			return front[i].Pass > front[j].Pass
		}
		return front[i].Tokens < front[j].Tokens
	})
	return front
}

func evalDominates(a, b EvalResult) bool {
	better := a.Pass > b.Pass || a.Tokens < b.Tokens || a.Cost < b.Cost
	noWorse := a.Pass >= b.Pass && a.Tokens <= b.Tokens && a.Cost <= b.Cost
	return better && noWorse
}

// RejectRule implements the roadmap's holdout gate: a candidate that
// improves (or ties) the dev success rate but regresses the holdout success
// rate below baseline must be rejected — a Dev-only win is not trustworthy.
func RejectRule(devBaseline, devCandidate, holdBaseline, holdCandidate float64) bool {
	return devCandidate >= devBaseline && holdCandidate < holdBaseline
}

// Recommendation is the final output of one optimizer loop.
type Recommendation struct {
	// Baseline is the dev/holdout success rate of the current harness.
	Baseline EvalResult `json:"baseline"`
	// Accepted candidates passed the dev improvement AND holdout gate.
	Accepted []EvalResult `json:"accepted"`
	// Rejected candidates failed the holdout gate (Dev-only wins).
	Rejected []EvalResult `json:"rejected"`
}

// Recommend applies the Pareto selection on dev results, then the holdout
// REJECT rule for every candidate that was on the dev front.
func Recommend(dev []EvalResult, holdout []EvalResult) Recommendation {
	var rec Recommendation
	for _, r := range dev {
		if r.Variant == "baseline" {
			rec.Baseline = r
		}
	}
	front := SelectCandidates(dev)
	byName := map[string]EvalResult{}
	for _, h := range holdout {
		byName[h.Variant] = h
	}
	for _, f := range front {
		if f.Variant == "baseline" {
			continue
		}
		h, ok := byName[f.Variant]
		if !ok {
			// No holdout result: cannot validate, treat as rejected.
			rec.Rejected = append(rec.Rejected, f)
			continue
		}
		if RejectRule(rec.Baseline.Pass, f.Pass, rec.Baseline.Pass, h.Pass) {
			rec.Rejected = append(rec.Rejected, f)
			continue
		}
		rec.Accepted = append(rec.Accepted, h)
	}
	return rec
}
