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

func TestSpreadFingerprintTracksFieldInputs(t *testing.T) {
	fieldType := reflect.TypeOf(Field{})
	for i := 0; i < fieldType.NumField(); i++ {
		field := fieldType.Field(i)
		t.Run(field.Name, func(t *testing.T) {
			original := ObjectDefinition{Fields: map[string]Field{"Example": {}}}
			changedField := Field{}
			setFingerprintTestValue(t, reflect.ValueOf(&changedField).Elem().FieldByIndex(field.Index))
			changed := ObjectDefinition{Fields: map[string]Field{"Example": changedField}}
			if spreadDefinitionFingerprint(original) == spreadDefinitionFingerprint(changed) {
				t.Fatal("field change did not invalidate fingerprint")
			}
		})
	}
}

func setFingerprintTestValue(t *testing.T, value reflect.Value) {
	t.Helper()
	switch value.Kind() {
	case reflect.String:
		value.SetString("changed")
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value.SetUint(1)
	case reflect.Float32, reflect.Float64:
		value.SetFloat(1)
	case reflect.Map:
		value.Set(reflect.MakeMap(value.Type()))
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 0, 0))
	case reflect.Pointer:
		value.Set(reflect.New(value.Type().Elem()))
	case reflect.Struct:
		if value.NumField() == 0 {
			t.Fatal("cannot change empty struct")
		}
		setFingerprintTestValue(t, value.Field(0))
	default:
		t.Fatalf("add a fingerprint input case for %s", value.Kind())
	}
}
