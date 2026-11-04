package soql

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase332ExtendedQueryFingerprints(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Alpha")}, System: storage.SystemFields{CreatedDate: "2026-05-02T13:00:00Z"}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Beta")}, System: storage.SystemFields{CreatedDate: "2026-04-28T13:00:00Z"}},
		},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"LastName": {APIName: "LastName", Type: storage.FieldString}, "AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}, Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account", ChildRelationship: "Contacts"}}},
		Records:    map[storage.ID]storage.Record{"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Child"), "AccountId": storage.IDValue("001000000000001")}}},
	}
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, query string
		rows        int
	}{
		{"child", "SELECT Id, Name, (SELECT Id, LastName FROM Contacts ORDER BY LastName) FROM Account ORDER BY Name", 2},
		{"date", "SELECT Id, Name FROM Account WHERE CreatedDate = LAST_N_DAYS:2 ORDER BY Name", 1},
	} {
		result, err := ParseAndExecuteAt(org, tc.query, now)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if result.Rows != tc.rows {
			t.Fatalf("%s: rows %d want %d", tc.name, result.Rows, tc.rows)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("SOQL_FINGERPRINT %s result=%s", tc.name, data)
	}
}
