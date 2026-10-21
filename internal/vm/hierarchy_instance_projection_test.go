package vm

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHierarchyInheritedInstanceJSONProjection(t *testing.T) {
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	org.Objects["TriadPreference__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "TriadPreference__c", Label: "Triad Preference", KeyPrefix: "a02",
		Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
		Fields: map[string]storage.Field{
			"Id":           {APIName: "Id", Type: storage.FieldID},
			"Name":         {APIName: "Name", Type: storage.FieldString},
			"SetupOwnerId": {APIName: "SetupOwnerId", Type: storage.FieldString},
			"Armed__c":     {APIName: "Armed__c", Type: storage.FieldBoolean, DefaultValue: "false"},
			"Tone__c":      {APIName: "Tone__c", Type: storage.FieldString, DefaultValue: "\"base\""},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`
insert new TriadPreference__c(SetupOwnerId=UserInfo.getOrganizationId(),Armed__c=true,Tone__c='bright');
TriadPreference__c inherited=TriadPreference__c.getInstance();
System.assertEquals(null,inherited.Id);
System.assertEquals(null,TriadPreference__c.getValues(UserInfo.getUserId()));
String payload=JSON.serialize(inherited);
String owner=UserInfo.getUserId();
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(machine.Globals["payload"].Text), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"attributes": map[string]any{"type": "TriadPreference__c"},
		"Name":       "Triad Preference (Organization)", "SetupOwnerId": machine.Globals["owner"].Text,
		"IsDeleted": false, "Armed__c": true, "Tone__c": "bright",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inherited JSON = %#v, want %#v", got, want)
	}
}

func TestHierarchyInheritedNameProjectionScope(t *testing.T) {
	const recordID storage.ID = "a02000000000001"
	for _, tc := range []struct {
		name      string
		value     storage.Value
		generated bool
		want      string
	}{
		{"generated ID15", storage.StringValue(string(recordID)), true, string(recordID)},
		// These are unchanged local scope controls, not native answers for
		// explicit, empty or null Name input shapes.
		{"assigned name unchanged", storage.StringValue("Assigned marker"), false, "Triad Preference (Organization)"},
		{"empty name unchanged", storage.StringValue(""), false, "Triad Preference (Organization)"},
		{"null name unchanged", storage.NullValue(), false, "Triad Preference (Organization)"},
		{"unmarked ID15 unchanged", storage.StringValue(string(recordID)), false, "Triad Preference (Organization)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testDataOrg()
			org.Objects["TriadPreference__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{
					APIName: "TriadPreference__c", Label: "Triad Preference",
					Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
					Fields: map[string]storage.Field{
						"Name":         {APIName: "Name", Type: storage.FieldString},
						"SetupOwnerId": {APIName: "SetupOwnerId", Type: storage.FieldReference},
					},
				},
				Records: map[storage.ID]storage.Record{
					recordID: {ID: recordID, Object: "TriadPreference__c", HierarchyNameUsesRecordID: tc.generated, Fields: map[string]storage.Value{
						"Name": tc.value, "SetupOwnerId": storage.StringValue("00D000000000mH2"),
					}},
				},
			}
			machine := New(nil)
			machine.SetOrg(&org)
			value, err := machine.hierarchyCustomSettingInstance("TriadPreference__c", "custom setting", nil)
			if err != nil || value.Fields["Name"].Text != tc.want {
				t.Fatalf("Name=%v, want %q; error=%v", value.Fields["Name"], tc.want, err)
			}
		})
	}
}
