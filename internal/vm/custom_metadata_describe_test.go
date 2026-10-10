package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestCustomMetadataDescribeInventoryDoesNotChangeOrdinaryObjects(t *testing.T) {
	org := storage.NewOrgState()
	for _, name := range []string{"Owned__mdt", "Owned__c", "Account"} {
		org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Fields: map[string]storage.Field{
			"Flag_Value__c":      {APIName: "Flag_Value__c", Type: storage.FieldString},
			"isEnabled__c":       {APIName: "isEnabled__c", Type: storage.FieldBoolean},
			"isEnabled_After__c": {APIName: "isEnabled_After__c", Type: storage.FieldDate},
		}}}
	}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, name := range []string{"Owned__mdt", "Owned__c", "Account"} {
		t.Run(name, func(t *testing.T) {
			description := machine.describeSObjectValue(name, org.Objects[name].Definition)
			fields := description.Fields["fields"].Fields["map"]
			keys, _ := machine.sObjectFieldMapCanonicalKeySet(fields)
			if name == "Owned__mdt" {
				expected := map[string]Value{
					"developername": String("Test_Flag"), "flag_value__c": String("Test Value"), "id": Null,
					"isenabled__c": Bool(true), "isenabled_after__c": Null, "label": String("Test Flag"),
					"language": Null, "masterlabel": Null, "namespaceprefix": Null, "qualifiedapiname": Null, "systemmodstamp": Null,
				}
				if len(keys.Set) != len(expected) {
					t.Fatalf("CMT keys=%v", keys)
				}
				row := Object(name)
				row.Fields["DeveloperName"] = String("Test_Flag")
				row.Fields["Label"] = String("Test Flag")
				row.Fields["Flag_Value__c"] = String("Test Value")
				row.Fields["isEnabled__c"] = Bool(true)
				for _, key := range keys.Set {
					want, exists := expected[key.Text]
					if !exists {
						t.Fatalf("unexpected CMT field %s", key.Text)
					}
					value, handled, err := machine.callSObjectMember(row, "get", []Value{key})
					if !handled || err != nil || !value.Equal(want) {
						t.Errorf("get(%s)=%v, handled %v, error %v; want %v", key.Text, value, handled, err, want)
					}
				}
				if len(row.Fields) != 4 {
					t.Fatalf("dynamic reads populated record: %v", row.Fields)
				}
			} else {
				for _, field := range []string{"Name", "OwnerId", "CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById", "SystemModstamp"} {
					if _, ok := fields.Map[mapKey(String(field))]; !ok {
						t.Errorf("%s lost %s", name, field)
					}
				}
			}
		})
	}
}
