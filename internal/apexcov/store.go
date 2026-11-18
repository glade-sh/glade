package apexcov

import "strings"

// Store models the coverage an org keeps between test runs: ApexCodeCoverage
// per-test rows and ApexCodeCoverageAggregate rows. Native rows show that a
// run of a test class replaces every per-test row of that class, so an earlier
// method's row can vanish, while aggregates only grow: a later run never
// uncovers a line. glade test reports one run against a fresh org; Store
// replays a sequence of runs the way an org accumulates them.
type Store struct {
	executable map[string][]int
	aggregate  map[string]map[int]struct{}
	// perTest maps a lower-case test class to its method rows.
	perTest map[string]map[string]map[string][]int
}

// NewStore returns an empty store over the components' executable lines,
// keyed by component name.
func NewStore(executable map[string][]int) *Store {
	s := &Store{executable: map[string][]int{}, aggregate: map[string]map[int]struct{}{}, perTest: map[string]map[string]map[string][]int{}}
	for name, lines := range executable {
		s.executable[name] = append([]int(nil), lines...)
		s.aggregate[name] = map[int]struct{}{}
	}
	return s
}

// TestRun is one test method's covered lines per component.
type TestRun struct {
	TestClass string
	Method    string
	Covered   map[string][]int
}

// Record stores one run's per-test rows. Every earlier row of a test class in
// the run is dropped first; aggregates take the union.
func (s *Store) Record(runs []TestRun) {
	for _, run := range runs {
		delete(s.perTest, strings.ToLower(run.TestClass))
	}
	for _, run := range runs {
		class := strings.ToLower(run.TestClass)
		if s.perTest[class] == nil {
			s.perTest[class] = map[string]map[string][]int{}
		}
		rows := map[string][]int{}
		for name, lines := range run.Covered {
			rows[name] = append([]int(nil), lines...)
			agg := s.aggregate[name]
			if agg == nil {
				continue
			}
			exec := map[int]struct{}{}
			for _, line := range s.executable[name] {
				exec[line] = struct{}{}
			}
			for _, line := range lines {
				if _, ok := exec[line]; ok {
					agg[line] = struct{}{}
				}
			}
		}
		s.perTest[class][strings.ToLower(run.Method)] = rows
	}
}

// Aggregate is a component's stored line arrays.
func (s *Store) Aggregate(name string) Lines {
	var covered []int
	for line := range s.aggregate[name] {
		covered = append(covered, line)
	}
	return Union(s.executable[name], covered)
}

// PerTest returns a stored per-test row and whether it exists.
func (s *Store) PerTest(testClass, method, component string) ([]int, bool) {
	rows, ok := s.perTest[strings.ToLower(testClass)][strings.ToLower(method)]
	if !ok {
		return nil, false
	}
	lines, ok := rows[component]
	return lines, ok
}

// OrgWide is ApexOrgWideCoverage over every stored component.
func (s *Store) OrgWide() int {
	parts := make([]Counts, 0, len(s.executable))
	for name := range s.executable {
		parts = append(parts, s.Aggregate(name).Of())
	}
	return OrgWidePercent(parts)
}
