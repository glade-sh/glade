package apextest

import "testing"

func TestPageParameterOrderNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "page_parameter_order", "PageParameterOrderProbeTest", "62.0", "67.0")
}
