package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestFieldPermissionTargetUsesDeclaredPermissionableMetadata(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Owned__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Owned__c", Fields: map[string]storage.Field{
		"Denied__c":  {APIName: "Denied__c", Permissionable: storage.BoolFlag(false)},
		"Allowed__c": {APIName: "Allowed__c", Permissionable: storage.BoolFlag(true)},
		"Unknown__c": {APIName: "Unknown__c"},
	}}}
	engine := NewEngine(&org)
	for _, target := range []string{"Owned__c.Denied__c", "Owned__c.Allowed__c", "Owned__c.Unknown__c", "Owned__c.Missing__c", "Missing__c.Value__c", "malformed", "Owned__c."} {
		err := engine.validateFieldPermissionTarget(storage.Record{Object: "FieldPermissions", Fields: map[string]storage.Value{"Field": storage.StringValue(target)}})
		if (err != nil) != (target == "Owned__c.Denied__c") {
			t.Fatalf("target=%s err=%v", target, err)
		}
	}
	// This control preserves unknown/malformed behavior; it does not grant it SF credit.
	if err := engine.validateFieldPermissionTarget(storage.Record{Object: "Other", Fields: map[string]storage.Value{"Field": storage.StringValue("Owned__c.Denied__c")}}); err != nil {
		t.Fatal(err)
	}
}
