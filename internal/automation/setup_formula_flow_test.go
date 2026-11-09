package automation

import (
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"path/filepath"
	"testing"
)

func TestSetupFormulaFlowCriterionNativeProject(t *testing.T) {
	path := filepath.Join("..", "apextest", "testdata", "conformance", "setup_formula_query_shapes", "project", "force-app", "main", "default", "flows", "SetupCriterionFlow.flow-meta.xml")
	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil || len(idx.Diagnostics) != 0 || len(idx.Flows) != 1 {
		t.Fatalf("load err=%v index=%+v", err, idx)
	}
	for _, flag := range []bool{false, true} {
		t.Run(map[bool]string{false: "actor_false", true: "actor_true"}[flag], func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			actor := storage.ID("005000000000002")
			account := org.Objects["Account"]
			account.Definition.Fields["Criterion__c"] = storage.Field{APIName: "Criterion__c", Type: storage.FieldBoolean}
			org.Objects["Account"] = account
			def := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
			org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Flag__c": storage.BooleanValue(flag)}}}}
			ApplyToOrg(&org, idx)
			engine := dml.NewEngine(&org)
			engine.UserID = actor
			rows := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe"), "Criterion__c": storage.BooleanValue(false)}}}
			if err := engine.ApplyBeforeSaveFlows(rows); err != nil {
				t.Fatal(err)
			}
			if got := rows[0].Fields["Criterion__c"]; got.Kind != storage.ValueBoolean || got.Boolean == flag {
				t.Fatalf("flag=%v criterion=%#v want=%v", flag, got, !flag)
			}
		})
	}
}
