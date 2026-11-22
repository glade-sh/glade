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

// ciConcurrentPeak computes the largest set of simultaneously runnable job
// copies. Matrix copies of one job are independent; dependency edges apply to
// every copy. The maximum antichain follows from the DAG's minimum path cover.
func ciConcurrentPeak(widths map[string]int, needs map[string][]string) int {
	ancestors := make(map[string]map[string]bool)
	for name := range widths {
		seen := map[string]bool{}
		var visit func(string)
		visit = func(job string) {
			for _, dependency := range needs[job] {
				if !seen[dependency] {
					seen[dependency] = true
					visit(dependency)
				}
			}
		}
		visit(name)
		ancestors[name] = seen
	}
	var copies []string
	for name, width := range widths {
		for i := 0; i < width; i++ {
			copies = append(copies, name)
		}
	}
	match := make([]int, len(copies))
	for i := range match {
		match[i] = -1
	}
	var augment func(int, []bool) bool
	augment = func(from int, seen []bool) bool {
		for to := range copies {
			if seen[to] || !ancestors[copies[to]][copies[from]] {
				continue
			}
			seen[to] = true
			if match[to] < 0 || augment(match[to], seen) {
				match[to] = from
				return true
			}
		}
		return false
	}
	matched := 0
	for i := range copies {
		if augment(i, make([]bool, len(copies))) {
			matched++
		}
	}
	return len(copies) - matched
}
