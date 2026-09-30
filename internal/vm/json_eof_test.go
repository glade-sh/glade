package vm

import "testing"

func TestJSONUntypedEOFAfterCompleteValueUsesCloseMarker(t *testing.T) {
	for _, tc := range []struct {
		name      string
		input     string
		container string
		column    int
	}{
		{name: "array number", input: "[12", container: "ARRAY", column: 10},
		{name: "object boolean", input: `{"a":true`, container: "OBJECT", column: 28},
		{name: "array string", input: `["x"`, container: "ARRAY", column: 9},
		{name: "nested array string", input: `{"a":["x"`, container: "ARRAY", column: 19},
		{name: "nested object string", input: `[{"a":"x"`, container: "OBJECT", column: 19},
		{name: "nested object after number", input: `{"a":[{"b":1`, container: "OBJECT", column: 37},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeJSONUntypedValue(tc.input)
			if err == nil {
				t.Fatal("decodeJSONUntypedValue returned nil error")
			}
			got, ok := err.(*jsonContainerInputError)
			if !ok {
				t.Fatalf("error type = %T, want *jsonContainerInputError", err)
			}
			if got.withinEntries {
				t.Fatalf("withinEntries = true, want close-marker error: %v", got)
			}
			if got.container != tc.container || got.line != 1 || got.column != tc.column {
				t.Fatalf("error = %#v, want container %q at [line:1, column:%d]", got, tc.container, tc.column)
			}
		})
	}
}

func TestJSONUntypedEOFWithoutCompleteValueStaysWithinEntries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		input     string
		container string
		column    int
	}{
		{name: "object missing value", input: `{"a":`, container: "OBJECT", column: 11},
		{name: "object whitespace missing value", input: `{"a": `, container: "OBJECT", column: 13},
		{name: "array after comma", input: `{"a":[1,`, container: "ARRAY", column: 17},
		{name: "object after comma", input: `{"a":{"b":1,`, container: "OBJECT", column: 25},
	} {
		_, err := decodeJSONUntypedValue(tc.input)
		if err == nil {
			t.Fatalf("%s: decodeJSONUntypedValue returned nil error", tc.name)
		}
		got, ok := err.(*jsonContainerInputError)
		if !ok {
			t.Fatalf("%s: error type = %T, want *jsonContainerInputError", tc.name, err)
		}
		if !got.withinEntries || got.container != tc.container || got.line != 1 || got.column != tc.column {
			t.Fatalf("%s: error = %#v, want within %s at [line:1, column:%d]", tc.name, got, tc.container, tc.column)
		}
	}
}
