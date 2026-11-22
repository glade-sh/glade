package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowProfileDecisionUncapturedOperator(t *testing.T) {
	engine := &Engine{Org: &storage.OrgState{}}
	frame := &flowFrame{capturedAfterCreate: true, saveDecisions: true}
	branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "$Profile.Name", Operation: "notequal", Value: "{!$Record.Title}"}}}
	matched, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, storage.ObjectDefinition{}, frame)
	if matched || supported {
		t.Fatalf("uncaptured operator changed: matched=%v supported=%v", matched, supported)
	}
}

func BenchmarkFlowProfileDecisionUncapturedOperator(b *testing.B) {
	engine := &Engine{Org: &storage.OrgState{}}
	frame := &flowFrame{capturedAfterCreate: true, saveDecisions: true}
	branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "$Profile.Name", Operation: "notequal", Value: "{!$Record.Title}"}}}
	for i := 0; i < b.N; i++ {
		matched, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, storage.ObjectDefinition{}, frame)
		if matched || supported {
			b.Fatal("uncaptured operator changed")
		}
	}
}

func TestFlowProfileDecisionUncapturedNonASCII(t *testing.T) {
	org := testOrg()
	storage.EnsureStandardObject(&org, "User")
	storage.EnsureStandardObject(&org, "Profile")
	profileState := org.Objects["Profile"]
	ids := storage.NewIDGenerator(map[string]string{"Profile": profileState.Definition.KeyPrefix})
	profileID, err := ids.Next("Profile")
	if err != nil {
		t.Fatal(err)
	}
	profileState.Records[profileID] = storage.Record{ID: profileID, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Prøfile")}}
	org.Objects["Profile"] = profileState
	userState := org.Objects["User"]
	userState.Records[storage.DefaultUserID] = storage.Record{ID: storage.DefaultUserID, Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue(profileID)}}
	org.Objects["User"] = userState
	definition := storage.ObjectDefinition{Fields: map[string]storage.Field{"Title": {APIName: "Title", Type: storage.FieldString, DisplayType: "STRING"}}}
	record := storage.Record{Fields: map[string]storage.Value{"Title": storage.StringValue("PRØFILE")}}
	rule := storage.FlowRule{TriggerType: "RecordAfterSave", RecordTriggerType: "Create", Branches: []storage.FlowBranch{{Name: "Match", Criteria: []storage.WorkflowCriteriaItem{{Field: "$Profile.Name", Operation: "equals", Value: "{!$Record.Title}"}}}}}
	engine := NewEngine(&org)
	_, _, err = engine.selectFlowBranch(rule, record, definition)
	if err == nil || !strings.Contains(err.Error(), "unsupported criteria") {
		t.Fatalf("uncaptured comparison changed: %v", err)
	}
}
