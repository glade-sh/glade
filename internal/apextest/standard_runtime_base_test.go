package apextest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"weak"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Different controller projects must share immutable platform registration,
// while their methods, static values and orgs remain independent.
func TestDistinctRequestRuntimesShareStandardRegistration(t *testing.T) {
	releaseRuntimeCachesAfterTest(t)
	// Process preparation is useful once there is a second cold project.
	if _, err := NewProjectRuntimeForRequest(typesys.Index{}); err != nil {
		t.Fatal(err)
	}
	firstIndex := standardRuntimeBaseIndex(t, "62.0", "FirstRow", 1)
	secondIndex := standardRuntimeBaseIndex(t, "67.0", "SecondRow", 1)
	first, err := NewProjectRuntimeForRequest(firstIndex)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewProjectRuntimeForRequest(secondIndex)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Methods) == 0 || reflect.ValueOf(first.Methods).Pointer() != reflect.ValueOf(second.Methods).Pointer() {
		t.Fatal("distinct projects rebuilt the immutable standard method table")
	}
	for _, machine := range []*vm.VM{first, second} {
		if err := machine.RegisterMethod(vm.Method{Name: "PrivateRow.method"}); err != nil {
			t.Fatal(err)
		}
	}
	third, err := NewProjectRuntimeForRequest(firstIndex)
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := third.Methods["PrivateRow.method"]; leaked {
		t.Fatal("row registration changed the shared standard method table")
	}
}

// A one-project CLI process should retain the release's complete method table
// and avoid paying for a reusable platform prefix that it will never reuse.
func TestSingleProjectRequestStartupKeepsFullMethodTables(t *testing.T) {
	if os.Getenv("GLADE_STANDARD_STARTUP_CHILD") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command(executable, "-test.run=^TestSingleProjectRequestStartupKeepsFullMethodTables$", "-test.count=1")
		command.Env = append(os.Environ(), "GLADE_STANDARD_STARTUP_CHILD=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("fresh startup: %v\n%s", err, output)
		}
		return
	}
	index := standardRuntimeBaseIndex(t, "67.0", "FirstColdRow", 1)
	machine, err := NewProjectRuntimeForRequest(index)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := machine.Methods["FirstColdRow.read"]; !exists {
		t.Fatal("single-project startup retained a standard prefix instead of its complete method table")
	}
}

// Run follows the CLI's no-cache route with authoritative build artifacts.
// Each API/perf combination needs a fresh process: the first-use guard and the
// platform indexes must not have been warmed by another test or an initializer.
func TestSingleProjectRunStartupDoesNotRetainStandardRuntime(t *testing.T) {
	api := os.Getenv("GLADE_STANDARD_RUN_STARTUP_API")
	if api == "" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		for _, api := range []string{"62.0", "67.0"} {
			for _, perf := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/perf-%t", api, perf), func(t *testing.T) {
					command := exec.Command(executable, "-test.run=^TestSingleProjectRunStartupDoesNotRetainStandardRuntime$", "-test.count=1")
					command.Env = append(os.Environ(),
						"GLADE_STANDARD_RUN_STARTUP_API="+api,
						fmt.Sprintf("GLADE_STANDARD_RUN_STARTUP_PERF=%t", perf))
					if output, err := command.CombinedOutput(); err != nil {
						t.Fatalf("fresh Run startup: %v\n%s", err, output)
					}
				})
			}
		}
		return
	}
	if api != "62.0" && api != "67.0" {
		t.Fatalf("unexpected child API %q", api)
	}
	perfValue := os.Getenv("GLADE_STANDARD_RUN_STARTUP_PERF")
	if perfValue != "false" && perfValue != "true" {
		t.Fatalf("unexpected child perf mode %q", perfValue)
	}
	if standardRuntimeBase.used.Load() || standardRuntimeBase.prefix.Value() != nil {
		t.Fatal("fresh child already prepared a standard runtime")
	}

	root := t.TempDir()
	classesDir := filepath.Join(root, "force-app/main/default/classes")
	if err := os.MkdirAll(classesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(path, source string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	writeClass := func(name, source string) {
		t.Helper()
		path := filepath.Join(classesDir, name+".cls")
		write(path, source)
		write(path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	}
	// Eager field initializers call vm.New themselves and would hide the cold
	// VM-construction boundary. These helpers deliberately contain only methods.
	for i := 0; i < 512; i++ {
		name := fmt.Sprintf("ColdRunHelper%d", i)
		writeClass(name, fmt.Sprintf("public class %s { public static Integer read() { return %d; } }", name, i))
	}
	writeClass("ColdRunProbeTest", `@IsTest private class ColdRunProbeTest {
 @IsTest static void observe() {
  System.assertEquals(0, ColdRunHelper0.read());
  System.assertEquals(511, ColdRunHelper511.read());
 }
}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, sch)
	if index.HasErrors() {
		t.Fatalf("index diagnostics: %v", index.Diagnostics)
	}
	runtimeBuilds := 0
	restoreHook := setSnapshotValidationHookForTesting(func(stage string) {
		if stage == "runtime_after_initial_validation" {
			runtimeBuilds++
		}
	})
	defer restoreHook()
	opts := Options{
		NoDiskCache:    true,
		Parallelism:    1,
		PerfCounters:   perfValue == "true",
		BuildArtifacts: &artifacts,
		SourceDigests:  artifacts.SourceDigests,
	}
	run := Run(index, opts)
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || !summary.OK() {
		t.Fatalf("cold Run summary = %#v; problem = %q", summary, firstRunProblem(run))
	}
	if runtimeBuilds != 1 {
		t.Fatalf("runtime preparations = %d, want 1", runtimeBuilds)
	}
	if !standardRuntimeBase.used.Load() {
		t.Fatal("cold Run did not exercise the first runtime preparation")
	}
	if standardRuntimeBase.prefix.Value() != nil {
		t.Fatal("single-project Run retained a reusable standard runtime")
	}
	if passiveStandardRuntimeClassCache.templates != nil || passiveStandardRuntimeClassCache.builds != 0 {
		t.Fatal("single-project Run retained pristine passive templates alongside its runtime classes")
	}

	// A second cold generation establishes the reusable prefix normally. Cache
	// invalidation alone must not replace this transition with a warm cache hit.
	InvalidateRuntimeCaches()
	run = Run(index, opts)
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || !summary.OK() {
		t.Fatalf("second cold Run summary = %#v; problem = %q", summary, firstRunProblem(run))
	}
	if runtimeBuilds != 2 {
		t.Fatalf("runtime preparations after second cold Run = %d, want 2", runtimeBuilds)
	}
	if standardRuntimeBase.prefix.Value() == nil {
		t.Fatal("second cold Run did not establish the reusable standard runtime")
	}
	if len(passiveStandardRuntimeClassCache.templates) == 0 || passiveStandardRuntimeClassCache.builds != 1 {
		t.Fatalf("second cold Run template cache: %d classes, %d builds; want nonempty, 1 build", len(passiveStandardRuntimeClassCache.templates), passiveStandardRuntimeClassCache.builds)
	}
}

// First-cold preparation owns its passive declarations directly. One fresh
// process combines the exclusion shapes and preserves their registration order.
func TestFirstColdRuntimePassiveClassesMatchCachedRegistration(t *testing.T) {
	if os.Getenv("GLADE_FIRST_COLD_PASSIVE_CHILD") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command(executable, "-test.run=^TestFirstColdRuntimePassiveClassesMatchCachedRegistration$", "-test.count=1")
		command.Env = append(os.Environ(), "GLADE_FIRST_COLD_PASSIVE_CHILD=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("first-cold passive registration: %v\n%s", err, output)
		}
		return
	}
	if standardRuntimeBase.used.Load() || passiveStandardRuntimeClassCache.templates != nil {
		t.Fatal("first-cold passive child was already warmed")
	}
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "apex.page", Namespace: "Component"},
		{Kind: apexast.DeclarationClass, Name: "ReportMetadata", Namespace: "Reports"},
		{Kind: apexast.DeclarationClass, Name: " Component.apex.page "},
	}}
	// Include the existing filter oracle's folded, namespaced, enum and
	// ordinary-class exclusions in the same first-cold registration.
	for _, tc := range passiveStandardRuntimeClassFilterCases() {
		index.Types = append(index.Types, tc.indexTypes...)
		for _, class := range tc.existing {
			index.Types = append(index.Types, typesys.TypeSymbol{Kind: apexast.DeclarationClass, Name: class.Name, Namespace: class.Namespace})
		}
	}
	private := compileProjectClassesWhere(index, nil, func(typesys.TypeSymbol) bool { return true }, newSourceCache())
	prepared := prepareProjectClassRuntime(index, nil, newSourceCache())
	if passiveStandardRuntimeClassCache.templates != nil || passiveStandardRuntimeClassCache.builds != 0 {
		t.Fatal("first-cold preparation retained the pristine passive template cache")
	}
	if prepared.machine != nil || standardRuntimeBase.prefix.Value() != nil {
		t.Fatal("first-cold preparation constructed a VM before org construction")
	}
	wantPassive := freshPassiveStandardRuntimeClasses(index.Types, private)
	if len(prepared.classes) < len(private) || !reflect.DeepEqual(prepared.classes[len(private):], wantPassive) {
		t.Fatal("first-cold passive suffix changed class values, order or exclusions")
	}
	want := compileProjectClasses(index, nil, newSourceCache())
	if !reflect.DeepEqual(prepared.classes, want) || !reflect.DeepEqual(prepared.unregistered, want) {
		t.Fatal("first-cold class list differs from cached compilation")
	}

	gotMachine, wantMachine := vm.New(nil), vm.New(nil)
	for _, registration := range []struct {
		machine *vm.VM
		classes []vm.Class
	}{{gotMachine, prepared.unregistered}, {wantMachine, want}} {
		if err := registerBaseRuntime(registration.machine, nil, registration.classes, nil); err != nil {
			t.Fatal(err)
		}
		registration.machine.FreezeClassLookup()
	}
	if len(gotMachine.Classes) != len(wantMachine.Classes) {
		t.Fatalf("registered alias count = %d, want %d", len(gotMachine.Classes), len(wantMachine.Classes))
	}
	for alias, expected := range wantMachine.Classes {
		actual, exists := gotMachine.Classes[alias]
		if !exists || actual.Name != expected.Name || actual.Namespace != expected.Namespace {
			t.Fatalf("registered alias %q changed owner", alias)
		}
	}
	program, err := vm.CompileAnonymous(`
System.debug(Type.forName('Auth.JWT'));
System.debug(Type.forName('Auth.AuthProviderPlugin'));
System.debug(Type.forName('Reports.ReportMetadata'));
System.debug(Type.forName('Component.apex.page'));
System.debug(Type.forName('XmlTag'));
System.debug(Type.forName('LoggingLevel'));`)
	if err != nil {
		t.Fatal(err)
	}
	actual, actualErr := gotMachine.Execute(program)
	expected, expectedErr := wantMachine.Execute(program)
	if actualErr != nil || expectedErr != nil || !reflect.DeepEqual(actual.Debug, expected.Debug) {
		t.Fatalf("first-cold reflection = %v/%v, cached = %v/%v", actual.Debug, actualErr, expected.Debug, expectedErr)
	}

	// Mutating the first runtime's maps and nested slices must not reach the
	// later cache, nor the immutable symbols used to build fresh declarations.
	for _, class := range prepared.classes[len(private):] {
		class.Fields["OwnedMutation"] = vm.Field{Name: "OwnedMutation", Type: "String"}
		class.StaticFields["OwnedMutation"] = vm.Field{Name: "OwnedMutation", Type: "Integer", Static: true}
		for name, method := range class.Methods {
			if len(method.Params) > 0 {
				method.Params[0].Type = "OwnedMutation"
			}
			if len(method.Modifiers) > 0 {
				method.Modifiers[0] = "OwnedMutation"
			}
			method.ClassName = "OwnedMutation"
			class.Methods[name] = method
		}
		class.Methods["OwnedMutation()"] = vm.Method{Name: "OwnedMutation"}
		for _, values := range [][]string{class.Interfaces, class.Modifiers, class.FieldOrder, class.StaticFieldOrder, class.EnumValues} {
			if len(values) > 0 {
				values[0] = "OwnedMutation"
			}
		}
		for i := range class.Constructors {
			class.Constructors[i].ClassName = "OwnedMutation"
			if len(class.Constructors[i].Params) > 0 {
				class.Constructors[i].Params[0].Type = "OwnedMutation"
			}
		}
	}
	if got := passiveStandardRuntimeClasses(index.Types, private); !reflect.DeepEqual(got, wantPassive) {
		t.Fatal("mutating first-cold declarations changed cached passive classes")
	}
	if got := freshPassiveStandardRuntimeClasses(index.Types, private); !reflect.DeepEqual(got, wantPassive) {
		t.Fatal("mutating first-cold declarations changed the platform symbol view")
	}
}

// Compare the new request route with the release's full registration path.
// Native row isolation remains covered by TestFrozenStandardOrgNativeConformance.
func TestStandardRuntimeBaseMatchesFreshRegistration(t *testing.T) {
	releaseRuntimeCachesAfterTest(t)
	for _, api := range []string{"59.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := standardRuntimeBaseIndex(t, api, "RowState", 1)
			for row := 0; row < 2; row++ {
				got, err := NewProjectRuntimeForRequest(index)
				if err != nil {
					t.Fatal(err)
				}
				want := vm.New(nil)
				sources := newSourceCache()
				methods := compileProjectMethods(index, sources)
				classes := compileProjectClasses(index, methods, sources)
				if err := registerBaseRuntime(want, methods, classes, nil); err != nil {
					t.Fatal(err)
				}
				want.SetExpressionTypeResolver(sema.RuntimeExpressionTypeResolver(index))
				for _, machine := range []*vm.VM{got, want} {
					org := orgFromIndex(index)
					machine.SetOrg(&org)
				}
				program, err := vm.CompileAnonymousWithOptions(`
System.assertEquals(0, RowState.calls);
RowState.calls++;
System.assertNotEquals(null, Type.forName('Auth.JWT'));
System.assertNotEquals(null, Type.forName('Auth.AuthProviderPlugin'));
System.assertEquals(0, [SELECT count() FROM Account]);
insert new Account(Name='row');
System.debug(RowState.calls);
System.debug([SELECT count() FROM Account]);`, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				actual, actualErr := got.Execute(program)
				expected, expectedErr := want.Execute(program)
				if actualErr != nil || expectedErr != nil || !reflect.DeepEqual(actual.Debug, expected.Debug) {
					t.Fatalf("row %d: shared=%v/%v fresh=%v/%v", row, actual.Debug, actualErr, expected.Debug, expectedErr)
				}
			}
		})
	}
}

func TestStandardRuntimeBasePreservesShadowedClasses(t *testing.T) {
	releaseRuntimeCachesAfterTest(t)
	for _, symbol := range []typesys.TypeSymbol{
		{Name: "XmlTag"}, {Name: "loggingLevel"}, {Name: "Auth.JWT"},
		{Name: "apex.page", Namespace: "Component"},
		{Name: "apex.page", Namespace: "component"},
		{Name: "JWT", Namespace: "Auth"},
		{Name: "ReportMetadata", Namespace: "Reports"},
		{Name: " Component.apex.page "},
	} {
		label := symbol.Name
		if symbol.Namespace != "" {
			label = symbol.Namespace + "/" + label
		}
		t.Run(label, func(t *testing.T) {
			symbol.Kind = apexast.DeclarationClass
			index := typesys.Index{Types: []typesys.TypeSymbol{symbol}}
			got, err := NewProjectRuntimeForRequest(index)
			if err != nil {
				t.Fatal(err)
			}
			want := vm.New(nil)
			classes := compileProjectClasses(index, nil)
			if err := registerBaseRuntime(want, nil, classes, nil); err != nil {
				t.Fatal(err)
			}
			// Compare the shadow and every alias owner. Other classes' static
			// values have been reset by the request clone; the fresh, unexecuted
			// builder is not that state. Methods also keeps an arbitrary last
			// overload, so raw table equality is not a behavior comparison.
			name := typeSymbolRuntimeName(symbol)
			if !reflect.DeepEqual(got.Classes[name], want.Classes[name]) || len(got.Classes) != len(want.Classes) {
				t.Fatal("shadowed platform class differs from fresh registration")
			}
			for alias, class := range want.Classes {
				other, exists := got.Classes[alias]
				if !exists || class.Name != other.Name || class.Namespace != other.Namespace {
					t.Fatalf("alias %s changed owner", alias)
				}
			}
		})
	}
}

func TestStandardRuntimeBaseAllocationBudget(t *testing.T) {
	releaseRuntimeCachesAfterTest(t)
	index := standardRuntimeBaseIndex(t, "67.0", "AllocationRow", 1)
	legacy := func() {
		sources := newSourceCache()
		methods := compileProjectMethods(index, sources)
		classes := compileProjectClasses(index, methods, sources)
		org, err := orgFromIndexWithError(index, sources)
		if err != nil {
			t.Fatal(err)
		}
		machine := vm.New(nil)
		if err := registerBaseRuntime(machine, methods, classes, nil); err != nil {
			t.Fatal(err)
		}
		template := vm.NewRestoredRuntimeTemplate(org, machine)
		_ = template.CloneMachine(nil)
	}
	reused := func() {
		InvalidateRuntimeCaches()
		if _, err := NewProjectRuntimeForRequest(index); err != nil {
			t.Fatal(err)
		}
	}
	fresh := testing.AllocsPerRun(3, legacy)
	reused()
	reused()
	shared := testing.AllocsPerRun(3, reused)
	t.Logf("cold request allocations: full registration %.0f, shared registration %.0f", fresh, shared)
	if shared > fresh/2 {
		t.Fatalf("cold request allocations %.0f exceed half of full registration %.0f", shared, fresh)
	}
}

func BenchmarkStandardRuntimeProjectTestStartup(b *testing.B) {
	index := standardRuntimeBaseIndex(b, "67.0", "StartupRow", 128)
	path := filepath.Join(index.Project.Root, "force-app/main/default/classes/StartupProbeTest.cls")
	if err := os.WriteFile(path, []byte(`@IsTest private class StartupProbeTest {
 @IsTest static void observe(){System.assertEquals(0, StartupRow.calls);}
}`), 0o600); err != nil {
		b.Fatal(err)
	}
	p, err := project.Load(index.Project.Root)
	if err != nil {
		b.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		b.Fatal(err)
	}
	index = typesys.Build(p, sch)
	for _, perf := range []bool{false, true} {
		b.Run(fmt.Sprintf("perf-%t", perf), func(b *testing.B) {
			options := Options{NoDiskCache: true, Parallelism: 1, PerfCounters: perf}
			if summary := Run(index, options).Summary(); summary.Total != 1 || summary.Passed != 1 {
				b.Fatalf("startup workload: %#v", summary)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				InvalidateRuntimeCaches()
				if summary := Run(index, options).Summary(); summary.Total != 1 || summary.Passed != 1 {
					b.Fatalf("startup workload: %#v", summary)
				}
			}
		})
	}
}

func BenchmarkDistinctProjectRequestStartup(b *testing.B) {
	for _, count := range []int{1, 128} {
		b.Run(fmt.Sprintf("classes-%d", count), func(b *testing.B) {
			index := standardRuntimeBaseIndex(b, "67.0", "StartupRow", count)
			// Warm immutable process preparation. Each timed iteration is a
			// cold project runtime, as for distinct VF controller rows.
			if _, err := NewProjectRuntimeForRequest(index); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				InvalidateRuntimeCaches()
				if _, err := NewProjectRuntimeForRequest(index); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func standardRuntimeBaseIndex(t testing.TB, api, name string, count int) typesys.Index {
	t.Helper()
	root := t.TempDir()
	write := func(path, source string) {
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	for i := 0; i < count; i++ {
		class := name
		if i != 0 {
			class = fmt.Sprintf("%s%d", name, i)
		}
		path := "force-app/main/default/classes/" + class + ".cls"
		write(path, "public class "+class+" { public static Integer calls=0; public static Integer read(){return calls;} }")
		write(path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, sch)
	if index.HasErrors() {
		t.Fatalf("index diagnostics: %v", index.Diagnostics)
	}
	return index
}

// The prefix is reused while a compiled runtime that shares it is cached, and
// is released with the cache so a long-lived process does not retain it.
func TestStandardRuntimePrefixReleasedWithRuntimeCache(t *testing.T) {
	releaseRuntimeCachesAfterTest(t)
	InvalidateRuntimeCaches()
	for i := 0; i < 2; i++ {
		if _, err := NewProjectRuntimeForRequest(standardRuntimeBaseIndex(t, "67.0", fmt.Sprintf("PrefixOwner%d", i), 1)); err != nil {
			t.Fatal(err)
		}
		InvalidateRuntimeCaches()
	}
	// Two cold builds in a row ran with no cached owner, so the prefix may
	// already be gone. A cached build must (re)establish and then keep it.
	first := standardRuntimeIndexForPrefix(t, "PrefixKeep0")
	if _, err := NewProjectRuntimeForRequest(first); err != nil {
		t.Fatal(err)
	}
	owned := standardRuntimeBase.prefix.Value()
	if owned == nil {
		t.Fatal("cached runtime did not hold the standard prefix")
	}
	if _, err := NewProjectRuntimeForRequest(standardRuntimeIndexForPrefix(t, "PrefixKeep1")); err != nil {
		t.Fatal(err)
	}
	if standardRuntimeBase.prefix.Value() != owned {
		t.Fatal("second distinct runtime did not reuse the cached standard prefix")
	}
	runtime.KeepAlive(owned)
	owned = nil
	InvalidateRuntimeCaches()
	WaitForRuntimeCacheWork()
	for i := 0; i < 3 && standardRuntimeBase.prefix.Value() != nil; i++ {
		runtime.GC()
	}
	if standardRuntimeBase.prefix.Value() != nil {
		t.Fatal("standard prefix outlived every cached runtime")
	}
}

func standardRuntimeIndexForPrefix(t testing.TB, name string) typesys.Index {
	return standardRuntimeBaseIndex(t, "67.0", name, 1)
}

// A failed shared-prefix registration must fall back to the legacy per-project
// path instead of dereferencing a nil prefix.
func TestStandardRuntimeNilPrefixFallsBackToLegacyRegistration(t *testing.T) {
	releaseRuntimeCachesAfterTest(t)
	InvalidateRuntimeCaches()
	// Drop the weak reference directly instead of waiting for the collector:
	// another test's cache may still hold the prefix strongly, and the fallback
	// is only reachable when the lookup returns nil. Holders keep their strong
	// pointer; the cleanup restores the original reference and flag.
	standardRuntimeBase.mu.Lock()
	oldFailed := standardRuntimeBase.failed
	oldPrefix := standardRuntimeBase.prefix
	standardRuntimeBase.failed = true
	standardRuntimeBase.prefix = weak.Pointer[standardRuntimePrefix]{}
	standardRuntimeBase.mu.Unlock()
	t.Cleanup(func() {
		standardRuntimeBase.mu.Lock()
		standardRuntimeBase.failed = oldFailed
		standardRuntimeBase.prefix = oldPrefix
		standardRuntimeBase.mu.Unlock()
	})
	wasUsed := standardRuntimeBase.used.Swap(true)
	t.Cleanup(func() { standardRuntimeBase.used.Store(wasUsed) })
	if standardRuntimeBase.prefix.Value() != nil {
		t.Fatal("standard prefix still resolvable after the weak reference was cleared")
	}
	index := standardRuntimeBaseIndex(t, "67.0", "NilPrefixRow", 1)
	got := prepareProjectClassRuntime(index, nil, newSourceCache())
	want := compileProjectClasses(index, nil, newSourceCache())
	if got.prefix != nil || got.machine != nil {
		t.Fatal("nil prefix path used a shared machine")
	}
	if !reflect.DeepEqual(got.classes, want) || !reflect.DeepEqual(got.unregistered, want) {
		t.Fatal("nil prefix fallback differs from legacy registration")
	}
}
