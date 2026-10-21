package apextest

import "testing"

func TestOptionalFieldExposureNativeConformance(t *testing.T) {
	runOrgFeatureObservations(t, "org_feature_surfaces", "FeatureSurface", 12, []string{
		"Contact.IsPersonAccount", "Opportunity.SyncedQuoteId",
		"Case.EntitlementId", "Case.BusinessHoursId", "Case.MilestoneStatus",
		"Case.SlaStartDate", "Case.SlaExitDate", "Case.IsStopped",
	})
}

func TestUserCurrencyFieldNativeConformance(t *testing.T) {
	runOrgFeatureObservations(t, "org_feature_settings", "FeatureSettings", 8, []string{"User.DefaultCurrencyIsoCode"})
}
