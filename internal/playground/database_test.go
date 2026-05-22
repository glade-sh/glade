package playground

import (
	"testing"

	"github.com/open-aer/oaer/internal/storage"
)

func TestDatabaseSnapshotClassifiesObjectKinds(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account"}}
	org.Objects["Invoice__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Invoice__c"}}
	org.Objects["Feature__mdt"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Feature__mdt"}}
	org.Objects["VerifiableProtectedListSetting__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:  "VerifiableProtectedListSetting__c",
			Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "List"},
		},
	}
	org.Objects["FieldPermissions"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "FieldPermissions"}}
	org.Objects["ApexClass"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "ApexClass"}}

	snapshot := databaseSnapshot(org)

	want := map[string]string{
		"Account":                           "standard",
		"Invoice__c":                        "custom",
		"Feature__mdt":                      "custom_metadata",
		"VerifiableProtectedListSetting__c": "custom_setting",
		"FieldPermissions":                  "system",
		"ApexClass":                         "system",
	}
	for objectName, kind := range want {
		object := databaseObjectByName(snapshot, objectName)
		if object == nil {
			t.Fatalf("%s missing from database snapshot", objectName)
		}
		if object.Kind != kind {
			t.Fatalf("%s kind = %q, want %q", objectName, object.Kind, kind)
		}
	}
}
