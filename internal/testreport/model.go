package testreport

import (
	"encoding/json"

	"github.com/glade-sh/glade/internal/profile"
	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/typesys"
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

// Reason separates terminal causes without changing the legacy status enum.
type Reason string

const (
	ReasonTimeout            Reason = "timeout"
	ReasonCancelled          Reason = "cancelled"
	ReasonUnsupportedFeature Reason = "unsupported-feature"
	ReasonAssertion          Reason = "assertion"
	ReasonRuntimeError       Reason = "runtime-error"
)

type Run struct {
	RuntimeRESTAPIVersion string                   `json:"runtimeRestApiVersion,omitempty"`
	Name                  string                   `json:"name,omitempty"`
	DurationMS            int64                    `json:"durationMs,omitempty"`
	Dependencies          []typesys.DependencyInfo `json:"dependencies,omitempty"`
	Suites                []Suite                  `json:"suites"`
	// ExcludedClasses lists every class the dependency-scoped compile gate kept
	// from running: classes with compile errors and the classes that depend on
	// them. Their selected test methods also appear as compile_error cases.
	ExcludedClasses []CompileExclusion `json:"excludedClasses,omitempty"`
}

// CompileExclusion is one class excluded by the compile gate. Root names the
// class with its own compile error; for that class Root equals ClassName and
// DependsOn is empty. A dependent names the excluded class it references in
// DependsOn, and Message nests that class's message the way Salesforce does.
type CompileExclusion struct {
	ClassName string `json:"className"`
	File      string `json:"file,omitempty"`
	// Line is the line Salesforce reports. A dependent reports its root's
	// line, which is not a line of the dependent's own file.
	Line      int    `json:"line,omitempty"`
	Root      string `json:"root"`
	DependsOn string `json:"dependsOn,omitempty"`
	Message   string `json:"message"`
	// Problems lists every compile problem of a root class that has more than
	// one, in the order Salesforce lists them. Line and Message repeat the
	// first, which is the problem a dependent message nests.
	Problems []CompileProblem `json:"problems,omitempty"`
}

// CompileProblem is one compile problem of an excluded root class.
type CompileProblem struct {
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

type Suite struct {
	Name       string `json:"name"`
	DurationMS int64  `json:"durationMs,omitempty"`
	Cases      []Case `json:"cases"`
}

type Case struct {
	Name       string `json:"name,omitempty"`
	ClassName  string `json:"className,omitempty"`
	MethodName string `json:"methodName,omitempty"`
	SourceFile string `json:"sourceFile,omitempty"`
	// SelectedSourceFile identifies the planned occurrence, not an executed body.
	SelectedSourceFile string          `json:"selectedSourceFile,omitempty"`
	Status             Status          `json:"status"`
	Reason             Reason          `json:"reason,omitempty"`
	DurationMS         int64           `json:"durationMs,omitempty"`
	Problem            *Problem        `json:"problem,omitempty"`
	Trace              []trace.Event   `json:"trace,omitempty"`
	Profile            *profile.Report `json:"profile,omitempty"`
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
	// ExcludedClasses counts Run.ExcludedClasses. An excluded class fails the
	// run even when it has no selected test method.
	ExcludedClasses int `json:"excludedClasses,omitempty"`
}

// OK reports whether every selected test passed and no class was excluded
// by a compile error.
func (s Summary) OK() bool {
	return s.Failed == 0 && s.Errors == 0 && s.ExcludedClasses == 0
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
	summary.ExcludedClasses = len(r.ExcludedClasses)
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
		RuntimeRESTAPIVersion string                   `json:"runtimeRestApiVersion,omitempty"`
		Name                  string                   `json:"name,omitempty"`
		DurationMS            int64                    `json:"durationMs,omitempty"`
		Dependencies          []typesys.DependencyInfo `json:"dependencies,omitempty"`
		Summary               Summary                  `json:"summary"`
		Suites                []Suite                  `json:"suites"`
		ExcludedClasses       []CompileExclusion       `json:"excludedClasses,omitempty"`
	}
	suites := r.Suites
	if suites == nil {
		suites = []Suite{}
	}
	return json.Marshal(jsonRun{
		RuntimeRESTAPIVersion: r.RuntimeRESTAPIVersion,
		Name:                  r.Name,
		DurationMS:            r.DurationMS,
		Dependencies:          r.Dependencies,
		Summary:               r.Summary(),
		Suites:                suites,
		ExcludedClasses:       r.ExcludedClasses,
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
		Name       string `json:"name,omitempty"`
		ClassName  string `json:"className,omitempty"`
		MethodName string `json:"methodName,omitempty"`
		SourceFile string `json:"sourceFile,omitempty"`
		// SelectedSourceFile identifies the planned occurrence, not an executed body.
		SelectedSourceFile string          `json:"selectedSourceFile,omitempty"`
		Status             Status          `json:"status"`
		Reason             Reason          `json:"reason,omitempty"`
		DurationMS         int64           `json:"durationMs,omitempty"`
		Problem            *Problem        `json:"problem,omitempty"`
		Trace              []trace.Event   `json:"trace,omitempty"`
		Profile            *profile.Report `json:"profile,omitempty"`
	}
	return json.Marshal(jsonCase{
		Name:               c.Name,
		ClassName:          c.ClassName,
		MethodName:         c.MethodName,
		SourceFile:         c.SourceFile,
		SelectedSourceFile: c.SelectedSourceFile,
		Status:             normalizeStatus(c.Status),
		Reason:             c.Reason,
		DurationMS:         c.DurationMS,
		Problem:            c.Problem,
		Trace:              c.Trace,
		Profile:            c.Profile,
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
