package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// unloadedDependencyProblems builds the compile-error shapes explainDependencyRun
// sees for a consumer whose declared package is not loaded: several excluded
// helpers and failing cases that share the same files.
func unloadedDependencyProblems(t *testing.T, root, suffix string, repeats int) []*vm.RuntimeError {
	t.Helper()
	var problems []*vm.RuntimeError
	for i := 0; i < repeats; i++ {
		for _, letter := range "AB" {
			file := filepath.Join(root, "force-app/main/default/classes", fmt.Sprintf("CedarHelper%c%s.cls", letter, suffix))
			problems = append(problems, &vm.RuntimeError{
				Type:    "CompileError",
				Message: fmt.Sprintf(`method "apply" calls unknown method "CedarLibrary%c%s.value"`, letter, suffix),
				Stack:   []vm.StackFrame{{File: file}},
			})
		}
	}
	return problems
}

// One explainer per run answers exactly as a fresh ExplainDependencyError per
// problem, for the excluded helpers and the cases that repeat their files.
func TestDependencyExplainerMatchesFreshExplanations(t *testing.T) {
	root, suffix := multiHelperProject(t, "62.0", 2, false)
	index := loadTestIndex(t, root)
	explainer := newDependencyExplainer(index)
	for _, problem := range unloadedDependencyProblems(t, root, suffix, 3) {
		want := ExplainDependencyError(index, "", problem)
		got := explainer.explain("", problem)
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("explanation differs for %s:\n fresh: %#v\n memo:  %#v", problem.Message, want, got)
		}
		if explained, ok := got.(*vm.RuntimeError); !ok || explained.Type != "dependency_missing" {
			t.Fatalf("problem was not explained: %#v", got)
		}
	}
	// Unknown files, a flow action and an error outside the explained types
	// keep their answers too.
	for _, problem := range []error{
		&vm.RuntimeError{Type: "CompileError", Message: `method "apply" calls unknown method "CedarLibraryA` + suffix + `.value"`, Stack: []vm.StackFrame{{File: filepath.Join(root, "missing.cls")}}},
		&vm.RuntimeError{Type: "NullPointerException", Message: "Attempt to de-reference a null object"},
		fmt.Errorf("Invalid type: CedarLibraryA%s", suffix),
	} {
		want := ExplainDependencyError(index, "", problem)
		got := explainer.explain("", problem)
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("explanation differs for %v:\n fresh: %#v\n memo:  %#v", problem, want, got)
		}
	}
	// A changed text for a remembered file is answered afresh, not from memory.
	helper := filepath.Join(root, "force-app/main/default/classes", "CedarHelperA"+suffix+".cls")
	rootName := "CedarLibraryA" + suffix
	targets := compileGateUnits{fileOf: map[string]string{"cedarlibrarya" + suffix: rootName}, types: explainer.compileGateUnits().types, namespace: explainer.compileGateUnits().namespace}
	if answer := explainer.referenceAnswer(helper, "public class CedarHelperA"+suffix+" {}", rootName, targets); answer.referenced {
		t.Fatal("rewritten text without the reference was answered from the remembered text")
	}
}

// explainDependencyRun rewrites the report the same way with one explainer as
// with a fresh explanation per entry.
func TestExplainDependencyRunMatchesPerEntryExplanations(t *testing.T) {
	root, suffix := multiHelperProject(t, "67.0", 2, false)
	index := loadTestIndex(t, root)
	build := func() testreport.Run {
		var run testreport.Run
		for _, letter := range "AB" {
			name := fmt.Sprintf("CedarHelper%c%s", letter, suffix)
			file := filepath.Join(root, "force-app/main/default/classes", name+".cls")
			run.ExcludedClasses = append(run.ExcludedClasses, testreport.CompileExclusion{ClassName: name, Root: name, File: file, Message: fmt.Sprintf(`method "apply" calls unknown method "CedarLibrary%c%s.value"`, letter, suffix)})
		}
		suite := testreport.Suite{Name: "CedarMultiObserve"}
		for i := 0; i < 4; i++ {
			letter := "AB"[i%2]
			suite.Cases = append(suite.Cases, testreport.Case{
				ClassName: fmt.Sprintf("CedarHelper%c%s", letter, suffix), MethodName: fmt.Sprintf("observe%d", i), Status: testreport.StatusCompileError,
				Problem: &testreport.Problem{Type: "CompileError", Message: fmt.Sprintf(`method "apply" calls unknown method "CedarLibrary%c%s.value"`, letter, suffix), Stack: []testreport.StackFrame{{File: filepath.Join(root, "force-app/main/default/classes", fmt.Sprintf("CedarHelper%c%s.cls", letter, suffix))}}},
			})
		}
		run.Suites = []testreport.Suite{suite}
		return run
	}
	got := build()
	explainDependencyRun(index, &got)
	want := build()
	referenceExplainDependencyRun(index, &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runs differ:\n memo:  %+v\n fresh: %+v", got, want)
	}
	explained := 0
	for _, entry := range got.ExcludedClasses {
		if len(entry.Message) > len("dependency_missing: ") && entry.Message[:len("dependency_missing: ")] == "dependency_missing: " {
			explained++
		}
	}
	for _, c := range got.Suites[0].Cases {
		if c.Problem.Type == "dependency_missing" {
			explained++
		}
	}
	if explained != 6 {
		t.Fatalf("explained %d of 6 entries", explained)
	}
}

// referenceExplainDependencyRun is the per-entry form explainDependencyRun
// replaced: every excluded class and failing case explained afresh.
func referenceExplainDependencyRun(index typesys.Index, run *testreport.Run) {
	if len(missingDependencyNames(index)) == 0 {
		return
	}
	for i := range run.ExcludedClasses {
		entry := &run.ExcludedClasses[i]
		problem := &testreport.Problem{Type: "CompileError", Message: entry.Message, Stack: []testreport.StackFrame{{File: entry.File}}}
		referenceExplainProblem(index, problem, run.ExcludedClasses, entry.Root)
		if problem.Type == "dependency_missing" {
			entry.Message = problem.Message
		}
	}
	for suiteIndex := range run.Suites {
		for caseIndex := range run.Suites[suiteIndex].Cases {
			result := &run.Suites[suiteIndex].Cases[caseIndex]
			if result.Status == testreport.StatusPass || result.Status == testreport.StatusSkipped {
				continue
			}
			var rootName string
			for _, entry := range run.ExcludedClasses {
				if strings.EqualFold(entry.ClassName, result.ClassName) {
					rootName = entry.Root
				}
			}
			referenceExplainProblem(index, result.Problem, run.ExcludedClasses, rootName)
			if result.Problem != nil && result.Problem.Type == "dependency_missing" {
				result.Reason = dependencyMissingReason
				if result.Status != testreport.StatusCompileError {
					result.Status = testreport.StatusRuntimeError
				}
			}
		}
	}
}

func referenceExplainProblem(index typesys.Index, problem *testreport.Problem, excluded []testreport.CompileExclusion, rootName string) {
	if problem == nil {
		return
	}
	var root testreport.CompileExclusion
	for _, entry := range excluded {
		if strings.EqualFold(entry.ClassName, rootName) {
			root = entry
		}
	}
	original := &vm.RuntimeError{Type: problem.Type, Message: problem.Message}
	for _, frame := range problem.Stack {
		original.Stack = append(original.Stack, vm.StackFrame{Symbol: frame.Symbol, File: frame.File, Line: frame.Line, Column: frame.Column})
	}
	if root.File != "" {
		original.Stack = append(original.Stack, vm.StackFrame{File: root.File})
		for _, detail := range root.Problems {
			if !strings.Contains(original.Message, detail.Message) {
				original.Message += "\n" + detail.Message
			}
		}
	}
	if err := ExplainDependencyError(index, "", original); err != original {
		if explained, ok := err.(*vm.RuntimeError); ok && explained.Type == "dependency_missing" {
			if problem.Detail == "" {
				problem.Detail = problem.Type + ": " + problem.Message
			}
			problem.Type, problem.Message = explained.Type, "dependency_missing: "+explained.Message
		}
	}
}

// Explaining fifty problems that share two files costs one explainer two
// parses; a fresh explanation per problem parses every time.
func TestDependencyExplainerParsesSharedFilesOnce(t *testing.T) {
	root, suffix := multiHelperProject(t, "62.0", 2, false)
	// Give the helpers the size of an ordinary project class so the parse is
	// the cost that matters, as it is for a real project.
	for _, letter := range "AB" {
		path := filepath.Join(root, "force-app/main/default/classes", fmt.Sprintf("CedarHelper%c%s.cls", letter, suffix))
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.TrimRight(strings.TrimSpace(string(body)), "}")
		for i := 0; i < 120; i++ {
			text += fmt.Sprintf("\n    public static Integer padding%d(Integer value) { Integer total = value; for (Integer i = 0; i < %d; i++) { total += i * %d; } return total; }", i, i+1, i+2)
		}
		text += "\n}\n"
		writeFile(t, path, text)
	}
	index := loadTestIndex(t, root)
	problems := unloadedDependencyProblems(t, root, suffix, 25)
	check := func(explain func(error) error) {
		for _, problem := range problems {
			if explained, ok := explain(problem).(*vm.RuntimeError); !ok || explained.Type != "dependency_missing" {
				t.Fatalf("not explained: %#v", problem)
			}
		}
	}
	fresh := testing.AllocsPerRun(3, func() {
		check(func(err error) error { return ExplainDependencyError(index, "", err) })
	})
	memo := testing.AllocsPerRun(3, func() {
		explainer := newDependencyExplainer(index)
		check(func(err error) error { return explainer.explain("", err) })
	})
	t.Logf("allocations for %d problems over 2 files: fresh explanations %.0f, one explainer %.0f", len(problems), fresh, memo)
	if memo*3 > fresh {
		t.Fatalf("one explainer allocated %.0f against %.0f for fresh explanations; shared files must be parsed once", memo, fresh)
	}
}
