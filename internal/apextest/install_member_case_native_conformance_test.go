package apextest

import "testing"

func TestInstallMemberCaseNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "install_member_case", "InstallMemberTest", []string{"62.0", "67.0"}, 2)
}
