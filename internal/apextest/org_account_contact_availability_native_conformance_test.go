package apextest

import "testing"

func TestOrgAccountContactAvailabilityNativeConformance(t *testing.T) {
	runOrgFeatureObservations(t, "org_account_contact_availability", "ObjectAvailabilityProbe", 8, []string{
		"AccountContactRelation.describe", "AccountContactRelation.type", "AccountContactRelation.query",
		"Account.describe", "Account.type", "Account.query",
		"AbsentObject__c.describe", "AbsentObject__c.type", "AbsentObject__c.query",
	})
}
