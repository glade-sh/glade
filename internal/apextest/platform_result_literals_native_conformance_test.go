package apextest

import "testing"

func TestPlatformResultLiteralsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "platform_result_literals", "ResultLiteralProbeTest", "62.0", "67.0")
}
