package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// CD-080 BASELINE records Developer scratch defaults; explicit Private and
// Read models in its query matrix must override the same fallback.
func TestStandardObjectSharingDefaults(t *testing.T) {
	org := testDataOrg()
	for _, object := range []string{"Account", "Opportunity", "Case", "Contact"} {
		storage.EnsureStandardObject(&org, object)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, tc := range []struct {
		object, model string
		read          bool
	}{
		{"Account", "", true}, {"Opportunity", "", true}, {"Case", "", true},
		{"Contact", "", false}, // ControlledByParent is not a blanket grant.
		{"Account", "Private", false}, {"Opportunity", "Private", false},
		{"Case", "Private", false}, {"Account", "Read", true},
	} {
		t.Run(tc.object+"/"+tc.model, func(t *testing.T) {
			state := org.Objects[tc.object]
			state.Definition.SharingModel = tc.model
			org.Objects[tc.object] = state
			if got := machine.soqlObjectHasPublicReadSharing(tc.object); got != tc.read {
				t.Fatalf("public read = %v, want %v", got, tc.read)
			}
		})
	}
}
