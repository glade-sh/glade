package apextest

import (
	"reflect"
)

// Require both the native-facing shape comparison and each observed local field.
func c14ExactCarryEqual(got, want, expected map[string]any, fields []string) bool {
	if !runtimeShapeObservationsEqual(got, want, &runtimeShapeCarry{Fields: fields}) {
		return false
	}
	for _, field := range fields {
		actual, present := got[field]
		value, expectedPresent := expected[field]
		if present != expectedPresent || (present && !reflect.DeepEqual(actual, value)) {
			return false
		}
	}
	for field := range expected {
		found := false
		for _, allowed := range fields {
			found = found || field == allowed
		}
		if !found {
			return false
		}
	}
	return true
}

func c14CarrySnapshotMatch(name string, got, want map[string]any, fields []string, records map[string]map[string]any) bool {
	expected, found := records[name]
	return found && c14ExactCarryEqual(got, want, expected, fields)
}
