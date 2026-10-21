package apextest

import "testing"

func TestSettingOwnerQueryNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "setting_owner_queries", "SettingOwnerQueryTest", []string{"62.0", "67.0"}, 6)
}
