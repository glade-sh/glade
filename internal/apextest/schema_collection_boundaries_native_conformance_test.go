package apextest

import (
	"reflect"
	"sort"
	"testing"
)

func TestSchemaCollectionBoundariesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "schema_collection_boundaries", "SchemaCollectionProbeTest", func(t *testing.T, got, want any) {
		actual, actualOK := got.(map[string]any)
		expected, expectedOK := want.(map[string]any)
		if !actualOK || !expectedOK || len(actual) != len(expected) {
			t.Fatalf("invalid observation: native=%v; Glade=%v", want, got)
		}
		shapes := make([]string, 0, len(expected))
		for shape := range expected {
			shapes = append(shapes, shape)
		}
		sort.Strings(shapes)
		for _, shape := range shapes {
			t.Run(shape, func(t *testing.T) {
				if !reflect.DeepEqual(actual[shape], expected[shape]) {
					t.Fatalf("native=%v; Glade=%v", expected[shape], actual[shape])
				}
			})
		}
	})
}
