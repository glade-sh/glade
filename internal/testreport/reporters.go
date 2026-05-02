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

func WriteConsole(w io.Writer, run Run) error {
	var out strings.Builder
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			writeConsoleCase(&out, suite.Name, testCase)
		}
	}
	summary := run.Summary()
	fmt.Fprintf(&out, "result: %d passed, %d failed, %d skipped, %d errors, %d total, %dms\n",
		summary.Passed, summary.Failed, summary.Skipped, summary.Errors, summary.Total, summary.DurationMS)
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
	fmt.Fprintf(out, "%s %s", label, testCase.displayName(suiteName))
	if status == StatusCompileError || status == StatusRuntimeError || status == StatusUnsupported {
		fmt.Fprintf(out, " (%s, %dms)\n", status, testCase.DurationMS)
	} else {
		fmt.Fprintf(out, " (%dms)\n", testCase.DurationMS)
	}
	if status == StatusPass {
		return
	}
	if testCase.Problem == nil {
		return
	}
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
	for _, frame := range testCase.Problem.Stack {
		fmt.Fprintf(out, "  at %s\n", formatFrame(frame))
	}
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
