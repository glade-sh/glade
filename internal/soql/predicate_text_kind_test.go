package soql

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A WHERE clause classifies each equality field once per candidate scan. The
// declaration walk copies a storage.Field per call, so a scan over N records
// must not allocate N copies.
func TestEqualityPredicateResolvesDeclaredFieldOncePerScan(t *testing.T) {
	org := aggregateTestOrg()
	object := org.Objects["Account"]
	record := object.Records["001000000000001"]
	record.Object = "Account"
	conditions := []*Condition{
		{Field: "Name", Op: "=", Value: storage.StringValue("Acme")},
		{Field: "Name", Op: "!=", Value: storage.StringValue("acme")},
		{Field: "Rating", Op: "IN", Values: []storage.Value{storage.StringValue("Cold"), storage.StringValue("hot")}},
		{Field: "Rating", Op: "NOT IN", Values: []storage.Value{storage.StringValue("Cold")}},
	}
	for _, condition := range conditions {
		t.Run(condition.Op, func(t *testing.T) {
			scratch := &predicateScratch{}
			want := matchesWithContext(org, object.Definition, record, condition, ExecutionContext{})
			if got := matchesWithScratch(org, object.Definition, record, condition, ExecutionContext{}, scratch); got != want {
				t.Fatalf("scratch match %v, unshared match %v", got, want)
			}
			allocations := testing.AllocsPerRun(100, func() {
				if got := matchesWithScratch(org, object.Definition, record, condition, ExecutionContext{}, scratch); got != want {
					t.Fatalf("scratch match %v, unshared match %v", got, want)
				}
			})
			if allocations != 0 {
				t.Fatalf("repeated predicate evaluation allocated %.0f times per record; the declared field must be resolved once per scan", allocations)
			}
		})
	}
}

// Text equality keeps its declared-field semantics through the memo: exact on
// unique case-sensitive text, folded on ordinary text, value equality elsewhere.
func TestEqualityPredicateMemoKeepsDeclaredSemantics(t *testing.T) {
	org := aggregateTestOrg()
	object := org.Objects["Account"]
	object.Definition.Fields["External__c"] = storage.Field{APIName: "External__c", Type: storage.FieldString, Unique: true, CaseSensitive: true}
	object.Definition.Fields["Count__c"] = storage.Field{APIName: "Count__c", Type: storage.FieldInteger}
	record := storage.Record{ID: "001000000000009", Object: "Account", Fields: map[string]storage.Value{
		"Name":        storage.StringValue("Acme"),
		"External__c": storage.StringValue("Key"),
		"Count__c":    storage.StringValue("7"),
	}}
	cases := []struct {
		condition Condition
		want      bool
	}{
		{Condition{Field: "Name", Op: "=", Value: storage.StringValue("ACME")}, true},
		{Condition{Field: "External__c", Op: "=", Value: storage.StringValue("KEY")}, false},
		{Condition{Field: "External__c", Op: "=", Value: storage.StringValue("Key")}, true},
		{Condition{Field: "Count__c", Op: "=", Value: storage.StringValue("7")}, true},
		{Condition{Field: "Unknown__c", Op: "=", Value: storage.StringValue("x")}, false},
	}
	scratch := &predicateScratch{}
	for _, tc := range cases {
		for repeat := 0; repeat < 3; repeat++ {
			condition := tc.condition
			if got := matchesWithScratch(org, object.Definition, record, &condition, ExecutionContext{}, scratch); got != tc.want {
				t.Fatalf("%s %s %q (repeat %d): got %v, want %v", condition.Field, condition.Op, condition.Value.String, repeat, got, tc.want)
			}
			if got := matchesWithContext(org, object.Definition, record, &condition, ExecutionContext{}); got != tc.want {
				t.Fatalf("%s %s %q unshared: got %v, want %v", condition.Field, condition.Op, condition.Value.String, got, tc.want)
			}
		}
	}
}

func BenchmarkWhereTextEqualityScan(b *testing.B) {
	org := aggregateTestOrg()
	object := org.Objects["Account"]
	for i := 0; i < 2000; i++ {
		id := storage.ID(fmt.Sprintf("001%012d", i+10))
		object.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{
			"Name":   storage.StringValue(fmt.Sprintf("Account %d", i)),
			"Rating": storage.StringValue("Warm"),
		}}
	}
	org.Objects["Account"] = object
	query, err := Parse("SELECT Id FROM Account WHERE Name = 'Account 1999'")
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
		if result.Rows != 1 {
			b.Fatalf("rows %d", result.Rows)
		}
	}
}
