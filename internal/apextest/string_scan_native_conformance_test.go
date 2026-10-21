package apextest

import "testing"

func TestStringScanNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "string_scan", "StringScanTest", []string{"62.0", "67.0"}, 3)
}
