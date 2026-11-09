package apextest

import "testing"

func TestURLFieldLimitNativeConformance(t *testing.T) {
	// ORG-RUN.txt: sf org assign permset --name LinkRecordAccess, then both APIs were recaptured.
	runNativeObservationProjectAssigned(t, "url_field_limit", "UrlFieldLimitTest", []string{"62.0", "67.0"}, 4, "LinkRecordAccess")
}
