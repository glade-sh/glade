package soql

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func childIndexBenchOrg(parents, perParent int) storage.OrgState {
	org := storage.NewOrgState()
	accounts := map[storage.ID]storage.Record{}
	contacts := map[storage.ID]storage.Record{}
	for i := 0; i < parents; i++ {
		aid := storage.ID(fmt.Sprintf("001%012d", i))
		accounts[aid] = storage.Record{ID: aid, Object: "Account"}
		for j := 0; j < perParent; j++ {
			cid := storage.ID(fmt.Sprintf("003%06d%06d", i, j))
			contacts[cid] = storage.Record{ID: cid, Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(aid)}}
		}
	}
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account"}, Records: accounts}
	org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact",
		Fields:    map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}},
		Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account", ChildRelationship: "Contacts"}},
	}, Records: contacts}
	return org
}

func BenchmarkChildRelationshipIndexBuild(b *testing.B) {
	org := childIndexBenchOrg(200, 3)
	obj := org.Objects["Contact"]
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		childRelationshipIndex(org, "Contact", obj, "AccountId", nil)
	}
}

func childIndexVersionedOrg(parents, perParent int) storage.OrgState {
	org := childIndexBenchOrg(parents, perParent)
	child := org.Objects["Contact"]
	var versioned storage.ObjectState
	versioned.Definition = child.Definition
	versioned.SetRecords(child.Records)
	org.Objects["Contact"] = versioned
	return org
}

// BenchmarkChildSubqueryRepeated runs a point query with a child subquery
// repeatedly against one generation cache, the shape a test loop produces.
func BenchmarkChildSubqueryRepeated(b *testing.B) {
	for _, parents := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("parents%d", parents), func(b *testing.B) {
			org := childIndexVersionedOrg(parents, 3)
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
		})
	}
}

// BenchmarkChildSubqueryFull runs one query over every parent, repeated.
func BenchmarkChildSubqueryFull(b *testing.B) {
	for _, parents := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("parents%d", parents), func(b *testing.B) {
			org := childIndexVersionedOrg(parents, 3)
			query, err := Parse("SELECT Id,(SELECT Id,AccountId FROM Contacts) FROM Account")
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
				if len(result.Records) != parents {
					b.Fatalf("unexpected rows %d", len(result.Records))
				}
			}
		})
	}
}

// BenchmarkChildSubqueryCold builds the index with a fresh cache each run, so
// reuse never applies and any bookkeeping cost shows.
func BenchmarkChildSubqueryCold(b *testing.B) {
	for _, parents := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("parents%d", parents), func(b *testing.B) {
			org := childIndexVersionedOrg(parents, 3)
			query, err := Parse("SELECT Id,(SELECT Id,AccountId FROM Contacts) FROM Account WHERE Id = '001000000000001'")
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := ExecuteWithCache(org, query, NewExecutionCache()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
