package optimize

import "testing"

func TestSelectCandidatesPareto(t *testing.T) {
	res := []EvalResult{
		{Variant: "baseline", Pass: 0.6, Tokens: 100, Cost: 0.1},
		{Variant: "a", Pass: 0.8, Tokens: 120, Cost: 0.12}, // better pass, worse tokens/cost
		{Variant: "b", Pass: 0.7, Tokens: 80, Cost: 0.08},  // dominates baseline (pass up, tokens/cost down)
		{Variant: "c", Pass: 0.9, Tokens: 200, Cost: 0.2},  // best pass, worst cost
		{Variant: "d", Pass: 0.5, Tokens: 50, Cost: 0.05},  // cheapest: lowest pass, lowest tokens/cost
	}
	front := SelectCandidates(res)
	names := map[string]bool{}
	for _, f := range front {
		names[f.Variant] = true
	}
	// b dominates baseline (higher pass, lower tokens, lower cost).
	if names["baseline"] {
		t.Fatalf("dominated baseline on the front: %v", names)
	}
	// a (best pass), b (balanced), c (max pass), d (cheapest) are all
	// non-dominated along different trade-off axes.
	for _, want := range []string{"a", "b", "c", "d"} {
		if !names[want] {
			t.Fatalf("missing front member %q: %v", want, names)
		}
	}
	// Sorted by pass desc.
	if front[0].Variant != "c" || front[len(front)-1].Variant != "d" {
		t.Fatalf("front order = %v", front)
	}
}

func TestRejectRule(t *testing.T) {
	cases := []struct {
		devB, devC, holdB, holdC float64
		want                     bool
	}{
		{0.6, 0.8, 0.6, 0.7, false}, // dev up, holdout up -> accept
		{0.6, 0.8, 0.6, 0.5, true},  // dev up, holdout down -> REJECT
		{0.6, 0.6, 0.6, 0.6, false}, // ties everywhere -> accept (neutral)
		{0.6, 0.5, 0.6, 0.4, false}, // dev down -> not a dev win, no reject
	}
	for i, c := range cases {
		if got := RejectRule(c.devB, c.devC, c.holdB, c.holdC); got != c.want {
			t.Fatalf("case %d: RejectRule = %v, want %v", i, got, c.want)
		}
	}
}

func TestRecommend(t *testing.T) {
	dev := []EvalResult{
		{Variant: "baseline", Pass: 0.6, Tokens: 100},
		{Variant: "candidate-001", Pass: 0.8, Tokens: 90},  // dev win
		{Variant: "candidate-002", Pass: 0.7, Tokens: 80},  // dev win, but holdout regresses
		{Variant: "candidate-003", Pass: 0.5, Tokens: 200}, // dominated on dev -> not selected
	}
	hold := []EvalResult{
		{Variant: "baseline", Pass: 0.6, Tokens: 100},
		{Variant: "candidate-001", Pass: 0.7, Tokens: 90}, // holdout up -> accept
		{Variant: "candidate-002", Pass: 0.4, Tokens: 80}, // holdout down -> REJECT
		{Variant: "candidate-003", Pass: 0.6, Tokens: 40},
	}
	rec := Recommend(dev, hold)
	if len(rec.Accepted) != 1 || rec.Accepted[0].Variant != "candidate-001" {
		t.Fatalf("accepted = %+v, want [candidate-001]", rec.Accepted)
	}
	if len(rec.Rejected) != 1 || rec.Rejected[0].Variant != "candidate-002" {
		t.Fatalf("rejected = %+v, want [candidate-002]", rec.Rejected)
	}
	if rec.Baseline.Pass != 0.6 {
		t.Fatalf("baseline = %+v", rec.Baseline)
	}
}

func TestRecommendNoHoldoutResult(t *testing.T) {
	dev := []EvalResult{
		{Variant: "baseline", Pass: 0.6},
		{Variant: "candidate-001", Pass: 0.8}, // dev win but no holdout result
	}
	hold := []EvalResult{{Variant: "baseline", Pass: 0.6}}
	rec := Recommend(dev, hold)
	if len(rec.Accepted) != 0 || len(rec.Rejected) != 1 {
		t.Fatalf("unevaluated candidate must be rejected: %+v", rec)
	}
}
