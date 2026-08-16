package optimize

import "sort"

// Point is one harness variant's aggregated benchmark result.
type Point struct {
	Variant    string  `json:"variant"`
	Pass       float64 `json:"pass"`        // 0..1
	Tokens     int64   `json:"tokens"`      // avg per run
	DurationMS int64   `json:"duration_ms"` // avg per run
	ModelCalls int     `json:"model_calls"`
	ToolCalls  int     `json:"tool_calls"`
}

// Front returns the non-dominated points of the set. Dominance maximizes
// Pass and minimizes Tokens and DurationMS; a point on the front is not
// strictly worse than any other point in every objective.
func Front(points []Point) []Point {
	var front []Point
	for i, a := range points {
		dominated := false
		for j, b := range points {
			if i == j {
				continue
			}
			if dominates(b, a) {
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

// dominates reports whether a is at least as good as b in every objective
// and strictly better in at least one.
func dominates(a, b Point) bool {
	better := a.Pass > b.Pass || a.Tokens < b.Tokens || a.DurationMS < b.DurationMS
	noWorse := a.Pass >= b.Pass && a.Tokens <= b.Tokens && a.DurationMS <= b.DurationMS
	return better && noWorse
}
