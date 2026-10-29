package apextest

import "testing"

// No profile or permission-set metadata is present in the owned project.
// System.runAs requires the named test route.
func TestDefaultCustomAccessNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "default_custom_access", "DefaultAccessProbeTest", []string{"62.0", "67.0"}, 3)
}
