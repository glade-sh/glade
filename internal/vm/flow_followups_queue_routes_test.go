package vm

import (
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native Private custom-object capture observes direct queue-member reads
// through both with-sharing and USER_MODE queries at APIs 62 and 67.
func TestPrivateQueueMemberQueryRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []struct {
			name, sharing, suffix string
			count                 bool
		}{
			{"with-sharing", "with sharing", "", false},
			{"user-mode", "without sharing", " WITH USER_MODE", false},
			{"with-sharing-count", "with sharing", "", true},
			{"user-mode-count", "without sharing", " WITH USER_MODE", true},
		} {
			t.Run(api+"/"+route.name, func(t *testing.T) {
				machine := recordSharingControlMachine(t, "Private")
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
				objects := machine.Org.Objects["Widget__c"]
				ids := make([]string, 0, len(objects.Records))
				for id := range objects.Records {
					ids = append(ids, string(id))
				}
				sort.Strings(ids)
				if len(ids) != 2 {
					t.Fatalf("sharing fixture records=%d, want 2", len(ids))
				}
				for i, id := range ids {
					row := objects.Records[storage.ID(id)]
					row.System.OwnerID = "005000000000001"
					if i == 0 {
						row.System.OwnerID = queue
					}
					objects.Records[storage.ID(id)] = row
				}
				machine.Org.Objects["Widget__c"] = objects
				machine.SetOrg(machine.Org)
				query := "SELECT Name FROM Widget__c" + route.suffix
				source := "return Database.query('" + query + "');"
				if route.count {
					source = "return [SELECT COUNT() FROM Widget__c" + route.suffix + "];"
				}
				body, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				class := Class{Name: "SyntheticQueueReader", APIVersion: api, Modifiers: []string{route.sharing}}
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
				method := Method{Name: class.Name + ".read", ClassName: class.Name, IsStatic: true, Access: "public", APIVersion: api, SourceContextBound: true, SharingMode: route.sharing, Program: body}
				if err := machine.RegisterMethod(method); err != nil {
					t.Fatal(err)
				}
				value, err := machine.callMethod(machine.Methods[method.Name], nil, &Result{})
				if err != nil {
					t.Fatal(err)
				}
				if route.count {
					if value.Kind != ValueInt || value.Int != 1 {
						t.Fatalf("visible count=%v, native=1", value)
					}
				} else if len(value.List) != 1 {
					t.Fatalf("visible records=%d, native=1", len(value.List))
				}
			})
		}
	}
}
