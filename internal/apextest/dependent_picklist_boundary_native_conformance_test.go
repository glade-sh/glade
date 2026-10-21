package apextest

import (
	"reflect"
	"testing"
)

func TestDependentPicklistBoundaryNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "dependent_picklist_boundary", "ChoiceBoundaryProbeTest", func(t *testing.T, got, want any) {
		// Retain native messages while asserting the DML outcome and status.
		for _, observation := range []any{got, want} {
			for _, value := range observation.(map[string]any) {
				if row, ok := value.(map[string]any); ok {
					delete(row, "message")
				}
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("native=%v; Glade=%v", want, got)
		}
	}, "62.0", "67.0")
}
