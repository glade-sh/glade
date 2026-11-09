package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// P0-field-access/org.tsv records the named route before and after the grant.
func TestStandardAPI68NamedFieldPermissionTransition(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper})
	for _, name := range []string{"PermissionSet", "PermissionSetAssignment", "FieldPermissions", "ObjectPermissions"} {
		storage.EnsureStandardObject(&org, name)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "68.0", SourceContextBound: true}
	machine.currentClass = "CapturedNamedClass"
	machine.executionUser = Object("User")
	machine.executionUser.Fields["Id"] = String("005000000000998")
	check := func(field string, read, create, update bool) {
		t.Helper()
		desc, err := machine.describeFieldValue("ShiftAllocationRequest", field)
		if err != nil {
			t.Fatal(err)
		}
		if desc.Fields["accessible"].Bool != read || desc.Fields["createable"].Bool != create || desc.Fields["updateable"].Bool != update {
			t.Fatalf("%s flags read/create/update=%t/%t/%t, want %t/%t/%t", field, desc.Fields["accessible"].Bool, desc.Fields["createable"].Bool, desc.Fields["updateable"].Bool, read, create, update)
		}
	}
	check("CurrentAssigneeId", false, false, false)
	check("RequestType", false, false, false)
	org.Objects["PermissionSetAssignment"].Records["0Pa000000000998"] = storage.Record{ID: "0Pa000000000998", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000998"), "PermissionSetId": storage.IDValue("0PS000000000998")}}
	org.Objects["ObjectPermissions"].Records["110000000000998"] = storage.Record{ID: "110000000000998", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000998"), "SObjectType": storage.StringValue("ShiftAllocationRequest"), "PermissionsRead": storage.BooleanValue(true)}}
	for i, name := range []string{"CurrentAssigneeId", "RequestType"} {
		id := storage.ID([]string{"0FP000000000997", "0FP000000000998"}[i])
		org.Objects["FieldPermissions"].Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000998"), "SObjectType": storage.StringValue("ShiftAllocationRequest"), "Field": storage.StringValue("ShiftAllocationRequest." + name), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(false)}}
	}
	check("CurrentAssigneeId", true, true, true)
	check("RequestType", true, true, false)
	desc, err := machine.describeFieldValue("ShiftAllocationRequest", "RequestType")
	if err != nil {
		t.Fatal(err)
	}
	values := desc.Fields["picklistValues"].List
	if len(values) != 1 || values[0].Fields["value"].Text != "Pickup" || values[0].Fields["label"].Text != "Pick Up" || !values[0].Fields["active"].Bool || values[0].Fields["defaultValue"].Bool {
		t.Fatalf("RequestType picklist = %v, want active non-default Pickup / Pick Up", values)
	}
}
