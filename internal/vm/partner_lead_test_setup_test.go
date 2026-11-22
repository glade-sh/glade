package vm

import "github.com/glade-sh/glade/internal/storage"

func addPartnerLeadReadPermission(org *storage.OrgState) {
	for _, name := range []string{"User", "PermissionSet", "PermissionSetAssignment", "ObjectPermissions"} {
		storage.EnsureStandardObject(org, name)
	}
	users := org.Objects["User"]
	if _, exists := users.Records["005000000000002"]; !exists {
		users.PutRecord("005000000000002", storage.Record{ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000002"), "UserType": storage.StringValue("PowerPartner")}})
	}
	org.Objects["User"] = users
	sets := org.Objects["PermissionSet"]
	sets.PutRecord("0PS000000000001", storage.Record{ID: "0PS000000000001", Object: "PermissionSet"})
	org.Objects["PermissionSet"] = sets
	assignments := org.Objects["PermissionSetAssignment"]
	assignments.PutRecord("0Pa000000000001", storage.Record{ID: "0Pa000000000001", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000002"), "PermissionSetId": storage.IDValue("0PS000000000001")}})
	org.Objects["PermissionSetAssignment"] = assignments
	permissions := org.Objects["ObjectPermissions"]
	permissions.PutRecord("110000000000001", storage.Record{ID: "110000000000001", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000001"), "SObjectType": storage.StringValue("Lead"), "PermissionsRead": storage.BooleanValue(true)}})
	org.Objects["ObjectPermissions"] = permissions
}
