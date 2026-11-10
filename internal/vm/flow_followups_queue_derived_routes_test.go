package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func queueDerivedRouteMachine(t *testing.T, machine *VM) {
	t.Helper()
	machine.testContext.CurrentUser.Fields["UserType"] = String("Standard")
	for _, name := range []string{"Group", "GroupMember"} {
		storage.EnsureStandardObject(machine.Org, name)
	}
	queue := storage.ID("00G000000000001")
	groups := machine.Org.Objects["Group"]
	groups.Records = map[storage.ID]storage.Record{queue: {ID: queue, Object: "Group", Fields: map[string]storage.Value{"Type": storage.StringValue("Queue")}}}
	machine.Org.Objects["Group"] = groups
	members := machine.Org.Objects["GroupMember"]
	members.Records = map[storage.ID]storage.Record{"011000000000001": {ID: "011000000000001", Object: "GroupMember", Fields: map[string]storage.Value{"GroupId": storage.IDValue(queue), "UserOrGroupId": storage.IDValue("005000000000002")}}}
	machine.Org.Objects["GroupMember"] = members
}

// Native rows exercise grouped visibility and projection of a queue-owned
// private parent under the current direct member at APIs 62 and 67.
func TestPrivateQueueMemberDerivedQueryRoutes(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/flow_followups_custom_queue_routes/org.tsv")
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
			for _, route := range []struct{ name, sharing, suffix, countKey, rowsKey string }{
				{"with-sharing", "with sharing", "", "groupCount", "groupRows"},
				{"user-mode", "without sharing", " WITH USER_MODE", "groupCountUser", "groupRowsUser"},
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
					query := "SELECT OwnerId,COUNT(Id) total FROM Widget__c" + route.suffix + " GROUP BY OwnerId"
					body, err := CompileAnonymousWithOptions("return Database.query('"+query+"');", CompileOptions{APIVersion: row[1]})
					if err != nil {
						t.Fatal(err)
					}
					class := Class{Name: "SyntheticGroupedReader", APIVersion: row[1], Modifiers: []string{route.sharing}}
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
					if float64(len(value.List)) != want[route.rowsKey] {
						t.Fatalf("groups=%d, native=%v", len(value.List), want[route.rowsKey])
					}
					if len(value.List) != 1 {
						t.Fatalf("expected one accessible group, got %d", len(value.List))
					}
					_, count, ok := objectFieldValue(value.List[0], "total")
					if !ok || count.Kind != ValueInt || float64(count.Int) != want[route.countKey] {
						t.Fatalf("group count=%v, native=%v", count, want[route.countKey])
					}
				})
			}
			t.Run("parent-projection", func(t *testing.T) {
				machine := hierarchyRecordVisibilityMachine(t)
				queueDerivedRouteMachine(t, machine)
				objects := machine.Org.Objects["Widget__c"]
				parent := objects.Records["a00000000000002"]
				parent.System.OwnerID = "00G000000000001"
				parent.Fields["Name"] = storage.StringValue("Full")
				objects.Records[parent.ID] = parent
				child := objects.Records["a00000000000003"]
				child.System.OwnerID = "005000000000002"
				objects.Records[child.ID] = child
				machine.Org.Objects["Widget__c"] = objects
				machine.SetOrg(machine.Org)
				body, err := CompileAnonymousWithOptions("return [SELECT Parent__r.Name FROM Widget__c WHERE Id='a00000000000003'];", CompileOptions{APIVersion: row[1]})
				if err != nil {
					t.Fatal(err)
				}
				class := Class{Name: "SyntheticParentReader", APIVersion: row[1], Modifiers: []string{"with sharing"}}
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
				method := Method{Name: class.Name + ".read", ClassName: class.Name, IsStatic: true, Access: "public", APIVersion: row[1], SourceContextBound: true, SharingMode: "with sharing", Program: body}
				if err := machine.RegisterMethod(method); err != nil {
					t.Fatal(err)
				}
				value, err := machine.callMethod(machine.Methods[method.Name], nil, &Result{})
				if err != nil {
					t.Fatal(err)
				}
				if len(value.List) != 1 {
					t.Fatalf("child rows=%d, native=1", len(value.List))
				}
				_, parentValue, ok := objectFieldValue(value.List[0], "Parent__r")
				if !ok || parentValue.Kind != ValueObject || stringField(parentValue, "Name") != want["parentProjected"] {
					t.Fatalf("parent projection=%v, native=%v", parentValue, want["parentProjected"])
				}
			})
		})
	}
}
