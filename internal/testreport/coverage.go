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
