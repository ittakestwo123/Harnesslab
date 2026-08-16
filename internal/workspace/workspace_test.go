package workspace

import "testing"

func TestDiffChanged(t *testing.T) {
	cases := []struct {
		name string
		d    *Diff
		want bool
	}{
		{"nil", nil, false},
		{"empty", &Diff{}, false},
		{"patch", &Diff{Patch: "diff --git"}, true},
		{"stat", &Diff{Stat: "1 file changed"}, true},
		{"untracked", &Diff{Untracked: []string{"new.txt"}}, true},
		{"whitespace patch", &Diff{Patch: "  \n\t"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.d.Changed(); got != tc.want {
				t.Fatalf("Changed() = %v, want %v", got, tc.want)
			}
		})
	}
}
