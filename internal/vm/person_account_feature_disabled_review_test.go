package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The shipping implementation requires edit permission for these Account
// upsert shapes. With PersonAccounts disabled the review fix must retain that
// existing behavior; no new native behavior is asserted for this control.
func TestPersonAccountFeatureDisabledPreservesUpsertPermissionBaseline(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, source string }{
			{"statement_scalar", `upsert as user new Account(Name='Boundary');`},
			{"statement_list", `List<Account> rows=new List<Account>{new Account(Name='Boundary'),new Account(Name='Control')}; upsert as user rows;`},
			{"database_user", `Database.upsert(new Account(Name='Boundary'),false,AccessLevel.USER_MODE);`},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := testDataOrg()
				storage.EnsureDeterministicPlatformData(&org)
				if _, ok := storage.ResolveFieldName(org.Objects["Account"].Definition, "", "IsPersonAccount"); ok {
					t.Fatal("control unexpectedly enables PersonAccounts")
				}
				storage.EnsureStandardObject(&org, "PermissionSetAssignment")
				const userID = storage.ID("005000000000998")
				const psID = storage.ID("0PS000000000998")
				org.Objects["ObjectPermissions"].Records["110000000000998"] = storage.Record{ID: "110000000000998", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(psID), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(false)}}
				org.Objects["PermissionSetAssignment"].Records["0Pa000000000998"] = storage.Record{ID: "0Pa000000000998", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(userID), "PermissionSetId": storage.IDValue(psID)}}
				user := Object("User")
				user.Fields["Id"] = String(string(userID))
				user.Fields["profileId"] = String("00e000000000002")
				machine := New(nil)
				machine.SetOrg(&org)
				machine.executionUser = user
				program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.Execute(program)
				if err == nil || !strings.Contains(err.Error(), "Access to entity 'Account' denied") {
					t.Fatalf("got %v, want shipping Account edit-permission denial", err)
				}
			})
		}
	}
}
