package apextest

import (
	"crypto/sha256"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexcov"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// coverageSession collects raw coverage events for one run. Each test method
// and each class's @TestSetup gets its own recorder; methods run in parallel,
// so the maps are guarded.
type coverageSession struct {
	mu     sync.Mutex
	cases  map[string]*apexcov.Recorder
	setups map[string]*apexcov.Recorder
}

func newCoverageSession() *coverageSession {
	return &coverageSession{cases: map[string]*apexcov.Recorder{}, setups: map[string]*apexcov.Recorder{}}
}

// coverageCaseKey and coverageSetupKey include the test class file: two
// same-named test classes from different source units keep their own rows.
func coverageCaseKey(className, methodName, file string) string {
	return strings.ToLower(className) + "\x00" + strings.ToLower(methodName) + "\x00" + filepath.Clean(file)
}

func coverageSetupKey(className, file string) string {
	return strings.ToLower(className) + "\x00" + filepath.Clean(file)
}

// install gives machine a fresh recorder when coverage is on.
func (s *coverageSession) install(machine *vm.VM) *apexcov.Recorder {
	if s == nil {
		return nil
	}
	recorder := apexcov.NewRecorder()
	machine.SetCoverage(recorder)
	return recorder
}

func (s *coverageSession) recordCase(className, methodName, file string, recorder *apexcov.Recorder) {
	if s == nil || recorder == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := coverageCaseKey(className, methodName, file)
	if previous := s.cases[key]; previous != nil {
		previous.Merge(recorder)
		return
	}
	s.cases[key] = recorder
}

// recordSetup keeps a class's @TestSetup events. Salesforce attributes them to
// each test method's coverage row.
func (s *coverageSession) recordSetup(className, file string, recorder *apexcov.Recorder) {
	if s == nil || recorder == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := coverageSetupKey(className, file)
	if previous := s.setups[key]; previous != nil {
		previous.Merge(recorder)
		return
	}
	s.setups[key] = recorder
}

// coveragePlanCache keeps executable-line plans by file and content, so a
// daemon or watch loop parses an unchanged class once.
var coveragePlanCache = struct {
	sync.Mutex
	plans map[coveragePlanKey]*apexcov.Plan
}{plans: map[coveragePlanKey]*apexcov.Plan{}}

type coveragePlanKey struct {
	file   string
	digest [sha256.Size]byte
}

const coveragePlanCacheLimit = 8192

func cachedCoveragePlan(file, source string) *apexcov.Plan {
	key := coveragePlanKey{file: file, digest: sha256.Sum256([]byte(source))}
	coveragePlanCache.Lock()
	plan, ok := coveragePlanCache.plans[key]
	coveragePlanCache.Unlock()
	if ok {
		return plan
	}
	plan = apexcov.NewPlan(file, source)
	coveragePlanCache.Lock()
	if len(coveragePlanCache.plans) >= coveragePlanCacheLimit {
		clear(coveragePlanCache.plans)
	}
	coveragePlanCache.plans[key] = plan
	coveragePlanCache.Unlock()
	return plan
}

// coverageProject plans every project class and trigger the run measures:
// source-backed, not from a package dependency, and not a test class.
// Interfaces and enums have no executable lines and are not components.
func coverageProject(index typesys.Index, sources *sourceCache) *apexcov.Project {
	var components []*apexcov.Component
	seen := map[string]bool{}
	add := func(file, kind string) {
		if file == "" || seen[file] {
			return
		}
		seen[file] = true
		source, err := sources.read(file)
		if err != nil {
			return
		}
		plan := cachedCoveragePlan(file, source)
		if plan == nil || plan.Kind != kind {
			return
		}
		components = append(components, &apexcov.Component{Name: plan.Name, Kind: plan.Kind, File: file, Plan: plan})
	}
	for _, typ := range index.Types {
		if typ.Dependency || typ.IsTest || typ.NestingDepth > 0 || typ.Kind != apexast.DeclarationClass || !typ.HasSourceSnapshot() {
			continue
		}
		add(typ.File, apexcov.KindClass)
	}
	for _, trigger := range index.Triggers {
		if trigger.Dependency || !trigger.HasSourceSnapshot() {
			continue
		}
		add(trigger.File, apexcov.KindTrigger)
	}
	sort.SliceStable(components, func(i, j int) bool {
		if components[i].Kind != components[j].Kind {
			return components[i].Kind == apexcov.KindClass
		}
		return strings.ToLower(components[i].Name) < strings.ToLower(components[j].Name)
	})
	return apexcov.NewProject(components)
}

// attach resolves the session's events and writes per-test, per-component,
// run and org-wide coverage, and the requested deployment gate, into run.
func (s *coverageSession) attach(run *testreport.Run, project *apexcov.Project, gate string, synchronous bool) {
	if s == nil || project == nil {
		return
	}
	runCovered := map[*apexcov.Component]map[int]struct{}{}
	listed := map[*apexcov.Component]bool{}
	if synchronous {
		// A synchronous run lists every trigger, called or not; an
		// asynchronous run lists only what its tests reached (native cv2-ab
		// and cov-r2 async rows).
		for _, c := range project.Components {
			if c.Kind == apexcov.KindTrigger {
				listed[c] = true
			}
		}
	}
	// @TestSetup coverage belongs to the first test of its class that ran,
	// in method name order; later tests see only its carried declaration
	// lines (native CarrySetupTest and Setup*Test rows).
	setupOwner := map[string]*testreport.Case{}
	for si := range run.Suites {
		for ci := range run.Suites[si].Cases {
			c := &run.Suites[si].Cases[ci]
			if !coverageCaseRan(*c) {
				continue
			}
			key := coverageSetupKey(c.ClassName, c.SelectedSourceFile)
			if owner := setupOwner[key]; owner == nil || c.MethodName < owner.MethodName {
				setupOwner[key] = c
			}
		}
	}
	for si := range run.Suites {
		suite := &run.Suites[si]
		for ci := range suite.Cases {
			testCase := &suite.Cases[ci]
			if !coverageCaseRan(*testCase) {
				// A method that never ran has no coverage rows natively; it
				// also takes no part in the per-test carry.
				testCase.Coverage = nil
				testCase.CoverageNotRun = true
				continue
			}
			recorder := apexcov.NewRecorder()
			s.mu.Lock()
			if setupKey := coverageSetupKey(testCase.ClassName, testCase.SelectedSourceFile); setupOwner[setupKey] == testCase {
				recorder.Merge(s.setups[setupKey])
			}
			recorder.Merge(s.cases[coverageCaseKey(testCase.ClassName, testCase.MethodName, testCase.SelectedSourceFile)])
			s.mu.Unlock()
			resolved := project.Resolve(recorder)
			var rows []testreport.TestCoverage
			for _, c := range project.Components {
				// Natively every per-test result also lists each uncalled
				// trigger with no lines. Those rows carry nothing and would
				// grow with tests times triggers, so a test lists only what it
				// reached; the run's components still list every trigger.
				covered, touched := resolved[c]
				if !touched {
					continue
				}
				listed[c] = true
				set := runCovered[c]
				if set == nil {
					set = map[int]struct{}{}
					runCovered[c] = set
				}
				for _, line := range covered {
					set[line] = struct{}{}
				}
				rows = append(rows, testreport.TestCoverage{
					Name:              c.Name,
					Type:              c.Kind,
					File:              c.File,
					NumLinesCovered:   len(covered),
					NumLinesUncovered: len(c.Plan.Lines) - len(covered),
					CoveredLines:      nonNilLines(covered),
				})
			}
			testCase.Coverage = rows
		}
	}
	out := &testreport.Coverage{Components: []testreport.ComponentCoverage{}}
	var runParts, orgParts []apexcov.Counts
	counts := map[*apexcov.Component]apexcov.Counts{}
	for _, c := range project.Components {
		var hit []int
		for line := range runCovered[c] {
			hit = append(hit, line)
		}
		lines := apexcov.Union(c.Plan.Lines, hit)
		count := lines.Of()
		counts[c] = count
		orgParts = append(orgParts, count)
		if !listed[c] {
			continue
		}
		runParts = append(runParts, count)
		out.Components = append(out.Components, testreport.ComponentCoverage{
			Name:           c.Name,
			Type:           c.Kind,
			File:           c.File,
			TotalLines:     count.Total,
			TotalCovered:   count.Covered,
			CoveredPercent: apexcov.Percent(count.Covered, count.Total),
			CoveredLines:   nonNilLines(lines.Covered),
			UncoveredLines: nonNilLines(lines.Uncovered),
			Declaration:    append([]int(nil), c.Plan.Decl...),
		})
	}
	runTotal, orgTotal := sumCounts(runParts), sumCounts(orgParts)
	out.Summary = testreport.CoverageSummary{
		TotalLines:        runTotal.Total,
		CoveredLines:      runTotal.Covered,
		TestRunCoverage:   apexcov.RunPercent(runParts),
		OrgWideTotalLines: orgTotal.Total,
		OrgWideCovered:    orgTotal.Covered,
		OrgWideCoverage:   apexcov.OrgWidePercent(orgParts),
	}
	if gate != "" {
		out.Gate = coverageGate(gate, project, counts, orgParts)
	}
	out.Synchronous = synchronous
	run.Coverage = out
}

// coverageCaseRan reports a test whose method body ran: it passed or failed
// on its own. Compile errors, skipped and unsupported cases, and methods a
// failed @TestSetup kept from running did not.
func coverageCaseRan(c testreport.Case) bool {
	switch c.Status {
	case testreport.StatusPass, testreport.StatusFail, "":
		return !c.EntryNotRun
	}
	return false
}

func coverageGate(level string, project *apexcov.Project, counts map[*apexcov.Component]apexcov.Counts, orgParts []apexcov.Counts) *testreport.CoverageGate {
	gate := &testreport.CoverageGate{TestLevel: level, Percent: apexcov.OrgWidePercent(orgParts), Passed: true}
	fail := func(warning string) {
		gate.Passed = false
		gate.Warnings = append(gate.Warnings, warning)
	}
	switch level {
	case testreport.CoverageGateRunSpecifiedTests:
		for _, c := range project.Components {
			if c.Kind != apexcov.KindClass || counts[c].Total == 0 {
				continue
			}
			if ok, warning := apexcov.SpecifiedClassGate(counts[c]); !ok {
				fail(c.Name + ": " + warning)
			}
		}
	default:
		if _, ok, warning := apexcov.OverallGate(orgParts); !ok {
			fail(warning)
		}
		for _, c := range project.Components {
			// A trigger with no executable lines cannot be covered; no row
			// shows Salesforce gating one.
			if c.Kind != apexcov.KindTrigger || counts[c].Total == 0 {
				continue
			}
			if ok, warning := apexcov.TriggerGate(counts[c]); !ok {
				fail(c.Name + ": " + warning)
			}
		}
	}
	return gate
}

func sumCounts(parts []apexcov.Counts) apexcov.Counts {
	var out apexcov.Counts
	for _, part := range parts {
		out.Covered += part.Covered
		out.Total += part.Total
	}
	return out
}

func nonNilLines(lines []int) []int {
	if lines == nil {
		return []int{}
	}
	return lines
}
