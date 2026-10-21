package dml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestScheduledFlowDMLNativeRows(t *testing.T) {
	casesRaw, err := os.ReadFile("testdata/conformance/scheduled_flow_dml/cases.tsv")
	if err != nil {
		t.Fatal(err)
	}
	caseRows := strings.Split(strings.TrimSpace(string(casesRaw)), "\n")
	if len(caseRows) != 21 || caseRows[0] != "id\tapi\ttrigger\tflow_shape\toperation\tcontext\tstart_filter\tobserved" {
		t.Fatal("incomplete scheduled flow DML case table")
	}
	cases := make(map[string][]string, len(caseRows)-1)
	for _, line := range caseRows[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 8 || cells[0] == "" || cases[cells[0]] != nil {
			t.Fatalf("invalid or duplicate case row: %s", line)
		}
		cases[cells[0]] = cells
	}

	for _, api := range []struct{ number, directory, insert, update, delete string }{
		{number: "62", directory: "api62", insert: "S01", update: "S02", delete: "S03"},
		{number: "67", directory: "api67", insert: "S09", update: "S10", delete: "S11"},
	} {
		orgRaw, err := os.ReadFile(filepath.Join("testdata/conformance/scheduled_flow_dml", api.directory, "native", "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		observations := make(map[string]string)
		for _, line := range strings.Split(strings.TrimSpace(string(orgRaw)), "\n")[1:] {
			cells := strings.SplitN(line, "\t", 5)
			if len(cells) != 5 || cells[1] != "SCHEDULED_FLOW_DML" || cells[2] != "istest" || cells[3] != "runtime" {
				t.Fatalf("invalid native observation row: %s", line)
			}
			observations[cells[0]] = cells[4]
		}
		for _, id := range []string{api.insert, api.update, api.delete} {
			row := cases[id]
			if len(row) != 8 || row[1] != api.number || row[2] != "Scheduled" || row[3] != "unconditional" || observations[id] == "" {
				t.Fatalf("missing native DML observation for case %s", id)
			}
			t.Run(api.number+"/"+id, func(t *testing.T) {
				var native map[string]any
				if err := json.Unmarshal([]byte(observations[id]), &native); err != nil || native["counterCount"] != float64(0) {
					t.Fatalf("invalid native observation: %s", observations[id])
				}
				org := testOrg()
				account := org.Objects["Account"]
				account.Definition.FlowRules = []storage.FlowRule{{
					Name: "ScheduledDMLProbe", Active: true, TriggerType: "Scheduled",
					RecordCreates: []storage.FlowRecordCreate{{
						Name: "CreateCounter", ObjectName: "ScheduledFlowCounter__c",
						InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Name", LiteralValue: "flow-run-counter"}},
					}},
				}}
				org.Objects["Account"] = account
				org.Objects["ScheduledFlowCounter__c"] = storage.ObjectState{
					Definition: storage.ObjectDefinition{
						APIName: "ScheduledFlowCounter__c", KeyPrefix: "a00",
						Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
					},
					Records: make(map[storage.ID]storage.Record),
				}
				engine := NewEngine(&org)
				id := storage.ID("001000000000001AAA")
				var dmlResults []Result
				switch row[4] {
				case "insert":
					dmlResults = engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(string(id))}}})
					if len(dmlResults) == 1 {
						id = dmlResults[0].ID
					}
				case "update":
					account = org.Objects["Account"]
					account.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{
						"Name": storage.StringValue("scheduled-update"),
					}}
					org.Objects["Account"] = account
					dmlResults = engine.Update([]storage.Record{{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("scheduled-update")}}})
				case "delete":
					account = org.Objects["Account"]
					account.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("scheduled-delete")}}
					org.Objects["Account"] = account
					dmlResults = engine.Delete([]storage.Record{{ID: id, Object: "Account"}})
				default:
					t.Fatalf("unexpected DML operation %s", row[4])
				}
				if len(dmlResults) != 1 || !dmlResults[0].Success {
					t.Fatalf("Glade DML result: %+v", dmlResults)
				}
				counterCount := scheduledDMLCounterCount(org.Objects["ScheduledFlowCounter__c"])
				if row[4] == "delete" {
					deleted, exists := org.Objects["Account"].Records[id]
					if (exists && !deleted.System.IsDeleted) || native["accountCount"] != float64(0) {
						t.Fatalf("native=%s; Glade account remains or delete row changed", observations[row[0]])
					}
				}
				if counterCount != int(native["counterCount"].(float64)) {
					t.Fatalf("native=%s; Glade counterCount=%d", observations[row[0]], counterCount)
				}
			})
		}
	}
}

func scheduledDMLCounterCount(object storage.ObjectState) int {
	count := 0
	for _, record := range object.Records {
		if record.Fields["Name"].String == "flow-run-counter" {
			count++
		}
	}
	return count
}
