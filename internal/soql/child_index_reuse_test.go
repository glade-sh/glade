package soql

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Record tokens cover map writes, but not edits to a row shared by other maps.
func TestChildRelationshipIndexReuseValidation(t *testing.T) {
	org := childIndexBenchOrg(2, 1)
	child := org.Objects["Contact"]
	var owner, alias storage.ObjectState
	owner.Definition = child.Definition
	owner.SetRecords(child.Records)
	alias.SetRecords(child.Records)
	org.Objects["Contact"] = owner
	cache := NewExecutionCache()
	index := func() map[storage.ID][]string {
		return childRelationshipIndex(org, "Contact", owner, "AccountId", newChildRelationshipQueryCache(cache))
	}
	first := index()
	second := index()
	if reflect.ValueOf(first).UnsafePointer() != reflect.ValueOf(second).UnsafePointer() {
		t.Fatal("unchanged rows must reuse the child index")
	}
	id := storage.ID("003000000000000")
	row := alias.Records[id]
	before, ok := owner.RecordsToken()
	if !ok {
		t.Fatal("expected versioned owner")
	}
	row.Fields["AccountId"] = storage.IDValue("001000000000001")
	alias.PutRecord(id, row)
	after, _ := owner.RecordsToken()
	if before != after {
		t.Fatal("test setup: alias edit moved owner token")
	}
	got := index()
	want := childRelationshipIndex(org, "Contact", owner, "AccountId", nil)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("alias edit: got %v, want %v", got, want)
	}
}

func BenchmarkChildRelationshipPointQueries(b *testing.B) {
	org := childIndexBenchOrg(200, 3)
	child := org.Objects["Contact"]
	var versioned storage.ObjectState
	versioned.Definition = child.Definition
	versioned.SetRecords(child.Records)
	org.Objects["Contact"] = versioned
	query, err := Parse("SELECT Id,(SELECT Id,AccountId FROM Contacts) FROM Account WHERE Id = '001000000000001'")
	if err != nil {
		b.Fatal(err)
	}
	cache := NewExecutionCache()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := ExecuteWithCache(org, query, cache)
		if err != nil {
			b.Fatal(err)
		}
		if len(result.Records) != 1 || len(result.Records[0].Children["Contacts"]) != 3 {
			b.Fatalf("unexpected result: %#v", result)
		}
	}
}

func TestChildRelationshipIndexMatchesFreshAfterWrites(t *testing.T) {
	org := childIndexBenchOrg(2, 2)
	child := org.Objects["Contact"]
	var versioned storage.ObjectState
	versioned.Definition = child.Definition
	versioned.SetRecords(child.Records)
	org.Objects["Contact"] = versioned
	query, err := Parse("SELECT Id,(SELECT Id,AccountId FROM Contacts) FROM Account ORDER BY Id")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	compare := func(label string) {
		t.Helper()
		got, err := ExecuteWithCache(org, query, cache)
		if err != nil {
			t.Fatalf("%s cached: %v", label, err)
		}
		want, err := ExecuteWithCache(org, query, NewExecutionCache())
		if err != nil {
			t.Fatalf("%s fresh: %v", label, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: cached %#v, fresh %#v", label, got, want)
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("outcome %s %x", label, sha256.Sum256(encoded))
	}
	compare("first")
	compare("warm")
	id := storage.ID("003000000000000")
	child = org.Objects["Contact"]
	row := child.Records[id].Clone()
	row.Fields["AccountId"] = storage.IDValue("001000000000001")
	child.PutRecord(id, row)
	org.Objects["Contact"] = child
	compare("move")
	row.System.IsDeleted = true
	child.PutRecord(id, row)
	org.Objects["Contact"] = child
	compare("delete")
	row.System.IsDeleted = false
	child.PutRecord(id, row)
	org.Objects["Contact"] = child
	compare("undelete")
	child.DeleteRecord(id)
	org.Objects["Contact"] = child
	compare("remove")
	child.PutRecord(id, row)
	org.Objects["Contact"] = child
	compare("insert")
	row.ExplicitNulls = map[string]bool{"AccountId": true}
	child.PutRecord(id, row)
	org.Objects["Contact"] = child
	compare("explicit null")
	child.SetRecords(map[storage.ID]storage.Record{})
	org.Objects["Contact"] = child
	compare("replace map")
	row.ExplicitNulls = nil
	child.PutRecord(id, row)
	org.Objects["Contact"] = child
	compare("insert after replacement")
	child.ClearRecords()
	org.Objects["Contact"] = child
	compare("clear")
}

func TestChildRelationshipIndexConcurrentReuse(t *testing.T) {
	org := childIndexBenchOrg(8, 3)
	original := org.Objects["Contact"]
	var versioned storage.ObjectState
	versioned.Definition = original.Definition
	versioned.SetRecords(original.Records)
	org.Objects["Contact"] = versioned
	other := org.Clone()
	child := other.Objects["Contact"]
	row := child.Records["003000000000000"].Clone()
	row.Fields["AccountId"] = storage.IDValue("001000000000001")
	child.PutRecord(row.ID, row)
	other.Objects["Contact"] = child
	orgs := []storage.OrgState{org, other}
	cache := NewExecutionCache()
	query, err := Parse("SELECT Id,(SELECT Id FROM Contacts) FROM Account ORDER BY Id")
	if err != nil {
		t.Fatal(err)
	}
	wants := make([]Result, len(orgs))
	for i, item := range orgs {
		wants[i], err = ExecuteWithCache(item, query, NewExecutionCache())
		if err != nil {
			t.Fatal(err)
		}
	}
	const workers = 8
	failures := make(chan error, workers)
	for worker := range workers {
		go func() {
			item, want := orgs[worker%len(orgs)], wants[worker%len(orgs)]
			for range 10 {
				got, err := ExecuteWithCache(item, query, cache)
				if err != nil {
					failures <- err
					return
				}
				if !reflect.DeepEqual(got, want) {
					failures <- fmt.Errorf("cached result differs from fresh result")
					return
				}
			}
			failures <- nil
		}()
	}
	for range workers {
		if err := <-failures; err != nil {
			t.Error(err)
		}
	}
}

func TestChildRelationshipIndexCacheScope(t *testing.T) {
	org := childIndexBenchOrg(2, 1)
	child := org.Objects["Contact"]
	child.Definition.Fields["OtherId"] = storage.Field{APIName: "OtherId", Type: storage.FieldReference}
	for id, row := range child.Records {
		row.Fields["OtherId"] = row.Fields["AccountId"]
		child.Records[id] = row
	}
	unversioned := child
	var versioned storage.ObjectState
	versioned.Definition = child.Definition
	versioned.SetRecords(child.Records)
	child = versioned
	cache := NewExecutionCache()
	index := func(name, field string) map[storage.ID][]string {
		return childRelationshipIndex(org, name, child, field, newChildRelationshipQueryCache(cache))
	}
	first := index("Contact", "AccountId")
	if got := index("Contact", "AccountId"); reflect.ValueOf(first).UnsafePointer() != reflect.ValueOf(got).UnsafePointer() {
		t.Fatal("versioned unchanged rows should be checked and reused")
	}
	child.TouchRecords()
	if got := index("Contact", "AccountId"); reflect.ValueOf(first).UnsafePointer() == reflect.ValueOf(got).UnsafePointer() {
		t.Fatal("a new record version must invalidate the index")
	}
	uncached := func() map[storage.ID][]string {
		return childRelationshipIndex(org, "Contact", unversioned, "AccountId", newChildRelationshipQueryCache(cache))
	}
	first = uncached()
	if got := uncached(); reflect.ValueOf(first).UnsafePointer() == reflect.ValueOf(got).UnsafePointer() {
		t.Fatal("unversioned rows must not be cached")
	}
	first = index("Contact", "AccountId")
	if got := index("Contact", "OtherId"); reflect.ValueOf(first).UnsafePointer() == reflect.ValueOf(got).UnsafePointer() {
		t.Fatal("different field reused index")
	}
	first = index("Contact", "AccountId")
	if got := index("Other", "AccountId"); reflect.ValueOf(first).UnsafePointer() == reflect.ValueOf(got).UnsafePointer() {
		t.Fatal("different object reused index")
	}
	first = index("Contact", "AccountId")
	org.Namespace = "sample"
	if got := index("Contact", "AccountId"); reflect.ValueOf(first).UnsafePointer() == reflect.ValueOf(got).UnsafePointer() {
		t.Fatal("different namespace reused index")
	}
	row := child.Records["003000000000000"].Clone()
	row.Fields["accountid"] = row.Fields["AccountId"]
	delete(row.Fields, "AccountId")
	child.Records[row.ID] = row
	first = index("Contact", "AccountId")
	if got := index("Contact", "AccountId"); reflect.ValueOf(first).UnsafePointer() == reflect.ValueOf(got).UnsafePointer() {
		t.Fatal("case-folded field lookup must build a fresh index")
	}
	first = index("Contact", "Id")
	if got := index("Contact", "Id"); reflect.ValueOf(first).UnsafePointer() == reflect.ValueOf(got).UnsafePointer() {
		t.Fatal("non-plain field must build a fresh index")
	}
}

func TestChildRelationshipIndexNeighborDigests(t *testing.T) {
	for _, shape := range []string{"unversioned", "case-folded", "non-plain"} {
		t.Run(shape, func(t *testing.T) {
			org := childIndexBenchOrg(2, 1)
			child := org.Objects["Contact"]
			field := "AccountId"
			if shape != "unversioned" {
				var versioned storage.ObjectState
				versioned.Definition = child.Definition
				versioned.SetRecords(child.Records)
				child = versioned
			}
			if shape == "case-folded" {
				id := storage.ID("003000000000000")
				row := child.Records[id].Clone()
				row.Fields["accountid"] = row.Fields["AccountId"]
				delete(row.Fields, "AccountId")
				child.PutRecord(id, row)
			}
			if shape == "non-plain" {
				field = "Id"
			}
			cache := NewExecutionCache()
			var result map[storage.ID][]string
			for range 2 {
				result = childRelationshipIndex(org, "Contact", child, field, newChildRelationshipQueryCache(cache))
			}
			fresh := childRelationshipIndex(org, "Contact", child, field, nil)
			if !reflect.DeepEqual(result, fresh) {
				t.Fatalf("cached index differs: %v != %v", result, fresh)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("neighbor %s %x", shape, sha256.Sum256(encoded))
		})
	}
}

func BenchmarkChildRelationshipColdQuery(b *testing.B) {
	org := childIndexBenchOrg(200, 3)
	child := org.Objects["Contact"]
	var versioned storage.ObjectState
	versioned.Definition = child.Definition
	versioned.SetRecords(child.Records)
	org.Objects["Contact"] = versioned
	query, err := Parse("SELECT Id,(SELECT Id,AccountId FROM Contacts) FROM Account WHERE Id = '001000000000001'")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result, err := ExecuteWithCache(org, query, NewExecutionCache())
		if err != nil {
			b.Fatal(err)
		}
		if len(result.Records) != 1 || len(result.Records[0].Children["Contacts"]) != 3 {
			b.Fatalf("unexpected result: %#v", result)
		}
	}
}
