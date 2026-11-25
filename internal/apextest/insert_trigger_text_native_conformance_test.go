package apextest

import "testing"

func TestInsertTriggerTextNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "insert_trigger_text", "ZephyrTextProbeTest", "62.0", "67.0")
}
