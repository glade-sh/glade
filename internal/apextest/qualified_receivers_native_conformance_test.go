package apextest

import "testing"

// Both routes call the same receiver methods, then execute both object triggers
// through DML. Each trigger also contains the receiver expressions directly.
func TestQualifiedReceiversNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "qualified_receivers", "QualifiedReceiverProbeTest", "62.0", "67.0")
}
