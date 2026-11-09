package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestSetupFormulaFlowComparesRecordField(t *testing.T) {
	path := filepath.Join("..", "apextest", "testdata", "conformance", "setup_formula_flow_rhs", "project", "force-app", "main", "default", "flows", "SetupTextDecision.flow-meta.xml")
	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil || len(idx.Diagnostics) != 0 || len(idx.Flows) != 1 {
		t.Fatalf("load err=%v index=%+v", err, idx)
	}
	for _, tc := range []struct {
		phrase string
		want   bool
	}{{"Probe", true}, {"other", false}} {
		t.Run(tc.phrase, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			actor := storage.ID("005000000000002")
			account := org.Objects["Account"]
			account.Definition.Fields["TextMatch__c"] = storage.Field{APIName: "TextMatch__c", Type: storage.FieldBoolean}
			org.Objects["Account"] = account
			def := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Phrase__c": {APIName: "Phrase__c", Type: storage.FieldString}}}
			org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Phrase__c": storage.StringValue(tc.phrase)}}}}
			ApplyToOrg(&org, idx)
			engine := dml.NewEngine(&org)
			engine.UserID = actor
			rows := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe"), "TextMatch__c": storage.BooleanValue(false)}}}
			if err := engine.ApplyBeforeSaveFlows(rows); err != nil {
				t.Fatal(err)
			}
			if got := rows[0].Fields["TextMatch__c"]; got.Kind != storage.ValueBoolean || got.Boolean != tc.want {
				t.Fatalf("result=%#v want=%v", got, tc.want)
			}
		})
	}
}

func TestSetupFormulaFlowUnsupportedReferenceOperands(t *testing.T) {
	source := filepath.Join("..", "apextest", "testdata", "conformance", "setup_formula_flow_rhs", "project", "force-app", "main", "default", "flows", "SetupTextDecision.flow-meta.xml")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"other-resource", "$Record.Name", "Unmodeled.Value"},
		{"unsupported-operator", "<operator>EqualTo</operator>", "<operator>GreaterThan</operator>"},
		{"formula-resource", "<elementReference>$Record.Name</elementReference>", "<formula>1+1</formula>"},
		{"expression-resource", "<elementReference>$Record.Name</elementReference>", "<expression>1+1</expression>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Replace(string(data), tc.old, tc.replacement, 1)
			path := filepath.Join(t.TempDir(), "Probe.flow-meta.xml")
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
			if err != nil {
				t.Fatal(err)
			}
			if len(idx.Diagnostics) == 0 {
				t.Fatalf("expected unsupported diagnostic, index=%+v", idx)
			}
		})
	}
}
