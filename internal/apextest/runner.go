package apextest

import (
	"fmt"
	"os"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/vm"
)

type Options struct {
	Filter string
}

type TestCase struct {
	ClassName  string
	MethodName string
	File       string
	Range      diagnostic.Range
	Body       string
}

func Discover(index typesys.Index, opts Options) []TestCase {
	var out []TestCase
	filter := strings.ToLower(strings.TrimSpace(opts.Filter))
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass {
			continue
		}
		classIsTest := typ.IsTest
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod {
				continue
			}
			if !member.IsTest && !classIsTest {
				continue
			}
			if isTestSetup(member.Modifiers) {
				continue
			}
			testName := typ.Name + "." + member.Name
			if filter != "" && !strings.Contains(strings.ToLower(testName), filter) {
				continue
			}
			out = append(out, TestCase{
				ClassName:  typ.Name,
				MethodName: member.Name,
				File:       typ.File,
				Range:      member.Range,
			})
		}
	}
	return out
}

func Run(index typesys.Index, opts Options) testreport.Run {
	cases := Discover(index, opts)
	suites := make(map[string][]testreport.Case)
	order := make([]string, 0)
	for _, testCase := range cases {
		if _, ok := suites[testCase.ClassName]; !ok {
			order = append(order, testCase.ClassName)
		}
		suites[testCase.ClassName] = append(suites[testCase.ClassName], runCase(testCase))
	}

	run := testreport.Run{Name: "oaer test"}
	for _, name := range order {
		run.Suites = append(run.Suites, testreport.Suite{Name: name, Cases: suites[name]})
	}
	return run
}

func runCase(testCase TestCase) testreport.Case {
	out := testreport.Case{
		ClassName:  testCase.ClassName,
		MethodName: testCase.MethodName,
		Status:     testreport.StatusPass,
	}
	source, err := os.ReadFile(testCase.File)
	if err != nil {
		out.Status = testreport.StatusCompileError
		out.Problem = problem("FileError", err.Error(), testCase)
		return out
	}
	body, err := extractMethodBody(string(source), testCase.Range)
	if err != nil {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", err.Error(), testCase)
		return out
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", err.Error(), testCase)
		return out
	}
	if _, err := vm.Execute(program, nil); err != nil {
		out.Status = testreport.StatusFail
		out.Problem = problem("RuntimeError", err.Error(), testCase)
		return out
	}
	return out
}

func extractMethodBody(source string, r diagnostic.Range) (string, error) {
	start := r.Start.Offset
	end := r.End.Offset
	if start < 0 || start >= len(source) || end <= start || end > len(source) {
		return "", fmt.Errorf("method source range is unavailable")
	}
	text := source[start:end]
	open := strings.IndexByte(text, '{')
	if open < 0 {
		return "", fmt.Errorf("test method has no executable body")
	}
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '\'':
			i = skipApexString(text, i)
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[open+1 : i], nil
			}
		}
	}
	return "", fmt.Errorf("test method body is incomplete")
}

func skipApexString(source string, start int) int {
	for i := start + 1; i < len(source); i++ {
		if source[i] == '\'' {
			if i+1 < len(source) && source[i+1] == '\'' {
				i++
				continue
			}
			return i
		}
	}
	return len(source) - 1
}

func isTestSetup(modifiers []string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(strings.TrimPrefix(modifier, "@"), "TestSetup") {
			return true
		}
	}
	return false
}

func problem(kind, message string, testCase TestCase) *testreport.Problem {
	return &testreport.Problem{
		Type:    kind,
		Message: message,
		Stack: []testreport.StackFrame{{
			Symbol: testCase.ClassName + "." + testCase.MethodName,
			File:   testCase.File,
			Line:   testCase.Range.Start.Line,
			Column: testCase.Range.Start.Column,
		}},
	}
}
