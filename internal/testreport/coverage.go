package testreport

import (
	"fmt"
	"sort"
	"strings"
)

// Coverage is the Apex code coverage of one run, recorded by
// glade test --code-coverage. It follows sf apex run test --code-coverage:
// Components lists each class and trigger the run's tests reached, and every
// trigger; Summary weights by lines. See docs/CODE_COVERAGE.md.
type Coverage struct {
	Summary    CoverageSummary     `json:"summary"`
	Components []ComponentCoverage `json:"components"`
	Gate       *CoverageGate       `json:"gate,omitempty"`
}

// CoverageSummary holds the run totals. TestRunCoverage is the covered share
// of the lines of Components, rounded half up. OrgWideCoverage is the
// covered share over every class and trigger in the project, floored, as
// ApexOrgWideCoverage reports it.
type CoverageSummary struct {
	TotalLines        int `json:"totalLines"`
	CoveredLines      int `json:"coveredLines"`
	TestRunCoverage   int `json:"testRunCoverage"`
	OrgWideTotalLines int `json:"orgWideTotalLines"`
	OrgWideCovered    int `json:"orgWideCoveredLines"`
	OrgWideCoverage   int `json:"orgWideCoverage"`
}

// ComponentCoverage is one class or trigger: its executable lines, split into
// covered and uncovered. CoveredPercent rounds half up.
type ComponentCoverage struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	File           string `json:"file,omitempty"`
	TotalLines     int    `json:"totalLines"`
	TotalCovered   int    `json:"totalCovered"`
	CoveredPercent int    `json:"coveredPercent"`
	CoveredLines   []int  `json:"coveredLines"`
	UncoveredLines []int  `json:"uncoveredLines"`
	// Declaration lists the executable lines a method, constructor or
	// property declaration covers. NativePerTestRows carries the covered ones
	// from earlier tests into later per-test rows, as Salesforce does.
	Declaration []int `json:"declarationLines,omitempty"`
}

// TestCoverage is one test method's coverage of one class or trigger: the
// lines that test covered, including its @TestSetup method's. The
// component's uncovered lines for this test are its executable lines in
// Run.Coverage.Components less CoveredLines.
type TestCoverage struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	NumLinesCovered   int    `json:"numLinesCovered"`
	NumLinesUncovered int    `json:"numLinesUncovered"`
	CoveredLines      []int  `json:"coveredLines"`
}

// CoverageGate is the deployment coverage rule of one test level applied to
// the run. Percent is the floored org-wide figure the RunLocalTests rule
// compares with 75. Warnings carry Salesforce's deployment messages.
type CoverageGate struct {
	TestLevel string   `json:"testLevel"`
	Percent   int      `json:"percent"`
	Passed    bool     `json:"passed"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Coverage gate test levels.
const (
	CoverageGateRunLocalTests     = "RunLocalTests"
	CoverageGateRunSpecifiedTests = "RunSpecifiedTests"
)

// ParseCoverageGate returns the canonical test level for a --coverage-gate
// value, case-insensitively.
func ParseCoverageGate(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case strings.ToLower(CoverageGateRunLocalTests):
		return CoverageGateRunLocalTests, nil
	case strings.ToLower(CoverageGateRunSpecifiedTests):
		return CoverageGateRunSpecifiedTests, nil
	}
	return "", fmt.Errorf("--coverage-gate must be %s or %s", CoverageGateRunLocalTests, CoverageGateRunSpecifiedTests)
}

// coverageConsoleLimit caps the component table in console output.
const coverageConsoleLimit = 40

func writeConsoleCoverage(out *strings.Builder, coverage *Coverage) {
	if coverage == nil {
		return
	}
	s := coverage.Summary
	fmt.Fprintf(out, "\nCode coverage: %d%% of %d lines in this run (%d covered), %d%% org-wide\n",
		s.TestRunCoverage, s.TotalLines, s.CoveredLines, s.OrgWideCoverage)
	components := append([]ComponentCoverage(nil), coverage.Components...)
	sort.SliceStable(components, func(i, j int) bool {
		return strings.ToLower(components[i].Name) < strings.ToLower(components[j].Name)
	})
	width := 0
	for i, c := range components {
		if i >= coverageConsoleLimit {
			break
		}
		width = max(width, len(c.Name))
	}
	for i, c := range components {
		if i >= coverageConsoleLimit {
			fmt.Fprintf(out, "  ... %d more\n", len(components)-coverageConsoleLimit)
			break
		}
		fmt.Fprintf(out, "  %-*s  %3d%%  %d/%d", width, c.Name, c.CoveredPercent, c.TotalCovered, c.TotalLines)
		if len(c.UncoveredLines) > 0 {
			fmt.Fprintf(out, "  uncovered %s", formatLineList(c.UncoveredLines, 12))
		}
		out.WriteString("\n")
	}
	if gate := coverage.Gate; gate != nil {
		status := "passed"
		if !gate.Passed {
			status = "failed"
		}
		fmt.Fprintf(out, "Coverage gate %s: %s (%d%%)\n", gate.TestLevel, status, gate.Percent)
		for _, warning := range gate.Warnings {
			fmt.Fprintf(out, "  %s\n", warning)
		}
	}
}

func formatLineList(lines []int, limit int) string {
	parts := make([]string, 0, min(len(lines), limit)+1)
	for i, line := range lines {
		if i >= limit {
			parts = append(parts, fmt.Sprintf("+%d", len(lines)-limit))
			break
		}
		parts = append(parts, fmt.Sprint(line))
	}
	return strings.Join(parts, ",")
}

// NativePerTestRows returns run with per-test coverage rows shaped as
// Salesforce stores them for one run (ApexCodeCoverage, sf --code-coverage
// records). The run records compact rows, each test's own lines for the
// components it reached; the report writers expand them here, after the test
// daemon wire, so the wire stays small. Within each test class, in method
// name order:
//   - a test has a row for every component it or an earlier test reached, and
//     for every trigger, empty when no test fired it;
//   - a row holds the test's own lines plus the declaration lines (method,
//     constructor and property) that earlier tests covered.
//
// The rule reproduces the 115 per-test rows of a 14-method native run at API
// 62.0 and 67.0 (advisor cov-holdout, mirrored in
// internal/apextest/testdata/native_conformance/test_runner_coverage).
// A run without coverage is returned unchanged.
func NativePerTestRows(run Run) Run {
	if run.Coverage == nil {
		return run
	}
	components := run.Coverage.Components
	decl := make([]map[int]bool, len(components))
	for i, c := range components {
		decl[i] = map[int]bool{}
		for _, line := range c.Declaration {
			decl[i][line] = true
		}
	}
	byName := map[string]int{}
	for i, c := range components {
		byName[c.Type+"\x00"+c.Name] = i
	}
	suites := make([]Suite, len(run.Suites))
	for si, suite := range run.Suites {
		cases := append([]Case(nil), suite.Cases...)
		order := make([]int, len(cases))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(a, b int) bool {
			return lessTestMethodName(cases[order[a]].MethodName, cases[order[b]].MethodName)
		})
		reached := make([]bool, len(components))
		carried := make([]map[int]bool, len(components))
		for i := range carried {
			carried[i] = map[int]bool{}
		}
		for _, ci := range order {
			own := map[int][]int{}
			for _, row := range cases[ci].Coverage {
				if i, ok := byName[row.Type+"\x00"+row.Name]; ok {
					own[i] = row.CoveredLines
				}
			}
			var rows []TestCoverage
			for i, c := range components {
				lines, ownRow := own[i]
				if !ownRow && !reached[i] && c.Type != "trigger" {
					continue
				}
				set := map[int]bool{}
				for _, line := range lines {
					set[line] = true
				}
				for line := range carried[i] {
					set[line] = true
				}
				merged := make([]int, 0, len(set))
				for line := range set {
					merged = append(merged, line)
				}
				sort.Ints(merged)
				rows = append(rows, TestCoverage{
					Name:              c.Name,
					Type:              c.Type,
					NumLinesCovered:   len(merged),
					NumLinesUncovered: c.TotalLines - len(merged),
					CoveredLines:      merged,
				})
			}
			for i := range components {
				if lines, ok := own[i]; ok {
					reached[i] = true
					for _, line := range lines {
						if decl[i][line] {
							carried[i][line] = true
						}
					}
				}
			}
			cases[ci].Coverage = rows
		}
		suites[si] = suite
		suites[si].Cases = cases
	}
	run.Suites = suites
	return run
}

// lessTestMethodName orders test methods the way the native carry follows
// them: by name, ignoring case, then by the exact name.
func lessTestMethodName(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if la != lb {
		return la < lb
	}
	return a < b
}
