package apextest

import "testing"

func TestRTEMDynamicPutNativeConformance(t *testing.T) {
	runStandardEventNativeRows(t, "rtem_dynamic_put")
}
