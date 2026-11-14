package storage

import (
	"reflect"
	"testing"
)

// Each definition field is part of the snapshot. This exercises additions to
// the definition as well as the existing scalar and collection fields.
func TestSpreadFingerprintTracksDefinitionInputs(t *testing.T) {
	definitionType := reflect.TypeOf(ObjectDefinition{})
	for i := 0; i < definitionType.NumField(); i++ {
		field := definitionType.Field(i)
		t.Run(field.Name, func(t *testing.T) {
			original := ObjectDefinition{}
			changed := ObjectDefinition{}
			value := reflect.ValueOf(&changed).Elem().FieldByIndex(field.Index)
			switch value.Kind() {
			case reflect.String:
				value.SetString("changed")
			case reflect.Bool:
				value.SetBool(true)
			case reflect.Map:
				value.Set(reflect.MakeMap(value.Type()))
			case reflect.Slice:
				value.Set(reflect.MakeSlice(value.Type(), 0, 0))
			default:
				t.Fatalf("add a fingerprint input case for %s", value.Kind())
			}
			if spreadDefinitionFingerprint(original) == spreadDefinitionFingerprint(changed) {
				t.Fatal("definition change did not invalidate fingerprint")
			}
		})
	}
}
