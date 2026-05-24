package playground

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDatabaseSnapshotSortsObjectsByName(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Zulu__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Zulu__c"},
		Records: map[storage.ID]storage.Record{
			"001": {ID: "001", Object: "Zulu__c"},
			"002": {ID: "002", Object: "Zulu__c"},
		},
	}
	org.Objects["Alpha__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Alpha__c"},
	}
	org.Objects["Middle__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Middle__c"},
		Records: map[storage.ID]storage.Record{
			"003": {ID: "003", Object: "Middle__c"},
		},
	}

	snapshot := databaseSnapshot(org)
	got := []string{snapshot.Objects[0].Name, snapshot.Objects[1].Name, snapshot.Objects[2].Name}
	want := []string{"Alpha__c", "Middle__c", "Zulu__c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("object order = %v, want %v", got, want)
	}
}

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
