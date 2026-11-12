package sema

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// methodBodyWorkersProject writes an SFDX project whose classes produce body
// diagnostics of several kinds, with inner classes (reported before their
// outer class) and one class name declared in two files.
func methodBodyWorkersProject(t *testing.T, count int) string {
	t.Helper()
	root := t.TempDir()
	classes := filepath.Join(root, "force-app/main/default/classes")
	if err := os.MkdirAll(classes, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	write := func(name, body string) {
		writeSemaFile(t, filepath.Join(classes, name+".cls"), body)
		writeSemaFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	write("Shared", "public class Shared {\n public void run() { missingShared(); }\n}\n")
	write("SharedTwin", "public class Shared {\n public void other() { missingTwin(); }\n}\n")
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("Worker%d", i)
		var body string
		switch i % 5 {
		case 0:
			body = fmt.Sprintf("public class %s {\n public class Inner { public void go() { unknownInner%d(); } }\n public void run() { Integer n = 'text'; unknownOuter%d(); }\n}\n", name, i, i)
		case 1:
			body = fmt.Sprintf("public class %s {\n public Integer total(List<Account> rows) {\n  Integer sum = 0;\n  for (Account a : rows) { sum += a.NumberOfEmployees; }\n  return sum;\n }\n}\n", name)
		case 2:
			body = fmt.Sprintf("public class %s {\n public void run() {\n  Account a = new Account(Name='x');\n  a.NoSuchField__c = 1;\n  String s = a.Name.noSuchMethod();\n }\n}\n", name)
		case 3:
			body = fmt.Sprintf("public class %s {\n public void run() {\n  Map<String,Integer> m = new Map<String,Integer>();\n  m.put('a', 'b');\n  undeclared%d++;\n }\n}\n", name, i)
		default:
			body = fmt.Sprintf("public class %s {\n public void run() { Shared s = new Shared(); s.run(); Decimal d = 1.5; Integer k = d; }\n}\n", name)
		}
		write(name, body)
	}
	return root
}

func methodBodyWorkersIndex(root string) (typesys.Index, bool) {
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, false
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		return typesys.Index{}, false
	}
	return typesys.Build(p, s), true
}

func methodBodyWorkersRoots(t *testing.T) []string {
	roots := []string{methodBodyWorkersProject(t, 80)}
	for _, base := range []string{"../../testdata", "testdata"} {
		_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && d.Name() == "sfdx-project.json" {
				roots = append(roots, filepath.Dir(path))
			}
			return nil
		})
	}
	return roots
}

// bodyDiagnosticsWithWorkers runs the body pass the way analyzeWithOptions
// does: after the passes that warm the member view, over a fresh view.
func bodyDiagnosticsWithWorkers(index typesys.Index, workers int) []diagnostic.Diagnostic {
	a := NewAnalyzer()
	a.prepareAnalysisContext(index, AnalyzeOptions{Diagnostics: true})
	index = prepareAnalysisIndexWithSources(index, a.sources)
	a.prepareAnalysisModel(index)
	state := buildSemaTypeMemberState(index, nil, a.sources)
	view := state.view()
	a.checkMemberTypes(index, view)
	a.checkMethodParameters(index, view)
	return a.checkMethodBodiesWithViewWorkers(index, view, nil, workers)
}

func TestMethodBodyDiagnosticsIdenticalAcrossWorkerCounts(t *testing.T) {
	compared := 0
	for i, root := range methodBodyWorkersRoots(t) {
		index, ok := methodBodyWorkersIndex(root)
		if !ok {
			continue
		}
		want := bodyDiagnosticsWithWorkers(index, 1)
		if i == 0 && len(want) < 40 {
			t.Fatalf("generated project produced %d body diagnostics, want many: %v", len(want), want)
		}
		workerCounts, rounds := []int{2, 8}, 1
		if i == 0 {
			workerCounts, rounds = []int{2, 3, 8}, 3
		}
		for _, workers := range workerCounts {
			for round := 0; round < rounds; round++ {
				if got := bodyDiagnosticsWithWorkers(index, workers); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s: %d workers (round %d) differ from one worker\none:  %v\nmany: %v", strings.TrimPrefix(root, "../../"), workers, round, want, got)
				}
			}
		}
		compared++
	}
	if compared < 5 {
		t.Fatalf("compared %d projects, want the generated project and the repo testdata projects", compared)
	}
}

// Analyze must report the same diagnostics whatever GOMAXPROCS is, since the
// default worker count follows it.
func TestAnalyzeDiagnosticsIdenticalAcrossGOMAXPROCS(t *testing.T) {
	previous := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(previous)
	for _, root := range methodBodyWorkersRoots(t) {
		index, ok := methodBodyWorkersIndex(root)
		if !ok {
			continue
		}
		runtime.GOMAXPROCS(1)
		want := Analyze(index)
		runtime.GOMAXPROCS(6)
		{
			got := Analyze(index)
			if !reflect.DeepEqual(got.Diagnostics, want.Diagnostics) {
				t.Fatalf("%s: GOMAXPROCS=6 diagnostics differ from GOMAXPROCS=1\none:  %v\nmany: %v", strings.TrimPrefix(root, "../../"), want.Diagnostics, got.Diagnostics)
			}
		}
	}
}

func TestMethodBodyWorkerCountFollowsGOMAXPROCS(t *testing.T) {
	previous := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(previous)
	for procs, want := range map[int]int{1: 1, 2: 2, 4: 4, 64: semaMethodBodyMaxWorkers} {
		runtime.GOMAXPROCS(procs)
		if got := semaMethodBodyWorkerCount(); got != want {
			t.Fatalf("GOMAXPROCS=%d: worker count = %d, want %d", procs, got, want)
		}
	}
}

// bodiesWithGOMAXPROCS runs the default body pass over a fresh view, as
// analyzeWithOptions does, and returns the diagnostics and the view's hydrated
// keys afterwards. Later passes read that view, so it must match too.
func bodiesWithGOMAXPROCS(t *testing.T, index typesys.Index, procs int) ([]diagnostic.Diagnostic, []string) {
	t.Helper()
	previous := runtime.GOMAXPROCS(procs)
	defer runtime.GOMAXPROCS(previous)
	a := NewAnalyzer()
	a.prepareAnalysisContext(index, AnalyzeOptions{Diagnostics: true})
	index = prepareAnalysisIndexWithSources(index, a.sources)
	a.prepareAnalysisModel(index)
	view := buildSemaTypeMemberState(index, nil, a.sources).view()
	a.checkMemberTypes(index, view)
	diagnostics := a.checkMethodBodiesWithView(index, view, nil)
	keys := make([]string, 0, len(view.hydrated))
	for key := range view.hydrated {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return diagnostics, keys
}

func TestSpeculativeBodiesLeaveTheSerialViewHistory(t *testing.T) {
	index, ok := methodBodyWorkersIndex(methodBodyWorkersProject(t, 60))
	if !ok {
		t.Fatal("generated project did not load")
	}
	wantDiagnostics, wantKeys := bodiesWithGOMAXPROCS(t, index, 1)
	gotDiagnostics, gotKeys := bodiesWithGOMAXPROCS(t, index, 4)
	if !reflect.DeepEqual(gotDiagnostics, wantDiagnostics) {
		t.Fatalf("diagnostics differ between GOMAXPROCS=1 and 4")
	}
	if len(wantKeys) == 0 {
		t.Fatal("serial run hydrated nothing; the history comparison would prove nothing")
	}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("hydrated history differs\nserial:   %v\nparallel: %v", wantKeys, gotKeys)
	}
}

// An earlier type's hydration can silence a later type's diagnostic. Workers
// that have not seen that type must not report it: the run falls back to the
// serial pass.
func TestSpeculativeBodiesFallBackWhenEarlierHydrationMatters(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "LaterDiagnostics.cls")
	source := "public class LaterDiagnostics {\n    public void run() {\n        OverlayOnly = 1;\n    }\n}\n"
	writeSemaFile(t, file, source)
	methodStart := strings.Index(source, "public void run()")
	methodEnd := strings.LastIndex(source, "    }") + len("    }")
	types := []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "DuplicateWithoutBody", Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationField, Name: "Seed", Type: "OverlaySeed__c"}}},
		{Kind: apexast.DeclarationClass, Name: "DuplicateWithoutBody"},
		{Kind: apexast.DeclarationClass, Name: "Filler"},
		{
			Kind: apexast.DeclarationClass, Name: "LaterDiagnostics", File: file,
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationMethod, Name: "run", Type: "void",
				BodyRange: semaParserBodyRange(source, methodStart, methodEnd),
				Range:     diagnostic.Range{Start: diagnostic.Position{Offset: methodStart}, End: diagnostic.Position{Offset: methodEnd}},
			}},
		},
	}
	index := typesys.Index{Types: types}
	run := func(procs int) []diagnostic.Diagnostic {
		previous := runtime.GOMAXPROCS(procs)
		defer runtime.GOMAXPROCS(previous)
		lazyPlatform := newSemaLazyPlatformTypeMemberModel(nil)
		lazyPlatform.lookups.Store(normalizeName("OverlaySeed__c"), semaPlatformTypeMemberLookup{
			ok: true,
			members: typeMembers{
				name: "OverlaySeed__c", kind: apexast.DeclarationClass,
				fields:  map[string]typesys.MemberSymbol{normalizeName("OverlayOnly"): {Kind: apexast.DeclarationField, Name: "OverlayOnly", Type: "Integer"}},
				methods: make(map[string][]typesys.MemberSymbol),
			},
		})
		base := buildTypeMemberLayerWithSources(index, newSemaSources(nil, nil), nil)
		state := newSemaTypeMemberStateWithPlatform(base, &semaTypeMemberModel{members: make(map[string]typeMembers), platform: lazyPlatform})
		return NewAnalyzer().checkMethodBodiesWithView(index, state.view(), nil)
	}
	want := run(1)
	for _, item := range want {
		if strings.Contains(item.Message, "assigns unknown variable") {
			t.Fatalf("serial run must silence the assignment diagnostic: %v", want)
		}
	}
	for _, procs := range []int{2, 4, 8} {
		if got := run(procs); !reflect.DeepEqual(got, want) {
			t.Fatalf("GOMAXPROCS=%d differs from serial\nserial: %v\n   got: %v", procs, want, got)
		}
	}
}
