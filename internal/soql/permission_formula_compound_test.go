package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestPermissionFormulaQueryUsesOnePrincipal(t *testing.T) {
	const user storage.ID = "005000000000001"
	const profile storage.ID = "00e000000000001"
	org := storage.NewOrgState()
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{user: {ID: user, Fields: map[string]storage.Value{"Permissions": storage.StringValue("Bypass"), "ProfileId": storage.IDValue(profile)}}}}
	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{profile: {ID: profile, Fields: map[string]storage.Value{"Name": storage.StringValue("Standard User")}}}}
	org.Objects["Probe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{"Info__c": {APIName: "Info__c", Type: storage.FieldCalculated, DisplayType: "TEXT", Formula: `IF($Permission.Bypass, $User.Id & ":" & $Profile.Name, "none")`}}}, Records: map[storage.ID]storage.Record{"a01000000000001": {ID: "a01000000000001", Object: "Probe__c", Fields: map[string]storage.Value{}}}}
	query, err := Parse("SELECT Info__c FROM Probe__c")
	if err != nil {
		t.Fatal(err)
	}
	result, err := ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{UserID: user})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("rows=%d", len(result.Records))
	}
	if got := result.Records[0].Fields["Info__c"].String; got != string(user)+":Standard User" {
		t.Fatalf("mixed principal: %q", got)
	}
}
