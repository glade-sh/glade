package scripts

import "testing"

func TestCIConcurrentPeakFollowsDependencies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		widths map[string]int
		needs  map[string][]string
		want   int
	}{
		{"parallel", map[string]int{"a": 2, "b": 3}, nil, 5},
		{"serial matrices", map[string]int{"a": 2, "b": 3}, map[string][]string{"b": {"a"}}, 3},
		{"join", map[string]int{"a": 2, "b": 3, "end": 1}, map[string][]string{"end": {"a", "b"}}, 5},
		{"transitive", map[string]int{"a": 4, "middle": 1, "b": 4}, map[string][]string{"middle": {"a"}, "b": {"middle"}}, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ciConcurrentPeak(tc.widths, tc.needs); got != tc.want {
				t.Fatalf("peak=%d, want %d", got, tc.want)
			}
		})
	}
}
