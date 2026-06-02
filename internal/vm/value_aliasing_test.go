package vm

import "testing"

func TestMapKeyTypeCannotContainCollectionAliasKind(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		mapType      string
		previousKind ValueKind
		previousType string
		want         bool
	}{
		{
			name:         "type key cannot contain list alias",
			mapType:      "Map<Type,List<BaseDomain>>",
			previousKind: ValueList,
			want:         true,
		},
		{
			name:         "list key can contain list alias",
			mapType:      "Map<List<String>,String>",
			previousKind: ValueList,
			want:         false,
		},
		{
			name:         "custom key stays conservative for list alias",
			mapType:      "Map<Key,List<String>>",
			previousKind: ValueList,
			want:         false,
		},
		{
			name:         "non-type object alias skips type keys",
			mapType:      "Map<Type,List<BaseDomain>>",
			previousKind: ValueObject,
			previousType: "Account",
			want:         true,
		},
		{
			name:         "type object alias still scans type keys",
			mapType:      "Map<Type,List<BaseDomain>>",
			previousKind: ValueObject,
			previousType: "Type",
			want:         false,
		},
		{
			name:         "custom key stays conservative for object alias",
			mapType:      "Map<Key,List<String>>",
			previousKind: ValueObject,
			previousType: "Account",
			want:         false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			previous := aliasSnapshot{kind: tc.previousKind, typeName: tc.previousType}
			if got := mapKeyTypeCannotContainAlias(tc.mapType, previous); got != tc.want {
				t.Fatalf("mapKeyTypeCannotContainAlias(%q, %#v) = %v, want %v", tc.mapType, previous, got, tc.want)
			}
		})
	}
}
