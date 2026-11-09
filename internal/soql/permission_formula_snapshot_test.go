package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestPermissionFormulaQueryExecutionSnapshot(t *testing.T) {
	const user storage.ID = "005000000000001"
	org := storage.NewOrgState()
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{user: {ID: user, Fields: map[string]storage.Value{}}}}
	org.Objects["Probe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{"Granted__c": {APIName: "Granted__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Permission.Bypass"}}}, Records: map[storage.ID]storage.Record{"a01000000000001": {ID: "a01000000000001", Object: "Probe__c", Fields: map[string]storage.Value{}}}}
	query, err := Parse("SELECT Granted__c FROM Probe__c")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"Permissions", "PermissionSets"} {
		result, err := ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{UserID: user, PermissionUserFields: map[string]storage.Value{field: storage.StringValue("Bypass")}})
		if err != nil || len(result.Records) != 1 {
			t.Fatalf("field=%s result=%+v err=%v", field, result, err)
		}
		if got := result.Records[0].Fields["Granted__c"]; got.Kind != storage.ValueBoolean || !got.Boolean {
			t.Errorf("field=%s result=%+v", field, got)
		}
	}
}
