package testreport

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
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

func WriteConsole(w io.Writer, run Run) error {
	return WriteConsoleWithOptions(w, run, ConsoleOptions{})
}

func WriteConsoleWithOptions(w io.Writer, run Run, opts ConsoleOptions) error {
	var out strings.Builder
	summary := run.Summary()
	fmt.Fprintf(&out, "Selected %d %s.\n\n", summary.Total, plural(summary.Total, "test", "tests"))
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			writeConsoleCase(&out, suite.Name, testCase)
		}
	}
	if problemCase, ok := firstProblemCase(run); ok {
		writeConsoleProblem(&out, problemCase.suiteName, problemCase.testCase)
	}
	writeConsoleSummary(&out, summary)
	if strings.TrimSpace(opts.ReportPath) != "" {
		fmt.Fprintf(&out, "Report: %s\n", opts.ReportPath)
	}
	_, err := io.WriteString(w, out.String())
	return err
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
		Tests:    summary.Total,
		Failures: summary.Failed,
		Errors:   summary.Errors,
		Skipped:  summary.Skipped,
		Time:     seconds(summary.DurationMS),
		Suites:   make([]junitTestSuite, 0, len(run.Suites)),
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

func writeConsoleCase(out *strings.Builder, suiteName string, testCase Case) {
	status := normalizeStatus(testCase.Status)
	label := consoleStatus(status)
	fmt.Fprintf(out, "%s %s  %dms\n", label, testCase.displayName(suiteName), testCase.DurationMS)
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

func writeConsoleProblem(out *strings.Builder, suiteName string, testCase Case) {
	if testCase.Problem == nil {
		return
	}
	out.WriteString("\n")
	if testCase.Problem.Type != "" {
		fmt.Fprintf(out, "%s: %s\n", testCase.Problem.Type, testCase.Problem.Message)
	} else if testCase.Problem.Message != "" {
		fmt.Fprintf(out, "%s\n", testCase.Problem.Message)
	}
	if testCase.Problem.Detail != "" {
		for _, line := range strings.Split(testCase.Problem.Detail, "\n") {
			fmt.Fprintf(out, "%s\n", line)
		}
	}
	if len(testCase.Problem.Stack) > 0 {
		out.WriteString("\n")
		frame := testCase.Problem.Stack[0]
		symbol := frame.Symbol
		if symbol == "" {
			symbol = testCase.displayName(suiteName)
		}
		if frame.Line > 0 {
			fmt.Fprintf(out, "at %s:%d\n", symbol, frame.Line)
		} else {
			fmt.Fprintf(out, "at %s\n", symbol)
		}
	}
	out.WriteString("\n")
}

func writeConsoleSummary(out *strings.Builder, summary Summary) {
	parts := []string{
		fmt.Sprintf("%d %s", summary.Passed, plural(summary.Passed, "passed", "passed")),
		fmt.Sprintf("%d %s", summary.Failed, plural(summary.Failed, "failed", "failed")),
		fmt.Sprintf("%d %s", summary.Skipped, plural(summary.Skipped, "skipped", "skipped")),
	}
	if summary.CompileErrors > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", summary.CompileErrors, plural(summary.CompileErrors, "compile error", "compile errors")))
	}
	if summary.RuntimeErrors > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", summary.RuntimeErrors, plural(summary.RuntimeErrors, "runtime error", "runtime errors")))
	}
	if summary.Unsupported > 0 {
		parts = append(parts, fmt.Sprintf("%d unsupported", summary.Unsupported))
	}
	parts = append(parts, fmt.Sprintf("%d total", summary.Total), fmt.Sprintf("%dms", summary.DurationMS))
	fmt.Fprintf(out, "Result: %s\n", strings.Join(parts, ", "))
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func consoleStatus(status Status) string {
	switch status {
	case StatusPass:
		return "PASS"
	case StatusFail:
		return "FAIL"
	case StatusSkipped:
		return "SKIP"
	case StatusCompileError:
		return "COMPILE"
	case StatusRuntimeError:
		return "ERROR"
	case StatusUnsupported:
		return "UNSUPPORTED"
	default:
		return "ERROR"
	}
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
		return &junitProblem{Type: fallbackType, Message: fallbackType}
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
		Message: problem.Message,
		Text:    text,
	}
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
