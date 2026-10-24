package dml

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowEntryMatrixBeforeAfterSaveNativeRows(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/flow_entry_matrix/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	cells := 0
	for _, row := range rows[1:] {
		if len(row) != 8 {
			t.Fatal("invalid native matrix row")
		}
		if row[0] != "62.0" || row[1] != "anonymous" || row[6] != "apex_dml" {
			continue
		}
		cells++
		t.Run(strings.Join(row[2:7], "/"), func(t *testing.T) {
			var native map[string][]int
			if err := json.Unmarshal([]byte(row[7]), &native); err != nil {
				t.Fatal(err)
			}
			stage := "created"
			if row[3] == "update" {
				stage = "updated"
			}
			if !reflect.DeepEqual(native[stage], []int{1, 0}) {
				t.Fatalf("expected native saved effects for matching/control records: %v", native)
			}
			for _, control := range []bool{false, true} {
				t.Run(map[bool]string{false: "matching", true: "control"}[control], func(t *testing.T) {
					org := storage.NewOrgState()
					definition := storage.ObjectDefinition{APIName: "CedarEntry__c", Fields: map[string]storage.Field{}}
					for _, field := range []string{"Name", "Match__c", "Value__c", "Link__c"} {
						definition.Fields[field] = storage.Field{APIName: field, Type: storage.FieldString}
					}
					for _, field := range []string{"Runs__c", "Amount__c"} {
						definition.Fields[field] = storage.Field{APIName: field, Type: storage.FieldInteger}
					}
					criterion := storage.WorkflowCriteriaItem{Field: "Value__c"}
					switch row[5] {
					case "EqualTo":
						criterion.Operation, criterion.Value = "equals", "ready"
					case "NotEqualTo":
						criterion.Operation, criterion.Value = "notEqual", "blocked"
					case "GreaterThan":
						criterion.Field, criterion.Operation, criterion.Value = "Amount__c", "greaterThan", "5"
					case "IsNull":
						criterion.Field, criterion.Operation, criterion.Value = "Link__c", "isNull", "true"
					case "IsChanged":
						criterion.Operation, criterion.Value = "isChanged", "true"
						if row[3] == "create" {
							criterion.Value = "false"
						}
					default:
						t.Fatalf("unknown native operator: %s", row[5])
					}
					rule := storage.FlowRule{Name: "CedarEntry", Active: true,
						TriggerType: "RecordBeforeSave", RecordTriggerType: "Create",
						DoesRequireRecordChangedToMeetCriteria: row[4] == "changed",
						Criteria: []storage.WorkflowCriteriaItem{
							{Field: "Match__c", Operation: "equals", Value: "Match"}, criterion,
						},
						FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Runs__c", LiteralValue: "1"}},
					}
					if row[2] == "after" {
						rule.TriggerType = "RecordAfterSave"
					}
					if row[3] == "update" {
						rule.RecordTriggerType = "Update"
					} else if row[5] == "IsChanged" {
						rule.RecordTriggerType = "CreateAndUpdate"
					}
					definition.FlowRules = []storage.FlowRule{rule}
					object := storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
					id := storage.ID("a10000000000001")
					prior := storage.Record{Object: definition.APIName, ID: id, Fields: map[string]storage.Value{
						"Name": storage.StringValue("Cedar"), "Match__c": storage.StringValue("Match"),
						"Value__c": storage.StringValue("blocked"), "Link__c": storage.StringValue("set"),
						"Runs__c": storage.IntegerValue(0), "Amount__c": storage.IntegerValue(0),
					}}
					current := prior.Clone()
					if row[3] == "create" && row[2] == "before" {
						current.ID = ""
					}
					if control {
						current.Fields["Match__c"] = storage.StringValue("Control")
					}
					switch row[5] {
					case "EqualTo", "NotEqualTo", "IsChanged":
						current.Fields["Value__c"] = storage.StringValue("ready")
					case "GreaterThan":
						current.Fields["Amount__c"] = storage.IntegerValue(10)
					case "IsNull":
						delete(current.Fields, "Link__c")
					}
					if row[3] == "update" {
						object.Records[id] = prior
					}
					if row[2] == "after" {
						object.Records[id] = current
					}
					org.Objects[definition.APIName] = object
					engine := NewEngine(&org)
					if row[2] == "after" && row[3] == "create" {
						engine.markCreatedAutomation(id)
					}
					if row[2] == "after" && row[3] == "update" {
						engine.PriorRecords = map[storage.ID]storage.Record{id: prior}
					}
					if row[2] == "before" {
						records := []storage.Record{current}
						if err := engine.ApplyBeforeSaveFlows(records); err != nil {
							t.Fatal(err)
						}
						current = records[0]
					} else {
						if _, err := engine.ApplyAutomation(definition.APIName, id); err != nil {
							t.Fatal(err)
						}
						current = org.Objects[definition.APIName].Records[id]
					}
					want := int64(native[stage][0])
					if control {
						want = int64(native[stage][1])
					}
					if got := current.Fields["Runs__c"].Integer; got != want {
						t.Fatalf("Flow field effect: native=%d; Glade=%d", want, got)
					}
					if row[2] == "before" && row[3] == "update" && !reflect.DeepEqual(org.Objects[definition.APIName].Records[id], prior) {
						t.Fatal("before-save entry evaluation mutated the saved prior record")
					}
				})
			}
		})
	}
	if cells != 26 {
		t.Fatalf("native engine cells=%d, want 26", cells)
	}
}
