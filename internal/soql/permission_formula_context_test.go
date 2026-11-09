package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestPermissionFormulaQueryPrincipal(t *testing.T) {
	const uid storage.ID = "005000000000001"
	org := storage.NewOrgState()
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{uid: {ID: uid, Fields: map[string]storage.Value{"Permissions": storage.StringValue("Bypass")}}}}
	org.Objects["Probe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{"Granted__c": {APIName: "Granted__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Permission.Bypass"}}}, Records: map[storage.ID]storage.Record{"a01000000000001": {ID: "a01000000000001", Object: "Probe__c", Fields: map[string]storage.Value{}}}}
	for _, text := range []string{"SELECT Granted__c FROM Probe__c", "SELECT Granted__c FROM Probe__c WHERE Granted__c = true"} {
		t.Run(text, func(t *testing.T) {
			query, err := Parse(text)
			if err != nil {
				t.Fatal(err)
			}
			result, err := ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{UserID: uid})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Records) != 1 {
				t.Fatalf("rows=%d", len(result.Records))
			}
			got := result.Records[0].Fields["Granted__c"]
			if got.Kind != storage.ValueBoolean || !got.Boolean {
				t.Fatalf("formula=%+v", got)
			}
		})
	}
}
