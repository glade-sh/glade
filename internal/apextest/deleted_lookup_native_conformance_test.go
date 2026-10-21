package apextest

import "testing"

func TestDeletedLookupNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "deleted_lookup", "DeletedLookupObservationTest", []string{"62.0", "67.0"}, 7)
}
