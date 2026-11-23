package dml

import (
	"strconv"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// BenchmarkFlowAfterSaveDecisionInsert inserts 200 records through an
// after-save create Flow that decides on a record field, looks up a parent
// row and updates the triggering record. It covers the decision and lookup
// paths that every record-triggered Flow uses.
func BenchmarkFlowAfterSaveDecisionInsert(b *testing.B) {
	base := testOrg()
	account := base.Objects["Account"]
	account.Definition.Fields["Status__c"] = storage.Field{APIName: "Status__c", Type: storage.FieldString, DisplayType: "STRING"}
	account.Definition.Fields["Tier__c"] = storage.Field{APIName: "Tier__c", Type: storage.FieldString, DisplayType: "STRING"}
	account.Definition.FlowRules = []storage.FlowRule{{
		Name:              "DecideAndLookup",
		Active:            true,
		ProcessType:       "AutoLaunchedFlow",
		TriggerType:       "RecordAfterSave",
		RecordTriggerType: "Create",
		RecordLookups: []storage.FlowRecordLookup{{
			Name:               "Self",
			ObjectName:         "Account",
			GetFirstRecordOnly: true,
			Criteria:           []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "equals", SourceField: "Id"}},
		}},
		Branches: []storage.FlowBranch{{
			Name:         "Gold",
			Criteria:     []storage.WorkflowCriteriaItem{{Field: "Tier__c", Operation: "equals", Value: "Gold"}},
			FieldUpdates: []storage.WorkflowFieldUpdate{{Name: "SetStatus", Field: "Status__c", LiteralValue: "matched"}},
		}},
	}}
	base.Objects["Account"] = account
	records := make([]storage.Record, 200)
	for i := range records {
		tier := "Silver"
		if i%2 == 0 {
			tier = "Gold"
		}
		records[i] = storage.Record{Object: "Account", Fields: map[string]storage.Value{
			"Name":    storage.StringValue("Bench " + strconv.Itoa(i)),
			"Tier__c": storage.StringValue(tier),
		}}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		org := base.Clone()
		batch := make([]storage.Record, len(records))
		for j := range records {
			batch[j] = records[j].Clone()
		}
		engine := NewEngine(&org)
		b.StartTimer()
		results := engine.Insert(batch)
		b.StopTimer()
		matched := 0
		for _, result := range results {
			if !result.Success {
				b.Fatalf("insert failed: %#v", result)
			}
			if org.Objects["Account"].Records[result.ID].Fields["Status__c"].String == "matched" {
				matched++
			}
		}
		if matched != 100 {
			b.Fatalf("matched = %d, want 100", matched)
		}
		b.StartTimer()
	}
}
