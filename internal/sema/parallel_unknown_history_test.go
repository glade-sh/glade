package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

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
