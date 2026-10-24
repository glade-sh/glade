package apextest

import "testing"

func TestMetadataLanguageDescribeNativeConformance(t *testing.T) {
	runNativeProjectRoutes(t, "metadata_language_describe", "LocaleRuleProbeTest", []string{"62.0", "67.0"}, 6)
}
