package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHierarchyDefaultRelayPreservesFieldsAndIdentity(t *testing.T) {
	for _, configured := range []bool{false, true} {
		org := testDataOrg()
		storage.EnsureDeterministicPlatformData(&org)
		org.Objects["TriadPreference__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName: "TriadPreference__c", KeyPrefix: "a02",
			Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
			Fields: map[string]storage.Field{
				"SetupOwnerId": {APIName: "SetupOwnerId", Type: storage.FieldString},
				"Armed__c":     {APIName: "Armed__c", Type: storage.FieldBoolean, DefaultValue: "false"},
				"Tone__c":      {APIName: "Tone__c", Type: storage.FieldString, DefaultValue: "\"base\""},
			},
		}}
		machine := New(nil)
		machine.SetOrg(&org)
		if err := machine.RegisterClass(Class{Name: "PreferenceRelay", StaticFields: map[string]Field{
			"cached": {Name: "cached", Type: "TriadPreference__c", Static: true},
		}}); err != nil {
			t.Fatal(err)
		}
		methods := map[string]string{
			"create": `return (TriadPreference__c)Schema.TriadPreference__c.SObjectType.newSObject(null,true);`,
			"read":   `if(PreferenceRelay.cached==null){User actor=new User(Id=UserInfo.getUserId(),ProfileId=UserInfo.getProfileId());PreferenceRelay.cached=TriadPreference__c.getOrgDefaults();if(PreferenceRelay.cached.Id==null)PreferenceRelay.cached=PreferenceRelay.create();if(PreferenceRelay.cached.SetupOwnerId!=actor.Id){PreferenceRelay.cached.Id=null;PreferenceRelay.cached.SetupOwnerId=actor.Id;}}return PreferenceRelay.cached;`,
		}
		for name, source := range methods {
			body, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{Name: "PreferenceRelay." + name, ClassName: "PreferenceRelay", IsStatic: true, ReturnType: "TriadPreference__c", Program: body}); err != nil {
				t.Fatal(err)
			}
		}
		setup := ""
		if configured {
			setup = `insert new TriadPreference__c(SetupOwnerId=UserInfo.getOrganizationId(),Tone__c='bright');`
		}
		program, err := CompileAnonymous(setup + `
TriadPreference__c expected=PreferenceRelay.create();
expected.SetupOwnerId=UserInfo.getUserId();
TriadPreference__c actual=PreferenceRelay.read();
System.assertEquals(expected.SetupOwnerId,actual.SetupOwnerId);
System.assertEquals(false,actual.Armed__c);
System.assertEquals(null,actual.Id);
`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := machine.Execute(program); err != nil {
			t.Fatalf("configured=%v: %v", configured, err)
		}
		left, right := machine.Globals["expected"], machine.Globals["actual"]
		if left.Equal(right) && sObjectHashCode(left, map[uint64]bool{}) != sObjectHashCode(right, map[uint64]bool{}) {
			t.Fatal("equal relayed values have different hashes")
		}
		if configured {
			stored, _, err := machine.callCustomDataStaticMember("TriadPreference__c", "getOrgDefaults", nil)
			if err != nil || stored.Fields["Id"].Kind == ValueNull || stored.Fields["Tone__c"].Text != "bright" {
				t.Fatal("relay mutation escaped into persisted defaults")
			}
		}
	}
}
