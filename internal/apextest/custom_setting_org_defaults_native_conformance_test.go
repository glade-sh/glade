package apextest

import "testing"

// Hierarchy custom setting org defaults: getOrgDefaults() without a stored row
// has no Id, update fails with MISSING_ARGUMENT, and an inserted or upserted
// org default is owned by the organization Id, readable by a SetupOwnerId
// query and comparable with UserInfo.getOrganizationId().
func TestCustomSettingOrgDefaultsNativeConformance(t *testing.T) {
	runNativeShapeMatrix(t, "custom_setting_org_defaults", "SettingProbe", nil)
}
