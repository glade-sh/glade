package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Internal relationship routing for the native accepted and rejected parents.
func TestRoad174RecordAccessRelationshipRouting(t *testing.T) {
	org := storage.NewOrgState()
	for _, row := range []struct {
		name string
		want bool
	}{{"Lead", true}, {"Campaign", true}, {"Asset", true}, {"Task", false}, {"Event", false}, {"User", false}} {
		storage.EnsureStandardObject(&org, row.name)
		machine := &VM{Org: &org}
		definition := org.Objects[row.name].Definition
		if got := machine.isParentRelationshipField(definition, "UserRecordAccess"); got != row.want {
			t.Errorf("%s relationship field: got %t want %t", row.name, got, row.want)
		}
		target, got := machine.parentRelationshipObjectType(row.name, "UserRecordAccess")
		if got != row.want || (got && target != "UserRecordAccess") {
			t.Errorf("%s relationship type: got %q/%t want %t", row.name, target, got, row.want)
		}
	}
}
