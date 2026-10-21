package dml

import (
	"fmt"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSummaryCandidatesPreserveChildRelations(t *testing.T) {
	org := storage.NewOrgState()
	org.RuntimeSchemaStamp = "summary-generation"
	for i := 0; i < 16; i++ {
		parent, child := fmt.Sprintf("Parent%d__c", i), fmt.Sprintf("Child%d__c", i)
		org.Objects[child] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: child, Fields: map[string]storage.Field{"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference}}}}
		fields := map[string]storage.Field{"Count__c": {APIName: "Count__c", Type: storage.FieldSummary, SummaryOperation: "count", SummaryForeignKey: child + ".Parent__c"}}
		for j := 0; j < 512; j++ {
			key := fmt.Sprintf("Value%d__c", j)
			fields[key] = storage.Field{APIName: key, Type: storage.FieldString}
		}
		org.Objects[parent] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: parent, Fields: fields}}
	}
	engine := NewEngine(&org)
	start := time.Now()
	for i := 0; i < 16; i++ {
		got := engine.summaryRelationsForChild(fmt.Sprintf("Child%d__c", i))
		if len(got) != 1 || got[0].parentObject != fmt.Sprintf("Parent%d__c", i) || got[0].parentField != "Count__c" || got[0].fkFieldName != "Parent__c" {
			t.Fatalf("child %d: %#v", i, got)
		}
	}
	if got := engine.summaryRelationsForChild("Missing__c"); len(got) != 0 {
		t.Fatalf("negative lookup: %#v", got)
	}
	t.Logf("16 children and one negative lookup: %.6fs", time.Since(start).Seconds())
}

func TestSummaryCandidatesKeepMutableOrgLookupsLive(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["First__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "First__c", Fields: map[string]storage.Field{"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference}}}}
	org.Objects["Second__c"] = org.Objects["First__c"]
	org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{}}}
	engine := NewEngine(&org)
	if got := engine.summaryRelationsForChild("First__c"); len(got) != 0 {
		t.Fatal(got)
	}
	org.Objects["Parent__c"].Definition.Fields["Count__c"] = storage.Field{APIName: "Count__c", Type: storage.FieldSummary, SummaryOperation: "count", SummaryForeignKey: "Second__c.Parent__c"}
	if got := engine.summaryRelationsForChild("Second__c"); len(got) != 1 {
		t.Fatal(got)
	}
}
