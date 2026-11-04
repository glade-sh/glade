package storage

import (
	"reflect"
	"testing"
)

func TestStandardFieldPolicyPicklistPreservesProjectValues(t *testing.T) {
	for _, values := range [][]PicklistValue{nil, {{Value: "ProjectChoice", Label: "Project choice", Active: true}}} {
		def, ok := StandardObjectDefinition("Lead")
		if !ok {
			t.Fatal("missing Lead")
		}
		f := def.Fields["GenderIdentity"]
		f.Type, f.DisplayType = FieldAny, "ANY"
		f.PicklistValuesConfigured = true
		f.PicklistValues = append([]PicklistValue(nil), values...)
		def.Fields["GenderIdentity"] = f
		EnsureStandardObjectFields(&def)
		got := def.Fields["GenderIdentity"]
		if !got.PicklistValuesConfigured || !reflect.DeepEqual(got.PicklistValues, values) {
			t.Errorf("configured values = %#v, want %#v", got.PicklistValues, values)
		}
	}
}
