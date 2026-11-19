package apexcov

import "sort"

// Lines is one component's native line arrays: Covered and Uncovered are
// disjoint, sorted, and together make the executable set.
type Lines struct {
	Covered   []int
	Uncovered []int
}

// Total is the executable line count.
func (l Lines) Total() int { return len(l.Covered) + len(l.Uncovered) }

// Union combines the covered lines of several runs over one executable set.
// Salesforce stores distinct lines, so overlapping runs add nothing: a line
// hit by two tests is one covered line. Covered lines outside the executable
// set are dropped; they cannot count.
func Union(executable []int, runs ...[]int) Lines {
	exec := make(map[int]struct{}, len(executable))
	for _, line := range executable {
		exec[line] = struct{}{}
	}
	hit := map[int]struct{}{}
	for _, run := range runs {
		for _, line := range run {
			if _, ok := exec[line]; ok {
				hit[line] = struct{}{}
			}
		}
	}
	var out Lines
	for _, line := range executable {
		if _, ok := hit[line]; ok {
			out.Covered = append(out.Covered, line)
		} else {
			out.Uncovered = append(out.Uncovered, line)
		}
	}
	sort.Ints(out.Covered)
	sort.Ints(out.Uncovered)
	return out
}

// Percent is the per-component figure sf apex run test prints: the covered
// share of executable lines rounded to the nearest whole percent, halves up.
// A component with no executable lines reports 0.
func Percent(covered, total int) int {
	if total <= 0 {
		return 0
	}
	return (200*covered + total) / (2 * total)
}

// Counts is a component's covered and total executable line counts.
type Counts struct {
	Covered int
	Total   int
}

// Of reduces a Lines value to counts.
func (l Lines) Of() Counts { return Counts{Covered: len(l.Covered), Total: l.Total()} }

func sum(parts []Counts) Counts {
	var out Counts
	for _, part := range parts {
		out.Covered += part.Covered
		out.Total += part.Total
	}
	return out
}

// RunPercent is the testRunCoverage figure: the line-weighted share over the
// components a run reported, rounded to the nearest percent, halves up. It is
// never the mean of component percentages.
func RunPercent(parts []Counts) int {
	total := sum(parts)
	return Percent(total.Covered, total.Total)
}

// OrgWidePercent is ApexOrgWideCoverage.PercentCovered: the line-weighted
// share over every stored class and trigger, floored. 74.9 reports 74. An org
// with no executable lines reports 0.
func OrgWidePercent(parts []Counts) int {
	total := sum(parts)
	if total.Total <= 0 {
		return 0
	}
	return 100 * total.Covered / total.Total
}
