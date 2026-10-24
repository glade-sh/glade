package dml

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These engine twins pin AR04/AR07/AR08/AR09/AR12/AR14 from the
// assignment_rules capture extensions; the project test runs their Apex source.
func TestAssignmentRuleEngineInsertUpdateSelectionRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, operation := range []string{"insert", "update"} {
			for _, tc := range []struct{ name, company, selection, owner string }{
				{"default_first_match", "multi", `{"UseDefault":true}`, "00G000000000001"},
				{"default_formula", "formula", `{"UseDefault":true}`, "00G000000000001"},
				{"inactive_id", "miss", `{"ID":"01Q000000000002"}`, "00G000000000002"},
				{"unknown_id", "multi", `{"ID":"01Q000000000099"}`, string(storage.DefaultUserID)},
				{"disabled", "multi", `{"UseDefault":false}`, string(storage.DefaultUserID)},
				{"no_match", "miss", `{"UseDefault":true}`, string(storage.DefaultUserID)},
			} {
				t.Run(api+"/"+operation+"/"+tc.name, func(t *testing.T) {
					org := storage.NewOrgState()
					org.APIVersion = api
					storage.EnsureStandardObject(&org, "Lead")
					storage.EnsureStandardObject(&org, "Group")
					storage.EnsureDeterministicPlatformData(&org)
					state := org.Objects["Lead"]
					if err := json.Unmarshal([]byte(`{"assignmentRules":[{"id":"01Q000000000001","name":"Active","active":true,"entries":[{"assignedTo":"First","assignedToType":"Queue","formula":"CONTAINS(Company, \"formula\")"},{"assignedTo":"First","assignedToType":"Queue","booleanFilter":"1 OR 2","criteria":[{"field":"Company","operation":"equals","value":"match"},{"field":"Company","operation":"equals","value":"multi"}]},{"assignedTo":"Second","assignedToType":"Queue","criteria":[{"field":"Company","operation":"equals","value":"multi"}]}]},{"id":"01Q000000000002","name":"Inactive","entries":[{"assignedTo":"Second","assignedToType":"Queue","formula":"TRUE"}]}]}`), &state.Definition); err != nil {
						t.Fatal(err)
					}
					org.Objects["Lead"] = state
					groups := org.Objects["Group"]
					if groups.Records == nil {
						groups.Records = map[storage.ID]storage.Record{}
					}
					for i, name := range []string{"First", "Second"} {
						id := []storage.ID{"00G000000000001", "00G000000000002"}[i]
						groups.Records[id] = storage.Record{Object: "Group", ID: id, Fields: map[string]storage.Value{"Type": storage.StringValue("Queue"), "DeveloperName": storage.StringValue(name)}}
					}
					org.Objects["Group"] = groups
					engine := NewEngine(&org)
					input := storage.Record{Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue("Synthetic"), "Company": storage.StringValue(tc.company)}}
					if operation == "update" {
						saved := engine.Insert([]storage.Record{input})
						if !saved[0].Success {
							t.Fatal(saved)
						}
						input.ID = saved[0].ID
					}
					if err := json.Unmarshal([]byte(`{"AssignmentRule":`+tc.selection+`}`), &engine.Options); err != nil {
						t.Fatal(err)
					}
					var saved []Result
					if operation == "update" {
						saved = engine.Update([]storage.Record{input})
					} else {
						saved = engine.Insert([]storage.Record{input})
					}
					if len(saved) != 1 || !saved[0].Success {
						t.Fatal(saved)
					}
					owner := org.Objects["Lead"].Records[saved[0].ID].System.OwnerID
					if !storage.IDsEqual(owner, storage.ID(tc.owner)) {
						t.Fatalf("owner=%s, native owner=%s", owner, tc.owner)
					}
				})
			}
		}
	}
}
