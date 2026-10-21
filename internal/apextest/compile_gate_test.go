package apextest

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// The reference walk binds names as Apex does: variables, fields, method
// names, inner and inherited inner classes shadow a project class of the same
// spelling; literals, comments and SOQL fields name nothing; a bind
// expression is Apex at any depth.
func TestCompileGateReferencesBindNames(t *testing.T) {
	units := compileGateUnits{fileOf: map[string]string{}, types: map[string]typesys.TypeSymbol{}, namespace: "gns"}
	for _, name := range []string{"bad", "helper", "other", "caster", "generic", "missing", "shadowed", "inherited", "param", "literal"} {
		units.fileOf[name] = name + ".cls"
	}
	units.types["base"] = typesys.TypeSymbol{Name: "Base"}
	units.types["base.inherited"] = typesys.TypeSymbol{Name: "Base.Inherited", OwnerName: "Base"}
	units.types["probe"] = typesys.TypeSymbol{Name: "Probe", SuperClass: "Base"}
	source := `public class Probe extends Base {
  // Missing and Literal are named in comments.
  /* Missing, Literal */
  public String bad = 'field';
  public class Shadowed { }
  static String missing() { return 'method'; }
  public static String run(Object value, String param) {
    Type t = Type.forName('Literal');
    String label = 'it\'s Missing' + missing();
    value.helper();
    String other = 'local';
    Integer n = other.length() + bad.length() + param.length();
    Inherited i = new Inherited();
    Shadowed s = new Shadowed();
    List<Account> rows = [SELECT Missing FROM Account WHERE Name = :String.valueOf(HELPER.NAME)];
    Object cast = (Caster) value;
    List<Generic> items = new List<Generic>();
    return gns.Other.label();
  }
}`
	refs := compileGateReferences("probe.cls", source, units)
	if got := strings.Join(refs, ","); got != "helper,caster,generic,other" {
		t.Fatalf("references = %s", got)
	}
	// The token scan that picks files to parse names every reference and
	// skips names that occur only in comments and literals.
	mentions := compileGateMentions(source, units.fileOf)
	for _, ref := range refs {
		if !mentions[ref] {
			t.Fatalf("mentions %v miss reference %s", mentions, ref)
		}
	}
	if mentions["literal"] || !mentions["missing"] || !mentions["bad"] {
		t.Fatalf("mentions = %v", mentions)
	}
}

// A setter's implicit parameter value binds before a class named Value, in
// any case; a getter or method has no such parameter.
func TestCompileGateReferencesBindSetterValue(t *testing.T) {
	units := compileGateUnits{fileOf: map[string]string{"value": "Value.cls"}, types: map[string]typesys.TypeSymbol{}}
	for _, tc := range []struct {
		name, source, want string
	}{
		{"setter", `public class P { public String label { get; set { label = value == null ? null : value.trim(); } } }`, ""},
		{"private setter upper case", `public class P { public String label { get; private SET { label = VALUE.trim() + Value.NAME; } } }`, ""},
		{"block comment before set", `public class P { public String label { get; private /* set? */ set { label = value.trim(); } } }`, ""},
		{"line comment and odd spacing", "public class P { public String label { get; private // get\n\t  set\n  { label = value.trim(); } } }", ""},
		{"annotated setter first", `public class P { public String label { @TestVisible private /*x*/set{ label = Value.one(); } get; } }`, ""},
		{"comment naming set before get", `public class P { public Integer n { /* set */ get { return Value.one(); } } }`, "value"},
		{"getter", `public class P { public Integer n { get { return Value.one(); } } }`, "value"},
		{"method", `public class P { public static Integer probe() { return value.one(); } }`, "value"},
		{"field initializer", `public class P { public Integer n = Value.one(); public String label { set { label = value; } } }`, "value"},
	} {
		if got := strings.Join(compileGateReferences("p.cls", tc.source, units), ","); got != tc.want {
			t.Errorf("%s: references = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A missing implementation that an excluded interface or superclass of the
// file requires is not the file's own error; a file left with none is a
// dependent, and a mixed file keeps only its own. Attribution comes from the
// diagnostic's code and RequiredBy, never its text, so an own error on a
// local named like the interface stays the file's own.
func TestCompileGateOwnErrors(t *testing.T) {
	units := compileGateUnits{fileOf: map[string]string{}, types: map[string]typesys.TypeSymbol{}}
	for _, typ := range []typesys.TypeSymbol{
		{Name: "BadI", File: "BadI.cls"},
		{Name: "Base", File: "Base.cls", Interfaces: []string{"BadI"}},
		{Name: "Impl", File: "Impl.cls", Interfaces: []string{"BadI"}},
		{Name: "Sub", File: "Sub.cls", SuperClass: "Base"},
		{Name: "Mixed", File: "Mixed.cls", Interfaces: []string{"BadI"}},
		{Name: "Decoy", File: "Decoy.cls", Interfaces: []string{"BadI"}},
		{Name: "Platform", File: "Platform.cls", Interfaces: []string{"BadI", "Schedulable"}},
		{Name: "Caller", File: "Caller.cls"},
	} {
		units.fileOf[strings.ToLower(typ.Name)] = typ.File
		units.types[strings.ToLower(typ.Name)] = typ
	}
	missing := func(requiredBy string) diagnostic.Diagnostic {
		return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA017", Message: "must implement", RequiredBy: requiredBy}
	}
	own := func(message string) diagnostic.Diagnostic {
		return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA018", Message: message}
	}
	byFile := map[string][]diagnostic.Diagnostic{
		"BadI.cls":     {own("Invalid type: NoSuchType")},
		"Impl.cls":     {missing("BadI")},
		"Sub.cls":      {missing("BadI")},
		"Mixed.cls":    {missing("BadI"), own(`method "one" initializes Integer local "x" with String`)},
		"Decoy.cls":    {own(`method "one" initializes Integer local "BadI" with String: BadI.two()`)},
		"Platform.cls": {missing("System.Schedulable")},
		"Caller.cls":   {own(`method "two" calls BadI.two()`)},
	}
	closure := map[string]bool{"BadI.cls": true, "Base.cls": true, "Impl.cls": true, "Sub.cls": true, "Mixed.cls": true, "Decoy.cls": true, "Platform.cls": true, "Caller.cls": true}
	got := compileGateOwnErrors(units, byFile, closure)
	counts := make(map[string]int)
	for file, diags := range got {
		counts[file] = len(diags)
	}
	want := map[string]int{"BadI.cls": 1, "Mixed.cls": 1, "Decoy.cls": 1, "Platform.cls": 1, "Caller.cls": 1}
	if !reflect.DeepEqual(counts, want) || got["Mixed.cls"][0].Code != "GLADESEMA018" {
		t.Fatalf("own errors = %v", got)
	}
}

// the captured row: an invalid class and its trigger are excluded while
// an unrelated test runs under the partial-run policy.
func TestCompileGateExcludesTriggerReferencingExcludedClass(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"GateBad.cls":         `public class GateBad { public static Integer one() { Integer x = 'not a number'; return 1; } }`,
		"GateIndep.cls":       `public class GateIndep { public static Integer five() { return 5; } }`,
		"GateIndepTest.cls":   `@IsTest private class GateIndepTest { @IsTest static void observe() { System.assertEquals(5, GateIndep.five()); } }`,
		"GateTrigger.trigger": `trigger GateTrigger on Account (before insert) { GateBad.one(); }`,
	}
	run := func(t *testing.T, names ...string) testreport.Run {
		t.Helper()
		var paths []string
		for _, name := range names {
			path := filepath.Join(root, name)
			writeFile(t, path, files[name])
			paths = append(paths, path)
		}
		index := typesys.Build(project.Project{Root: root, ApexFiles: paths}, gladeschema.Schema{})
		return RunCasesContext(context.Background(), index, Options{NoDiskCache: true}, nil)
	}
	scoped := run(t, "GateBad.cls", "GateIndep.cls", "GateIndepTest.cls")
	if summary := scoped.Summary(); summary.Passed != 1 || summary.CompileErrors != 0 || summary.ExcludedClasses != 1 {
		t.Fatalf("scoped summary = %#v suites = %#v", summary, scoped.Suites)
	}
	whole := run(t, "GateBad.cls", "GateIndep.cls", "GateIndepTest.cls", "GateTrigger.trigger")
	if summary := whole.Summary(); summary.Passed != 1 || summary.CompileErrors != 0 || summary.ExcludedClasses != 2 {
		t.Fatalf("trigger scope summary = %#v suites = %#v", summary, whole.Suites)
	}
	if len(whole.ExcludedClasses) != 2 || whole.ExcludedClasses[1].ClassName != "GateTrigger" || whole.ExcludedClasses[1].Message != "Variable does not exist: GateBad" {
		t.Fatalf("trigger exclusion = %#v", whole.ExcludedClasses)
	}
}

// Every top-level class in a file with an error is invalid, and a test class
// with its own error keeps its formatted diagnostics.
func TestCompileGateTreatsFileAsCompileUnit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Pair.cls")
	writeFile(t, path, `public class PairHelper { public static Integer one() { return 1; } }
@IsTest private class PairTest { @IsTest static void observe() { Integer x = 'nope'; System.assertEquals(1, PairHelper.one()); } }`)
	otherPath := filepath.Join(root, "UsesPair.cls")
	writeFile(t, otherPath, `public class UsesPair { public static Integer two() { return PairHelper.one() + 1; } }`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path, otherPath}}, gladeschema.Schema{})
	run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true}, nil)
	var excluded []string
	for _, entry := range run.ExcludedClasses {
		excluded = append(excluded, entry.ClassName+"<"+entry.DependsOn)
	}
	if got := strings.Join(excluded, ","); got != "PairHelper<,PairTest<,UsesPair<PairHelper" {
		t.Fatalf("excluded = %s", got)
	}
	if summary := run.Summary(); summary.Total != 1 || summary.CompileErrors != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	if problem := firstRunProblem(run); !strings.Contains(problem, "Pair.cls:2:") || !strings.Contains(problem, "GLADESEMA018") {
		t.Fatalf("problem = %q", problem)
	}
}

// A rerun whose semantic gate finds another invalid class excludes it and
// its dependents too, and reports each excluded test exactly once.
func TestCompileGateRerunExcludesNewlyInvalidClasses(t *testing.T) {
	root := t.TempDir()
	sources := map[string]string{
		"GateA":      `public class GateA { public static Integer one() { Integer x = 'a'; return 1; } }`,
		"GateB":      `public class GateB { public static Integer two() { Integer x = 'b'; return 2; } }`,
		"GateBTest":  `@IsTest private class GateBTest { @IsTest static void observe() { System.assertEquals(2, GateB.two()); } }`,
		"GateOk":     `public class GateOk { public static Integer five() { return 5; } }`,
		"GateOkTest": `@IsTest private class GateOkTest { @IsTest static void observe() { System.assertEquals(5, GateOk.five()); } }`,
	}
	var paths []string
	for _, name := range []string{"GateA", "GateB", "GateBTest", "GateOk", "GateOkTest"} {
		path := filepath.Join(root, name+".cls")
		writeFile(t, path, sources[name])
		paths = append(paths, path)
	}
	index := typesys.Build(project.Project{Root: root, ApexFiles: paths}, gladeschema.Schema{})
	// Hand the gate only GateA's error so GateB is found by the rerun's own
	// semantic gate, as a reference the scan missed would be.
	var onlyA []diagnostic.Diagnostic
	for _, diag := range sema.Analyze(index).Diagnostics {
		if diag.Severity == diagnostic.Error && filepath.Base(diag.File) == "GateA.cls" {
			onlyA = append(onlyA, diag)
		}
	}
	if len(onlyA) == 0 {
		t.Fatal("GateA has no compile error")
	}
	state := runExecution{counters: newRunPerfCounters(false)}
	run := runWithCompileGate(context.Background(), index, Options{NoDiskCache: true}, Discover(index, Options{}), semanticGateHooks{}, &state, nil, newSourceCache(), onlyA, errors.New("GateA"), time.Now())
	var excluded []string
	for _, entry := range run.ExcludedClasses {
		excluded = append(excluded, entry.ClassName+"<"+entry.DependsOn)
	}
	if got := strings.Join(excluded, ","); got != "GateA<,GateB<,GateBTest<GateB" {
		t.Fatalf("excluded = %s", got)
	}
	statuses := make(map[string]testreport.Status)
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			statuses[testCase.ClassName+"."+testCase.MethodName] = testCase.Status
		}
	}
	if len(statuses) != 2 || statuses["GateBTest.observe"] != testreport.StatusCompileError || statuses["GateOkTest.observe"] != testreport.StatusPass {
		t.Fatalf("statuses = %#v suites = %#v", statuses, run.Suites)
	}
	if summary := run.Summary(); summary.Total != 2 || len(run.Suites) != 2 {
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
}

// The compile phase is reported once, as failed, before any test result, and
// each selected test reports exactly once.
func TestCompileGateProgressReportsOneFailedCompilePhase(t *testing.T) {
	root := t.TempDir()
	var paths []string
	for name, source := range map[string]string{
		"GateA":        `public class GateA { public static Integer one() { Integer x = 'a'; return 1; } }`,
		"GateADepTest": `@IsTest private class GateADepTest { @IsTest static void observe() { System.assertEquals(1, GateA.one()); } }`,
		"GateOk":       `public class GateOk { public static Integer five() { return 5; } }`,
		"GateOkTest":   `@IsTest private class GateOkTest { @IsTest static void observe() { System.assertEquals(5, GateOk.five()); } }`,
	} {
		path := filepath.Join(root, name+".cls")
		writeFile(t, path, source)
		paths = append(paths, path)
	}
	index := typesys.Build(project.Project{Root: root, ApexFiles: paths}, gladeschema.Schema{})
	var events []string
	run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true, Progress: func(event TestProgress) {
		if event.Event == "compile_start" || event.Event == "compile_done" || event.Event == "test_done" {
			events = append(events, event.Event+":"+event.ClassName+":"+event.Status)
		}
	}}, nil)
	if len(events) != 4 || events[0] != "compile_start::" || events[1] != "compile_done::fail" {
		t.Fatalf("events = %v", events)
	}
	done := strings.Join(events[2:], ",")
	if done != "test_done:GateOkTest:pass,test_done:GateADepTest:compile_error" && done != "test_done:GateADepTest:compile_error,test_done:GateOkTest:pass" {
		t.Fatalf("events = %v", events)
	}
	if summary := run.Summary(); summary.Passed != 1 || summary.CompileErrors != 1 || summary.ExcludedClasses != 2 {
		t.Fatalf("summary = %#v", summary)
	}
}

// A rerun can discover another invalid class and its trigger without
// blocking unrelated tests. Each selected test is reported once.
func TestCompileGateRerunScopesTriggerDependency(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"GateA.cls":           `public class GateA { public static Integer one() { Integer x = 'a'; return 1; } }`,
		"GateB.cls":           `public class GateB { public static Integer two() { Integer x = 'b'; return 2; } }`,
		"GateBTest.cls":       `@IsTest private class GateBTest { @IsTest static void observe() { System.assertEquals(2, GateB.two()); } }`,
		"GateOk.cls":          `public class GateOk { public static Integer five() { return 5; } }`,
		"GateOkTest.cls":      `@IsTest private class GateOkTest { @IsTest static void observe() { System.assertEquals(5, GateOk.five()); } }`,
		"GateTrigger.trigger": `trigger GateTrigger on Account (before insert) { GateB.two(); }`,
	}
	var paths []string
	for name, source := range files {
		path := filepath.Join(root, name)
		writeFile(t, path, source)
		paths = append(paths, path)
	}
	index := typesys.Build(project.Project{Root: root, ApexFiles: paths}, gladeschema.Schema{})
	// Hand the gate only GateA's error, so GateB, which the trigger references,
	// is found by the rerun's own semantic gate.
	var onlyA []diagnostic.Diagnostic
	for _, diag := range sema.Analyze(index).Diagnostics {
		if diag.Severity == diagnostic.Error && filepath.Base(diag.File) == "GateA.cls" {
			onlyA = append(onlyA, diag)
		}
	}
	if len(onlyA) == 0 {
		t.Fatal("GateA has no compile error")
	}
	var events []string
	opts := Options{NoDiskCache: true, Progress: func(event TestProgress) {
		if event.Event == "compile_done" || event.Event == "test_done" {
			events = append(events, event.Event+":"+event.ClassName)
		}
	}}
	state := runExecution{counters: newRunPerfCounters(false)}
	cases := Discover(index, Options{})
	run := runWithCompileGate(context.Background(), index, opts, cases, semanticGateHooks{}, &state, nil, newSourceCache(), onlyA, indexCompileError(onlyA), time.Now())
	if len(run.ExcludedClasses) != 4 {
		t.Fatalf("scoped exclusions = %#v", run.ExcludedClasses)
	}
	if summary := run.Summary(); summary.Total != 2 || summary.CompileErrors != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
	if problem := firstRunProblem(run); !strings.Contains(problem, "GateB") {
		t.Fatalf("fallback problem = %q", problem)
	}
	sort.Strings(events)
	if got := strings.Join(events, ","); got != "compile_done:,test_done:GateBTest,test_done:GateOkTest" {
		t.Fatalf("events = %s", got)
	}
}
