package apextest

import "testing"

// Replay the six owned native utility snapshots with their exact source versions.
func TestUtilityNativeConformance(t *testing.T) {
	runNativeSnapshotConformance(t, "utility_native.json", "UtilityNative")
}
