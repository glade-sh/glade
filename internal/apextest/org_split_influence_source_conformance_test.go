package apextest

import "testing"

// OpportunitySplit and OpportunitySplitType exist natively only after the
// Opportunity Splits Setup wizard finishes, and no deployable setting does
// that. CampaignInfluence stays absent in every captured phase, including
// after campaignSettings.enableCampaignInfluence2 and the Setup pages
// showing it enabled. CampaignInfluence.type is not held: natively
// Type.forName flips true after the setting, which Glade does not model.
func TestOrgSplitInfluenceSourceNativeConformance(t *testing.T) {
	runOrgFeatureObservations(t, "org_split_influence_source", "", 12, []string{
		"OpportunitySplit.describe", "OpportunitySplit.type", "OpportunitySplit.query",
		"OpportunitySplitType.describe", "OpportunitySplitType.type", "OpportunitySplitType.query",
		"CampaignInfluence.describe", "CampaignInfluence.query",
		"CampaignInfluenceModel.describe", "CampaignInfluenceModel.type", "CampaignInfluenceModel.query",
		"Account.describe", "Account.type", "Account.query",
		"AbsentObject__c.describe", "AbsentObject__c.type", "AbsentObject__c.query",
	})
}
