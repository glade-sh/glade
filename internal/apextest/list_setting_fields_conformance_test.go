package apextest

import "testing"

func TestListSettingFieldsOrgConformance(t *testing.T) {
	runNativeBoundaryConformance(t, "list_setting_fields.json", "C125C06", 8)
}
