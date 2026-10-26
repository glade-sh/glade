package dml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPlatformEventFlowElementsEngineNativeRows(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/platform_event_flow_elements"
	for _, api := range []string{"62.0", "67.0"} {
		raw, err := os.ReadFile(filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		rows := map[string]map[string]float64{}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
			cells := strings.SplitN(line, "\t", 3)
			if len(cells) != 3 || cells[1] != "deliver" {
				continue
			}
			var row map[string]any
			if err := json.Unmarshal([]byte(cells[2]), &row); err != nil {
				t.Fatal(err)
			}
			rows[cells[0]] = map[string]float64{}
			for _, field := range []string{"flow", "accounts", "updated", "posts"} {
				value, ok := row[field].(float64)
				if !ok {
					t.Fatalf("missing native %s/%s", cells[0], field)
				}
				rows[cells[0]][field] = value
			}
		}
		for _, cell := range []struct{ id, operation string }{{"PE01", "create"}, {"PE02", "reject"}, {"PE03", "update"}, {"PE04", "delete"}, {"PE05", "loop"}, {"PE10", "batch"}, {"PE11", "user"}, {"PE12", "chatter"}} {
			t.Run(api+"/engine_delivery/"+cell.id, func(t *testing.T) {
				org := testOrg()
				account := org.Objects["Account"]
				account.Definition.Fields["Phone"] = storage.Field{APIName: "Phone", Type: storage.FieldString}
				org.Objects["Account"] = account
				for name, fields := range map[string][]string{"ElementSignal__e": {"Key__c", "Operation__c", "Parent__c"}, "ElementLog__c": {"Name", "Phase__c", "Actor__c", "Note__c"}} {
					definition := storage.ObjectDefinition{APIName: name, Fields: map[string]storage.Field{}}
					for _, field := range fields {
						definition.Fields[field] = storage.Field{APIName: field, Type: storage.FieldString, Length: 80}
					}
					if name == "ElementSignal__e" {
						definition.Metadata = map[string]string{"publishBehavior": "PublishImmediately"}
					}
					org.Objects[name] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
				}
				source, err := os.ReadFile(filepath.Join(fixture, "project/force-app/main/default/flows/ElementSubscriber.flow-meta.xml"))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "ElementSubscriber.flow-meta.xml")
				if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(source), "62.0", api)), 0600); err != nil {
					t.Fatal(err)
				}
				index, err := automation.LoadProject(project.Project{Root: filepath.Dir(path), SourceAPIVersion: api, FlowFiles: []string{path}})
				if err != nil || len(index.Diagnostics) != 0 {
					t.Fatalf("native flow accepted: %v %v", err, index.Diagnostics)
				}
				automation.ApplyToOrg(&org, index)
				engine := NewEngine(&org)
				fields := map[string]storage.Value{"Key__c": storage.StringValue("Engine signal"), "Operation__c": storage.StringValue(cell.operation)}
				if cell.operation == "update" || cell.operation == "delete" || cell.operation == "loop" || cell.operation == "chatter" {
					count := 1
					if cell.operation == "loop" {
						count = 2
					}
					for range count {
						inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": fields["Key__c"], "Phone": storage.StringValue("old")}}})
						if len(inserted) != 1 || !inserted[0].Success {
							t.Fatalf("seed: %+v", inserted)
						}
						fields["Parent__c"] = storage.StringValue(string(inserted[0].ID))
					}
				}
				payload := storage.Record{Object: "ElementSignal__e", Fields: fields}
				before := payload.Clone()
				records := []storage.Record{payload}
				if cell.operation == "batch" {
					records = append(records, payload, payload)
				}
				if err := engine.ApplyPlatformEventFlows(records); err != nil {
					t.Fatal(err)
				}
				got := map[string]float64{"flow": float64(len(org.Objects["ElementLog__c"].Records)), "accounts": 0, "updated": 0, "posts": float64(len(org.Objects["FeedItem"].Records))}
				for _, record := range org.Objects["Account"].Records {
					if record.System.IsDeleted {
						continue
					}
					got["accounts"]++
					if record.Fields["Phone"].String == "updated" {
						got["updated"]++
					}
				}
				if cell.operation == "user" {
					for _, record := range org.Objects["ElementLog__c"].Records {
						if !storage.IDsEqual(storage.ID(record.Fields["Actor__c"].String), record.System.CreatedByID) {
							t.Fatal("native current-user resource matches the created record user")
						}
					}
				}
				if !reflect.DeepEqual(got, rows[cell.id]) {
					t.Fatalf("native=%v; engine=%v", rows[cell.id], got)
				}
				if len(org.Objects["ElementSignal__e"].Records) != 0 || !reflect.DeepEqual(payload, before) {
					t.Fatal("event interview persisted or mutated the delivered payload")
				}
			})
		}
	}
}
