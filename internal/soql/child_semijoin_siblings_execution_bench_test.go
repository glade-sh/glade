package soql

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The baseline parser rejects the captured parent query. Measure the shared
// child executor directly with the same three child queries on both versions.
func BenchmarkChildSemiJoinSiblingExecution(b *testing.B) {
	org := storage.NewOrgState()
	for _, name := range []string{"Account", "Contact", "Opportunity", "OpportunityContactRole", "Case"} {
		storage.EnsureStandardObject(&org, name)
	}
	accountID := storage.ID("001000000000001")
	parent := storage.Record{Object: "Account", ID: accountID, Fields: map[string]storage.Value{"Name": storage.StringValue("Example")}}
	account := org.Objects["Account"]
	account.PutRecord(accountID, parent)
	org.Objects["Account"] = account
	contact := org.Objects["Contact"]
	contactID := storage.ID("003000000000001")
	contact.PutRecord(contactID, storage.Record{Object: "Contact", ID: contactID, Fields: map[string]storage.Value{"AccountId": storage.IDValue(accountID), "Email": storage.StringValue("contact@example.invalid")}})
	org.Objects["Contact"] = contact
	cases := org.Objects["Case"]
	caseID := storage.ID("500000000000001")
	cases.PutRecord(caseID, storage.Record{Object: "Case", ID: caseID, Fields: map[string]storage.Value{"AccountId": storage.IDValue(accountID)}})
	org.Objects["Case"] = cases
	opportunities := org.Objects["Opportunity"]
	roles := org.Objects["OpportunityContactRole"]
	for i := 0; i < 500; i++ {
		id := storage.ID(fmt.Sprintf("006%012d", i+1))
		opportunities.PutRecord(id, storage.Record{Object: "Opportunity", ID: id, Fields: map[string]storage.Value{"AccountId": storage.IDValue(accountID), "Name": storage.StringValue(fmt.Sprintf("Example %04d", i)), "StageName": storage.StringValue("Prospecting")}})
		if i%2 == 0 {
			rid := storage.ID(fmt.Sprintf("00K%012d", i+1))
			roles.PutRecord(rid, storage.Record{Object: "OpportunityContactRole", ID: rid, Fields: map[string]storage.Value{"OpportunityId": storage.IDValue(id), "Role": storage.StringValue("Decision Maker")}})
		}
	}
	org.Objects["Opportunity"] = opportunities
	org.Objects["OpportunityContactRole"] = roles
	inputs := []struct{ relationship, source string }{
		{"Contacts", "SELECT Id FROM Contact WHERE Email != null"},
		{"Opportunities", "SELECT Id, Name FROM Opportunity WHERE StageName = 'Prospecting' AND Id IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = 'Decision Maker') LIMIT 1"},
		{"Cases", "SELECT Id FROM Case LIMIT 1"},
	}
	children := make([]ChildQuery, 0, len(inputs))
	for _, input := range inputs {
		q, err := Parse(input.source)
		if err != nil {
			b.Fatal(err)
		}
		children = append(children, ChildQuery{Relationship: input.relationship, Query: q})
	}
	var results [3][]storage.Record
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		cache := newChildRelationshipQueryCache(NewExecutionCache())
		for i, child := range children {
			var err error
			results[i], err = executeChildRelationshipQuery(org, org.Objects["Account"].Definition, parent, child, cache, ExecutionContext{})
			if err != nil {
				b.Fatal(err)
			}
		}
	}
	b.StopTimer()
	for i, rows := range results {
		if len(rows) != 1 {
			b.Fatalf("child %d: got %d rows, want 1", i, len(rows))
		}
	}
	data, err := json.Marshal(results)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("outcome sha256: %x", sha256.Sum256(data))
}
