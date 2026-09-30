package dml_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func TestDeleteSetNullIndexedQueriesSurviveChildRebuild(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		}}, Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Borrower__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Borrower__c", KeyPrefix: "a00", Fields: map[string]storage.Field{
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			"Note__c":   {APIName: "Note__c", Type: storage.FieldString},
		}, Relations: []storage.Relationship{{Field: "Parent__c", ParentObjects: []string{"Account"}, SetNullOnDelete: true}},
			Indexes: []storage.IndexDefinition{{Name: "Borrower.Parent", Object: "Borrower__c", Fields: []string{"Parent__c"}}},
		}, Records: map[storage.ID]storage.Record{},
	}
	engine := dml.NewEngine(&org)
	mustSucceed := func(results []dml.Result, count int) {
		t.Helper()
		if len(results) != count {
			t.Fatalf("result count = %d, want %d", len(results), count)
		}
		for _, result := range results {
			if !result.Success {
				t.Fatalf("DML failed: %#v", result)
			}
		}
	}
	parents := engine.Insert([]storage.Record{
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("deleted")}},
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("retained")}},
	})
	mustSucceed(parents, 2)
	children := engine.Insert([]storage.Record{
		{Object: "Borrower__c", Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parents[0].ID)}},
		{Object: "Borrower__c", Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parents[1].ID)}},
	})
	mustSucceed(children, 2)
	queryIDs := func(label string, value storage.Value, want ...storage.ID) {
		t.Helper()
		result, err := soql.Execute(org, soql.Query{Object: "Borrower__c", Fields: []string{"Id"},
			Where: &soql.Condition{Field: "Parent__c", Op: "=", Value: value}})
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		got := make([]storage.ID, 0, len(result.Records))
		for _, record := range result.Records {
			got = append(got, record.ID)
		}
		wantIDs := append([]storage.ID{}, want...)
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		sort.Slice(wantIDs, func(i, j int) bool { return wantIDs[i] < wantIDs[j] })
		if !reflect.DeepEqual(got, wantIDs) {
			t.Errorf("%s: query IDs = %v, want %v", label, got, wantIDs)
		}
	}
	assertClearedQueries := func(phase string) {
		t.Helper()
		queryIDs(phase+" null", storage.NullValue(), children[0].ID)
		queryIDs(phase+" old parent", storage.IDValue(parents[0].ID))
		queryIDs(phase+" different parent", storage.IDValue(parents[1].ID), children[1].ID)
	}
	rebuild := func() {
		t.Helper()
		// This is the same storage entry point the VM calls after ordinary child DML.
		storage.RebuildObjectIndexes(&org, "Borrower__c")
		if _, indexed := storage.LookupIndex(org.Objects["Borrower__c"], "Parent__c", storage.NullValue()); !indexed {
			t.Fatal("regression must exercise a rebuilt index, not a scan")
		}
	}
	rebuild()
	queryIDs("before null", storage.NullValue())
	queryIDs("before old parent", storage.IDValue(parents[0].ID), children[0].ID)
	queryIDs("before different parent", storage.IDValue(parents[1].ID), children[1].ID)

	mustSucceed(engine.Delete([]storage.Record{{Object: "Account", ID: parents[0].ID}}), 1)
	assertClearedQueries("after SetNull")
	if _, indexed := storage.LookupIndex(org.Objects["Borrower__c"], "Parent__c", storage.NullValue()); indexed {
		t.Error("indirectly changed child retained a stale index")
	}

	mustSucceed(engine.Update([]storage.Record{{Object: "Borrower__c", ID: children[0].ID,
		Fields: map[string]storage.Value{"Note__c": storage.StringValue("ordinary sparse update")}}}), 1)
	rebuild()
	assertClearedQueries("after sparse update and rebuild")

	mustSucceed(engine.Update([]storage.Record{{Object: "Borrower__c", ID: children[0].ID,
		Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parents[1].ID)}}}), 1)
	rebuild()
	queryIDs("reparented null", storage.NullValue())
	queryIDs("reparented old parent", storage.IDValue(parents[0].ID))
	queryIDs("reparented different parent", storage.IDValue(parents[1].ID), children[0].ID, children[1].ID)

	mustSucceed(engine.Update([]storage.Record{{Object: "Borrower__c", ID: children[0].ID,
		ExplicitNulls: map[string]bool{"Parent__c": true}}}), 1)
	rebuild()
	assertClearedQueries("after ordinary explicit-null update and rebuild")
}
