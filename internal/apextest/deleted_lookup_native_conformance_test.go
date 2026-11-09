package apextest

import "testing"

func TestDeletedLookupNativeConformance(t *testing.T) {
	// ORG-RUN.txt: sf org assign permset --name OwnedLookupAccess before both captures.
	runNativeObservationProjectAssigned(t, "deleted_lookup", "DeletedLookupObservationTest", []string{"62.0", "67.0"}, 7, "OwnedLookupAccess")
}
