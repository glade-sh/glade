package sema

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// inheritedFinalProject writes classes in the shape the inherited-final
// helpers inspect: a zero-argument constructor that assigns a final field the
// class does not own.
func inheritedFinalProject(t *testing.T) string {
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
	write("FinalBase", "public virtual class FinalBase {\n protected static final Integer shared = 1;\n private final Integer secret = 1;\n}\n")
	write("FinalChild", "public class FinalChild extends FinalBase {\n public FinalChild() { FinalBase.shared = 7; }\n}\n")
	write("FinalOther", "public class FinalOther extends FinalBase {\n public FinalOther() { secret = 7; }\n}\n")
	return root
}

// The anonymous-declarations route analyzes the project and then runs the
// lifecycle pass again over the transient file's diagnostics. Both must read
// the one member model the analysis built.
func TestAnonymousDeclarationsInContextBuildsOneTypeMemberModel(t *testing.T) {
	index, ok := memberViewSharingIndex(t, inheritedFinalProject(t))
	if !ok {
		t.Fatal("generated project did not load")
	}
	var transient typesys.Index
	transient.Project = index.Project
	for _, typ := range index.Types {
		if typ.Name == "FinalChild" || typ.Name == "FinalOther" {
			transient.Types = append(transient.Types, typ)
		}
	}
	if len(transient.Types) != 2 {
		t.Fatalf("transient types = %d, want 2", len(transient.Types))
	}
	AnalyzeAnonymousDeclarationsInContext(index, transient)
	before := semaTypeMemberLayerBuilds.Load()
	result := AnalyzeAnonymousDeclarationsInContext(index, transient)
	if builds := semaTypeMemberLayerBuilds.Load() - before; builds != 1 {
		t.Fatalf("anonymous declaration analysis built %d type-member models, want exactly 1", builds)
	}
	reached := 0
	for _, d := range result.Diagnostics {
		if d.Code == "GLADESEMA010" || d.Code == "GLADESEMA027" {
			reached++
		}
	}
	if reached == 0 {
		t.Fatalf("the project must give the inherited-final helpers a candidate; diagnostics: %v", result.Diagnostics)
	}
}

// The helpers read class text through the analysis' source resolver, so a
// class an earlier pass read is a hit, not a second disk read.
func TestInheritedFinalHelpersReuseAnalysisSources(t *testing.T) {
	index, ok := memberViewSharingIndex(t, inheritedFinalProject(t))
	if !ok {
		t.Fatal("generated project did not load")
	}
	var input []diagnostic.Diagnostic
	for _, d := range Analyze(index).Diagnostics {
		if d.Code == "GLADESEMA010" || d.Code == "GLADESEMA027" {
			d.NativeMessage = ""
			input = append(input, d)
		}
	}
	if len(input) == 0 {
		t.Fatal("setup: the project produced no assignment diagnostics for the helpers")
	}
	var counters PerfCounters
	counters.Enabled = true
	recorder := newPerfRecorder(&counters)
	sources := newSemaSources(nil, &recorder)
	for _, typ := range index.Types {
		sources.rawForType(typ)
	}
	warm := recorder.counters
	state := buildSemaTypeMemberState(index, nil, sources)
	out := capturedInheritedFinalDiagnostics(index, state, sources, input)
	if recorder.counters.SourceArenaFallbackReads != warm.SourceArenaFallbackReads || recorder.counters.SourceArenaHits <= warm.SourceArenaHits {
		t.Fatalf("helpers did not read through the shared resolver: fallback reads %d -> %d, hits %d -> %d",
			warm.SourceArenaFallbackReads, recorder.counters.SourceArenaFallbackReads, warm.SourceArenaHits, recorder.counters.SourceArenaHits)
	}
	native := 0
	for _, d := range out {
		if strings.HasPrefix(d.NativeMessage, "Final members can only be assigned") || strings.HasPrefix(d.NativeMessage, "Variable is not visible") {
			native++
		}
	}
	if native == 0 {
		t.Fatalf("helpers produced no native text: %v", out)
	}
}

// A discarded speculative attempt must not leave the source reads counted
// twice, and its time must stay in the method-bodies phase.
func TestSpeculativeFallbackCountsSourcesOnce(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "LaterDiagnostics.cls")
	source := "public class LaterDiagnostics {\n    public void run() {\n        OverlayOnly = 1;\n    }\n}\n"
	writeSemaFile(t, file, source)
	methodStart := strings.Index(source, "public void run()")
	methodEnd := strings.LastIndex(source, "    }") + len("    }")
	index := typesys.Index{Types: []typesys.TypeSymbol{
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
	}}
	run := func(procs int) (PerfCounters, []diagnostic.Diagnostic) {
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
		var counters PerfCounters
		counters.Enabled = true
		recorder := newPerfRecorder(&counters)
		a := NewAnalyzer()
		a.sources = newSemaSources(nil, &recorder)
		diagnostics := a.checkMethodBodiesWithView(index, state.view(), &recorder)
		return recorder.counters, diagnostics
	}
	want, wantDiagnostics := run(1)
	if want.SourceArenaHits+want.SourceArenaMisses == 0 {
		t.Fatal("the serial run read no sources; the comparison would prove nothing")
	}
	got, gotDiagnostics := run(4)
	if !reflect.DeepEqual(gotDiagnostics, wantDiagnostics) {
		t.Fatalf("diagnostics differ\nserial:   %v\nfallback: %v", wantDiagnostics, gotDiagnostics)
	}
	if got.SourceArenaHits != want.SourceArenaHits || got.SourceArenaMisses != want.SourceArenaMisses || got.SourceArenaFallbackReads != want.SourceArenaFallbackReads {
		t.Fatalf("source counters after the fallback differ from the serial run\nserial:   hits=%d misses=%d fallback=%d\nfallback: hits=%d misses=%d fallback=%d",
			want.SourceArenaHits, want.SourceArenaMisses, want.SourceArenaFallbackReads,
			got.SourceArenaHits, got.SourceArenaMisses, got.SourceArenaFallbackReads)
	}
	if got.MethodBodies.Calls != want.MethodBodies.Calls {
		t.Fatalf("method-bodies calls = %d, want %d", got.MethodBodies.Calls, want.MethodBodies.Calls)
	}
}

func TestAddPhaseDurationKeepsCallCount(t *testing.T) {
	var counters PerfCounters
	counters.Enabled = true
	recorder := newPerfRecorder(&counters)
	started := recorder.beginPhase()
	recorder.endPhase(&recorder.counters.MethodBodies, started)
	calls := recorder.counters.MethodBodies.Calls
	before := recorder.counters.MethodBodies.DurationNS
	spent := recorder.beginPhase()
	for recorder.counters.MethodBodies.DurationNS == before && !spent.IsZero() {
		recorder.addPhaseDuration(&recorder.counters.MethodBodies, spent)
	}
	if recorder.counters.MethodBodies.Calls != calls {
		t.Fatalf("calls = %d, want %d", recorder.counters.MethodBodies.Calls, calls)
	}
	if recorder.counters.MethodBodies.DurationNS <= before {
		t.Fatal("addPhaseDuration added no time")
	}
	var off *perfRecorder
	off.addPhaseDuration(&PhaseCounters{}, started)
}

// A merged placeholder takes the hydrated entry's flags, so the next lookup
// does not hydrate it a second time.
func TestMergeHydratedTakesHydratedFlagsOverPlaceholder(t *testing.T) {
	state := newSemaTypeMemberState(&semaTypeMemberModel{members: make(map[string]typeMembers)})
	placeholder := func() typeMembers {
		return typeMembers{
			name: "Account", kind: apexast.DeclarationClass, platform: true, sobject: true, syntheticStandardSObject: true,
			fields:  make(map[string]typesys.MemberSymbol),
			methods: make(map[string][]typesys.MemberSymbol),
		}
	}
	held := placeholder()
	held.fields[normalizeName("Held__c")] = typesys.MemberSymbol{Kind: apexast.DeclarationField, Name: "Held__c", Type: "String"}
	dst, src := state.view(), state.view()
	dst.hydrated["account"] = held
	srcPlaceholder := placeholder()
	srcPlaceholder.fields[normalizeName("Other__c")] = typesys.MemberSymbol{Kind: apexast.DeclarationField, Name: "Other__c", Type: "String"}
	hydrated := semaEnsureStandardSObjectTypeMembers(src, "account", srcPlaceholder)
	if !hydrated.standardSObjectFieldsLoaded {
		t.Fatal("setup: hydration did not load the standard fields")
	}
	srcFieldCount := len(src.hydrated["account"].fields)
	dst.mergeHydrated(src)
	got := dst.hydrated["account"]
	if !got.standardSObjectFieldsLoaded {
		t.Fatal("merged entry kept the placeholder flag; the next lookup would rebuild it")
	}
	for _, name := range []string{"Held__c", "Other__c", "Name"} {
		if _, ok := got.fields[normalizeName(name)]; !ok {
			t.Fatalf("merged entry lost field %s", name)
		}
	}
	if len(src.hydrated["account"].fields) != srcFieldCount {
		t.Fatal("merge edited the source view's entry in place")
	}
	ensured := semaEnsureStandardSObjectTypeMembers(dst, "account", got)
	if !reflect.DeepEqual(ensured.fields, got.fields) {
		t.Fatal("a lookup after the merge changed the entry")
	}
}

// A worker's panic reaches the coordinator with the worker's stack, and still
// prints as the original value so the reported diagnostic is the serial one.
func TestWorkerPanicKeepsOriginalStack(t *testing.T) {
	index := typesys.Index{Types: []typesys.TypeSymbol{{Kind: apexast.DeclarationClass, Name: "Only"}}}
	result := NewAnalyzer().checkSemaMethodBodyTask(index, semaMethodBodyTask{typeIndex: 5}, nil, nil, nil, nil)
	wrapped, ok := result.recovered.(*semaWorkerPanic)
	if !ok {
		t.Fatalf("recovered = %T, want *semaWorkerPanic", result.recovered)
	}
	if !strings.Contains(string(wrapped.stack), "checkSemaMethodBodyTask") {
		t.Fatalf("stack does not name the failing task:\n%s", wrapped.stack)
	}
	var original error
	func() {
		defer func() { original, _ = recover().(error) }()
		_ = index.Types[5]
	}()
	if got, want := fmt.Sprintf("%v", wrapped), original.Error(); got != want {
		t.Fatalf("%%v = %q, want the original %q", got, want)
	}
	if !strings.Contains(fmt.Sprintf("%+v", wrapped), "checkSemaMethodBodyTask") {
		t.Fatal("the plus verb does not print the stack")
	}
	if again := semaCapturePanic(wrapped); again != wrapped {
		t.Fatal("a wrapped panic must keep its first stack")
	}
	var runtimeErr interface{ RuntimeError() }
	if !errors.As(wrapped, &runtimeErr) {
		t.Fatal("the wrapper hides the original error")
	}
}
