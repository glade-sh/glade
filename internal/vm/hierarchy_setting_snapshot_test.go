package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHierarchySettingAccessorSnapshots(t *testing.T) {
	for _, method := range []string{"getOrgDefaults", "getInstance", "getValues"} {
		for _, testRoute := range []bool{false, true} {
			route := "anonymous"
			if testRoute {
				route = "istest"
			}
			t.Run(method+"/"+route, func(t *testing.T) {
				owner := "00D000000000mH2"
				if method == "getValues" {
					owner = "005000000000001"
				}
				org := storage.NewOrgState()
				org.Objects["TriadPreference__c"] = storage.ObjectState{
					Definition: storage.ObjectDefinition{
						APIName: "TriadPreference__c", KeyPrefix: "a02",
						Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
						Fields: map[string]storage.Field{
							"SetupOwnerId": {APIName: "SetupOwnerId", Type: storage.FieldString},
							"Armed__c":     {APIName: "Armed__c", Type: storage.FieldBoolean},
							"Tone__c":      {APIName: "Tone__c", Type: storage.FieldString},
						},
					},
					Records: map[storage.ID]storage.Record{
						"a02000000000001": {ID: "a02000000000001", Object: "TriadPreference__c", Fields: map[string]storage.Value{
							"SetupOwnerId": storage.StringValue(owner),
							"Armed__c":     storage.BooleanValue(true), "Tone__c": storage.StringValue("bright"),
						}},
					},
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if testRoute {
					machine.EnableTestContext()
					machine.testContext.CurrentUser = Object("User")
					machine.testContext.CurrentUser.Fields["Id"] = String("005000000000001")
				}
				args := []Value(nil)
				if method == "getValues" {
					args = []Value{String(owner)}
				}
				read := func() Value {
					t.Helper()
					value, handled, err := machine.callCustomDataStaticMember("TriadPreference__c", method, args)
					if err != nil || !handled || value.Kind != ValueObject {
						t.Fatalf("read: value=%v handled=%v err=%v", value, handled, err)
					}
					return value
				}
				first := read()
				firstID, _ := sObjectIDFromValue(first)
				if method == "getOrgDefaults" {
					first.Fields["Id"] = Null
				}
				first.Fields["Armed__c"] = Bool(false)
				second := read()
				secondID, _ := sObjectIDFromValue(second)
				// Native rows distinguish route-specific getValues aliases from
				// fresh org defaults and current-user merged instances.
				switch method {
				case "getOrgDefaults":
					if second.Ref == first.Ref || secondID != firstID {
						t.Fatalf("org default snapshot changed: %v", second)
					}
				case "getInstance":
					if second.Ref == first.Ref || !second.Fields["Armed__c"].Bool {
						t.Fatalf("merged instance retained caller mutation: %v", second)
					}
				case "getValues":
					if testRoute {
						if second.Ref == first.Ref || !second.Fields["Armed__c"].Bool {
							t.Fatalf("test route retained caller mutation: %v", second)
						}
					} else if second.Ref != first.Ref || second.Fields["Armed__c"].Bool {
						t.Fatalf("anonymous route detached cached value: %v", second)
					}
				}
				stored := machine.Org.Objects["TriadPreference__c"].Records["a02000000000001"]
				if stored.Fields["Tone__c"].String != "bright" || !stored.Fields["Armed__c"].Boolean {
					t.Fatalf("read changed storage: %v", stored)
				}
			})
		}
	}
}
