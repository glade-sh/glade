package apextest

import "testing"

// No profile or permission-set metadata is present in the owned project.
// System.runAs requires the named test route.
func TestDefaultCustomAccessNativeConformance(t *testing.T) {
	strip := nativeObservationCarry{
		LocalJSON: `{"enforced":"System.NoAccessException","UPDATABLE":"true:true","CREATABLE":"false:true","READABLE":"true:true"}`,
		Reason:    "CREATABLE stripping retains Name despite absent object access; permission behavior awaits a decision",
	}
	carries := map[string]nativeObservationCarry{
		"62.0/stripAccess": strip,
		"67.0/stripAccess": strip,
		"67.0/queryAndDml": {
			LocalJSON: `{"systemInsert":"ok","userInsert":"System.SecurityException","plainInsert":"System.SecurityException","enforcedQuery":"System.QueryException","userQuery":"System.QueryException","systemQuery":"1","plainQuery":"System.QueryException"}`,
			Reason:    "plain COUNT access denial uses QueryException instead of native SecurityException",
		},
	}
	runNativeObservationProjectWithCarries(t, "default_custom_access", "DefaultAccessProbeTest", []string{"62.0", "67.0"}, 3, carries)
}
