package testreport

import (
	"encoding/json"

	"github.com/open-aer/oaer/internal/profile"
	"github.com/open-aer/oaer/internal/trace"
	"github.com/open-aer/oaer/internal/typesys"
)

type Status string

const (
	StatusPass         Status = "pass"
	StatusFail         Status = "fail"
	StatusSkipped      Status = "skipped"
	StatusCompileError Status = "compile_error"
	StatusRuntimeError Status = "runtime_error"
	StatusUnsupported  Status = "unsupported"
)

type Run struct {
	Name         string                   `json:"name,omitempty"`
	DurationMS   int64                    `json:"durationMs,omitempty"`
	Dependencies []typesys.DependencyInfo `json:"dependencies,omitempty"`
	Suites       []Suite                  `json:"suites"`
}

type Suite struct {
	Name       string `json:"name"`
	DurationMS int64  `json:"durationMs,omitempty"`
	Cases      []Case `json:"cases"`
}

type Case struct {
	Name       string          `json:"name,omitempty"`
	ClassName  string          `json:"className,omitempty"`
	MethodName string          `json:"methodName,omitempty"`
	Status     Status          `json:"status"`
	DurationMS int64           `json:"durationMs,omitempty"`
	Problem    *Problem        `json:"problem,omitempty"`
	Trace      []trace.Event   `json:"trace,omitempty"`
	Profile    *profile.Report `json:"profile,omitempty"`
}

type Problem struct {
	Type    string       `json:"type,omitempty"`
	Message string       `json:"message"`
	Detail  string       `json:"detail,omitempty"`
	Stack   []StackFrame `json:"stack,omitempty"`
}

type StackFrame struct {
	Symbol string `json:"symbol,omitempty"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

type Summary struct {
	Total         int   `json:"total"`
	Passed        int   `json:"passed"`
	Failed        int   `json:"failed"`
	Skipped       int   `json:"skipped"`
	CompileErrors int   `json:"compileErrors"`
	RuntimeErrors int   `json:"runtimeErrors"`
	Unsupported   int   `json:"unsupported"`
	Errors        int   `json:"errors"`
	DurationMS    int64 `json:"durationMs"`
}

func (r Run) Summary() Summary {
	var summary Summary
	if r.DurationMS > 0 {
		summary.DurationMS = r.DurationMS
	}
	for _, suite := range r.Suites {
		suiteSummary := suite.Summary()
		summary.Total += suiteSummary.Total
		summary.Passed += suiteSummary.Passed
		summary.Failed += suiteSummary.Failed
		summary.Skipped += suiteSummary.Skipped
		summary.CompileErrors += suiteSummary.CompileErrors
		summary.RuntimeErrors += suiteSummary.RuntimeErrors
		summary.Unsupported += suiteSummary.Unsupported
		summary.Errors += suiteSummary.Errors
		if r.DurationMS == 0 {
			summary.DurationMS += suiteSummary.DurationMS
		}
	}
	return summary
}

func (s Suite) Summary() Summary {
	var summary Summary
	if s.DurationMS > 0 {
		summary.DurationMS = s.DurationMS
	}
	for _, testCase := range s.Cases {
		summary.Total++
		switch normalizeStatus(testCase.Status) {
		case StatusPass:
			summary.Passed++
		case StatusFail:
			summary.Failed++
		case StatusSkipped:
			summary.Skipped++
		case StatusCompileError:
			summary.CompileErrors++
			summary.Errors++
		case StatusRuntimeError:
			summary.RuntimeErrors++
			summary.Errors++
		case StatusUnsupported:
			summary.Unsupported++
			summary.Errors++
		default:
			summary.RuntimeErrors++
			summary.Errors++
		}
		if s.DurationMS == 0 {
			summary.DurationMS += testCase.DurationMS
		}
	}
	return summary
}

func (r Run) MarshalJSON() ([]byte, error) {
	type jsonRun struct {
		Name         string                   `json:"name,omitempty"`
		DurationMS   int64                    `json:"durationMs,omitempty"`
		Dependencies []typesys.DependencyInfo `json:"dependencies,omitempty"`
		Summary      Summary                  `json:"summary"`
		Suites       []Suite                  `json:"suites"`
	}
	suites := r.Suites
	if suites == nil {
		suites = []Suite{}
	}
	return json.Marshal(jsonRun{
		Name:         r.Name,
		DurationMS:   r.DurationMS,
		Dependencies: r.Dependencies,
		Summary:      r.Summary(),
		Suites:       suites,
	})
}

func (s Suite) MarshalJSON() ([]byte, error) {
	type jsonSuite struct {
		Name       string `json:"name"`
		DurationMS int64  `json:"durationMs,omitempty"`
		Cases      []Case `json:"cases"`
	}
	cases := s.Cases
	if cases == nil {
		cases = []Case{}
	}
	return json.Marshal(jsonSuite{
		Name:       s.Name,
		DurationMS: s.DurationMS,
		Cases:      cases,
	})
}

func (c Case) MarshalJSON() ([]byte, error) {
	type jsonCase struct {
		Name       string          `json:"name,omitempty"`
		ClassName  string          `json:"className,omitempty"`
		MethodName string          `json:"methodName,omitempty"`
		Status     Status          `json:"status"`
		DurationMS int64           `json:"durationMs,omitempty"`
		Problem    *Problem        `json:"problem,omitempty"`
		Trace      []trace.Event   `json:"trace,omitempty"`
		Profile    *profile.Report `json:"profile,omitempty"`
	}
	return json.Marshal(jsonCase{
		Name:       c.Name,
		ClassName:  c.ClassName,
		MethodName: c.MethodName,
		Status:     normalizeStatus(c.Status),
		DurationMS: c.DurationMS,
		Problem:    c.Problem,
		Trace:      c.Trace,
		Profile:    c.Profile,
	})
}

func normalizeStatus(status Status) Status {
	if status == "" {
		return StatusPass
	}
	return status
}

func (c Case) displayName(fallbackSuite string) string {
	if c.Name != "" {
		return c.Name
	}
	if c.ClassName != "" && c.MethodName != "" {
		return c.ClassName + "." + c.MethodName
	}
	if c.MethodName != "" {
		return c.MethodName
	}
	if c.ClassName != "" {
		return c.ClassName
	}
	return fallbackSuite
}

func (c Case) junitClassName(fallbackSuite string) string {
	if c.ClassName != "" {
		return c.ClassName
	}
	return fallbackSuite
}

func (c Case) junitName(fallbackSuite string) string {
	if c.MethodName != "" {
		return c.MethodName
	}
	if c.Name != "" {
		return c.Name
	}
	if c.ClassName != "" {
		return c.ClassName
	}
	return fallbackSuite
}
