package dml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Replay the captured Get Records field assignments through the serialized
// non-step representation as well as the connected metadata conformance test.
func TestFlowLegacyLookupFieldNativeConformance(t *testing.T) {
	for _, api := range []string{"api62", "api67"} {
		for _, route := range []string{"anonymous", "istest"} {
			data, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/flow_lookup_field_update", api, "native", route, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(rows) != 5 || rows[0] != "id\tgroup\troute\tstatus\tvalue" {
				t.Fatal("incomplete native rows")
			}
			for _, row := range rows[1:] {
				cells := strings.SplitN(row, "\t", 5)
				if len(cells) != 5 || cells[1] != "FLOW_LOOKUP_FIELD_UPDATE" || cells[2] != route || cells[3] != "runtime" {
					t.Fatal("invalid native row")
				}
				var want struct {
					IsNull        bool    `json:"isNull"`
					MatchesTarget bool    `json:"matchesTarget"`
					Description   string  `json:"description"`
					Exception     *string `json:"exception"`
				}
				if err := json.Unmarshal([]byte(cells[4]), &want); err != nil {
					t.Fatal(err)
				}
				for _, branch := range []bool{false, true} {
					name := "rule"
					if branch {
						name = "branch"
					}
					t.Run(api+"/"+route+"/"+cells[0]+"/"+name, func(t *testing.T) {
						org := testOrg()
						id := storage.ID("001000000000001AAA")
						account := org.Objects["Account"]
						account.Definition.Fields["Id"] = storage.Field{APIName: "Id", Type: storage.FieldID}
						account.Records[id] = storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Grove lookup hit")}}
						org.Objects["Account"] = account
						updates := []storage.WorkflowFieldUpdate{{Name: "SetLookup", Field: "AccountId", SourceField: "Fetched.Id"}, {Name: "SetMarker", Field: "Description", LiteralValue: "lookup ran"}}
						lookups := []storage.FlowRecordLookup{{Name: "Fetched", ObjectName: "Account", GetFirstRecordOnly: true, StoreOutputAutomatically: true, Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", SourceField: "FirstName"}}}}
						rule := storage.FlowRule{Name: "GroveLookup", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "CreateAndUpdate", FieldUpdates: updates, RecordLookups: lookups}
						if branch {
							rule.FieldUpdates = nil
							rule.RecordLookups = nil
							rule.Branches = []storage.FlowBranch{{Name: "LookupBranch", Default: true, FieldUpdates: updates, RecordLookups: lookups}}
						}
						org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]storage.Field{"FirstName": {APIName: "FirstName", Type: storage.FieldString}, "Description": {APIName: "Description", Type: storage.FieldString}, "AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}, FlowRules: []storage.FlowRule{rule}}}
						query := "Grove lookup miss"
						if cells[0] == "L02" || cells[0] == "L04" {
							query = "Grove lookup hit"
						}
						records := []storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"FirstName": storage.StringValue(query), "AccountId": storage.IDValue(id)}}}
						engine := NewEngine(&org)
						if err := engine.ApplyBeforeSaveFlows(records); err != nil {
							t.Fatalf("native exception=%v; Glade: %v", want.Exception, err)
						}
						got := records[0]
						v, has := got.Fields["AccountId"]
						isNull := !has || v.Kind == storage.ValueNull
						matches := !isNull && idFromStorageValue(v) == id
						if isNull != want.IsNull || matches != want.MatchesTarget || got.Fields["Description"].String != want.Description {
							t.Fatalf("native %s; Glade isNull=%t matchesTarget=%t description=%q", cells[4], isNull, matches, got.Fields["Description"].String)
						}
					})
				}
			}
		}
	}
}
