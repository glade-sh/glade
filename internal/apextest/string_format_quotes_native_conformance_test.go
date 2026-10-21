package apextest

import "testing"

func TestStringFormatQuotesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "string_format_quotes", "StringFormatQuotesProbeTest", "62.0", "67.0")
}
