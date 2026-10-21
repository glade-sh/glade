package apextest

import "testing"

func TestURLFieldLimitNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "url_field_limit", "UrlFieldLimitTest", []string{"62.0", "67.0"}, 4)
}
