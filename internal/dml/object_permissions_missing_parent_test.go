package dml

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// case289_followups/org.tsv:12, :96, :184, :268 (OPV8, API 62.0 and 67.0,
// isTest and anonymous): inserting ObjectPermissions whose ParentId names no
// permission set fails with INVALID_CROSS_REFERENCE_KEY "invalid cross
// reference id" and no field list.
func TestObjectPermissionsMissingParentIsInvalidCrossReference(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	record := storage.Record{Object: "ObjectPermissions", Fields: map[string]storage.Value{
		"ParentId": storage.IDValue("0PS000000000001"), "SobjectType": storage.StringValue("Contact"), "PermissionsRead": storage.BooleanValue(true),
	}}
	err := missingReferenceError(record, "ParentId", "0PS000000000001")
	var typed dmlError
	if !errors.As(err, &typed) {
		t.Fatalf("not a dml error: %v", err)
	}
	code, fields, message := typed.code, typed.fields, typed.message
	if code != "INVALID_CROSS_REFERENCE_KEY" || len(fields) != 0 || !strings.Contains(message, "invalid cross reference id") {
		t.Fatalf("missing ObjectPermissions parent = %s %v %q; native INVALID_CROSS_REFERENCE_KEY () invalid cross reference id", code, fields, message)
	}
	// approval_reference api62/api67 istest and anonymous org.tsv ("wrong"):
	// ParentId set to a User Id keeps FIELD_INTEGRITY_EXCEPTION.
	wrong := storage.Record{Object: "ObjectPermissions", Fields: map[string]storage.Value{
		"ParentId": storage.IDValue(storage.DefaultUserID), "SobjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true),
	}}
	var wrongErr dmlError
	if err := missingReferenceError(wrong, "ParentId", storage.DefaultUserID); !errors.As(err, &wrongErr) || wrongErr.code != "FIELD_INTEGRITY_EXCEPTION" {
		t.Fatalf("ObjectPermissions.ParentId with a User Id = %v, want FIELD_INTEGRITY_EXCEPTION (approval_reference wrong)", err)
	}
	account := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue("001000000000999")}}
	var other dmlError
	if err := missingReferenceError(account, "AccountId", "001000000000999"); !errors.As(err, &other) || other.code != "FIELD_INTEGRITY_EXCEPTION" {
		t.Fatalf("Contact.AccountId missing target = %v, want the existing FIELD_INTEGRITY_EXCEPTION", err)
	}
}
