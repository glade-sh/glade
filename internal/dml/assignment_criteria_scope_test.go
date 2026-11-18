package dml

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The criteria rows are Lead rows. A Case rule keeps the shared evaluator,
// which compares text with case, until Case rows exist.
func TestAssignmentRuleCaseCriteriaKeepSharedEvaluator(t *testing.T) {
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	storage.EnsureStandardObject(&org, "Case")
	storage.EnsureStandardObject(&org, "Group")
	storage.EnsureDeterministicPlatformData(&org)
	state := org.Objects["Case"]
	if err := json.Unmarshal([]byte(`{"assignmentRules":[{"id":"01Q000000000001","name":"Active","active":true,"entries":[{"assignedTo":"First","assignedToType":"Queue","criteria":[{"field":"Subject","operation":"contains","value":"Ab"}]}]}]}`), &state.Definition); err != nil {
		t.Fatal(err)
	}
	org.Objects["Case"] = state
	groups := org.Objects["Group"]
	if groups.Records == nil {
		groups.Records = map[storage.ID]storage.Record{}
	}
	groups.Records["00G000000000001"] = storage.Record{Object: "Group", ID: "00G000000000001", Fields: map[string]storage.Value{"Type": storage.StringValue("Queue"), "DeveloperName": storage.StringValue("First")}}
	org.Objects["Group"] = groups
	engine := NewEngine(&org)
	if err := json.Unmarshal([]byte(`{"AssignmentRule":{"UseDefault":true}}`), &engine.Options); err != nil {
		t.Fatal(err)
	}
	saved := engine.Insert([]storage.Record{{Object: "Case", Fields: map[string]storage.Value{"Subject": storage.StringValue("xaby")}}})
	if len(saved) != 1 || !saved[0].Success {
		t.Fatal(saved)
	}
	if owner := org.Objects["Case"].Records[saved[0].ID].System.OwnerID; storage.IDsEqual(owner, "00G000000000001") {
		t.Fatalf("Case contains matched without case: owner=%s", owner)
	}
}
