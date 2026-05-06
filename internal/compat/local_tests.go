package compat

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/open-aer/oaer/internal/apextest"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sema"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/typesys"
)

type LocalTestOptions struct {
	Project      string
	Class        string
	Method       string
	BlockersOnly bool
}

type LocalTestReport struct {
	Target      string                  `json:"target"`
	Ready       bool                    `json:"ready"`
	Project     string                  `json:"project"`
	DurationMS  int64                   `json:"durationMs,omitempty"`
	Summary     LocalTestSummary        `json:"summary"`
	Outcomes    []LocalTestOutcome      `json:"outcomes"`
	Diagnostics []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}

type LocalTestSummary struct {
	Total          int `json:"total"`
	Pass           int `json:"pass"`
	Fail           int `json:"fail"`
	Unsupported    int `json:"unsupported"`
	LoadErrors     int `json:"loadError"`
	CompileErrors  int `json:"compileError"`
	InternalErrors int `json:"internalError"`
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

	index, loadDiagnostics, err := loadLocalTestIndex(root)
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
	report.Diagnostics = append(report.Diagnostics, loadDiagnostics...)

	testOpts := apextest.Options{Filter: localTestFilter(options)}
	cases := apextest.Discover(index, testOpts)
	cases = filterLocalTestCases(cases, options)
	sort.SliceStable(cases, func(i, j int) bool {
		if cases[i].ClassName == cases[j].ClassName {
			return cases[i].MethodName < cases[j].MethodName
		}
		return cases[i].ClassName < cases[j].ClassName
	})

	semaResult := sema.Analyze(index)
	report.Diagnostics = append(report.Diagnostics, semaResult.Diagnostics...)
	if firstError, ok := firstLocalTestError(semaResult.Diagnostics); ok {
		for _, testCase := range cases {
			report.Outcomes = append(report.Outcomes, localTestDiagnosticOutcome(projectLabel, testCase, "compile_error", "compile", firstError))
		}
		finalizeLocalTestReport(&report, options, started)
		return report, nil
	}

	run := apextest.Run(index, testOpts)
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if !matchesLocalTestCase(testCase.ClassName, testCase.MethodName, options) {
				continue
			}
			report.Outcomes = append(report.Outcomes, localTestRunOutcome(projectLabel, testCase))
		}
	}
	finalizeLocalTestReport(&report, options, started)
	return report, nil
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
	fmt.Fprintf(w, "summary: pass=%d fail=%d unsupported=%d load_error=%d compile_error=%d internal_error=%d total=%d\n",
		report.Summary.Pass,
		report.Summary.Fail,
		report.Summary.Unsupported,
		report.Summary.LoadErrors,
		report.Summary.CompileErrors,
		report.Summary.InternalErrors,
		report.Summary.Total,
	)
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
		fmt.Fprintf(w, "- %s: ready=%t pass=%d fail=%d unsupported=%d load_error=%d compile_error=%d internal_error=%d total=%d\n",
			project.Project,
			project.Ready,
			project.Summary.Pass,
			project.Summary.Fail,
			project.Summary.Unsupported,
			project.Summary.LoadErrors,
			project.Summary.CompileErrors,
			project.Summary.InternalErrors,
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
		outcome = "compile_error"
		phase = "compile"
	case testreport.StatusRuntimeError:
		outcome = "internal_error"
	default:
		outcome = "fail"
	}
	if testCase.Problem != nil && strings.EqualFold(testCase.Problem.Type, "UnsupportedFeature") {
		outcome = "unsupported"
	}
	if testCase.Problem != nil && strings.Contains(strings.ToLower(testCase.Problem.Type+" "+testCase.Problem.Message), "assert") {
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
		}
	}
	report.Ready = report.Summary.Fail == 0 &&
		report.Summary.Unsupported == 0 &&
		report.Summary.LoadErrors == 0 &&
		report.Summary.CompileErrors == 0 &&
		report.Summary.InternalErrors == 0
	report.DurationMS = time.Since(started).Milliseconds()
}
