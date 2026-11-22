package soql

import (
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
