package apextest

import "testing"

func TestStateCountryFeatureNativeConformance(t *testing.T) {
	runOrgFeatureObservations(t, "org_feature_surfaces", "FeatureSurface", 12, []string{
		"Account.BillingCountryCode", "Account.BillingStateCode", "Account.ShippingCountryCode",
		"Contact.MailingCountryCode", "Lead.CountryCode", "User.CountryCode", "Order.ShippingCountryCode", "countryPut",
	})
}
