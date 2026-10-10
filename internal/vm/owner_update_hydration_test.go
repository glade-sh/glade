package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestUpdateHydrationPreservesIncomingOwnerForValidation(t *testing.T) {
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	previous := storage.Record{Object: "Account", ID: "001000000000001", System: storage.SystemFields{OwnerID: "005000000000001"}}
	for _, owner := range []storage.ID{"", "00G000000000001", "003000000000001"} {
		input := storage.Record{Object: "Account", ID: previous.ID, System: storage.SystemFields{OwnerID: owner}}
		out := machine.hydrateUpdateTriggerRecords([]storage.Record{input}, []storage.Record{previous})
		want := owner
		if want == "" {
			want = previous.System.OwnerID
		}
		if len(out) != 1 || out[0].System.OwnerID != want {
			t.Fatalf("owner=%s hydrated=%+v", owner, out)
		}
		if previous.System.OwnerID != "005000000000001" || input.System.OwnerID != owner {
			t.Fatal("source record mutated")
		}
	}
}
