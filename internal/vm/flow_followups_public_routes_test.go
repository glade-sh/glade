package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Case native rows use ReadWriteTransfer for both with-sharing and USER_MODE
// count queries. Each row below isolates one captured non-owner shape.
func TestPublicTransferCaseQueryRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []struct{ name, sharing, suffix string }{
			{"with-sharing", "with sharing", ""},
			{"user-mode", "without sharing", " WITH USER_MODE"},
		} {
			for _, owner := range []storage.ID{"005000000000001", "00G000000000001"} {
				t.Run(api+"/"+route.name+"/"+string(owner), func(t *testing.T) {
					machine := recordSharingControlMachine(t, "Private")
					machine.testContext.CurrentUser.Fields["UserType"] = String("Standard")
					for _, name := range []string{"Case", "Group", "GroupMember"} {
						storage.EnsureStandardObject(machine.Org, name)
					}
					queue := storage.ID("00G000000000001")
					groups := machine.Org.Objects["Group"]
					groups.Records = map[storage.ID]storage.Record{queue: {ID: queue, Object: "Group", Fields: map[string]storage.Value{"Type": storage.StringValue("Queue")}}}
					machine.Org.Objects["Group"] = groups
					members := machine.Org.Objects["GroupMember"]
					members.Records = map[storage.ID]storage.Record{}
					machine.Org.Objects["GroupMember"] = members
					cases := machine.Org.Objects["Case"]
					cases.Definition.SharingModel = "ReadWriteTransfer"
					id := storage.ID("500000000000001")
					cases.Records = map[storage.ID]storage.Record{id: {ID: id, Object: "Case", System: storage.SystemFields{OwnerID: owner, CreatedByID: "005000000000001"}}}
					machine.Org.Objects["Case"] = cases
					permissions := machine.Org.Objects["ObjectPermissions"]
					for id, permission := range permissions.Records {
						permission.Fields["SObjectType"] = storage.StringValue("Case")
						permissions.Records[id] = permission
					}
					machine.Org.Objects["ObjectPermissions"] = permissions
					machine.SetOrg(machine.Org)
					body, err := CompileAnonymousWithOptions("return [SELECT COUNT() FROM Case"+route.suffix+"];", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					class := Class{Name: "SyntheticCaseReader", APIVersion: api, Modifiers: []string{route.sharing}}
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
					if value.Kind != ValueInt || value.Int != 1 {
						t.Fatalf("visible count=%v, native=1", value)
					}
				})
			}
		}
	}
}
