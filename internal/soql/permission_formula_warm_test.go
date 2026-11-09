package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// A cached query must not carry the prior user's permission result.
func TestPermissionFormulaQueryWarmUserIsolation(t *testing.T) {
	const user storage.ID = "005000000000001"
	const other storage.ID = "005000000000002"
	org := storage.NewOrgState()
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{user: {ID: user, Fields: map[string]storage.Value{"Permissions": storage.StringValue("Bypass")}}, other: {ID: other, Fields: map[string]storage.Value{}}}}
	org.Objects["Probe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{"Granted__c": {APIName: "Granted__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Permission.Bypass"}}}, Records: map[storage.ID]storage.Record{"a01000000000001": {ID: "a01000000000001", Object: "Probe__c", Fields: map[string]storage.Value{}}}}
	query, err := Parse("SELECT Granted__c FROM Probe__c")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	for _, tc := range []struct {
		id   storage.ID
		want bool
	}{{user, true}, {other, false}, {user, true}} {
		result, err := ExecuteWithCacheAndContext(org, query, cache, ExecutionContext{UserID: tc.id})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Records) != 1 {
			t.Fatalf("rows=%d", len(result.Records))
		}
		got := result.Records[0].Fields["Granted__c"]
		if got.Kind != storage.ValueBoolean || got.Boolean != tc.want {
			t.Errorf("user=%s got=%+v want=%v", tc.id, got, tc.want)
		}
	}
}
