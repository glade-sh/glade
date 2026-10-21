package apextest

import "testing"

func TestNameFieldQueryNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "name_field_query", "SchemaNameObservationTest", []string{"62.0", "67.0"}, 3)
}
