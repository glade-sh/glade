package apextest

import "testing"

func TestContentVersionNumberNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "content_version_number", "FileVersionNumberTest", []string{"62.0", "67.0"}, 2)
}
