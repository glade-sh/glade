package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestUpdatePreservesValidatedOwnerChange(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Owned")}}})
	if !inserted[0].Success {
		t.Fatal(inserted)
	}
	id := inserted[0].ID
	owner := storage.ID("005000000000099")
	changed := engine.Update([]storage.Record{{Object: "Account", ID: id, Fields: map[string]storage.Value{"OwnerId": storage.IDValue(owner)}}})
	if !changed[0].Success {
		t.Fatal(changed)
	}
	if got := org.Objects["Account"].Records[id].System.OwnerID; got != owner {
		t.Fatalf("owner=%s want %s", got, owner)
	}
	unchanged := engine.Update([]storage.Record{{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Changed")}}})
	if !unchanged[0].Success || org.Objects["Account"].Records[id].System.OwnerID != owner {
		t.Fatal("unspecified owner changed", unchanged)
	}
	rejected := engine.Update([]storage.Record{{Object: "Account", ID: id, Fields: map[string]storage.Value{"OwnerId": storage.IDValue("003000000000001")}}})
	if rejected[0].Success || rejected[0].StatusCode != "FIELD_INTEGRITY_EXCEPTION" || org.Objects["Account"].Records[id].System.OwnerID != owner {
		t.Fatal("invalid owner update was not atomic", rejected)
	}
}
