package vm

import (
	"strings"
	"testing"
)

func TestSOQLSetOptionsKeepsBaseOutsideCapturedShapes(t *testing.T) {
	build := func(fields map[string]Value) Value {
		options := Object(queryOptionsType)
		for _, name := range queryOptionsFields {
			options.Fields[name] = Null
		}
		for name, value := range fields {
			options.Fields[name] = value
		}
		return options
	}
	cases := []struct {
		name    string
		query   string
		options Value
		ok      bool
	}{
		{"dataspace default", "SELECT Id FROM Account LIMIT 1 SET OPTIONS :opt", build(map[string]Value{"dataspace": String("default")}), true},
		{"honor true", "SELECT Id FROM Account LIMIT 1 SET OPTIONS :opt", build(map[string]Value{"honorEmptyStrings": Bool(true)}), true},
		{"empty options", "SELECT Id FROM Account LIMIT 1 SET OPTIONS :opt", build(nil), true},
		{"explicit namespace", "SELECT Id FROM Account LIMIT 1 SET OPTIONS :opt", build(map[string]Value{"explicitNamespace": String("ns")}), false},
		{"other dataspace", "SELECT Id FROM Account LIMIT 1 SET OPTIONS :opt", build(map[string]Value{"dataspace": String("other")}), false},
		{"honor false", "SELECT Id FROM Account LIMIT 1 SET OPTIONS :opt", build(map[string]Value{"honorEmptyStrings": Bool(false)}), false},
		{"other object", "SELECT Id FROM Contact LIMIT 1 SET OPTIONS :opt", build(nil), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vm := &VM{}
			got, err := vm.applySOQLSetOptions(tc.query, func(string) (Value, error) { return tc.options, nil })
			if tc.ok {
				if err != nil || strings.Contains(strings.ToUpper(got), "SET OPTIONS") {
					t.Fatalf("got %q, %v; want the clause removed", got, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "SET OPTIONS") {
				t.Fatalf("got %q, %v; want the unsupported error", got, err)
			}
		})
	}
}
