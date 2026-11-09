package vm

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestOrdinaryRecordConverterEquivalence(t *testing.T) {
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	records := []storage.Record{
		{ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Example")}, ExplicitNulls: map[string]bool{"Description": true}},
		{ID: "001000000000002", Object: "Account", Children: map[string][]storage.Record{"Contacts": {{ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Example")}}}}},
	}
	for _, record := range records {
		want := machine.vmValueFromRecord(record)
		got := machine.recordConverterForOrg(&org)(record)
		gotJSON, gotErr := json.Marshal(got)
		wantJSON, wantErr := json.Marshal(want)
		if gotErr != nil || wantErr != nil || !bytes.Equal(gotJSON, wantJSON) {
			t.Fatalf("ordinary conversion differs for %s: got %s want %s", record.ID, gotJSON, wantJSON)
		}
	}
}
