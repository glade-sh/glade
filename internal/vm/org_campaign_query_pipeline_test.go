package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rejected-query accounting: org_influence_query_limits/org.tsv.
// Test-stub ordering retains the existing local pipeline contract.
func TestCampaignInfluenceQueryPipelinePreservesAccountingAndStubs(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{"CampaignInfluence2Enabled"})
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.currentMethod = Method{APIVersion: "67.0"}
	machine.dynamicSOQLBindRoute = "Database.countQuery"
	if _, err := machine.executeSOQL("SELECT COUNT() FROM CampaignInfluence", &Result{}); err == nil {
		t.Fatal("expected captured count denial")
	}
	if machine.limits.Queries != 0 {
		t.Fatalf("query accounting=%d want=0", machine.limits.Queries)
	}
	machine.ResetLimits()
	machine.dynamicSOQLBindRoute = "Database.query"
	machine.testContext.SoqlStubs["campaigninfluence"] = String("not a provider")
	_, err := machine.executeSOQL("SELECT Id FROM CampaignInfluence", &Result{})
	if err == nil || !strings.Contains(err.Error(), "provider for CampaignInfluence is not an object") {
		t.Fatalf("stub dispatch error=%v", err)
	}
}
