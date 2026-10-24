package apextest

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// ImpactReport separates observed method execution from inputs consumed by
// compilation and shared initialization. SharedInputs apply to every test.
// Absence from a method trace is not proof that a changed input is irrelevant.
type ImpactReport struct {
	Version int `json:"version"`
	// False (including on empty reports) requires consumers to run the full
	// suite. Project file lists do not establish a complete runner read set.
	InputInventoryComplete bool         `json:"inputInventoryComplete"`
	SharedInputs           []string     `json:"sharedInputs"`
	Tests                  []TestImpact `json:"tests"`
}

type TestImpact struct {
	ClassName          string            `json:"className"`
	MethodName         string            `json:"methodName"`
	SourceFile         string            `json:"sourceFile,omitempty"`
	SelectedSourceFile string            `json:"selectedSourceFile,omitempty"`
	Status             testreport.Status `json:"status"`
	SourceFiles        []string          `json:"sourceFiles"`
	Classes            []string          `json:"classes"`
	Methods            []string          `json:"methods"`
	Operations         []string          `json:"operations"`
	FallbackReasons    []string          `json:"fallbackReasons,omitempty"`
}

// RunCasesWithImpactContext uses the same runner, selection, and isolation as
// RunCasesContext. Detailed events are reduced to sets and removed from the
// result unless the caller independently requested trace/profile output.
// snapshot is the project already loaded by the caller; it is never reloaded.
func RunCasesWithImpactContext(ctx context.Context, index typesys.Index, opts Options, cases []TestCase, snapshot *project.Project) (testreport.Run, ImpactReport) {
	traced := opts
	traced.TraceAll = true
	run := RunCasesContext(ctx, index, traced, cases)
	report := impactForRun(index, snapshot, &run, opts)
	return run, report
}

func impactForRun(index typesys.Index, snapshot *project.Project, run *testreport.Run, opts Options) ImpactReport {
	shared := impactSet{}
	for _, typ := range index.Types {
		if typ.File != "" {
			shared.add(impactPath(index.Project.Root, typ.File))
			shared.add(impactPath(index.Project.Root, typ.File+"-meta.xml"))
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.File != "" {
			shared.add(impactPath(index.Project.Root, trigger.File))
			shared.add(impactPath(index.Project.Root, trigger.File+"-meta.xml"))
		}
	}
	if snapshot != nil {
		impactProjectInputs(index.Project.Root, snapshot, shared, map[*project.Project]bool{})
	}
	// Inherited configuration, runtime data reads and negative dependencies
	// are not captured exhaustively. Never infer completeness from a non-nil
	// snapshot or a complete execution trace.
	report := ImpactReport{Version: 1, InputInventoryComplete: false, SharedInputs: shared.sorted(), Tests: []TestImpact{}}
	for si := range run.Suites {
		for ci := range run.Suites[si].Cases {
			row := &run.Suites[si].Cases[ci]
			files, classes, methods, operations, fallback := impactSet{}, impactSet{}, impactSet{}, impactSet{}, impactSet{}
			fallback.add("project-input-inventory-incomplete")
			files.add(impactPath(index.Project.Root, row.SourceFile))
			if row.SourceFile != "" {
				classes.add(row.ClassName)
			}
			for _, event := range row.Trace {
				arg := func(key string) string { value, _ := event.Args[key].(string); return value }
				files.add(impactPath(index.Project.Root, arg(trace.ArgFile)))
				classes.add(arg(trace.ArgClass))
				methods.add(arg(trace.ArgMethod))
				switch event.Category {
				case "apex.dml":
					if op := arg(trace.ArgOperation); op != "" {
						operations.add("dml:" + op)
					}
				case "apex.soql":
					operations.add("soql:query")
				case "apex.describe":
					operations.add("describe:" + arg(trace.ArgOperation))
				}
				if event.Name == "apex.trace.truncated" {
					fallback.add("trace-truncated")
				}
			}
			if row.Status != testreport.StatusPass && row.Status != testreport.StatusFail {
				fallback.add("test-not-completed")
			}
			if len(row.Trace) == 0 {
				fallback.add("execution-trace-unavailable")
			}
			if snapshot == nil {
				fallback.add("project-input-inventory-unavailable")
			}
			report.Tests = append(report.Tests, TestImpact{
				ClassName: row.ClassName, MethodName: row.MethodName,
				SourceFile: impactPath(index.Project.Root, row.SourceFile), Status: row.Status,
				SelectedSourceFile: impactPath(index.Project.Root, row.SelectedSourceFile),
				SourceFiles:        files.sorted(), Classes: classes.sorted(), Methods: methods.sorted(),
				Operations: operations.sorted(), FallbackReasons: fallback.sorted(),
			})
			if opts.TraceAll {
				continue
			}
			events := row.Trace
			row.Trace, row.Profile = nil, nil
			attachTraceProfile(row, vm.Result{Trace: events}, opts)
		}
	}
	sort.Slice(report.Tests, func(i, j int) bool {
		a, b := report.Tests[i], report.Tests[j]
		if a.ClassName != b.ClassName {
			return a.ClassName < b.ClassName
		}
		if a.MethodName != b.MethodName {
			return a.MethodName < b.MethodName
		}
		if a.SelectedSourceFile != b.SelectedSourceFile {
			return a.SelectedSourceFile < b.SelectedSourceFile
		}
		return a.SourceFile < b.SourceFile
	})
	return report
}

// Project's file lists are a partial inventory, including dependency roots.
// Reflect only []string file lists so new metadata kinds are included
// without inspecting arbitrary strings such as labels or configuration values.
func impactProjectInputs(root string, p *project.Project, inputs impactSet, seen map[*project.Project]bool) {
	if p == nil || seen[p] {
		return
	}
	seen[p] = true
	for _, name := range []string{"sfdx-project.json", "glade.yml", ".forceignore"} {
		inputs.add(impactPath(root, filepath.Join(p.Root, name)))
	}
	for _, path := range project.OrgShapeInputFiles(p.Root) {
		inputs.add(impactPath(root, path))
	}
	value := reflect.ValueOf(*p)
	for i := 0; i < value.NumField(); i++ {
		name := value.Type().Field(i).Name
		if !strings.HasSuffix(name, "Files") && !strings.HasSuffix(name, "Metas") {
			continue
		}
		if paths, ok := value.Field(i).Interface().([]string); ok {
			for _, path := range paths {
				inputs.add(impactPath(root, path))
			}
		}
	}
	inputs.add(impactPath(root, p.SchemaSnapshot))
	for _, dep := range p.ManagedPackageDependencies {
		inputs.add(impactPath(root, dep.ArtifactPath))
		impactProjectInputs(root, dep.Project, inputs, seen)
	}
	for _, shim := range p.PackageShims {
		impactProjectInputs(root, shim.Project, inputs, seen)
	}
}

func impactPath(root, file string) string {
	if strings.TrimSpace(file) == "" {
		return ""
	}
	if root != "" && filepath.IsAbs(file) {
		if relative, err := filepath.Rel(root, file); err == nil {
			file = relative
		}
	}
	return filepath.ToSlash(filepath.Clean(file))
}

type impactSet map[string]struct{}

func (s impactSet) add(value string) {
	if value != "" {
		s[value] = struct{}{}
	}
}
func (s impactSet) sorted() []string {
	values := make([]string, 0, len(s))
	for value := range s {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
