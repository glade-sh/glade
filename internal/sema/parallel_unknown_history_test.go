package sema

import (
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
