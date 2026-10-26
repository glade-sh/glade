package apextest

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The dependency-scoped compile gate (roadmap doc-7 D40). A class with a
// compile error and every class that references it, transitively, are
// reported as compile errors; every other selected test runs. Salesforce
// reports such a dependent as
//
//	Dependent class is invalid and needs recompilation:
//	 Class <dependency> : <the dependency's own problem>
//
// and names the referenced class, not the root, so a transitive dependent's
// message nests its dependency's message (GATE rows G02-G06, GATE2-13B). A
// dependent reports its root's line (GATE2-04). A class with errors of its
// own reports only those, even when it also references, implements or
// extends an invalid class (GATE3-02, 02R, 03, GATE4-01, 01R, 01D, 02O). A
// class whose only errors are missing implementations an excluded supertype
// requires is a dependent of its nearest excluded supertype (GATE2-09,
// GATE4-02).
// Type.forName and string literals are not references (G08, G12), and using
// a valid interface does not depend on an invalid implementer (G09). The
// excluded classes are listed by name (G05, GATE2-13B).
//
// A Salesforce source file is one compile unit: every top-level class in a
// file with an error is invalid. Schema-reference errors are attributed to
// static users of their structured object identity. The gate falls back to the
// whole-project gate when an error has no attributable owner, or it is a class
// metadata error such as an unsupported apiVersion (Salesforce then reports
// only the -meta.xml file and compiles nothing, GATE2-14F). Invalid triggers
// and triggers referencing excluded classes are compile exclusions too.
const compileGateDependentPrefix = "Dependent class is invalid and needs recompilation:\n Class "

// compileGate carries exclusions across the reruns of one test invocation.
type compileGate struct {
	started       time.Time
	originalErr   error
	originalCases []TestCase
	// dependencyProject retains the structured resolver records across reruns.
	dependencySources *sourceCache
	dependencyIndex   *typesys.Index
	dependencyProject *project.Project
	dependencyErrors  []diagnostic.Diagnostic
	// projectErrors have structured schema owners but no referencing Apex unit.
	// They fail the report while unrelated tests still execute.
	projectErrors []diagnostic.Diagnostic
	// diagnostics holds the error diagnostics of every level, so a fallback
	// reports the error that caused it beside the earlier levels' errors.
	diagnostics   []diagnostic.Diagnostic
	excluded      []testreport.CompileExclusion
	excludedFiles map[string]bool
	// fileEntry maps an excluded file to its first exclusion entry.
	fileEntry map[string]int
	// classEntry maps a lowercased excluded class name to its entry.
	classEntry map[string]int
	// rootProblems holds the formatted diagnostics of each invalid root file.
	rootProblems map[string]string
	// depth counts the project-subset reruns; each level keeps its own
	// semantic cache slot.
	depth    int
	fellBack bool
}

func (g *compileGate) cacheSlot() string {
	if g == nil {
		return ""
	}
	return fmt.Sprintf("compile-gate-%d", g.depth)
}

// compileGateUnits indexes the project's source-backed classes: top-level
// classes by file (compile unit) and by lowercased name, and every type by
// lowercased full name for name binding.
type compileGateUnits struct {
	order           []string
	classes         map[string][]string
	fileOf          map[string]string
	types           map[string]typesys.TypeSymbol
	namespace       string
	objects         map[string]schema.Object
	features        []string
	querySchema     *compileGateQuerySchema
	schemaObjects   []schema.Object
	expressionType  vm.ExpressionTypeResolver
	triggerContexts map[string]compileGateTriggerContext
}

func newCompileGateUnits(index typesys.Index) compileGateUnits {
	units := compileGateUnits{
		classes:       make(map[string][]string),
		fileOf:        make(map[string]string),
		types:         make(map[string]typesys.TypeSymbol),
		namespace:     index.Project.Namespace,
		objects:       make(map[string]schema.Object, len(index.Objects)),
		features:      index.OrgShapeFeatures,
		querySchema:   &compileGateQuerySchema{},
		schemaObjects: index.Objects,
	}
	for _, object := range index.Objects {
		units.objects[strings.ToLower(object.Name)] = object
	}
	for _, typ := range index.Types {
		if !compileGateUnitType(typ) {
			continue
		}
		if key := compileGateTypeKey(typ); key != "" {
			if _, ok := units.types[key]; !ok {
				units.types[key] = typ
			}
		}
		if typ.OwnerName != "" {
			continue
		}
		file := filepath.Clean(typ.File)
		if _, ok := units.classes[file]; !ok {
			units.order = append(units.order, file)
		}
		units.classes[file] = append(units.classes[file], typ.Name)
		lower := strings.ToLower(typ.Name)
		if _, ok := units.fileOf[lower]; !ok {
			units.fileOf[lower] = file
		}
	}
	compileGateAddTriggerUnits(&units, index)
	// Trigger bodies need an expression-typing owner too. These private model
	// symbols never enter the published index or the dependency graph.
	expressionIndex := index
	expressionIndex.Types = append([]typesys.TypeSymbol(nil), index.Types...)
	units.triggerContexts = make(map[string]compileGateTriggerContext)
	for i, trigger := range index.Triggers {
		owner := fmt.Sprintf("__GladeCompileGateTrigger%d", i)
		for units.types[strings.ToLower(owner)].Name != "" {
			owner += "_"
		}
		expressionIndex.Types = append(expressionIndex.Types, typesys.TypeSymbol{Kind: apexast.DeclarationClass, Name: owner, EffectiveAPIVersion: trigger.EffectiveAPIVersion})
		units.triggerContexts[filepath.Clean(trigger.File)] = compileGateTriggerContext{owner: owner, object: trigger.ObjectName}
	}
	units.expressionType = sema.RuntimeExpressionTypeResolver(expressionIndex)
	return units
}

func compileGateUnitType(typ typesys.TypeSymbol) bool {
	return !typ.Dependency && !typ.Artifact && typ.File != "" && typ.HasSourceSnapshot()
}

// compileGateMetadataError reports a class metadata error. Salesforce
// rejects the deploy at the -meta.xml file before compiling any class, so no
// class is a dependent of it (GATE2-14F).
func compileGateMetadataError(diag diagnostic.Diagnostic) bool {
	return strings.HasSuffix(strings.ToLower(diag.File), "-meta.xml") || diag.Code == "GLADESEMA_VERSION"
}

// attribute groups error diagnostics by compile unit. It reports false when
// any error belongs to no project class file or is a metadata error.
func (units compileGateUnits) attribute(diagnostics []diagnostic.Diagnostic) (map[string][]diagnostic.Diagnostic, bool) {
	byFile := make(map[string][]diagnostic.Diagnostic)
	for _, diag := range diagnostics {
		if diag.Severity != diagnostic.Error {
			continue
		}
		if diag.File == "" || compileGateMetadataError(diag) {
			return nil, false
		}
		file := filepath.Clean(diag.File)
		if _, ok := units.classes[file]; !ok {
			return nil, false
		}
		byFile[file] = append(byFile[file], diag)
	}
	return byFile, true
}

// runWithCompileGate excludes the invalid classes in diagnostics and their
// dependents, reruns the remaining selected tests on the project without
// them, and merges compile-error results for the excluded tests.
func runWithCompileGate(ctx context.Context, index typesys.Index, opts Options, cases []TestCase, hooks semanticGateHooks, runState *runExecution, gate *compileGate, sources *sourceCache, diagnostics []diagnostic.Diagnostic, compileErr error, started time.Time) testreport.Run {
	outermost := gate == nil
	if outermost {
		gate = &compileGate{
			dependencyProject: compileGateLoadDependencyProject(index.Project.Root, diagnostics),
			started:           started,
			originalErr:       compileErr,
			originalCases:     cases,
			excludedFiles:     make(map[string]bool),
			fileEntry:         make(map[string]int),
			classEntry:        make(map[string]int),
			rootProblems:      make(map[string]string),
		}
	}
	if outermost && len(missingDependencyNames(index)) > 0 {
		original := index
		gate.dependencyIndex = &original
		gate.dependencySources = sources
	}
	for _, diag := range diagnostics {
		if diag.Severity == diagnostic.Error {
			gate.diagnostics = append(gate.diagnostics, diag)
		}
	}
	units := newCompileGateUnits(index)
	byFile, ok := gate.attributeDependencyDiagnostics(units, diagnostics, sources)
	if ok {
		ok = gate.exclude(index, units, byFile, sources)
	}
	if !ok {
		gate.fellBack = true
		fallbackErr := gate.originalErr
		if !outermost {
			fallbackErr = indexCompileError(gate.diagnostics)
		}
		return compileErrorRun(gate.originalCases, fallbackErr, gate.started, opts)
	}
	if outermost {
		reportProgress(opts, TestProgress{Event: "compile_done", DurationMS: time.Since(started).Milliseconds(), Status: "fail"})
	}
	// excludedHere marks this level's excluded cases. A nested rerun that
	// excludes more classes reports those cases itself.
	excludedHere := make([]bool, len(cases))
	runnable := make([]TestCase, 0, len(cases))
	for i, testCase := range cases {
		excludedHere[i] = gate.excludedFiles[filepath.Clean(testCase.File)]
		if !excludedHere[i] {
			runnable = append(runnable, testCase)
		}
	}
	var nested testreport.Run
	if len(runnable) > 0 {
		gate.depth++
		nested = runCasesGated(ctx, filterCompileGateIndex(index, gate.excludedFiles), opts, runnable, hooks, runState, gate)
		if gate.fellBack {
			return nested
		}
	}
	run := gate.merge(cases, excludedHere, nested)
	for i, testCase := range cases {
		if excludedHere[i] {
			reportProgress(opts, TestProgress{Event: "test_done", ClassName: testCase.ClassName, MethodName: testCase.MethodName, Status: string(testreport.StatusCompileError)})
		}
	}
	if outermost {
		run.ExcludedClasses = gate.sortedExclusions()
	}
	run.DurationMS = time.Since(started).Milliseconds()
	return run
}

// compileGateNestedProgress drops the compile phase events of a project
// subset rerun: the outermost run reports one compile phase, which failed.
func compileGateNestedProgress(progress func(TestProgress)) func(TestProgress) {
	return func(event TestProgress) {
		if event.Event != "compile_start" && event.Event != "compile_done" {
			progress(event)
		}
	}
}

// exclude adds the invalid files in byFile and the transitive closure of
// their dependents, including trigger compile units. It reports false when
// a source cannot be read.
func (g *compileGate) exclude(index typesys.Index, units compileGateUnits, byFile map[string][]diagnostic.Diagnostic, sources *sourceCache) bool {
	// mentions maps a lowercased class name to the other files whose tokens
	// name it. Only those files are parsed to bind their references.
	mentions := make(map[string][]string)
	for _, file := range units.order {
		source, err := sources.read(file)
		if err != nil {
			return false
		}
		for name := range compileGateMentions(source, units.fileOf) {
			if units.fileOf[name] != file {
				mentions[name] = append(mentions[name], file)
			}
		}
	}
	bound := make(map[string]map[string]bool)
	references := func(file string) (map[string]bool, bool) {
		if refs, ok := bound[file]; ok {
			return refs, true
		}
		source, err := sources.read(file)
		if err != nil {
			return nil, false
		}
		refs := make(map[string]bool)
		for _, target := range compileGateReferences(file, source, units) {
			refs[target] = true
		}
		bound[file] = refs
		return refs, true
	}
	invalid := make(map[string]bool)
	for file, diags := range byFile {
		invalid[file] = len(diags) > 0
	}
	// Bind the closure of files that reach an invalid file. dependents maps a
	// lowercased class name to the files that reference it, in file order.
	dependents := make(map[string][]string)
	closure := make(map[string]bool)
	var frontier []string
	for _, file := range units.order {
		if invalid[file] {
			closure[file] = true
			for _, className := range units.classes[file] {
				frontier = append(frontier, strings.ToLower(className))
			}
		}
	}
	for len(frontier) > 0 {
		var next []string
		for _, target := range frontier {
			for _, file := range mentions[target] {
				refs, ok := references(file)
				if !ok {
					return false
				}
				if !refs[target] {
					continue
				}
				dependents[target] = append(dependents[target], file)
				if !closure[file] {
					closure[file] = true
					for _, className := range units.classes[file] {
						next = append(next, strings.ToLower(className))
					}
				}
			}
		}
		frontier = next
	}
	own := compileGateOwnErrors(units, byFile, closure)
	addRoot := func(file string) {
		diags := own[file]
		if len(diags) == 0 {
			diags = byFile[file]
		}
		g.excludedFiles[file] = true
		g.rootProblems[file] = indexCompileError(diags).Error()
		problems := compileGateProblems(diags)
		for _, className := range units.classes[file] {
			entry := testreport.CompileExclusion{ClassName: className, File: file, Line: problems[0].Line, Root: className, Message: problems[0].Message}
			if len(problems) > 1 {
				entry.Problems = problems
			}
			g.add(entry)
			frontier = append(frontier, strings.ToLower(className))
		}
	}
	// spread excludes the dependents of the frontier, nearest first, so each
	// dependent names a class on a shortest path to its root.
	spread := func() {
		for len(frontier) > 0 {
			var next []string
			for _, target := range frontier {
				dependency := g.excluded[g.classEntry[target]]
				for _, file := range dependents[target] {
					if g.excludedFiles[file] {
						continue
					}
					g.excludedFiles[file] = true
					for _, className := range units.classes[file] {
						g.add(testreport.CompileExclusion{
							ClassName: className,
							File:      file,
							Line:      dependency.Line,
							Root:      dependency.Root,
							DependsOn: dependency.ClassName,
							Message:   compileGateTriggerDependencyMessage(index, file, dependency, sources),
						})
						next = append(next, strings.ToLower(className))
					}
				}
			}
			frontier = next
		}
	}
	for _, file := range units.order {
		if invalid[file] && len(own[file]) > 0 {
			addRoot(file)
		}
	}
	spread()
	// A file whose every error is supertype-derived references its excluded
	// supertype, so the walk above excludes it. Keep its errors if it did not.
	for _, file := range units.order {
		if invalid[file] && !g.excludedFiles[file] {
			addRoot(file)
			spread()
		}
	}

	return true
}

// compileGateOwnErrors returns each invalid file's own errors: every error
// except a missing implementation that an excluded supertype requires, an
// interface or superclass that a type of the file implements or extends,
// directly or through project supertypes, and that is invalid or depends on
// an invalid class. The diagnostic names that supertype in RequiredBy, so
// attribution never reads message text (a local or field spelled like the
// supertype stays an own error, GATE4-03, 03F).
//
// A file with no own error follows from its supertype, and Salesforce reports
// it as a dependent naming the nearest excluded supertype (GATE2-09, GATE4-02).
// A file with an own error reports only its own errors, even when it also
// implements an invalid interface or extends an invalid class (GATE4-01, 01R,
// 01D, 02O), calls an invalid class (GATE3-02, 02R) or sits in a reference
// cycle with one (GATE3-03).
func compileGateOwnErrors(units compileGateUnits, byFile map[string][]diagnostic.Diagnostic, closure map[string]bool) map[string][]diagnostic.Diagnostic {
	out := make(map[string][]diagnostic.Diagnostic, len(byFile))
	for file, diags := range byFile {
		excluded := make(map[string]bool)
		for _, name := range units.supertypes(file) {
			if closure[units.fileOf[name]] {
				excluded[name] = true
			}
		}
		for _, diag := range diags {
			if diag.Code == "GLADESEMA017" && excluded[units.topLevelClass(diag.RequiredBy)] {
				continue
			}
			out[file] = append(out[file], diag)
		}
	}
	return out
}

// topLevelClass resolves a project type name, top-level or inner, to its
// lowercased top-level class. It returns "" outside the project.
func (units compileGateUnits) topLevelClass(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if ns := strings.ToLower(units.namespace); ns != "" {
		name = strings.TrimPrefix(name, ns+".")
	}
	if _, ok := units.types[name]; !ok {
		return ""
	}
	top, _, _ := strings.Cut(name, ".")
	if _, ok := units.fileOf[top]; !ok {
		return ""
	}
	return top
}

// supertypes returns the lowercased project top-level classes that the types
// of file extend or implement, directly or through project supertypes.
func (units compileGateUnits) supertypes(file string) []string {
	var queue []string
	seen := make(map[string]bool)
	push := func(typ typesys.TypeSymbol) {
		for _, declared := range append([]string{typ.SuperClass}, typ.Interfaces...) {
			name := strings.ToLower(strings.TrimSpace(declared))
			if ns := strings.ToLower(units.namespace); ns != "" {
				name = strings.TrimPrefix(name, ns+".")
			}
			name, _, _ = strings.Cut(name, "<")
			name, _, _ = strings.Cut(name, ".")
			if target, ok := units.fileOf[name]; ok && target != file && !seen[name] {
				seen[name] = true
				queue = append(queue, name)
			}
		}
	}
	for _, typ := range units.types {
		if filepath.Clean(typ.File) == file {
			push(typ)
		}
	}
	for i := 0; i < len(queue); i++ {
		if typ, ok := units.types[queue[i]]; ok {
			push(typ)
		}
	}
	return queue
}

// compileGateProblems returns a file's compile problems in diagnostic order,
// which is the order Salesforce lists them: declarations before bodies, then
// source order (GATE2-05, 05S, 05P).
func compileGateProblems(diags []diagnostic.Diagnostic) []testreport.CompileProblem {
	problems := make([]testreport.CompileProblem, 0, len(diags))
	for _, diag := range diags {
		message := diag.NativeMessage
		if message == "" {
			message = diag.Message
		}
		line := 0
		if diag.NativeLine != nil {
			line = *diag.NativeLine
		} else if diag.Range != nil {
			line = diag.Range.Start.Line
		}
		problems = append(problems, testreport.CompileProblem{Line: line, Message: message})
	}
	return problems
}

func (g *compileGate) add(entry testreport.CompileExclusion) {
	index := len(g.excluded)
	g.excluded = append(g.excluded, entry)
	if _, ok := g.fileEntry[entry.File]; !ok {
		g.fileEntry[entry.File] = index
	}
	lower := strings.ToLower(entry.ClassName)
	if _, ok := g.classEntry[lower]; !ok {
		g.classEntry[lower] = index
	}
}

// sortedExclusions lists the excluded classes by name, the order in which
// Salesforce lists component failures (G05, GATE2-13B).
func (g *compileGate) sortedExclusions() []testreport.CompileExclusion {
	out := append([]testreport.CompileExclusion(nil), g.excluded...)
	sort.SliceStable(out, func(i, j int) bool {
		left, right := strings.ToLower(out[i].ClassName), strings.ToLower(out[j].ClassName)
		if left != right {
			return left < right
		}
		return out[i].ClassName < out[j].ClassName
	})
	return out
}

// caseResult reports one selected test of an excluded class. A test in an
// invalid root file keeps that file's formatted diagnostics; a dependent test
// carries the Salesforce dependent message and the root's diagnostics.
func (g *compileGate) caseResult(testCase TestCase) testreport.Case {
	file := filepath.Clean(testCase.File)
	entry := g.excluded[g.fileEntry[file]]
	message, detail := g.rootProblems[file], ""
	if message == "" {
		message = entry.Message
		if root, ok := g.classEntry[strings.ToLower(entry.Root)]; ok {
			detail = g.rootProblems[g.excluded[root].File]
		}
	}
	result := testreport.Case{
		ClassName:          testCase.ClassName,
		MethodName:         testCase.MethodName,
		SelectedSourceFile: testCase.File,
		Status:             testreport.StatusCompileError,
		Problem:            problem("CompileError", message, testCase),
	}
	result.Problem.Detail = detail
	return result
}

// merge rebuilds the suites in selection order: excluded tests become
// compile errors and every other test keeps its result from nested.
func (g *compileGate) merge(cases []TestCase, excluded []bool, nested testreport.Run) testreport.Run {
	run := nested
	run.Name = "glade test"
	run.Suites = nil
	pending := make(map[string][]testreport.Case)
	var nestedOrder []string
	for _, suite := range nested.Suites {
		if _, ok := pending[suite.Name]; !ok {
			nestedOrder = append(nestedOrder, suite.Name)
		}
		pending[suite.Name] = append(pending[suite.Name], suite.Cases...)
	}
	suiteIndex := make(map[string]int)
	for i, testCase := range cases {
		var result testreport.Case
		if excluded[i] {
			result = g.caseResult(testCase)
		} else if queued := pending[testCase.ClassName]; len(queued) > 0 {
			result, pending[testCase.ClassName] = queued[0], queued[1:]
		} else {
			continue
		}
		index, ok := suiteIndex[testCase.ClassName]
		if !ok {
			index = len(run.Suites)
			suiteIndex[testCase.ClassName] = index
			run.Suites = append(run.Suites, testreport.Suite{Name: testCase.ClassName})
		}
		run.Suites[index].Cases = append(run.Suites[index].Cases, result)
	}
	for _, name := range nestedOrder {
		if len(pending[name]) > 0 {
			run.Suites = append(run.Suites, testreport.Suite{Name: name, Cases: pending[name]})
		}
	}
	if len(g.projectErrors) > 0 {
		projectRun := compileErrorRun(nil, indexCompileError(g.projectErrors), g.started, Options{})
		run.Suites = append(run.Suites, projectRun.Suites...)
		// The shared gate merges from the deepest subset outward. Its project
		// result travels with nested suites and must be reported only once.
		g.projectErrors = nil
	}
	return run
}

// filterCompileGateIndex returns index without the excluded files' classes
// and their index diagnostics.
func filterCompileGateIndex(index typesys.Index, excluded map[string]bool) typesys.Index {
	types := make([]typesys.TypeSymbol, 0, len(index.Types))
	for _, typ := range index.Types {
		if compileGateUnitType(typ) && excluded[filepath.Clean(typ.File)] {
			continue
		}
		types = append(types, typ)
	}
	index.Types = types
	index.Triggers = compileGateFilterTriggers(index.Triggers, excluded)
	var diagnostics []diagnostic.Diagnostic
	for _, diag := range index.Diagnostics {
		if diag.File != "" && excluded[filepath.Clean(strings.TrimSuffix(diag.File, "-meta.xml"))] {
			continue
		}
		diagnostics = append(diagnostics, diag)
	}
	index.Diagnostics = diagnostics
	return index
}
