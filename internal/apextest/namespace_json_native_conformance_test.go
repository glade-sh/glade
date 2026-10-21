package apextest

import "testing"

func TestApexClassNamespaceNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "apex_class_namespace", "NamespaceLookupProbeTest", "62.0", "67.0")
}

func TestJSONDateTokensNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "json_date_tokens", "CalendarTokenProbeTest", "62.0", "67.0")
}
