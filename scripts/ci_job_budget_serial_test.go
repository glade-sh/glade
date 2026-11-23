package scripts

import "testing"

func TestCIJobBudgetSerialExclusion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		jobs   map[string][]string
		matrix bool
		want   bool
	}{
		{"serial", map[string][]string{"workers": nil, "union": {"workers"}}, false, true},
		{"missing dependency", map[string][]string{"workers": nil, "union": nil}, false, false},
		{"missing workers", map[string][]string{"union": {"workers"}}, false, false},
		{"missing union", map[string][]string{"workers": nil}, false, false},
		{"matrix union", map[string][]string{"workers": nil, "union": {"workers"}}, true, false},
		{"cycle", map[string][]string{"workers": {"union"}, "union": {"workers"}}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ciJobSerialAfter(tc.jobs, "union", "workers", tc.matrix); got != tc.want {
				t.Fatalf("serial = %v, want %v", got, tc.want)
			}
		})
	}
}
