package apextest

import "testing"

func TestOrganizationFeatureFlagsNativeConformance(t *testing.T) {
	runOrgFeatureObservations(t, "org_feature_surfaces", "FeatureSurface", 12, []string{
		"Organization.IsMultiCurrencyEnabled", "Organization.IsChatterEnabled",
		"Organization.IsLightningExperienceEnabled", "Organization.IsSetPasswordInApiEnabled",
		"Organization.IsHealthCloudEnabled", "Organization.HasAnalyticsAdminPerms",
	})
}
