package vm

import "testing"

func TestTypeTokenEqualityAndMapKeysUseSameIdentity(t *testing.T) {
	for _, tc := range []struct{ name, left, right string }{
		{"SObject casing", "Account", "account"},
		{"class casing", "OwnedClass", "OWNEDCLASS"},
		{"existing primitive alias", "System.Integer", "integer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			literal := Value{Kind: ValueObject, Type: "Type", Text: tc.left}
			lookup := platformScalar("Type", tc.right)
			if !literal.Equal(lookup) || !lookup.Equal(literal) {
				t.Fatalf("equal Type tokens differ: %#v / %#v", literal, lookup)
			}
			if mapKey(literal) != mapKey(lookup) {
				t.Fatalf("equal Type tokens have different map keys: %q / %q", mapKey(literal), mapKey(lookup))
			}
			if typeValueText(literal) != tc.left || typeValueText(lookup) != tc.right {
				t.Fatal("identity comparison changed token display spelling")
			}
			other := Value{Kind: ValueObject, Type: "Type", Text: "DistinctType"}
			if literal.Equal(other) || mapKey(literal) == mapKey(other) {
				t.Fatal("distinct Type tokens share identity")
			}
			if literal.Equal(Null) {
				t.Fatal("non-null Type equals null")
			}
		})
	}
}
