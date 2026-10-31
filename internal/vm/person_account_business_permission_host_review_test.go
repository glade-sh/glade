package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local host twins use the read/create-only business rows captured in review2/org.tsv.
func TestPersonAccountBusinessPermissionHostNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"lwc", "visualforce"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				storage.EnsureDeterministicPlatformData(&org)
				storage.EnsureStandardObject(&org, "PermissionSetAssignment")
				storage.EnsureStandardObject(&org, "FieldPermissions")
				const userID = storage.ID("005000000000998")
				const psID = storage.ID("0PS000000000998")
				org.Objects["ObjectPermissions"].Records["110000000000998"] = storage.Record{ID: "110000000000998", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(psID), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(false)}}
				org.Objects["FieldPermissions"].Records["0FP000000000998"] = storage.Record{ID: "0FP000000000998", Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(psID), "SObjectType": storage.StringValue("Account"), "Field": storage.StringValue("Account.Name"), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
				org.Objects["PermissionSetAssignment"].Records["0Pa000000000998"] = storage.Record{ID: "0Pa000000000998", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(userID), "PermissionSetId": storage.IDValue(psID)}}
				machine := New(nil)
				machine.SetOrg(&org)
				user := Object("User")
				user.Fields["Id"] = String(string(userID))
				user.Fields["profileId"] = String("00e000000000002")
				machine.executionUser = user
				body, err := CompileAnonymousWithOptions(`List<Account> rows=new List<Account>{new Account(Name='Boundary'),new Account(Name='Control')}; upsert as user rows; return 'success';`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				class := Class{Name: "DisabledAccountHost", Methods: map[string]Method{
					"lwc": {Name: "DisabledAccountHost.lwc", ClassName: "DisabledAccountHost", APIVersion: api, ReturnType: "String", IsStatic: true, Modifiers: []string{"public", "AuraEnabled"}, Program: body},
					"vf":  {Name: "DisabledAccountHost.vf", ClassName: "DisabledAccountHost", APIVersion: api, ReturnType: "String", Modifiers: []string{"public"}, Program: body},
				}}
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
				var result UIInvocationResult
				if route == "lwc" {
					result, err = machine.InvokeLWCMethod("DisabledAccountHost", "lwc", nil)
				} else {
					result, err = machine.InvokeVisualforceAction("DisabledAccountHost", "vf", "/apex/DisabledAccountHost", nil)
				}
				if err != nil || !result.Success || result.ReturnValue != "success" {
					t.Fatalf("result=%+v err=%v", result, err)
				}
			})
		}
	}
}
