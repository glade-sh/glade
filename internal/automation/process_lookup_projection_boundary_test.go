package automation

import (
	"path/filepath"
	"testing"
)

func TestProcessBuilderLookupProjectionBoundaries(t *testing.T) {
	path := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review_afterprojection", "project", "force-app", "main", "default", "flows", "Fern_Lookup_Sub.flow-meta.xml")
	flow, _, err := loadFlow(path)
	if err != nil {
		t.Fatal(err)
	}
	rule := flow.Rules[0]
	check := func(label string, want bool) {
		t.Helper()
		_, got := rule.CapturedProcessDependentLookupSteps()
		if got != want {
			t.Errorf("%s: admitted=%v want %v", label, got, want)
		}
	}
	check("captured projection", true)
	rule.Steps[1].RecordLookup.QueriedFields = []string{"Id"}
	check("missing source", false)
	rule.Steps[1].RecordLookup.QueriedFields = []string{"Id", "Unrelated__c"}
	check("wrong source", false)
	rule.Steps[1].RecordLookup.QueriedFields = []string{"Id", "Fern_Value__c", "Unrelated__c"}
	check("extra field", false)
	rule.Steps[1].RecordLookup.QueriedFields = []string{"Id", "Fern_Value__c"}
	rule.Steps[0].RecordLookup.QueriedFields = []string{"Id"}
	check("first lookup projection", false)
}
