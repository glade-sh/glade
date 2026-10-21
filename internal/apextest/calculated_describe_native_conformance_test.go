package apextest

import "testing"

func TestCalculatedFieldDescribeNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "calculated_field_describe", "CalculatedDescribeProbeTest", "62.0", "67.0")
}
