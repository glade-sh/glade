package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce's record-sharing documentation grants Read or Edit access to a
// recipient independently of record ownership; Edit does not grant deletion.
func TestDirectCustomRecordShareSeparatesReadAndEdit(t *testing.T) {
	for _, tc := range []struct {
		name, access           string
		otherParent, otherUser bool
		visible, write         bool
	}{
		{name: "none"},
		{name: "read", access: "Read", visible: true},
		{name: "edit", access: "Edit", visible: true, write: true},
		{name: "different_parent", access: "Edit", otherParent: true},
		{name: "different_recipient", access: "Edit", otherUser: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := recordSharingControlMachine(t, "Private")
			org := machine.Org
			state := org.Objects["Widget__c"]
			target := state.Records["a00000000000001"]
			if tc.access != "" {
				parent := target.ID
				if tc.otherParent {
					parent = "a00000000000002"
				}
				recipient := storage.ID("005000000000002")
				if tc.otherUser {
					recipient = "005000000000001"
				}
				org.Objects["Widget__Share"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__Share"}, Records: map[storage.ID]storage.Record{"a01000000000001": {ID: "a01000000000001", Object: "Widget__Share", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent), "UserOrGroupId": storage.IDValue(recipient), "AccessLevel": storage.StringValue(tc.access), "RowCause": storage.StringValue("Manual")}}}}
			}
			if got := machine.userModeRecordVisible("Widget__c", target, "005000000000002"); got != tc.visible {
				t.Errorf("visibility=%t want=%t", got, tc.visible)
			}
			for _, op := range []string{"update", "upsert", "delete", "undelete"} {
				want := tc.write && (op == "update" || op == "upsert")
				if got := machine.currentUserCanWriteRecord("Widget__c", target, "005000000000002", op); got != want {
					t.Errorf("%s access=%t want=%t", op, got, want)
				}
			}
			if err := machine.RegisterClass(Class{Name: "ShareWriter", APIVersion: "66.0", Modifiers: []string{"with sharing"}}); err != nil {
				t.Fatal(err)
			}
			body, err := CompileAnonymousWithOptions("Widget__c row=new Widget__c(Id='a00000000000001',Name='changed');update row;", CompileOptions{APIVersion: "66.0"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.callMethod(Method{Name: "ShareWriter.write", ClassName: "ShareWriter", IsStatic: true, APIVersion: "66.0", SourceContextBound: true, SharingMode: "with sharing", Program: body}, nil, &Result{})
			if tc.write {
				if err != nil {
					t.Errorf("Edit-share update: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY") {
				t.Errorf("denied update=%v", err)
			}
			wantName := "Owned"
			if tc.write {
				wantName = "changed"
			}
			if got := org.Objects["Widget__c"].Records[target.ID].Fields["Name"].String; got != wantName {
				t.Errorf("stored name=%q want=%q", got, wantName)
			}
		})
	}
}

// Record access does not supply object CRUD or field edit permissions in
// user mode. A direct Edit share must retain both front-door restrictions.
func TestDirectCustomRecordShareRetainsUserModePermissions(t *testing.T) {
	for _, permission := range []string{"object", "field"} {
		t.Run(permission, func(t *testing.T) {
			machine := recordSharingControlMachine(t, "Private")
			org := machine.Org
			org.Objects["Widget__Share"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__Share"}, Records: map[storage.ID]storage.Record{"a01000000000001": {ID: "a01000000000001", Object: "Widget__Share", Fields: map[string]storage.Value{"ParentId": storage.IDValue("a00000000000001"), "UserOrGroupId": storage.IDValue("005000000000002"), "AccessLevel": storage.StringValue("Edit")}}}}
			object, field := "ObjectPermissions", "PermissionsEdit"
			if permission == "field" {
				object = "FieldPermissions"
			}
			state := org.Objects[object]
			for id, row := range state.Records {
				row.Fields[field] = storage.BooleanValue(false)
				state.Records[id] = row
			}
			org.Objects[object] = state
			machine.SetOrg(org)
			if err := machine.RegisterClass(Class{Name: "SecureShareWriter", APIVersion: "67.0", Modifiers: []string{"with sharing"}}); err != nil {
				t.Fatal(err)
			}
			body, err := CompileAnonymousWithOptions("Widget__c row=new Widget__c(Id='a00000000000001',Name='changed');update row;", CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.callMethod(Method{Name: "SecureShareWriter.write", ClassName: "SecureShareWriter", IsStatic: true, APIVersion: "67.0", SourceContextBound: true, SharingMode: "with sharing", Program: body}, nil, &Result{})
			if err == nil {
				t.Fatal("Edit share bypassed user-mode permission denial")
			}
			if got := org.Objects["Widget__c"].Records["a00000000000001"].Fields["Name"].String; got != "Owned" {
				t.Fatalf("denied write changed name=%q", got)
			}
		})
	}
}
