package sema

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestBenchmarkFixtureExercisesMethodBodies(t *testing.T) {
	index := benchmarkIndexWithSources(t, 200)

	readableBodies := 0
	sources := make(map[string]string)
	for _, typ := range index.Types {
		if typ.Dependency {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod && member.Kind != apexast.DeclarationConstructor {
				continue
			}
			source, ok := readSemaSourceForType(typ, sources)
			if !ok || source == "" {
				t.Fatalf("fixture source for %s is empty or unreadable", typ.Name)
			}
			body, _, ok := extractBodyForSema(source, member.Range)
			if !ok || body == "" {
				t.Fatalf("fixture body for %s.%s is empty or unreadable", typ.Name, member.Name)
			}
			readableBodies++
		}
	}
	if readableBodies == 0 {
		t.Fatal("fixture contains no readable method bodies")
	}
	if diagnostics := NewAnalyzer().checkMethodBodies(index); len(diagnostics) == 0 {
		t.Fatal("fixture did not exercise method-body diagnostics")
	}
}

func TestBenchmarkFixtureExercisesSpecializedDiagnostics(t *testing.T) {
	index, analyzer := benchmarkPreparedAnalysisPhase(t, benchmarkIndexWithSources(t, 200))
	if diagnostics := analyzer.checkInheritanceContracts(index); len(diagnostics) == 0 {
		t.Error("fixture did not exercise inheritance diagnostics")
	}
	if diagnostics := analyzer.checkQuerySemantics(index); len(diagnostics) == 0 {
		t.Error("fixture did not exercise query-semantics diagnostics")
	}
}

func TestColdBenchmarkRequiresDedicatedProcess(t *testing.T) {
	const leaf = "BenchmarkAnalyzeIndex/size=200/mode=cold"
	t.Setenv(semaColdBenchmarkLeafEnv, "")
	if semaBenchmarkProcessMatches(leaf, "cold") {
		t.Fatal("cold benchmark matched without a dedicated-process leaf")
	}
	t.Setenv(semaColdBenchmarkLeafEnv, leaf)
	if !semaBenchmarkProcessMatches(leaf, "cold") {
		t.Fatal("cold benchmark did not match its dedicated-process leaf")
	}
	if semaBenchmarkProcessMatches(leaf+"-other", "cold") {
		t.Fatal("cold benchmark matched a different leaf")
	}
	if !semaBenchmarkProcessMatches(leaf, "warm") {
		t.Fatal("warm benchmark unexpectedly required the cold-process marker")
	}
}

func TestColdBenchmarkRejectsInvalidProcessMarker(t *testing.T) {
	const leaf = "BenchmarkAnalyzeIndex/size=200/mode=cold"
	for name, marker := range map[string]string{
		"missing":    "",
		"mismatched": leaf + "-other",
	} {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0],
				"-test.run=^$",
				"-test.bench=^BenchmarkAnalyzeIndex$/^size=200$/^mode=cold$",
				"-test.benchtime=1x",
			)
			cmd.Env = benchmarkEnvironmentWithColdLeaf(marker)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("cold benchmark accepted %s marker\n%s", name, output)
			}
			if !strings.Contains(string(output), "cold benchmark requires a dedicated process") {
				t.Fatalf("cold benchmark failed without lifecycle diagnostic: %v\n%s", err, output)
			}
		})
	}
}

func TestBenchmarkModesUseDistinctLifecycle(t *testing.T) {
	warmups := 0
	warmupSemaBenchmarkMode("cold", func() { warmups++ })
	if warmups != 0 {
		t.Fatalf("cold mode performed %d warm-ups, want 0", warmups)
	}

	warmupSemaBenchmarkMode("warm", func() { warmups++ })
	if warmups != 1 {
		t.Fatalf("warm mode performed %d warm-ups, want 1", warmups)
	}
}

func BenchmarkAnalyzeIndex(b *testing.B) {
	benchmarkSemaSizesAndModes(b, func(b *testing.B, index typesys.Index, mode string) {
		warmupSemaBenchmarkMode(mode, func() { _ = Analyze(index) })
		var got Result
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			got = Analyze(index)
		}
		b.StopTimer()
		if got.Summary.Types <= 0 {
			b.Fatalf("benchmark result has no project types: %#v", got)
		}
		if len(got.Types) == 0 {
			b.Fatalf("benchmark result has no exported types: %#v", got)
		}
		if len(got.Diagnostics) == 0 {
			b.Fatalf("benchmark result has no diagnostics: %#v", got)
		}
		want := Analyze(index)
		if !reflect.DeepEqual(got, want) {
			b.Fatalf("benchmark result differs from ordinary Analyze call\nwant: %#v\n got: %#v", want, got)
		}
	})
}

func BenchmarkCheckMethodBodies(b *testing.B) {
	benchmarkSemaSizesAndModes(b, func(b *testing.B, index typesys.Index, mode string) {
		index, analyzer := benchmarkPreparedAnalysisPhase(b, index)
		warmupSemaBenchmarkMode(mode, func() { _ = analyzer.checkMethodBodies(index) })
		var diagnostics []diagnostic.Diagnostic
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			diagnostics = analyzer.checkMethodBodies(index)
		}
		b.StopTimer()
		reference := analyzer.checkMethodBodies(index)
		if len(diagnostics) == 0 {
			b.Fatal("fixture did not exercise method-body diagnostics")
		}
		if !reflect.DeepEqual(diagnostics, reference) {
			b.Fatalf("measured method-body diagnostics differ from untimed reference\nwant: %#v\n got: %#v", reference, diagnostics)
		}
	})
}

func BenchmarkBuildTypeMembers(b *testing.B) {
	benchmarkSemaSizesAndModes(b, func(b *testing.B, index typesys.Index, mode string) {
		index, _ = benchmarkPreparedAnalysisPhase(b, index)
		warmupSemaBenchmarkMode(mode, func() {
			_ = buildTypeMembers(index)
		})
		var model *semaTypeMemberModel
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			model = buildTypeMembers(index)
		}
		b.StopTimer()
		reference := buildTypeMembers(index)
		if len(model.members) == 0 {
			b.Fatal("missing type-member model")
		}
		if !reflect.DeepEqual(model, reference) {
			b.Fatalf("measured type-member model differs from untimed reference\nwant: %#v\n got: %#v", reference, model)
		}
	})
}

func BenchmarkBuildPlatformTypeMemberModelCold(b *testing.B) {
	var model *semaTypeMemberModel
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model = buildSemaPlatformTypeMemberModel()
	}
	b.StopTimer()
	if model == nil || model.platform == nil || len(model.platform.symbols) == 0 {
		b.Fatal("missing platform type-member model")
	}
}

func BenchmarkLayeredModelCutoverProof(b *testing.B) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{Kind: apexast.DeclarationClass, Name: "Math", Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationField, Name: "ProjectOnly", Type: "String"}}},
			{Kind: apexast.DeclarationClass, Name: "Database", Dependency: true, Artifact: true, Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationField, Name: "DependencyOnly", Type: "String"}}},
		},
		Objects: []schema.Object{{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "BillingCountryCode", Type: "Text"},
				{Name: "ShippingStateCode", Type: "Text"},
				{Name: "CurrencyIsoCode", Type: "Text"},
			},
		}},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := buildSemaTypeMemberState(index, nil)
		view := state.view()
		for _, name := range []string{"Math", "Database", "Account"} {
			if _, _, ok := semaLookupTypeMembers(view, name); !ok {
				b.Fatalf("layered model omitted %s", name)
			}
		}
		checker := newQuerySemanticsChecker(index)
		for _, name := range []string{"PersonContactId", "BillingCountryCode", "ShippingStateCode", "CurrencyIsoCode"} {
			if _, ok := checker.field("Account", name); !ok {
				b.Fatalf("layered query model omitted Account.%s", name)
			}
		}
	}
}

func BenchmarkCheckInheritance(b *testing.B) {
	benchmarkSemaSizesAndModes(b, func(b *testing.B, index typesys.Index, mode string) {
		index, analyzer := benchmarkPreparedAnalysisPhase(b, index)
		warmupSemaBenchmarkMode(mode, func() { _ = analyzer.checkInheritanceContracts(index) })
		var diagnostics []diagnostic.Diagnostic
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			diagnostics = analyzer.checkInheritanceContracts(index)
		}
		b.StopTimer()
		reference := analyzer.checkInheritanceContracts(index)
		if len(diagnostics) == 0 {
			b.Fatal("fixture did not exercise inheritance diagnostics")
		}
		if !reflect.DeepEqual(diagnostics, reference) {
			b.Fatalf("measured inheritance diagnostics differ from untimed reference\nwant: %#v\n got: %#v", reference, diagnostics)
		}
	})
}

func BenchmarkCheckQuerySemantics(b *testing.B) {
	benchmarkSemaSizesAndModes(b, func(b *testing.B, index typesys.Index, mode string) {
		index, analyzer := benchmarkPreparedAnalysisPhase(b, index)
		warmupSemaBenchmarkMode(mode, func() { _ = analyzer.checkQuerySemantics(index) })
		var diagnostics []diagnostic.Diagnostic
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			diagnostics = analyzer.checkQuerySemantics(index)
		}
		b.StopTimer()
		reference := analyzer.checkQuerySemantics(index)
		if len(diagnostics) == 0 {
			b.Fatal("fixture did not exercise query-semantics diagnostics")
		}
		if !reflect.DeepEqual(diagnostics, reference) {
			b.Fatalf("measured query-semantics diagnostics differ from untimed reference\nwant: %#v\n got: %#v", reference, diagnostics)
		}
	})
}

func benchmarkSemaSizesAndModes(b *testing.B, run func(*testing.B, typesys.Index, string)) {
	for _, classes := range []int{200, 2000} {
		classes := classes
		b.Run(fmt.Sprintf("size=%d", classes), func(b *testing.B) {
			index := benchmarkIndexWithSources(b, classes)
			for _, mode := range []string{"cold", "warm"} {
				b.Run("mode="+mode, func(b *testing.B) {
					if !semaBenchmarkProcessMatches(b.Name(), mode) {
						b.Fatalf("cold benchmark requires a dedicated process with %s=%q", semaColdBenchmarkLeafEnv, b.Name())
					}
					run(b, index, mode)
				})
			}
		})
	}
}

const semaColdBenchmarkLeafEnv = "GLADE_SEMA_BENCH_COLD_LEAF"

func semaBenchmarkProcessMatches(name, mode string) bool {
	return mode != "cold" || os.Getenv(semaColdBenchmarkLeafEnv) == name
}

func benchmarkEnvironmentWithColdLeaf(leaf string) []string {
	environment := make([]string, 0, len(os.Environ())+1)
	prefix := semaColdBenchmarkLeafEnv + "="
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, prefix) {
			environment = append(environment, entry)
		}
	}
	if leaf != "" {
		environment = append(environment, prefix+leaf)
	}
	return environment
}

func benchmarkPreparedAnalysisPhase(tb testing.TB, index typesys.Index) (typesys.Index, *Analyzer) {
	tb.Helper()
	analyzer := NewAnalyzer()
	analyzer.prepareAnalysisContext(index, AnalyzeOptions{Diagnostics: true, ExportTypes: true})
	index = prepareAnalysisIndex(index)
	analyzer.prepareAnalysisModel(index)
	return index, analyzer
}

func warmupSemaBenchmarkMode(mode string, warmup func()) {
	if mode == "warm" {
		warmup()
	}
}

func benchmarkIndexWithSources(tb testing.TB, classes int) typesys.Index {
	tb.Helper()
	root := tb.TempDir()
	mainRoot := filepath.Join(root, "packages", "main")
	secondaryRoot := filepath.Join(root, "packages", "secondary")
	dependencyRoot := filepath.Join(root, "dependencies", "catalog")
	mainFiles := make([]string, 0, classes+3)

	writeApexBenchmarkFile(tb, filepath.Join(mainRoot, "BenchmarkBase.cls"), `
public virtual class BenchmarkBase {
    public virtual Integer calculate(Integer input) { return input; }
}
`)
	mainFiles = append(mainFiles, filepath.Join(mainRoot, "BenchmarkBase.cls"))
	for i := 0; i < classes; i++ {
		name := fmt.Sprintf("BenchmarkService%04d", i)
		extends := ""
		override := ""
		if i%4 == 0 {
			extends = " extends BenchmarkBase"
			override = "override "
		}
		bodyDiagnostic := ""
		if i == 0 {
			bodyDiagnostic = "MissingBenchmarkType missingValue = (MissingBenchmarkType) records[0];"
		}
		source := fmt.Sprintf(`
public class %[1]s%[2]s {
    public %[1]s() { Integer seed = 0; }
    public %[3]sInteger calculate(Integer input) { return input + 1; }
    public List<Account> run(List<Account> records) {
        Map<String, List<Account>> grouped = new Map<String, List<Account>>();
        Integer total = this.calculate(records.size());
        if (total > 0) {
            for (Account accountRecord : records) {
                String localName = accountRecord.Name;
                if (localName != null) { total = total + 1; }
            }
        }
        %[4]s
        List<Account> queried = [SELECT Id, Name FROM Account LIMIT 1];
        return queried;
    }
}
`, name, extends, override, bodyDiagnostic)
		path := filepath.Join(mainRoot, name+".cls")
		writeApexBenchmarkFile(tb, path, source)
		mainFiles = append(mainFiles, path)
	}
	diagnosticSentinel := filepath.Join(mainRoot, "BenchmarkDiagnosticSentinel.cls")
	writeApexBenchmarkFile(tb, diagnosticSentinel, `
public class BenchmarkDiagnosticSentinel extends BenchmarkBase {
    public override Integer missingOverride(Integer input) { return input; }
    public List<Account> invalidQuery() {
        return [SELECT MissingBenchmarkField__c FROM Account];
    }
}
`)
	mainFiles = append(mainFiles, diagnosticSentinel)

	duplicateMain := filepath.Join(mainRoot, "BoundaryDuplicate.cls")
	duplicateSecondary := filepath.Join(secondaryRoot, "BoundaryDuplicate.cls")
	writeApexBenchmarkFile(tb, duplicateMain, "public class BoundaryDuplicate { public void run() { Integer seed = 0; } }")
	writeApexBenchmarkFile(tb, duplicateSecondary, "public class BoundaryDuplicate { public void run() { Integer seed = 1; } }")
	mainFiles = append(mainFiles, duplicateMain, duplicateSecondary)

	dependencyFile := filepath.Join(dependencyRoot, "PackageBoundaryService.cls")
	writeApexBenchmarkFile(tb, dependencyFile, `
global class PackageBoundaryService {
    global PackageBoundaryService() {}
    global List<Account> load() { return [SELECT Id, Name FROM Account LIMIT 1]; }
}
`)
	dependencyProject := project.Project{
		Root:      dependencyRoot,
		Namespace: "catalogpkg",
		ApexFiles: []string{dependencyFile},
	}
	proj := project.Project{
		Root:      root,
		Namespace: "benchmark",
		PackageDirectories: []project.PackageDirectory{
			{Path: mainRoot, Default: true, Package: "main"},
			{Path: secondaryRoot, Package: "secondary"},
		},
		ApexFiles: mainFiles,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "catalogpkg",
			SourceRoot: dependencyRoot,
			Project:    &dependencyProject,
			Status:     "loaded",
		}},
	}
	fixtureSchema := schema.Schema{Objects: []schema.Object{
		{Name: "Account", Fields: []schema.Field{{Name: "Id", Type: "Id"}, {Name: "Name", Type: "Text"}}},
		{Name: "Contact", Fields: []schema.Field{{Name: "Id", Type: "Id"}, {Name: "AccountId", Type: "Lookup", ReferenceTo: []string{"Account"}}}},
		{Name: "Benchmark_Record__c", Fields: []schema.Field{{Name: "Name", Type: "Text"}}},
	}}
	return typesys.Build(proj, fixtureSchema)
}

func writeApexBenchmarkFile(tb testing.TB, path, source string) {
	tb.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		tb.Fatal(err)
	}
}
