package apextest

import (
	"github.com/glade-sh/glade/internal/storage"
)

// grantAdminFieldAccess gives the default administrator profile read and edit
// on object.field, as a source org whose probe fields are visible to the
// administrator has. The standard_ext_upsert_compile_visibility rows show that
// an ungranted custom field is not even visible to the administrator.
func grantAdminFieldAccess(org *storage.OrgState, object, field string) {
	storage.EnsureStandardObject(org, "FieldPermissions")
	id := storage.ID("0FP" + storage.DefaultProfileID[3:12] + "KEY")
	org.Objects["FieldPermissions"].Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{
		"ParentId":        storage.IDValue(storage.ID(storage.DefaultProfileID)),
		"SObjectType":     storage.StringValue(object),
		"Field":           storage.StringValue(object + "." + field),
		"PermissionsRead": storage.BooleanValue(true),
		"PermissionsEdit": storage.BooleanValue(true),
	}}
}
