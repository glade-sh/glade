package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// One unresolved assignment that cannot intersect any hydrated field keeps the
// parallel result (no serial retry), so this measures the kept-pass cost of
// recording the miss alone.
func BenchmarkParallelUnknownAssignment(b *testing.B) {
	root := b.TempDir()
	var files []string
	for i := 0; i < 200; i++ {
		name := fmt.Sprintf("Service%04d", i)
		missing := ""
		if i == 199 {
			missing = "unknownTarget = 1;"
		}
		source := fmt.Sprintf(`public class %s {
            public Integer run(Integer value) {
                Integer total = value;
                for (Integer i = 0; i < 10; i++) { total = total + i; }
                %s
                return total;
            }
        }`, name, missing)
		file := filepath.Join(root, name+".cls")
		writeApexBenchmarkFile(b, file, source)
		files = append(files, file)
	}
	index := typesys.Build(project.Project{Root: root, ApexFiles: files}, schema.Schema{})
	want := NewAnalyzer().checkMethodBodiesWithViewWorkers(index, buildSemaTypeMemberState(index, nil).view(), nil, 1)
	if len(want) == 0 {
		b.Fatal("fixture did not produce an unknown assignment diagnostic")
	}
	a := NewAnalyzer()
	b.ReportAllocs()
	b.ResetTimer()
	var got = want
	for i := 0; i < b.N; i++ {
		got = a.checkMethodBodies(index)
	}
	b.StopTimer()
	if !reflect.DeepEqual(got, want) {
		b.Fatalf("diagnostics differ: serial=%v parallel=%v", want, got)
	}
}

// speculativeFallbackServices writes the service classes of the fallback
// fixture: bodies that read SObjects, run SOQL and DML, and call other
// classes, with the first two classes and every tenth also iterating an
// Opportunity query so body checks hydrate more than one standard object.
func speculativeFallbackServices(tb testing.TB, root string, classes int) {
	tb.Helper()
	classesDir := filepath.Join(root, "force-app/main/default/classes")
	if err := os.MkdirAll(classesDir, 0o755); err != nil {
		tb.Fatal(err)
	}
	writeApexBenchmarkFile(tb, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeApexBenchmarkFile(tb, filepath.Join(classesDir, "BenchmarkBase.cls"), "public virtual class BenchmarkBase {\n public virtual Integer calculate(Integer input) { return input; }\n}\n")
	for i := 0; i < classes; i++ {
		name := fmt.Sprintf("Service%04d", i)
		opportunity := ""
		if i < 2 || i%10 == 5 {
			opportunity = " List<Opportunity> opportunities = [SELECT Id, StageName FROM Opportunity LIMIT 10];\n        for (Opportunity opportunityRecord : opportunities) { String stage = opportunityRecord.StageName; }"
		}
		source := fmt.Sprintf(`public class %[1]s extends BenchmarkBase {
    public Integer total;
    public %[1]s() { Integer seed = 0; }
    public override Integer calculate(Integer input) { return input + total; }
    public List<Account> run(List<Account> records) {
        Map<String, List<Account>> grouped = new Map<String, List<Account>>();
        Integer sum = this.calculate(records.size());
        for (Account accountRecord : records) {
            String localName = accountRecord.Name;
            grouped.put(localName, records);
        }
        List<Account> queried = [SELECT Id, Name, AnnualRevenue FROM Account LIMIT 50];%[2]s
        update queried;
        Contact contact = new Contact(LastName = 'x');
        insert contact;
        return queried;
    }
}
`, name, opportunity)
		writeApexBenchmarkFile(tb, filepath.Join(classesDir, name+".cls"), source)
	}
}

// speculativeFallbackFixture is the prepared analysis state of the fallback
// fixture. Both shapes carry the same realistic services and the same
// mid-edit class whose method assigns overlayOnly without declaring it;
// retry selects what that miss intersects:
//
//   - without retry the miss matches no hydrated field, so the speculative
//     pass keeps its parallel result. This is the realistic mid-edit project.
//   - with retry the services are preceded by file-less duplicate types, one
//     declaring a field typed OverlaySeed__c, and the platform carries a lazy
//     entry for that name. The duplicate-type overlay resolves it through the
//     view, which no source-backed route can do (the enriched member model
//     materializes every source-referenced object first), so the unresolved
//     assignment intersects a field only body hydration reveals and the pass
//     pays the parallel attempt plus the whole serial retry.
type speculativeFallbackFixture struct {
	index    typesys.Index
	state    *semaTypeMemberState
	analyzer *Analyzer
}

func buildSpeculativeFallbackFixture(tb testing.TB, classes int, retry bool) speculativeFallbackFixture {
	tb.Helper()
	root := tb.TempDir()
	speculativeFallbackServices(tb, root, classes)
	midEditSource := "public class MidEdit {\n public void halfTyped() { overlayOnly = 1; }\n}\n"
	midEditFile := filepath.Join(root, "force-app/main/default/classes", "MidEdit.cls")
	writeApexBenchmarkFile(tb, midEditFile, midEditSource)
	p, err := project.Load(root)
	if err != nil {
		tb.Fatalf("load fixture project: %v", err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		tb.Fatalf("load fixture schema: %v", err)
	}
	index := typesys.Build(p, s)
	// The duplicate-type shape precedes the services so the class that misses
	// lands on a worker that has not seen the overlay: the first worker
	// resolves the overlay's field type while a later worker checks the body
	// that assigns a field of it without declaring the variable.
	overlayTypes := []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "DuplicateWithoutBody", Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationField, Name: "Seed", Type: "OverlaySeed__c"}}},
		{Kind: apexast.DeclarationClass, Name: "DuplicateWithoutBody"},
		{Kind: apexast.DeclarationClass, Name: "Filler"},
	}
	methodStart := len("public class MidEdit {\n ")
	methodEnd := len(midEditSource) - len("\n}\n")
	overlayTypes = append(overlayTypes, typesys.TypeSymbol{
		Kind: apexast.DeclarationClass, Name: "MidEdit", File: midEditFile,
		Members: []typesys.MemberSymbol{{
			Kind: apexast.DeclarationMethod, Name: "halfTyped", Type: "void",
			BodyRange: semaParserBodyRange(midEditSource, methodStart, methodEnd),
			Range:     diagnostic.Range{Start: diagnostic.Position{Offset: methodStart}, End: diagnostic.Position{Offset: methodEnd}},
		}},
	})
	index.Types = append(overlayTypes, index.Types...)
	analyzer := NewAnalyzer()
	analyzer.prepareAnalysisContext(index, AnalyzeOptions{Diagnostics: true})
	index = prepareAnalysisIndexWithSources(index, analyzer.sources)
	analyzer.prepareAnalysisModel(index)
	lazyPlatform := newSemaLazyPlatformTypeMemberModel(nil)
	if retry {
		lazyPlatform.lookups.Store(normalizeName("OverlaySeed__c"), semaPlatformTypeMemberLookup{
			ok: true,
			members: typeMembers{
				name: "OverlaySeed__c", kind: apexast.DeclarationClass,
				fields:  map[string]typesys.MemberSymbol{normalizeName("OverlayOnly"): {Kind: apexast.DeclarationField, Name: "OverlayOnly", Type: "Integer"}},
				methods: make(map[string][]typesys.MemberSymbol),
			},
		})
	}
	base := buildTypeMemberLayerWithSources(index, analyzer.sources, nil)
	return speculativeFallbackFixture{
		index:    index,
		state:    newSemaTypeMemberStateWithPlatform(base, &semaTypeMemberModel{members: make(map[string]typeMembers), platform: lazyPlatform}),
		analyzer: analyzer,
	}
}

// The fixture must keep the two behaviors apart: the mid-edit miss alone
// never conflicts, and the injected lazy entry always makes it conflict. If
// this guard fails the benchmark above no longer measures what its name says.
func TestSpeculativeFallbackFixtureShapes(t *testing.T) {
	previous := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(previous)
	for _, retry := range []bool{false, true} {
		fixture := buildSpeculativeFallbackFixture(t, 40, retry)
		view := fixture.state.view()
		watch := newSemaUnknownHistory()
		watch.seed(view)
		scratch := view.cloneOnto(&semaTypeMemberState{base: fixture.state.base, platform: fixture.state.platform, unknownHistory: watch})
		run := &semaBodyRun{seedWorkers: true}
		got, recovered := fixture.analyzer.checkMethodBodiesRecovering(fixture.index, scratch, 4, run)
		if recovered != nil {
			t.Fatalf("retry=%v: speculative pass panicked: %v", retry, recovered)
		}
		if !watch.hasMissing() {
			t.Fatalf("retry=%v: fixture recorded no miss; the measurement would prove nothing", retry)
		}
		if watch.conflict() != retry {
			t.Fatalf("retry=%v: watch conflict = %v, missing = %v", retry, watch.conflict(), watch.hasMissing())
		}
		// Without a conflict the attempt is kept, so it already reports what
		// one worker reports. With a conflict the discarded attempt may
		// differ; the full pass below checks what callers receive.
		want := NewAnalyzer().checkMethodBodiesWithViewWorkers(fixture.index, fixture.state.view(), nil, 1)
		if !retry && !reflect.DeepEqual(got, want) {
			t.Fatalf("retry=false: kept attempt differs from one worker\nserial: %v\ngot:   %v", want, got)
		}
		runtime.GOMAXPROCS(4)
		parallel := NewAnalyzer().checkMethodBodiesWithView(fixture.index, fixture.state.view(), nil)
		runtime.GOMAXPROCS(1)
		serial := NewAnalyzer().checkMethodBodiesWithView(fixture.index, fixture.state.view(), nil)
		if !reflect.DeepEqual(parallel, serial) {
			t.Fatalf("retry=%v: diagnostics differ between the parallel and serial pass\nserial: %v\ngot:   %v", retry, serial, parallel)
		}
	}
}

// BenchmarkSpeculativeUnknownAssignmentFallback measures what the serial retry
// of the speculative body pass costs on a realistic large project when an
// unresolved assignment target does intersect a lazily hydrated field. Only
// file-less types with injected lazy platform entries can construct that shape
// today (see TestUnresolvedAssignmentsNeverReachTheSerialFallback), so this is
// the ceiling the retry would cost if a source-backed route ever reached it.
func BenchmarkSpeculativeUnknownAssignmentFallback(b *testing.B) {
	for _, classes := range []int{200, 600} {
		classes := classes
		b.Run(fmt.Sprintf("size=%d", classes), func(b *testing.B) {
			for _, mode := range []struct {
				name  string
				retry bool
				procs int
			}{
				{name: "parallel", retry: false, procs: 4},
				{name: "fallback", retry: true, procs: 4},
				{name: "serial", retry: true, procs: 1},
			} {
				mode := mode
				b.Run("mode="+mode.name, func(b *testing.B) {
					fixture := buildSpeculativeFallbackFixture(b, classes, mode.retry)
					previous := runtime.GOMAXPROCS(mode.procs)
					defer runtime.GOMAXPROCS(previous)
					warm := NewAnalyzer().checkMethodBodiesWithView(fixture.index, fixture.state.view(), nil)
					b.ReportAllocs()
					b.ResetTimer()
					got := warm
					for i := 0; i < b.N; i++ {
						got = NewAnalyzer().checkMethodBodiesWithView(fixture.index, fixture.state.view(), nil)
					}
					b.StopTimer()
					if !reflect.DeepEqual(got, warm) {
						b.Fatalf("measured diagnostics differ from the untimed reference\nwant: %v\ngot:  %v", warm, got)
					}
					runtime.GOMAXPROCS(1)
					serial := NewAnalyzer().checkMethodBodiesWithView(fixture.index, fixture.state.view(), nil)
					runtime.GOMAXPROCS(mode.procs)
					if !reflect.DeepEqual(got, serial) {
						b.Fatalf("measured diagnostics differ from one worker\nserial: %v\ngot:   %v", serial, got)
					}
				})
			}
		})
	}
}
