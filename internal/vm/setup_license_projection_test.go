package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// This checks local setup fixture completeness and shared relationship storage.
// It deliberately does not assert org-dependent license labels or capacities.
func TestExecSetupLicenseDirectAndRelationshipProjection(t *testing.T) {
	program, err := CompileAnonymous(`
Id profileId = UserInfo.getProfileId();
Profile selected = [SELECT UserLicenseId, UserLicense.Name, UserLicense.Status,
    UserLicense.MasterLabel, UserLicense.LicenseDefinitionKey, UserLicense.TotalLicenses
    FROM Profile WHERE Id = :profileId];
Assert.isNotNull(selected.UserLicense.Status, 'Selected license status');
UserLicense direct = [SELECT Name, Status, MasterLabel, LicenseDefinitionKey, TotalLicenses
    FROM UserLicense WHERE Id = :selected.UserLicenseId];
for (String field : new List<String>{'Name','Status','MasterLabel','LicenseDefinitionKey','TotalLicenses'}) {
    Assert.isNotNull(direct.get(field), 'Local setup fixture field: ' + field);
    Assert.areEqual(direct.get(field), selected.UserLicense.get(field), 'Projection: ' + field);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSetupLicensePreservesSuppliedValues(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	licenses := org.Objects["UserLicense"]
	license := licenses.Records["100000000000001"]
	license.Fields["Status"] = storage.StringValue("Synthetic state")
	license.Fields["MasterLabel"] = storage.StringValue("Synthetic label")
	license.Fields["TotalLicenses"] = storage.IntegerValue(17)
	storage.EnsureDeterministicPlatformData(&org)
	program, err := CompileAnonymous(`
Id profileId = UserInfo.getProfileId();
Profile selected = [SELECT UserLicense.Status, UserLicense.MasterLabel, UserLicense.TotalLicenses
    FROM Profile WHERE Id = :profileId];
Assert.areEqual('Synthetic state', selected.UserLicense.Status);
Assert.areEqual('Synthetic label', selected.UserLicense.MasterLabel);
Assert.areEqual(17, selected.UserLicense.TotalLicenses);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
