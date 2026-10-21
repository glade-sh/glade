package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func TestSOQLUserModePolymorphicOwnerContactProjection(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0"} {
		for _, field := range []struct{ name, user, queue string }{
			{"FirstName", "Atlas", "null"},
			{"LastName", "Reader", "null"},
			{"Phone", "5550101", "null"},
			{"Email", "reader@example.invalid", "queue@example.invalid"},
		} {
			t.Run(api+"/"+field.name, func(t *testing.T) {
				org, user := polymorphicOwnerContactOrg()
				queueExpected := "'" + field.queue + "'"
				if field.queue == "null" {
					queueExpected = "null"
				}
				source := fmt.Sprintf(`
List<Case> rows = [SELECT Id, Subject, Owner.%s FROM Case WITH USER_MODE ORDER BY Subject];
System.assertEquals(2, rows.size());
System.assertEquals('%s', rows[0].Owner.get('%s'));
System.assertEquals(%s, rows[1].Owner.get('%s'));
`, field.name, field.user, field.name, queueExpected, field.name)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				machine.executionUser = user
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestSOQLUserModePolymorphicOwnerContactSecurityControls(t *testing.T) {
	for _, field := range []string{"FirstName", "LastName", "Phone", "Email"} {
		denials := []string{"", "Case", "Case.OwnerId", "User." + field}
		if field == "Email" {
			denials = append(denials, "Group.Email")
		}
		for _, denied := range denials {
			t.Run(field+"/"+denied, func(t *testing.T) {
				org, user := polymorphicOwnerContactOrg()
				const scope = "0PS000000000001"
				org.Objects["ObjectPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{}}
				for i, object := range []string{"Case", "User", "Group"} {
					id := storage.ID(fmt.Sprintf("0OP00000000000%d", i+1))
					state := org.Objects["ObjectPermissions"]
					state.Records[id] = storage.Record{ID: id, Object: "ObjectPermissions", Fields: map[string]storage.Value{
						"ParentId": storage.IDValue(scope), "SObjectType": storage.StringValue(object),
						"PermissionsRead": storage.BooleanValue(denied != object),
					}}
					org.Objects["ObjectPermissions"] = state
				}
				org.Objects["FieldPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{}}
				for i, qualified := range []string{"Case.Id", "Case.OwnerId", "User." + field, "Group.Email"} {
					object, _, _ := strings.Cut(qualified, ".")
					id := storage.ID(fmt.Sprintf("0FP00000000000%d", i+1))
					state := org.Objects["FieldPermissions"]
					state.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{
						"ParentId": storage.IDValue(scope), "SObjectType": storage.StringValue(object), "Field": storage.StringValue(qualified),
						"PermissionsRead": storage.BooleanValue(denied != qualified),
					}}
					org.Objects["FieldPermissions"] = state
				}
				machine := New(nil)
				machine.SetOrg(&org)
				machine.executionUser = user
				err := machine.enforceSOQLSecurity(soql.Query{Object: "Case", Fields: []string{"Id", "Owner." + field}, SecurityMode: "USER_MODE"}, scope)
				if denied == "" {
					if err != nil {
						t.Fatalf("readable scoped projection: %v", err)
					}
					return
				}
				if err == nil {
					t.Fatal("expected permission denial")
				}
				if denied == "Case.OwnerId" && !strings.Contains(err.Error(), "OwnerId") {
					t.Fatalf("expected source reference denial, got %v", err)
				}
			})
		}
	}
}

func TestSOQLUserModePolymorphicOwnerExistingControls(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org, user := polymorphicOwnerContactOrg()
			source := `
List<Case> rows = [SELECT Id, Owner.Id, Owner.Name, Owner.Type FROM Case WITH USER_MODE ORDER BY Subject];
System.assertEquals(2, rows.size());
System.assertEquals('User', rows[0].Owner.Type);
System.assertEquals('Group', rows[1].Owner.Type);
System.assertEquals('Atlas Reader', rows[0].Owner.Name);
System.assertEquals('Synthetic queue', rows[1].Owner.Name);
Boolean unknownDenied = false;
try { Database.query('SELECT Id, Owner.UnknownLeaf__c FROM Case WITH USER_MODE'); }
catch (QueryException e) { unknownDenied = true; }
System.assert(unknownDenied);
`
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.executionUser = user
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSOQLUserModePolymorphicOwnerSharingControl(t *testing.T) {
	org, user := polymorphicOwnerContactOrg()
	state := org.Objects["Profile"]
	profile := state.Records["00e000000000001"]
	profile.Fields["Name"] = storage.StringValue("Synthetic readable profile")
	state.Records[profile.ID] = profile
	org.Objects["Profile"] = state
	cases := org.Objects["Case"]
	cases.Definition.SharingModel = "Private"
	for id, record := range cases.Records {
		record.System.OwnerID = "005000000000002"
		cases.Records[id] = record
	}
	org.Objects["Case"] = cases
	program, err := CompileAnonymousWithOptions(`
List<Case> rows = [SELECT Id, Owner.Name, Owner.Type FROM Case WITH USER_MODE];
System.assertEquals(0, rows.size());
`, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = user
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func polymorphicOwnerContactOrg() (storage.OrgState, Value) {
	org := storage.NewOrgState()
	for _, object := range []string{"Case", "User", "Group", "Profile"} {
		storage.EnsureStandardObject(&org, object)
	}
	put := func(object string, record storage.Record) {
		state := org.Objects[object]
		state.Records[record.ID] = record
		org.Objects[object] = state
	}
	put("Profile", storage.Record{ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}})
	put("User", storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{
		"ProfileId": storage.IDValue("00e000000000001"), "Name": storage.StringValue("Atlas Reader"),
		"FirstName": storage.StringValue("Atlas"), "LastName": storage.StringValue("Reader"),
		"Phone": storage.StringValue("5550101"), "Email": storage.StringValue("reader@example.invalid"),
	}})
	put("Group", storage.Record{ID: "00G000000000001", Object: "Group", Fields: map[string]storage.Value{
		"Name": storage.StringValue("Synthetic queue"), "Type": storage.StringValue("Queue"), "Email": storage.StringValue("queue@example.invalid"),
	}})
	put("Case", storage.Record{ID: "500000000000001", Object: "Case", Fields: map[string]storage.Value{
		"Subject": storage.StringValue("A user case"), "OwnerId": storage.IDValue("005000000000001"),
	}})
	put("Case", storage.Record{ID: "500000000000002", Object: "Case", Fields: map[string]storage.Value{
		"Subject": storage.StringValue("B queue case"), "OwnerId": storage.IDValue("00G000000000001"),
	}})
	user := Object("User")
	user.Fields["Id"] = String("005000000000001")
	user.Fields["ProfileId"] = String("00e000000000001")
	return org, user
}
