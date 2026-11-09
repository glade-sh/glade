package apextest

import "testing"

// Native rows: testdata/conformance/permission_scope_mute/org.tsv. Scoped
// permission sets add to the running user's grants and a permission-set
// group's muting set removes a grant for its own group only.
func TestPermissionScopeMuteNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "permission_scope_mute", "ScopeMuteProbeTest", []string{"62.0", "67.0"}, 18)
}
