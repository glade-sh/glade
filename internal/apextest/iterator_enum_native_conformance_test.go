package apextest

import "testing"

// Replay the exact owned sources and OBS transports against all six native cells.
func TestIteratorEnumNativeConformance(t *testing.T) {
	runNativeSnapshotConformance(t, "iterator_enum_native.json", "IteratorEnumNative")
}
