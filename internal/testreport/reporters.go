package testreport

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/cliui"
)

type Reporter interface {
	Write(io.Writer, Run) error
}

type ReporterFunc func(io.Writer, Run) error

func (f ReporterFunc) Write(w io.Writer, run Run) error {
	return f(w, run)
}

type ConsoleOptions struct {
	ReportPath string
}

// consoleDetailLimit caps per-test listing on large runs so result formatting
// stays responsive and stdout stays readable.
const consoleDetailLimit = 80

func WriteConsole(w io.Writer, run Run) error {
	return WriteConsoleWithOptions(w, run, ConsoleOptions{})
}

func WriteConsoleWithOptions(w io.Writer, run Run, opts ConsoleOptions) error {
	t := cliui.NewTheme(w)
	summary := run.Summary()
	var out strings.Builder

	out.WriteString("Glade test\n\n")
	fmt.Fprintf(&out, "%d selected, %d passed, %d failed\n\n", summary.Total, summary.Passed, summary.Failed)
	fmt.Fprintf(&out, "Selected: %d\n", summary.Total)
	fmt.Fprintf(&out, "Passed:   %d\n", summary.Passed)
	fmt.Fprintf(&out, "Failed:   %d\n", summary.Failed)
	if summary.Skipped > 0 {
		fmt.Fprintf(&out, "Skipped:  %d\n", summary.Skipped)
	}
	if summary.Errors > 0 {
		fmt.Fprintf(&out, "Errors:   %d\n", summary.Errors)
	}
	if summary.ExcludedClasses > 0 {
		fmt.Fprintf(&out, "Excluded: %d\n", summary.ExcludedClasses)
	}
	fmt.Fprintf(&out, "Runtime:  %s\n", cliui.FormatDurationMS(summary.DurationMS))
	out.WriteString("\n")

	compact := summary.Total > consoleDetailLimit
	if compact {
		writeCompactConsoleCases(&out, t, run, summary)
	} else {
		maxName := 20
		for _, suite := range run.Suites {
			for _, testCase := range suite.Cases {
				if n := len(testCase.displayName(suite.Name)); n > maxName {
					maxName = n
				}
			}
		}
		for _, suite := range run.Suites {
			for _, testCase := range suite.Cases {
				writeConsoleCaseLine(&out, t, suite.Name, testCase, maxName)
			}
		}
	}
	writeConsoleExcludedClasses(&out, run.ExcludedClasses)
	if problemCase, ok := firstProblemCase(run); ok {
		writeStyledConsoleProblem(&out, problemCase.suiteName, problemCase.testCase)
	}

	if strings.TrimSpace(opts.ReportPath) != "" {
		out.WriteString("\nArtifacts:\n")
		fmt.Fprintf(&out, "  Report  %s\n", opts.ReportPath)
		fmt.Fprintf(&out, "Report: %s\n", opts.ReportPath)
	}
	out.WriteString("\nNext:\n")
	for _, command := range NextCommands(summary) {
		fmt.Fprintf(&out, "  %s\n", command)
	}
	_, err := io.WriteString(w, out.String())
	return err
}

// writeConsoleExcludedClasses lists the classes the compile gate kept from
// running. A class with its own error shows that error; a dependent shows the
// excluded class it references and, when different, the root invalid class.
func writeConsoleExcludedClasses(out *strings.Builder, excluded []CompileExclusion) {
	if len(excluded) == 0 {
		return
	}
	noun := "classes"
	if len(excluded) == 1 {
		noun = "class"
	}
	fmt.Fprintf(out, "\nCompile errors excluded %d %s:\n", len(excluded), noun)
	width := 0
	for _, entry := range excluded {
		width = max(width, len(entry.ClassName))
	}
	for _, entry := range excluded {
		if entry.DependsOn != "" {
			fmt.Fprintf(out, "  %s  %s\n", cliui.PadVisible(entry.ClassName, width), excludedClassDetail(entry))
			continue
		}
		for i, problem := range entry.rootProblems() {
			name := entry.ClassName
			if i > 0 {
				name = ""
			}
			detail, _, _ := strings.Cut(problem.Message, "\n")
			if entry.File != "" && problem.Line > 0 {
				detail += fmt.Sprintf(" (%s:%d)", filepath.Base(entry.File), problem.Line)
			}
			fmt.Fprintf(out, "  %s  %s\n", cliui.PadVisible(name, width), detail)
		}
	}
}

// rootProblems returns every problem of a root entry: Problems when it lists
// more than one, otherwise Line and Message.
func (entry CompileExclusion) rootProblems() []CompileProblem {
	if len(entry.Problems) > 0 {
		return entry.Problems
	}
	return []CompileProblem{{Line: entry.Line, Message: entry.Message}}
}

func NextCommands(summary Summary) []string {
	commands := []string{"glade test --watch"}
	if !summary.OK() {
		commands = append(commands, "glade test failed")
	}
	return commands
}

func formatTestHeaderSummary(s Summary) string {
	return fmt.Sprintf("%d selected · %d passed · %d failed · %s",
		s.Total, s.Passed, s.Failed, cliui.FormatDurationMS(s.DurationMS))
}

func formatTestResultSummary(s Summary) string {
	parts := []string{
		fmt.Sprintf("%d passed", s.Passed),
		fmt.Sprintf("%d failed", s.Failed),
		fmt.Sprintf("%d total", s.Total),
		cliui.FormatDurationMS(s.DurationMS),
	}
	if s.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", s.Skipped))
	}
	if s.CompileErrors > 0 {
		parts = append(parts, fmt.Sprintf("%d compile errors", s.CompileErrors))
	}
	if s.RuntimeErrors > 0 {
		parts = append(parts, fmt.Sprintf("%d runtime errors", s.RuntimeErrors))
	}
	if s.Unsupported > 0 {
		parts = append(parts, fmt.Sprintf("%d unsupported", s.Unsupported))
	}
	return strings.Join(parts, " · ")
}

func testCaseIcon(t cliui.Theme, status Status) string {
	switch normalizeStatus(status) {
	case StatusPass:
		if t.Color {
			return t.Green(t.GlyphPass)
		}
		return t.GlyphPass
	case StatusFail:
		if t.Color {
			return t.Red(t.GlyphFail)
		}
		return t.GlyphFail
	case StatusSkipped:
		if t.Color {
			return t.Dim("○")
		}
		return "○"
	default:
		if t.Color {
			return t.Red(t.GlyphFail)
		}
		return t.GlyphFail
	}
}

func writeConsoleCaseLine(out *strings.Builder, t cliui.Theme, suiteName string, testCase Case, maxName int) {
	name := testCase.displayName(suiteName)
	timing := cliui.FormatDurationMS(testCase.DurationMS)
	if t.Color {
		timing = t.Dim(timing)
	}
	fmt.Fprintf(out, "  %s  %s  %s\n", testCaseIcon(t, testCase.Status), cliui.PadVisible(name, maxName), timing)
}

func writeCompactConsoleCases(out *strings.Builder, t cliui.Theme, run Run, summary Summary) {
	maxName := 20
	nonPass := 0
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if normalizeStatus(testCase.Status) == StatusPass {
				continue
			}
			nonPass++
			if n := len(testCase.displayName(suite.Name)); n > maxName {
				maxName = n
			}
		}
	}
	if nonPass == 0 {
		line := fmt.Sprintf("... %d passed tests omitted from listing", summary.Passed)
		if t.Color {
			line = t.Dim(line)
		}
		fmt.Fprintf(out, "  %s\n", line)
		return
	}
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if normalizeStatus(testCase.Status) == StatusPass {
				continue
			}
			writeConsoleCaseLine(out, t, suite.Name, testCase, maxName)
		}
	}
	if summary.Passed > 0 {
		line := fmt.Sprintf("... %d passed tests omitted from listing", summary.Passed)
		if t.Color {
			line = t.Dim(line)
		}
		fmt.Fprintf(out, "  %s\n", line)
	}
}

func writeStyledConsoleProblem(out *strings.Builder, suiteName string, testCase Case) {
	if testCase.Problem == nil {
		return
	}
	out.WriteString("\n")
	name := testCase.displayName(suiteName)
	fmt.Fprintf(out, "  %s\n", name)
	if testCase.Problem.Type != "" {
		fmt.Fprintf(out, "  %s: %s\n", testCase.Problem.Type, testCase.Problem.Message)
	} else if testCase.Problem.Message != "" {
		fmt.Fprintf(out, "  %s\n", testCase.Problem.Message)
	}
	if testCase.Problem.Detail != "" {
		for _, line := range strings.Split(testCase.Problem.Detail, "\n") {
			fmt.Fprintf(out, "  %s\n", line)
		}
	}
	if len(testCase.Problem.Stack) > 0 {
		out.WriteString("\n")
		frame := testCase.Problem.Stack[0]
		location := frame.File
		if location == "" {
			location = frame.Symbol
		}
		if location == "" {
			location = testCase.displayName(suiteName)
		}
		if frame.Line > 0 {
			fmt.Fprintf(out, "  %s:%d\n", location, frame.Line)
		} else {
			fmt.Fprintf(out, "  %s\n", location)
		}
	}
	out.WriteString("\n")
}

func WriteJSON(w io.Writer, run Run) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(run)
}

func WriteJUnitXML(w io.Writer, run Run) error {
	summary := run.Summary()
	testSuites := junitTestSuites{
		Name:     run.Name,
		Tests:    summary.Total + len(run.ExcludedClasses),
		Failures: summary.Failed,
		Errors:   summary.Errors + len(run.ExcludedClasses),
		Skipped:  summary.Skipped,
		Time:     seconds(summary.DurationMS),
		Suites:   make([]junitTestSuite, 0, len(run.Suites)+1),
	}
	for _, suite := range run.Suites {
		suiteSummary := suite.Summary()
		outSuite := junitTestSuite{
			Name:     suite.Name,
			Tests:    suiteSummary.Total,
			Failures: suiteSummary.Failed,
			Errors:   suiteSummary.Errors,
			Skipped:  suiteSummary.Skipped,
			Time:     seconds(suiteSummary.DurationMS),
			Cases:    make([]junitTestCase, 0, len(suite.Cases)),
		}
		for _, testCase := range suite.Cases {
			outSuite.Cases = append(outSuite.Cases, makeJUnitCase(suite.Name, testCase))
		}
		testSuites.Suites = append(testSuites.Suites, outSuite)
	}
	if len(run.ExcludedClasses) > 0 {
		testSuites.Suites = append(testSuites.Suites, junitExcludedClassesSuite(run.ExcludedClasses))
	}

	data, err := xml.MarshalIndent(testSuites, "", "  ")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err = io.WriteString(w, "\n")
	return err
}

type consoleProblemCase struct {
	suiteName string
	testCase  Case
}

func firstProblemCase(run Run) (consoleProblemCase, bool) {
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			status := normalizeStatus(testCase.Status)
			if status == StatusPass || status == StatusSkipped {
				continue
			}
			if testCase.Problem != nil {
				return consoleProblemCase{suiteName: suite.Name, testCase: testCase}, true
			}
		}
	}
	return consoleProblemCase{}, false
}

type junitTestSuites struct {
	XMLName  xml.Name         `xml:"testsuites"`
	Name     string           `xml:"name,attr,omitempty"`
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Errors   int              `xml:"errors,attr"`
	Skipped  int              `xml:"skipped,attr"`
	Time     string           `xml:"time,attr"`
	Suites   []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Errors   int             `xml:"errors,attr"`
	Skipped  int             `xml:"skipped,attr"`
	Time     string          `xml:"time,attr"`
	Cases    []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr,omitempty"`
	Time      string        `xml:"time,attr"`
	Failure   *junitProblem `xml:"failure,omitempty"`
	Error     *junitProblem `xml:"error,omitempty"`
	Skipped   *junitProblem `xml:"skipped,omitempty"`
}

type junitProblem struct {
	Reason  Reason `xml:"reason,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	Message string `xml:"message,attr,omitempty"`
	Text    string `xml:",chardata"`
}

func makeJUnitCase(suiteName string, testCase Case) junitTestCase {
	out := junitTestCase{
		Name:      testCase.junitName(suiteName),
		ClassName: testCase.junitClassName(suiteName),
		Time:      seconds(testCase.DurationMS),
	}
	switch normalizeStatus(testCase.Status) {
	case StatusFail:
		out.Failure = makeJUnitProblem(testCase, "assertion")
	case StatusSkipped:
		out.Skipped = makeJUnitProblem(testCase, "skipped")
	case StatusCompileError, StatusRuntimeError, StatusUnsupported:
		out.Error = makeJUnitProblem(testCase, string(normalizeStatus(testCase.Status)))
	}
	return out
}

func makeJUnitProblem(testCase Case, fallbackType string) *junitProblem {
	problem := testCase.Problem
	if problem == nil {
		return &junitProblem{Type: fallbackType, Message: fallbackType, Reason: testCase.Reason}
	}
	problemType := problem.Type
	if problemType == "" {
		problemType = fallbackType
	}
	text := problem.Detail
	for _, frame := range problem.Stack {
		if text != "" {
			text += "\n"
		}
		text += "at " + formatFrame(frame)
	}
	return &junitProblem{
		Type:    problemType,
		Reason:  testCase.Reason,
		Message: problem.Message,
		Text:    text,
	}
}

// junitExcludedClassesSuite reports each class the compile gate excluded as
// an errored <ClassName>.compile case, so a run that excludes a class without
// a selected test still fails in JUnit consumers.
func junitExcludedClassesSuite(excluded []CompileExclusion) junitTestSuite {
	suite := junitTestSuite{
		Name:   "compile",
		Tests:  len(excluded),
		Errors: len(excluded),
		Time:   seconds(0),
		Cases:  make([]junitTestCase, 0, len(excluded)),
	}
	for _, entry := range excluded {
		suite.Cases = append(suite.Cases, junitTestCase{
			Name:      "compile",
			ClassName: entry.ClassName,
			Time:      seconds(0),
			Error: &junitProblem{
				Type:    "CompileError",
				Message: entry.Message,
				Text:    excludedClassDetail(entry),
			},
		})
	}
	return suite
}

// excludedClassDetail describes an excluded class in plain text: the root's
// problems with their locations, or the class a dependent depends on.
func excludedClassDetail(entry CompileExclusion) string {
	if entry.DependsOn != "" {
		detail := "depends on " + entry.DependsOn
		if entry.Root != "" && !strings.EqualFold(entry.Root, entry.DependsOn) {
			detail += " (root " + entry.Root + ")"
		}
		return detail
	}
	var lines []string
	for _, problem := range entry.rootProblems() {
		line := problem.Message
		if entry.File != "" {
			location := entry.File
			if problem.Line > 0 {
				location = fmt.Sprintf("%s:%d", entry.File, problem.Line)
			}
			line = location + ": " + line
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func seconds(ms int64) string {
	return fmt.Sprintf("%.3f", float64(ms)/1000)
}

func formatFrame(frame StackFrame) string {
	location := frame.File
	if frame.Line > 0 {
		location += fmt.Sprintf(":%d", frame.Line)
		if frame.Column > 0 {
			location += fmt.Sprintf(":%d", frame.Column)
		}
	}
	if frame.Symbol == "" {
		return location
	}
	if location == "" {
		return frame.Symbol
	}
	return fmt.Sprintf("%s (%s)", frame.Symbol, location)
}
