package automation

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

func TestFlowReferenceLookupVisibilityMetadataScope(t *testing.T) {
	path := filepath.Join("..", "apextest", "testdata", "conformance", "flow_lookup_metadata_pricebook", "api62", "force-app", "main", "default", "flows", "PricebookLookupProbe.flow-meta.xml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*flowXML)
		want   bool
	}{
		{"captured", func(*flowXML) {}, true},
		{"before_save", func(f *flowXML) { f.Start.TriggerType = "RecordBeforeSave" }, false},
		{"update", func(f *flowXML) { f.Start.RecordTriggerType = "Update" }, false},
		{"fault_connector", func(f *flowXML) { f.RecordLookups[0].FaultConnector.TargetReference = "Recover" }, false},
		{"assign_null", func(f *flowXML) { f.RecordLookups[0].AssignNullValuesIfNoRecordsFound = true }, false},
		{"sort", func(f *flowXML) { f.RecordLookups[0].SortField = "Name" }, false},
		{"relationship_source", func(f *flowXML) { f.RecordLookups[0].Filters[0].Value.ElementReference = "$Record.Account.Name" }, false},
		{"other_operator", func(f *flowXML) { f.RecordLookups[0].Filters[0].Operator = "NotEqualTo" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw flowXML
			if err := xml.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			tc.change(&raw)
			if got := flowCapturedTestReferenceLookup(raw, raw.RecordLookups[0]); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
