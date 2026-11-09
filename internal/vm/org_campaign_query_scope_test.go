package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// The native observation covers a default dynamic count. Other query modes
// retain the prior availability policy until native rows establish them.
func TestCampaignInfluenceAvailabilityGuardScope(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{"CampaignInfluence2Enabled"})
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.currentMethod = Method{APIVersion: "67.0"}
	for _, mode := range []string{"", "USER_MODE", "SYSTEM_MODE"} {
		t.Run(mode, func(t *testing.T) {
			if err := machine.enforceOrgShapeObjectAvailability(soql.Query{Object: "CampaignInfluence", SecurityMode: mode}); err != nil {
				t.Fatalf("unobserved non-count availability guard: %v", err)
			}
		})
	}
}
