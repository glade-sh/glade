package vm

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// referenceObjectNameForKeyPrefix is the sorted scan the prefix lookup is
// defined by: the first object in name order whose key prefix matches.
func referenceObjectNameForKeyPrefix(org *storage.OrgState, prefix string) (string, bool) {
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if org.Objects[name].Definition.KeyPrefix == prefix {
			return name, true
		}
	}
	return "", false
}

// referenceObjectNameHoldingID is the sorted scan the Id lookup is defined
// by: the first object in name order that stores the Id in either form.
func referenceObjectNameHoldingID(org *storage.OrgState, id storage.ID) (string, bool) {
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, _, ok := storage.LookupRecordByID(org.Objects[name].Records, id); ok {
			return name, true
		}
	}
	return "", false
}

// idLookupOrg builds objects that share key prefixes and store overlapping
// Ids in 15 and 18 character forms, so name order decides every answer.
func idLookupOrg(rng *rand.Rand, objects int) (storage.OrgState, []string, []storage.ID) {
	org := storage.NewOrgState()
	prefixes := []string{"", "001", "003", "a00", "a01", "a0B", "m00", "zzz"}
	var ids []storage.ID
	for i := 0; i < 24; i++ {
		ids = append(ids, storage.ID(fmt.Sprintf("%s%012dAAA", prefixes[1+rng.Intn(len(prefixes)-1)], i)))
	}
	for i := 0; i < objects; i++ {
		name := fmt.Sprintf("%c%c_Object%d__c", 'A'+rune(rng.Intn(26)), 'a'+rune(rng.Intn(26)), i)
		state := storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, KeyPrefix: prefixes[rng.Intn(len(prefixes))]}}
		switch rng.Intn(4) {
		case 0:
			// No record map at all.
		case 1:
			state.Records = map[storage.ID]storage.Record{}
		default:
			state.Records = map[storage.ID]storage.Record{}
			for n := rng.Intn(4); n > 0; n-- {
				id := ids[rng.Intn(len(ids))]
				if rng.Intn(2) == 0 {
					id = id[:15]
				}
				state.Records[id] = storage.Record{ID: id}
			}
		}
		org.Objects[name] = state
	}
	return org, prefixes, ids
}

func TestObjectNameForIDLookupsMatchSortedScan(t *testing.T) {
	rng := rand.New(rand.NewSource(20261111))
	for round := 0; round < 60; round++ {
		org, prefixes, ids := idLookupOrg(rng, 1+rng.Intn(60))
		machine := New(nil)
		machine.Org = &org
		for _, prefix := range append(prefixes, "00Q", "xyz") {
			wantName, wantOK := referenceObjectNameForKeyPrefix(&org, prefix)
			gotName, gotOK := machine.orgObjectNameForKeyPrefix(prefix)
			if gotName != wantName || gotOK != wantOK {
				t.Fatalf("round %d prefix %q: got %q, %v; want %q, %v", round, prefix, gotName, gotOK, wantName, wantOK)
			}
		}
		probes := append([]storage.ID{"", "001", "001zzzzzzzzzzzz", "001zzzzzzzzzzzzAAA"}, ids...)
		for _, id := range ids {
			probes = append(probes, id[:15], id[:15]+"BBB")
		}
		for _, id := range probes {
			wantName, wantOK := referenceObjectNameHoldingID(&org, id)
			gotName, gotOK := machine.orgObjectNameHoldingID(id)
			if gotName != wantName || gotOK != wantOK {
				t.Fatalf("round %d id %q: got %q, %v; want %q, %v", round, id, gotName, gotOK, wantName, wantOK)
			}
		}
	}
}

// The lookups run for every Id a rollup or trigger framework touches, on an
// org of several thousand standard objects. They must not allocate.
func TestObjectNameForIDLookupsDoNotAllocate(t *testing.T) {
	org := storage.NewOrgState()
	for i := 0; i < 3000; i++ {
		name := fmt.Sprintf("Object%04d__c", i)
		org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, KeyPrefix: fmt.Sprintf("a%02d", i%100)}}
	}
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001"},
		Records:    map[storage.ID]storage.Record{"001000000000001AAA": {ID: "001000000000001AAA"}},
	}
	machine := New(nil)
	machine.Org = &org
	if name, ok := machine.sObjectNameForIDPrefix("001"); !ok || name != "Account" {
		t.Fatalf("prefix lookup = %q, %v", name, ok)
	}
	if name, ok := machine.sObjectNameForID("001000000000001AAA"); !ok || name != "Account" {
		t.Fatalf("id lookup = %q, %v", name, ok)
	}
	if allocs := testing.AllocsPerRun(20, func() { machine.sObjectNameForIDPrefix("001") }); allocs != 0 {
		t.Fatalf("prefix lookup allocates %.0f times per call", allocs)
	}
	if allocs := testing.AllocsPerRun(20, func() { machine.sObjectNameForID("001000000000001AAA") }); allocs != 0 {
		t.Fatalf("id lookup allocates %.0f times per call", allocs)
	}
}

func BenchmarkSObjectNameForIDPrefix(b *testing.B) {
	org := storage.NewOrgState()
	storage.EnsureStandardObjectSpread(&org)
	machine := New(nil)
	machine.Org = &org
	b.ReportAllocs()
	for b.Loop() {
		machine.sObjectNameForIDPrefix("003")
	}
}
