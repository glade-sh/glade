package apextest

import "testing"

func TestTransientLocalNativeConformance(t *testing.T) {
	data := loadValueCaptureNativeFixtureFile(t, "testdata/conformance/transient_local_native.json")
	for _, cell := range data.Cells {
		if len(cell.Regex) != 0 || len(cell.Map) != 4 || cell.Map["transientMapValue"] != float64(1) || cell.Map["transientInteger"] != float64(3) || cell.Map["ordinaryValue"] != float64(4) || cell.Map["transientFieldValue"] != float64(2) {
			t.Fatalf("unexpected native local-declaration snapshot: %s/%s: %#v", cell.API, cell.Context, cell.Map)
		}
	}
	// Both capture routes invoke the same helper method; its local declaration
	// is not moved into the anonymous caller for replay.
	runValueCaptureNativeFixture(t, data, "map", "observed", "TRANSIENT_LOCAL|")
}
