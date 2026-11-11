package storage

import "testing"

// A stub String field that joins the enforced restricted set
// (nativeRestrictedPicklistFields) must carry a value list, or DML would
// reject every non-empty value.
func TestStandardStubRestrictedEnforcedFieldsCarryValues(t *testing.T) {
	for object, fields := range nativeRestrictedPicklistFields {
		stubs, ok := standardSObjectStubFieldsFor(object)
		if !ok {
			continue
		}
		definition, ok := StandardObjectDefinition(object)
		if !ok {
			continue
		}
		for name := range fields {
			var stubName string
			for candidate, stub := range stubs {
				if equalFold(candidate, name) && stub.Type == FieldString {
					stubName = candidate
				}
			}
			if stubName == "" {
				continue
			}
			resolved, exists := ResolveFieldName(definition, "", stubName)
			if !exists {
				continue
			}
			field := definition.Fields[resolved]
			t.Logf("%s.%s stub String -> %v restricted=%v values=%d", object, stubName, field.Type, field.RestrictedPicklist, len(field.PicklistValues))
			if field.RestrictedPicklist && len(field.PicklistValues) == 0 {
				t.Errorf("%s.%s is enforced and restricted with no values", object, stubName)
			}
		}
	}
}

func equalFold(a, b string) bool {
	return len(a) == len(b) && stringsEqualFold(a, b)
}
