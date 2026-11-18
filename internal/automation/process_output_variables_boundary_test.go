package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE process_builder_review_afteroutputs/org.tsv:2-13 captures the
// all-input/output null-default form; mixed flags and non-null defaults stay gated.
func TestProcessBuilderOutputVariableBoundaries(t *testing.T) {
	source := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review_afteroutputs", "project", "force-app", "main", "default", "flows", "Fern_Lookup_Sub.flow-meta.xml")
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	check := func(label, text string, want bool) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "probe.flow-meta.xml")
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
		flow, _, err := loadFlow(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(flow.Rules) != 1 {
			t.Fatalf("%s: expected one rule", label)
		}
		_, got := flow.Rules[0].CapturedProcessDependentLookupSteps()
		if got != want {
			t.Fatalf("%s: admitted=%v want %v", label, got, want)
		}
	}
	text := string(raw)
	check("captured", text, true)
	check("one output flag changed", strings.Replace(text, "<isOutput>true</isOutput>", "<isOutput>false</isOutput>", 1), false)
	empty := strings.ReplaceAll(text, "<isOutput>true</isOutput>", "<isOutput>true</isOutput><value />")
	check("empty default normalizes to null", empty, true)
	nonnull := strings.Replace(text, "<isOutput>true</isOutput>", "<isOutput>true</isOutput><value><stringValue>x</stringValue></value>", 1)
	check("non-null default", nonnull, false)
}

func TestProcessBuilderOutputVariableTextBoundary(t *testing.T) {
	source := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review_afteroutputs", "project", "force-app", "main", "default", "flows", "Fern_Lookup_Sub.flow-meta.xml")
	flow, _, err := loadFlow(source)
	if err != nil {
		t.Fatal(err)
	}
	target := flow.Rules[0]
	action := storage.FlowAction{Inputs: []storage.WorkflowFieldUpdate{{Field: "recordId", SourceField: "Id"}, {Field: "rootId", SourceField: "Fern_Root_Link__c"}}}
	defs := map[string]storage.ObjectDefinition{"Contact": {Fields: map[string]storage.Field{"Fern_Root_Link__c": {APIName: "Fern_Root_Link__c", Type: storage.FieldReference, ReferenceTo: []string{"Fern_Root__c"}}, "Description": {APIName: "Description", Type: storage.FieldString, DisplayType: "TEXTAREA"}}}, "Fern_Root__c": {Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}}
	target.Steps[2].Assignment.SourceField = "FindRoot.Name"
	target.Steps[3].RecordUpdate.InputAssignments[0].Field = "Description"
	if processBuilderCapturedSubflow("Contact", action, target, defs) {
		t.Fatal("uncaptured input-output standard-text shape admitted")
	}
}
