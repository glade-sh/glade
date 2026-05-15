package compat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/open-aer/oaer/internal/apextest"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sema"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/watch"
)

type LocalTestOptions struct {
	Project             string
	Class               string
	Method              string
	BlockersOnly        bool
	TraceBlocked        bool
	SlowTestThresholdMS int64
	TimeoutMS           int64
	TopFailures         int
	ProfileOnTimeout    bool
	Parallelism         int
	ProgressWriter      io.Writer
	ForceAnalysis       bool
	MaxFailureGroups    int
	ChangedSince        string
	ParallelMethods     bool
}

type LocalTestPhaseTiming struct {
	Name       string `json:"name"`
	DurationMS int64  `json:"durationMs"`
}

type LocalTestReport struct {
	Target          string                   `json:"target"`
	Ready           bool                     `json:"ready"`
	Project         string                   `json:"project"`
	DurationMS      int64                    `json:"durationMs,omitempty"`
	CasesDiscovered int                      `json:"casesDiscovered,omitempty"`
	CasesRun        int                      `json:"casesRun,omitempty"`
	TriageStopped   bool                     `json:"triageStopped,omitempty"`
	Dependencies    []typesys.DependencyInfo `json:"dependencies,omitempty"`
	Selection       *watch.TestSelection     `json:"selection,omitempty"`
	Phases          []LocalTestPhaseTiming   `json:"phases,omitempty"`
	Summary         LocalTestSummary         `json:"summary"`
	Outcomes        []LocalTestOutcome       `json:"outcomes"`
	TopFailures     []LocalTestFailureGroup  `json:"topFailures,omitempty"`
	Diagnostics     []diagnostic.Diagnostic  `json:"diagnostics,omitempty"`
}

type LocalTestSummary struct {
	Total          int `json:"total"`
	Pass           int `json:"pass"`
	Fail           int `json:"fail"`
	Unsupported    int `json:"unsupported"`
	LoadErrors     int `json:"loadError"`
	CompileErrors  int `json:"compileError"`
	InternalErrors int `json:"internalError"`
	AssertFailures int `json:"assertFail,omitempty"`
	RuntimeGaps    int `json:"runtimeGap,omitempty"`
	CompileGaps    int `json:"compileGap,omitempty"`
	Timeouts       int `json:"timeout,omitempty"`
}

type LocalTestOutcome struct {
	ProjectLabel        string                 `json:"projectLabel"`
	Class               string                 `json:"class"`
	Method              string                 `json:"method"`
	Outcome             string                 `json:"outcome"`
	Phase               string                 `json:"phase,omitempty"`
	CapabilityID        string                 `json:"capabilityId,omitempty"`
	File                string                 `json:"file,omitempty"`
	Line                int                    `json:"line,omitempty"`
	TopFrame            *testreport.StackFrame `json:"topFrame,omitempty"`
	Error               string                 `json:"error,omitempty"`
	RelatedMetadataFile string                 `json:"relatedMetadataFile,omitempty"`
	DurationMS          int64                  `json:"durationMs,omitempty"`
	TraceEvents         int                    `json:"traceEvents,omitempty"`
	ProfileEvents       int                    `json:"profileEvents,omitempty"`
	ProfileCategories   map[string]int         `json:"profileCategories,omitempty"`
}

type LocalTestFailureGroup struct {
	Outcome      string `json:"outcome"`
	Phase        string `json:"phase,omitempty"`
	CapabilityID string `json:"capabilityId,omitempty"`
	Error        string `json:"error,omitempty"`
	Count        int    `json:"count"`
}

type LocalTestCorpusBaseline struct {
	Target   string                   `json:"target"`
	Projects []LocalTestCorpusProject `json:"projects"`
}

type LocalTestCorpusProject struct {
	Project  string                     `json:"project"`
	Ready    bool                       `json:"ready"`
	Summary  LocalTestSummary           `json:"summary"`
	Outcomes []LocalTestExpectedOutcome `json:"outcomes"`
}

type LocalTestExpectedOutcome struct {
	Class        string `json:"class"`
	Method       string `json:"method"`
	Outcome      string `json:"outcome"`
	CapabilityID string `json:"capabilityId,omitempty"`
}

type LocalTestCorpusReport struct {
	Target   string                         `json:"target"`
	Ready    bool                           `json:"ready"`
	Baseline string                         `json:"baseline"`
	Projects []LocalTestCorpusProjectResult `json:"projects"`
	Failures []string                       `json:"failures,omitempty"`
}

type LocalTestCorpusProjectResult struct {
	Project  string                     `json:"project"`
	Ready    bool                       `json:"ready"`
	Summary  LocalTestSummary           `json:"summary"`
	Outcomes []LocalTestExpectedOutcome `json:"outcomes"`
}

func RunLocalTests(options LocalTestOptions) (LocalTestReport, error) {
	started := time.Now()
	root := options.Project
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	absRoot, _ := filepath.Abs(root)
	report := LocalTestReport{
		Target:  "local Apex test execution readiness",
		Project: absRoot,
	}
	projectLabel := filepath.Base(absRoot)
	if projectLabel == "." || projectLabel == string(filepath.Separator) {
		projectLabel = absRoot
	}

	recordLocalTestPhase(&report, options, "load_start", started)
	index, loadDiagnostics, err := loadLocalTestIndex(root)
	recordLocalTestPhase(&report, options, "load_done", started)
	if err != nil {
		outcome := LocalTestOutcome{
			ProjectLabel: projectLabel,
			Outcome:      "load_error",
			Phase:        "load",
			CapabilityID: localTestCapabilityID("load", "", err.Error()),
			Error:        err.Error(),
		}
		report.Diagnostics = loadDiagnostics
		report.Outcomes = append(report.Outcomes, outcome)
		finalizeLocalTestReport(&report, options, started)
		return report, nil
	}
	report.Dependencies = append(report.Dependencies, index.Dependencies...)
	report.Diagnostics = append(report.Diagnostics, loadDiagnostics...)

	testOpts := apextest.Options{
		Filter:              localTestFilter(options),
		TraceBlocked:        shouldTraceFocusedLocalTests(options),
		SlowTestThresholdMS: options.SlowTestThresholdMS,
		TimeoutMS:           options.TimeoutMS,
		Parallelism:         localTestParallelism(options),
		ParallelMethods:     options.ParallelMethods || shouldParallelizeFocusedMethods(options),
		Progress:            localTestProgressReporter(options.ProgressWriter),
	}
	recordLocalTestPhase(&report, options, "discover_start", started)
	cases := apextest.Discover(index, testOpts)
	cases = filterLocalTestCases(cases, options)
	cases = selectChangedLocalTestCases(&report, index, cases, root, options)
	sort.SliceStable(cases, func(i, j int) bool {
		if cases[i].ClassName == cases[j].ClassName {
			return cases[i].MethodName < cases[j].MethodName
		}
		return cases[i].ClassName < cases[j].ClassName
	})
	report.CasesDiscovered = len(cases)
	recordLocalTestPhase(&report, options, fmt.Sprintf("discover_done total=%d", len(cases)), started)

	if shouldAnalyzeLocalTests(options, len(cases)) {
		recordLocalTestPhase(&report, options, "analyze_start", started)
		semaResult := sema.Analyze(index)
		recordLocalTestPhase(&report, options, fmt.Sprintf("analyze_done diagnostics=%d", len(semaResult.Diagnostics)), started)
		report.Diagnostics = append(report.Diagnostics, semaResult.Diagnostics...)
		if firstError, ok := firstLocalTestError(semaResult.Diagnostics); ok {
			for _, testCase := range cases {
				report.Outcomes = append(report.Outcomes, localTestDiagnosticOutcome(projectLabel, testCase, "compile_gap", "compile", firstError))
			}
			finalizeLocalTestReport(&report, options, started)
			return report, nil
		}
	} else if strings.TrimSpace(options.Class) == "" && strings.TrimSpace(options.Method) == "" {
		recordLocalTestPhase(&report, options, fmt.Sprintf("analyze_skip total=%d", len(cases)), started)
	}

	if options.ProfileOnTimeout {
		testOpts.TraceBlocked = true
	}
	recordLocalTestPhase(&report, options, "run_start", started)
	runOutcomes, casesRun, triageStopped := runLocalTestCases(index, testOpts, cases, projectLabel, options, started)
	report.Outcomes = append(report.Outcomes, runOutcomes...)
	report.CasesRun = casesRun
	report.TriageStopped = triageStopped
	recordLocalTestPhase(&report, options, "run_done", started)
	finalizeLocalTestReport(&report, options, started)
	return report, nil
}

func reportLocalTestPhase(options LocalTestOptions, event string, started time.Time) {
	if options.ProgressWriter != nil {
		fmt.Fprintf(options.ProgressWriter, "local-tests phase %s durationMs=%d\n", event, time.Since(started).Milliseconds())
	}
}

func recordLocalTestPhase(report *LocalTestReport, options LocalTestOptions, event string, started time.Time) {
	duration := time.Since(started).Milliseconds()
	report.Phases = append(report.Phases, LocalTestPhaseTiming{Name: event, DurationMS: duration})
	if options.ProgressWriter != nil {
		fmt.Fprintf(options.ProgressWriter, "local-tests phase %s durationMs=%d\n", event, duration)
	}
}

const largeLocalTestAnalysisThreshold = 5000
const localTestTriageClassBatchSize = 8

func runLocalTestCases(index typesys.Index, testOpts apextest.Options, cases []apextest.TestCase, projectLabel string, options LocalTestOptions, started time.Time) ([]LocalTestOutcome, int, bool) {
	if options.MaxFailureGroups <= 0 || options.Parallelism > 0 {
		run := apextest.RunCasesContext(context.Background(), index, testOpts, cases)
		return localTestOutcomesFromRun(projectLabel, run, options), len(cases), false
	}
	outcomes := make([]LocalTestOutcome, 0)
	casesRun := 0
	for _, batch := range localTestCaseBatches(cases, localTestTriageClassBatchSize) {
		reportLocalTestPhase(options, fmt.Sprintf("triage_batch_start cases=%d", len(batch)), started)
		run := apextest.RunCasesContext(context.Background(), index, testOpts, batch)
		outcomes = append(outcomes, localTestOutcomesFromRun(projectLabel, run, options)...)
		casesRun += len(batch)
		groups := localTestFailureGroupCount(outcomes)
		reportLocalTestPhase(options, fmt.Sprintf("triage_batch_done casesRun=%d failureGroups=%d", casesRun, groups), started)
		if groups >= options.MaxFailureGroups {
			reportLocalTestPhase(options, fmt.Sprintf("triage_stop casesRun=%d failureGroups=%d", casesRun, groups), started)
			return outcomes, casesRun, true
		}
	}
	return outcomes, casesRun, false
}

func localTestOutcomesFromRun(projectLabel string, run testreport.Run, options LocalTestOptions) []LocalTestOutcome {
	outcomes := make([]LocalTestOutcome, 0)
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if !matchesLocalTestCase(testCase.ClassName, testCase.MethodName, options) {
				continue
			}
			outcomes = append(outcomes, localTestRunOutcome(projectLabel, testCase))
		}
	}
	return outcomes
}

func localTestCaseBatches(cases []apextest.TestCase, classBatchSize int) [][]apextest.TestCase {
	if classBatchSize <= 0 || len(cases) == 0 {
		return nil
	}
	var batches [][]apextest.TestCase
	var batch []apextest.TestCase
	classesInBatch := 0
	lastClass := ""
	for _, testCase := range cases {
		if testCase.ClassName != lastClass {
			if classesInBatch >= classBatchSize && len(batch) > 0 {
				batches = append(batches, batch)
				batch = nil
				classesInBatch = 0
			}
			classesInBatch++
			lastClass = testCase.ClassName
		}
		batch = append(batch, testCase)
	}
	if len(batch) > 0 {
		batches = append(batches, batch)
	}
	return batches
}

func selectChangedLocalTestCases(report *LocalTestReport, index typesys.Index, cases []apextest.TestCase, root string, options LocalTestOptions) []apextest.TestCase {
	if strings.TrimSpace(options.ChangedSince) == "" {
		return cases
	}
	changes, err := watch.GitChangesSince(root, options.ChangedSince)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "changed_since",
			Message:  err.Error(),
		})
		return cases
	}
	selection := watch.SelectAffectedTests(index, changes)
	report.Selection = &selection
	if selection.Mode == watch.SelectionAll {
		return cases
	}
	if selection.Mode == watch.SelectionNone {
		return cases[:0]
	}
	selected := make(map[string]bool, len(selection.TestClasses))
	for _, className := range selection.TestClasses {
		selected[className] = true
	}
	out := cases[:0]
	for _, testCase := range cases {
		if selected[testCase.ClassName] {
			out = append(out, testCase)
		}
	}
	return out
}

func shouldAnalyzeLocalTests(options LocalTestOptions, totalCases int) bool {
	if strings.TrimSpace(options.Class) != "" || strings.TrimSpace(options.Method) != "" {
		return false
	}
	if options.ForceAnalysis {
		return true
	}
	if options.Parallelism > 0 || options.ProgressWriter != nil {
		return false
	}
	if options.BlockersOnly || options.TopFailures > 0 {
		return false
	}
	return totalCases <= largeLocalTestAnalysisThreshold
}

func localTestParallelism(options LocalTestOptions) int {
	if options.Parallelism > 0 {
		return options.Parallelism
	}
	if strings.TrimSpace(options.Method) != "" {
		return 1
	}
	if strings.TrimSpace(options.Class) != "" {
		procs := runtime.GOMAXPROCS(0)
		if procs < 2 {
			return procs
		}
		if procs < 4 {
			return procs
		}
		return 4
	}
	if runtime.GOMAXPROCS(0) < 2 {
		return 1
	}
	procs := runtime.GOMAXPROCS(0)
	if procs > 8 {
		return 8
	}
	if procs < 2 {
		return 1
	}
	return procs
}

func shouldParallelizeFocusedMethods(options LocalTestOptions) bool {
	return strings.TrimSpace(options.Class) != "" && strings.TrimSpace(options.Method) == ""
}

func localTestProgressReporter(w io.Writer) func(apextest.TestProgress) {
	if w == nil {
		return nil
	}
	var mu sync.Mutex
	return func(progress apextest.TestProgress) {
		mu.Lock()
		defer mu.Unlock()
		name := progress.ClassName
		if progress.MethodName != "" {
			name += "." + progress.MethodName
		}
		if progress.DurationMS > 0 || progress.Status != "" {
			fmt.Fprintf(w, "local-tests %s %s status=%s durationMs=%d\n", progress.Event, name, progress.Status, progress.DurationMS)
			return
		}
		fmt.Fprintf(w, "local-tests %s %s\n", progress.Event, name)
	}
}

func shouldTraceFocusedLocalTests(options LocalTestOptions) bool {
	return options.TraceBlocked || options.ProfileOnTimeout
}

func LocalTestReportFromRun(projectRoot string, run testreport.Run) LocalTestReport {
	root := projectRoot
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	absRoot, _ := filepath.Abs(root)
	projectLabel := filepath.Base(absRoot)
	if projectLabel == "." || projectLabel == string(filepath.Separator) {
		projectLabel = absRoot
	}
	report := LocalTestReport{
		Target:  "local Apex test execution readiness",
		Project: absRoot,
	}
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			report.Outcomes = append(report.Outcomes, localTestRunOutcome(projectLabel, testCase))
		}
	}
	finalizeLocalTestReport(&report, LocalTestOptions{}, time.Now())
	report.DurationMS = run.Summary().DurationMS
	return report
}

func CheckLocalTestCorpus(path string) (LocalTestCorpusReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LocalTestCorpusReport{}, err
	}
	var baseline LocalTestCorpusBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return LocalTestCorpusReport{}, err
	}
	absPath, _ := filepath.Abs(path)
	report := LocalTestCorpusReport{
		Target:   baseline.Target,
		Ready:    true,
		Baseline: absPath,
	}
	baseDir := filepath.Dir(absPath)
	for _, expected := range baseline.Projects {
		projectPath := expected.Project
		if !filepath.IsAbs(projectPath) {
			projectPath = filepath.Clean(filepath.Join(baseDir, projectPath))
		}
		actual, err := RunLocalTests(LocalTestOptions{Project: projectPath})
		if err != nil {
			return report, err
		}
		actualProject := LocalTestCorpusProjectResult{
			Project:  expected.Project,
			Ready:    actual.Ready,
			Summary:  actual.Summary,
			Outcomes: stableLocalTestOutcomes(actual.Outcomes),
		}
		report.Projects = append(report.Projects, actualProject)
		if actual.Ready != expected.Ready {
			report.Failures = append(report.Failures, fmt.Sprintf("%s ready = %t, want %t", expected.Project, actual.Ready, expected.Ready))
		}
		if actual.Summary != expected.Summary {
			report.Failures = append(report.Failures, fmt.Sprintf("%s summary = %+v, want %+v", expected.Project, actual.Summary, expected.Summary))
		}
		compareLocalTestOutcomes(expected.Project, actualProject.Outcomes, expected.Outcomes, &report.Failures)
	}
	report.Ready = len(report.Failures) == 0
	if !report.Ready {
		return report, fmt.Errorf("local-tests corpus baseline mismatch: %s", strings.Join(report.Failures, "; "))
	}
	return report, nil
}

func WriteLocalTestJSON(w io.Writer, report LocalTestReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteLocalTestCorpusJSON(w io.Writer, report LocalTestCorpusReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteLocalTestText(w io.Writer, report LocalTestReport) {
	state := "ready"
	if !report.Ready {
		state = "not ready"
	}
	fmt.Fprintf(w, "Local test readiness: %s\n", state)
	fmt.Fprintf(w, "project: %s\n", report.Project)
	fmt.Fprintf(w, "summary: pass=%d fail=%d unsupported=%d load_error=%d compile_error=%d internal_error=%d assert_fail=%d runtime_gap=%d compile_gap=%d timeout=%d total=%d\n",
		report.Summary.Pass,
		report.Summary.Fail,
		report.Summary.Unsupported,
		report.Summary.LoadErrors,
		report.Summary.CompileErrors,
		report.Summary.InternalErrors,
		report.Summary.AssertFailures,
		report.Summary.RuntimeGaps,
		report.Summary.CompileGaps,
		report.Summary.Timeouts,
		report.Summary.Total,
	)
	for _, group := range report.TopFailures {
		fmt.Fprintf(w, "* %s", group.Outcome)
		if group.CapabilityID != "" {
			fmt.Fprintf(w, " [%s]", group.CapabilityID)
		}
		fmt.Fprintf(w, ": %d", group.Count)
		if group.Error != "" {
			fmt.Fprintf(w, " - %s", group.Error)
		}
		fmt.Fprintln(w)
	}
	for _, outcome := range report.Outcomes {
		if outcome.Outcome == "pass" {
			continue
		}
		fmt.Fprintf(w, "- %s.%s: %s", outcome.Class, outcome.Method, outcome.Outcome)
		if outcome.CapabilityID != "" {
			fmt.Fprintf(w, " [%s]", outcome.CapabilityID)
		}
		if outcome.Error != "" {
			fmt.Fprintf(w, ": %s", outcome.Error)
		}
		fmt.Fprintln(w)
	}
}

func WriteLocalTestCorpusText(w io.Writer, report LocalTestCorpusReport) {
	state := "ready"
	if !report.Ready {
		state = "not ready"
	}
	fmt.Fprintf(w, "Local test corpus: %s\n", state)
	fmt.Fprintf(w, "baseline: %s\n", report.Baseline)
	for _, project := range report.Projects {
		fmt.Fprintf(w, "- %s: ready=%t pass=%d fail=%d unsupported=%d load_error=%d compile_error=%d internal_error=%d assert_fail=%d runtime_gap=%d compile_gap=%d timeout=%d total=%d\n",
			project.Project,
			project.Ready,
			project.Summary.Pass,
			project.Summary.Fail,
			project.Summary.Unsupported,
			project.Summary.LoadErrors,
			project.Summary.CompileErrors,
			project.Summary.InternalErrors,
			project.Summary.AssertFailures,
			project.Summary.RuntimeGaps,
			project.Summary.CompileGaps,
			project.Summary.Timeouts,
			project.Summary.Total,
		)
	}
	for _, failure := range report.Failures {
		fmt.Fprintf(w, "! %s\n", failure)
	}
}

func stableLocalTestOutcomes(outcomes []LocalTestOutcome) []LocalTestExpectedOutcome {
	stable := make([]LocalTestExpectedOutcome, 0, len(outcomes))
	for _, outcome := range outcomes {
		stable = append(stable, LocalTestExpectedOutcome{
			Class:        outcome.Class,
			Method:       outcome.Method,
			Outcome:      outcome.Outcome,
			CapabilityID: outcome.CapabilityID,
		})
	}
	sort.SliceStable(stable, func(i, j int) bool {
		if stable[i].Class == stable[j].Class {
			return stable[i].Method < stable[j].Method
		}
		return stable[i].Class < stable[j].Class
	})
	return stable
}

func compareLocalTestOutcomes(project string, actual, expected []LocalTestExpectedOutcome, failures *[]string) {
	if len(actual) != len(expected) {
		*failures = append(*failures, fmt.Sprintf("%s outcomes length = %d, want %d", project, len(actual), len(expected)))
		return
	}
	for i := range expected {
		if actual[i] != expected[i] {
			*failures = append(*failures, fmt.Sprintf("%s outcome[%d] = %+v, want %+v", project, i, actual[i], expected[i]))
		}
	}
}

func loadLocalTestIndex(root string) (typesys.Index, []diagnostic.Diagnostic, error) {
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, nil, err
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		index := typesys.Build(p, schema.Schema{})
		diag := diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "OAERSCHEMA001",
			Message:  fmt.Sprintf("metadata schema load failed: %v", err),
		}
		index.Diagnostics = append(index.Diagnostics, diag)
		return index, []diagnostic.Diagnostic{diag}, nil
	}
	return typesys.Build(p, s), nil, nil
}

func localTestFilter(options LocalTestOptions) string {
	switch {
	case options.Class != "" && options.Method != "":
		return options.Class + "." + options.Method
	case options.Class != "":
		return options.Class
	case options.Method != "":
		return options.Method
	default:
		return ""
	}
}

func filterLocalTestCases(cases []apextest.TestCase, options LocalTestOptions) []apextest.TestCase {
	out := make([]apextest.TestCase, 0, len(cases))
	for _, testCase := range cases {
		if matchesLocalTestCase(testCase.ClassName, testCase.MethodName, options) {
			out = append(out, testCase)
		}
	}
	return out
}

func matchesLocalTestCase(className, methodName string, options LocalTestOptions) bool {
	if options.Class != "" && !strings.EqualFold(className, options.Class) {
		return false
	}
	if options.Method != "" && !strings.EqualFold(methodName, options.Method) {
		return false
	}
	return true
}

func firstLocalTestError(diagnostics []diagnostic.Diagnostic) (diagnostic.Diagnostic, bool) {
	for _, diag := range diagnostics {
		if diag.Severity == diagnostic.Error {
			return diag, true
		}
	}
	return diagnostic.Diagnostic{}, false
}

func localTestDiagnosticOutcome(projectLabel string, testCase apextest.TestCase, outcome, phase string, diag diagnostic.Diagnostic) LocalTestOutcome {
	line := testCase.Range.Start.Line
	if diag.Range != nil && diag.Range.Start.Line > 0 {
		line = diag.Range.Start.Line
	}
	file := testCase.File
	if diag.File != "" {
		file = diag.File
	}
	return LocalTestOutcome{
		ProjectLabel: projectLabel,
		Class:        testCase.ClassName,
		Method:       testCase.MethodName,
		Outcome:      outcome,
		Phase:        phase,
		CapabilityID: localTestCapabilityID(phase, diag.Code, diag.Message),
		File:         file,
		Line:         line,
		Error:        diag.Message,
	}
}

func localTestRunOutcome(projectLabel string, testCase testreport.Case) LocalTestOutcome {
	outcome := "pass"
	phase := ""
	if testCase.Status != testreport.StatusPass {
		phase = "execute"
	}
	switch testCase.Status {
	case testreport.StatusPass:
		outcome = "pass"
	case testreport.StatusUnsupported:
		outcome = "unsupported"
	case testreport.StatusCompileError:
		outcome = "compile_gap"
		phase = "compile"
	case testreport.StatusRuntimeError:
		outcome = "internal_error"
	default:
		outcome = "runtime_gap"
	}
	if testCase.Problem != nil && strings.EqualFold(testCase.Problem.Type, "UnsupportedFeature") {
		outcome = "unsupported"
	}
	if testCase.Problem != nil && strings.EqualFold(testCase.Problem.Type, "Canceled") && strings.Contains(strings.ToLower(testCase.Problem.Message), "deadline exceeded") {
		outcome = "timeout"
		phase = "timeout"
	}
	if testCase.Problem != nil && strings.Contains(strings.ToLower(testCase.Problem.Type+" "+testCase.Problem.Message), "assert") {
		outcome = "assert_fail"
		phase = "assert"
	}
	out := LocalTestOutcome{
		ProjectLabel: projectLabel,
		Class:        testCase.ClassName,
		Method:       testCase.MethodName,
		Outcome:      outcome,
		Phase:        phase,
		DurationMS:   testCase.DurationMS,
	}
	if len(testCase.Trace) > 0 {
		out.TraceEvents = len(testCase.Trace)
	}
	if testCase.Profile != nil {
		out.ProfileEvents = testCase.Profile.Events
		out.ProfileCategories = testCase.Profile.Categories
	}
	if testCase.Problem != nil {
		out.Error = testCase.Problem.Message
		out.CapabilityID = localTestCapabilityID(phase, testCase.Problem.Type, testCase.Problem.Message)
		if len(testCase.Problem.Stack) > 0 {
			frame := testCase.Problem.Stack[0]
			out.TopFrame = &frame
			out.File = frame.File
			out.Line = frame.Line
		}
	}
	return out
}

func localTestCapabilityID(phase, code, message string) string {
	code = strings.TrimSpace(code)
	if strings.EqualFold(code, "UnsupportedFeature") {
		return "apex.test.unsupported"
	}
	if strings.EqualFold(code, "Canceled") {
		if strings.Contains(strings.ToLower(message), "deadline exceeded") {
			return "apex.test.timeout"
		}
		return "apex.test.canceled"
	}
	if code != "" {
		return strings.ToLower(code)
	}
	text := strings.ToLower(message)
	switch {
	case strings.Contains(text, "unsupported"):
		return "apex.test.unsupported"
	case strings.Contains(text, "metadata"):
		return "metadata.load"
	case strings.Contains(text, "parse"):
		return "apex.parse"
	case strings.Contains(text, "panic") || strings.Contains(text, "internal"):
		return "oaer.internal"
	case phase != "":
		return "apex.test." + strings.ReplaceAll(phase, "_", "-")
	default:
		return "apex.test.unknown"
	}
}

func finalizeLocalTestReport(report *LocalTestReport, options LocalTestOptions, started time.Time) {
	if options.BlockersOnly {
		outcomes := report.Outcomes[:0]
		for _, outcome := range report.Outcomes {
			if outcome.Outcome != "pass" {
				outcomes = append(outcomes, outcome)
			}
		}
		report.Outcomes = outcomes
	}
	for _, outcome := range report.Outcomes {
		report.Summary.Total++
		switch outcome.Outcome {
		case "pass":
			report.Summary.Pass++
		case "fail":
			report.Summary.Fail++
		case "unsupported":
			report.Summary.Unsupported++
		case "load_error":
			report.Summary.LoadErrors++
		case "compile_error":
			report.Summary.CompileErrors++
		case "internal_error":
			report.Summary.InternalErrors++
		case "assert_fail":
			report.Summary.AssertFailures++
		case "runtime_gap":
			report.Summary.RuntimeGaps++
		case "compile_gap":
			report.Summary.CompileGaps++
		case "timeout":
			report.Summary.Timeouts++
		}
	}
	if options.TopFailures > 0 {
		report.TopFailures = localTestTopFailures(report.Outcomes, options.TopFailures)
	}
	report.Ready = report.Summary.Fail == 0 &&
		report.Summary.Unsupported == 0 &&
		report.Summary.LoadErrors == 0 &&
		report.Summary.CompileErrors == 0 &&
		report.Summary.InternalErrors == 0 &&
		report.Summary.AssertFailures == 0 &&
		report.Summary.RuntimeGaps == 0 &&
		report.Summary.CompileGaps == 0 &&
		report.Summary.Timeouts == 0
	report.DurationMS = time.Since(started).Milliseconds()
}

func localTestTopFailures(outcomes []LocalTestOutcome, limit int) []LocalTestFailureGroup {
	if limit <= 0 {
		return nil
	}
	counts := localTestFailureGroups(outcomes)
	groups := make([]LocalTestFailureGroup, 0, len(counts))
	for key, count := range counts {
		groups = append(groups, LocalTestFailureGroup{
			Outcome:      key.outcome,
			Phase:        key.phase,
			CapabilityID: key.capabilityID,
			Error:        key.error,
			Count:        count,
		})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Count != groups[j].Count {
			return groups[i].Count > groups[j].Count
		}
		if groups[i].Outcome != groups[j].Outcome {
			return groups[i].Outcome < groups[j].Outcome
		}
		if groups[i].CapabilityID != groups[j].CapabilityID {
			return groups[i].CapabilityID < groups[j].CapabilityID
		}
		return groups[i].Error < groups[j].Error
	})
	if len(groups) > limit {
		groups = groups[:limit]
	}
	return groups
}

type localTestFailureGroupKey struct {
	outcome      string
	phase        string
	capabilityID string
	error        string
}

func localTestFailureGroupCount(outcomes []LocalTestOutcome) int {
	return len(localTestFailureGroups(outcomes))
}

func localTestFailureGroups(outcomes []LocalTestOutcome) map[localTestFailureGroupKey]int {
	counts := make(map[localTestFailureGroupKey]int)
	for _, outcome := range outcomes {
		if outcome.Outcome == "pass" {
			continue
		}
		errText := strings.TrimSpace(outcome.Error)
		if errText != "" && len(errText) > 180 {
			errText = errText[:180]
		}
		counts[localTestFailureGroupKey{
			outcome:      outcome.Outcome,
			phase:        outcome.Phase,
			capabilityID: outcome.CapabilityID,
			error:        errText,
		}]++
	}
	return counts
}

func IsLocalTestTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}
