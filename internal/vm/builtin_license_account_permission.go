package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Native actor captures deny Account and Opportunity access for the profile
// linked to this built-in license. Other object/license shapes retain their
// existing fallbacks; explicit permission grants are evaluated first.
func (vm *VM) profileHasCapturedAccountLicenseRestriction(profileID, objectName string) bool {
	if vm == nil || vm.Org == nil || profileID == "" ||
		!strings.EqualFold(objectName, "Account") && !strings.EqualFold(objectName, "Opportunity") {
		return false
	}
	profile, ok := vm.findOrgRecord("Profile", storage.ID(profileID))
	if !ok {
		return false
	}
	licenseID := storageValueIDText(profile.Fields["UserLicenseId"])
	if licenseID == "" {
		return false
	}
	license, ok := vm.findOrgRecord("UserLicense", storage.ID(licenseID))
	return ok && storageStringValueEquals(license.Fields["Name"], "Chatter Free") &&
		storageStringValueEquals(license.Fields["LicenseDefinitionKey"], "CSN_User")
}
