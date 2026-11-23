package automation

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

func TestFlowRelationshipFilterFaultScope(t *testing.T) {
	path := filepath.Join("..", "apextest", "testdata", "conformance", "flow_relationship_filter_fault", "api62", "force-app", "main", "default", "flows", "RelationshipFilterProbe.flow-meta.xml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var original flowXML
	if err := xml.Unmarshal(data, &original); err != nil {
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
		{"other_object", func(f *flowXML) { f.RecordLookups[0].Object = "Account" }, false},
		{"different_source", func(f *flowXML) { f.RecordLookups[0].Filters[0].Value.ElementReference = "$Record.Name" }, false},
		{"one_filter", func(f *flowXML) { f.RecordLookups[0].Filters = f.RecordLookups[0].Filters[:1] }, false},
		{"fault_connector", func(f *flowXML) { f.RecordLookups[0].FaultConnector.TargetReference = "Recover" }, false},
		{"assign_null", func(f *flowXML) { f.RecordLookups[0].AssignNullValuesIfNoRecordsFound = true }, false},
		{"other_process", func(f *flowXML) { f.ProcessType = "Workflow" }, false},
		{"sort_order", func(f *flowXML) { f.RecordLookups[0].SortOrder = "Asc" }, false},
		{"mixed_value", func(f *flowXML) { f.RecordLookups[0].Filters[0].Value.StringValue = "value" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw flowXML
			if err := xml.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			tc.change(&raw)
			if got := flowCapturedRelationshipFilterFault(raw, raw.RecordLookups[0]); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
