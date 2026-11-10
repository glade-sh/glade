package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rows use fixed test-context search results and a direct queue member.
func TestPrivateQueueMemberFixedSOSLRoutes(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/flow_followups_custom_queue_sosl/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("native rows=%d, want 2", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[1], func(t *testing.T) {
			var want map[string]any
			if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
				t.Fatal(err)
			}
			for _, route := range []struct{ name, key, sharing, suffix string }{
				{"with-sharing", "soslWithSharing", "with sharing", ""},
				{"user-mode", "soslUserMode", "without sharing", " WITH USER_MODE"},
			} {
				t.Run(route.name, func(t *testing.T) {
					machine := recordSharingControlMachine(t, "Private")
					queueDerivedRouteMachine(t, machine)
					objects := machine.Org.Objects["Widget__c"]
					ids := make([]string, 0, len(objects.Records))
					for id := range objects.Records {
						ids = append(ids, string(id))
					}
					sort.Strings(ids)
					if len(ids) != 2 {
						t.Fatalf("fixture records=%d, want 2", len(ids))
					}
					for i, id := range ids {
						record := objects.Records[storage.ID(id)]
						record.System.OwnerID = "005000000000001"
						if i == 0 {
							record.System.OwnerID = "00G000000000001"
						}
						objects.Records[storage.ID(id)] = record
					}
					machine.Org.Objects["Widget__c"] = objects
					machine.SetOrg(machine.Org)
					source := fmt.Sprintf("Test.setFixedSearchResults(new List<Id>{'%s','%s'}); List<List<SObject>> rows=[FIND 'Synthetic' IN ALL FIELDS RETURNING Widget__c(Id)%s]; return rows[0].size();", ids[0], ids[1], route.suffix)
					body, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[1]})
					if err != nil {
						t.Fatal(err)
					}
					class := Class{Name: "SyntheticSearchReader", APIVersion: row[1], Modifiers: []string{route.sharing}}
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
					method := Method{Name: class.Name + ".read", ClassName: class.Name, IsStatic: true, Access: "public", APIVersion: row[1], SourceContextBound: true, SharingMode: route.sharing, Program: body}
					if err := machine.RegisterMethod(method); err != nil {
						t.Fatal(err)
					}
					value, err := machine.callMethod(machine.Methods[method.Name], nil, &Result{})
					if err != nil {
						t.Fatal(err)
					}
					expected, ok := want[route.key].(float64)
					if !ok || value.Kind != ValueInt || float64(value.Int) != expected {
						t.Fatalf("search rows=%v, native=%v", value, want[route.key])
					}
				})
			}
		})
	}
}
