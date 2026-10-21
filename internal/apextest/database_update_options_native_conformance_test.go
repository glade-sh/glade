package apextest

import "testing"

func TestDatabaseUpdateOptionsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "database_update_options", "UpdateOptionsProbeTest", "62.0", "67.0")
}
