package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// speculativeAttemptProbe runs the speculative body attempt the way
// checkMethodBodiesSpeculative does over a loaded project index, and reports
// whether the unknown-assignment watch recorded a miss and whether that miss
// intersects a hydrated field (the serial-retry condition). The warm-up is a
// subset of the production passes, so a view here can only know less than a
// production view: a probe that reports no conflict proves the production pass
// has none either.
func speculativeAttemptProbe(t *testing.T, index typesys.Index, workers int) (conflict, missing, panicked bool) {
	t.Helper()
	a := NewAnalyzer()
	a.prepareAnalysisContext(index, AnalyzeOptions{Diagnostics: true})
	index = prepareAnalysisIndexWithSources(index, a.sources)
	a.prepareAnalysisModel(index)
	state := buildSemaTypeMemberState(index, nil, a.sources)
	view := state.view()
	a.checkMemberTypes(index, view)
	a.checkMethodParameters(index, view)
	watch := newSemaUnknownHistory()
	watch.seed(view)
	scratch := view.cloneOnto(&semaTypeMemberState{base: state.base, platform: state.platform, unknownHistory: watch})
	run := &semaBodyRun{seedWorkers: true}
	_, recovered := a.checkMethodBodiesRecovering(index, scratch, workers, run)
	return watch.conflict(), watch.hasMissing(), recovered != nil
}

// unresolvedAssignmentProject writes a source-backed project with realistic
// service bodies plus one mid-edit class whose method assigns target without
// declaring it.
func unresolvedAssignmentProject(t *testing.T, classes int, target string) string {
	t.Helper()
	root := t.TempDir()
	speculativeFallbackServices(t, root, classes)
	source := fmt.Sprintf("public class MidEdit {\n public void halfTyped() { %s; }\n}\n", target)
	writeApexBenchmarkFile(t, filepath.Join(root, "force-app/main/default/classes", "MidEdit.cls"), source)
	return root
}

// An unresolved assignment target in a source-backed project cannot reach the
// serial fallback of the speculative body pass. The analysis pipeline enriches
// the index with every source-referenced object and the member model
// materializes their fields before bodies run, so a missing assignment name
// either matches no hydrated field (the parallel result is kept, no retry) or
// is not missing at all. Only file-less types with injected lazy platform
// entries construct a conflict today; BenchmarkSpeculativeUnknownAssignmentFallback
// measures what that retry costs. If this probe ever reports a conflict on a
// source-backed project, the fallback became reachable and that cost applies
// to real analyses: re-triage before silencing this test.
func TestUnresolvedAssignmentsNeverReachTheSerialFallback(t *testing.T) {
	type project struct {
		name     string
		root     string
		wantMiss bool
	}
	projects := []project{
		{name: "typo", root: unresolvedAssignmentProject(t, 40, "unknownTarget = 1"), wantMiss: true},
		{name: "sobject-field", root: unresolvedAssignmentProject(t, 40, "stageName = 'Closed'"), wantMiss: false},
	}
	for _, root := range methodBodyWorkersRoots(t) {
		projects = append(projects, project{name: strings.TrimPrefix(root, "../../"), root: root})
	}
	for _, item := range projects {
		index, ok := methodBodyWorkersIndex(item.root)
		if !ok {
			t.Fatalf("%s: load project", item.name)
		}
		for _, workers := range []int{2, len(index.Types)} {
			if workers < 2 {
				continue
			}
			conflict, missing, panicked := speculativeAttemptProbe(t, index, workers)
			if panicked {
				t.Logf("%s workers=%d: speculative attempt recovered a panic; the GOMAXPROCS differential tests cover parity", item.name, workers)
			}
			if conflict {
				t.Fatalf("%s workers=%d: the speculative pass falls back to the serial run; the serial-retry cost now applies to a source-backed project", item.name, workers)
			}
			if item.wantMiss && !missing {
				t.Fatalf("%s workers=%d: fixture recorded no miss; the probe no longer exercises an unresolved assignment", item.name, workers)
			}
			if !item.wantMiss && missing {
				t.Fatalf("%s workers=%d: fixture recorded a miss; a name the services hydrate is no longer known before bodies run", item.name, workers)
			}
		}
	}
}

// A missing target need only trigger a serial retry if a worker ever
// hydrated a field with that name, even if the entry was later replaced.
func TestParallelUnknownHistoryTracksReplacedFields(t *testing.T) {
	watch := newSemaUnknownHistory()
	var workers sync.WaitGroup
	for _, name := range []string{"Missing", "Transient"} {
		workers.Add(1)
		go func(name string) { defer workers.Done(); watch.missing(name) }(name)
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		watch.hydrated(typeMembers{fields: map[string]typesys.MemberSymbol{"transient": {Name: "Transient"}}})
		watch.hydrated(typeMembers{fields: map[string]typesys.MemberSymbol{}})
	}()
	workers.Wait()
	if !watch.conflict() {
		t.Fatal("replaced hydrated field must force a serial retry")
	}
	safe := newSemaUnknownHistory()
	safe.missing("Missing")
	safe.hydrated(typeMembers{fields: map[string]typesys.MemberSymbol{"other": {Name: "Other"}}})
	if safe.conflict() {
		t.Fatal("unrelated field must not force a serial retry")
	}
}

func TestParallelUnknownHistoryObservesViewHydration(t *testing.T) {
	watch := newSemaUnknownHistory()
	view := (&semaTypeMemberState{unknownHistory: watch}).view()
	if semaAnyKnownField(view, "Target") {
		t.Fatal("unexpected known field")
	}
	view.storeHydrated("object", typeMembers{fields: map[string]typesys.MemberSymbol{"target": {Name: "Target"}}})
	view.storeHydrated("object", typeMembers{fields: map[string]typesys.MemberSymbol{}})
	if !watch.conflict() {
		t.Fatal("view must record a field even if replaced")
	}
}

func TestParallelUnknownAssignmentMatchesSerialWithoutRetry(t *testing.T) {
	root := t.TempDir()
	classes := filepath.Join(root, "force-app/main/default/classes")
	if err := os.MkdirAll(classes, 0755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("Worker%d", i)
		src := fmt.Sprintf("public class %s { Integer present; public void run() { present = 1; neverDeclared = 1; } }", name)
		writeSemaFile(t, filepath.Join(classes, name+".cls"), src)
		writeSemaFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	index, ok := methodBodyWorkersIndex(root)
	if !ok {
		t.Fatal("load synthetic project")
	}
	want := bodyDiagnosticsWithWorkers(index, 1)
	if len(want) == 0 {
		t.Fatal("missing assignment produced no diagnostic")
	}
	a := NewAnalyzer()
	a.prepareAnalysisContext(index, AnalyzeOptions{Diagnostics: true})
	index = prepareAnalysisIndexWithSources(index, a.sources)
	a.prepareAnalysisModel(index)
	state := buildSemaTypeMemberState(index, nil, a.sources)
	view := state.view()
	a.checkMemberTypes(index, view)
	a.checkMethodParameters(index, view)
	watch := newSemaUnknownHistory()
	scratch := view.cloneOnto(&semaTypeMemberState{base: state.base, platform: state.platform, unknownHistory: watch})
	run := &semaBodyRun{seedWorkers: true}
	got, recovered := a.checkMethodBodiesRecovering(index, scratch, 4, run)
	if recovered != nil {
		t.Fatalf("parallel pass panicked: %v", recovered)
	}
	if len(run.views) < 2 || !watch.hasMissing() || watch.conflict() {
		t.Fatalf("expected parallel pass with an unambiguous missing target: views=%d missing=%v conflict=%v", len(run.views), watch.hasMissing(), watch.conflict())
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostics differ\nserial: %v\nparallel: %v", want, got)
	}
}

func TestParallelUnknownHistoryObservesPlatformLookup(t *testing.T) {
	watch := newSemaUnknownHistory()
	lazy := newSemaLazyPlatformTypeMemberModel(nil)
	lazy.lookups.Store("example", semaPlatformTypeMemberLookup{ok: true, members: typeMembers{fields: map[string]typesys.MemberSymbol{"target": {Name: "Target"}}}})
	state := &semaTypeMemberState{platform: &semaTypeMemberModel{members: make(map[string]typeMembers), platform: lazy}, unknownHistory: watch}
	view := state.view()
	if semaAnyKnownField(view, "TARGET") {
		t.Fatal("field is not hydrated yet")
	}
	if _, ok := view.lookup("example"); !ok {
		t.Fatal("platform lookup failed")
	}
	if !watch.conflict() {
		t.Fatal("platform lookup did not record the case-insensitive field")
	}
}

func TestParallelUnknownHistoryIncludesSeededFields(t *testing.T) {
	watch := newSemaUnknownHistory()
	view := (&semaTypeMemberState{}).view()
	view.storeHydrated("object", typeMembers{fields: map[string]typesys.MemberSymbol{"target": {Name: "Target"}}})
	watch.seed(view)
	view.storeHydrated("object", typeMembers{fields: map[string]typesys.MemberSymbol{}})
	watch.missing("Target")
	if !watch.conflict() {
		t.Fatal("replaced initial field must force a serial retry")
	}
}
