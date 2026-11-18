package storage

import (
	"reflect"
	"testing"
)

// The definition read without an install is the one the completed spread
// holds, for owed and installed objects alike, and reading it installs
// nothing. Objects the shape removes and orgs without a source answer false.
func TestStandardSpreadObjectDefinitionMatchesCompletedSpread(t *testing.T) {
	for name, features := range map[string][]string{
		"unshaped": nil,
		"shaped":   {"MultiCurrency", "Quotes"},
	} {
		t.Run(name, func(t *testing.T) {
			lazy := sourceSpreadLazyOrg(features)
			complete := lazy.CloneRuntimeFrozenDefinition()
			MaterializeStandardObjectSpread(&complete)
			before := len(lazy.Objects)
			read := 0
			for i, objectName := range KnownStandardObjectNames() {
				if i%5 != 0 {
					continue
				}
				definition, ok := StandardSpreadObjectDefinition(&lazy, objectName)
				if canonical, known := ResolveKnownStandardObjectName(objectName); known {
					objectName = canonical
				}
				object, held := complete.Objects[objectName]
				if !held {
					if ok {
						t.Fatalf("%s: definition for an object the completed spread lacks", objectName)
					}
					continue
				}
				if !ok {
					// No shared template: the spread builds the object in place.
					continue
				}
				read++
				definition.KeyPrefix = object.Definition.KeyPrefix
				if _, installed := lazy.Objects[objectName]; installed {
					if !reflect.DeepEqual(definition.Relations, object.Definition.Relations) {
						t.Fatalf("%s: installed object relations differ from the template", objectName)
					}
					continue
				}
				if !reflect.DeepEqual(definition, object.Definition) {
					t.Fatalf("%s: definition differs from the completed spread", objectName)
				}
			}
			if read < 100 {
				t.Fatalf("only %d definitions read", read)
			}
			if after := len(lazy.Objects); after != before {
				t.Fatalf("reading definitions installed objects: %d before, %d after", before, after)
			}
		})
	}
	eager := sourceSpreadEagerOrg(nil)
	if _, ok := StandardSpreadObjectDefinition(&eager, "Account"); ok {
		t.Fatal("definition read from an org without a spread source")
	}
	if _, ok := StandardSpreadObjectDefinition(nil, "Account"); ok {
		t.Fatal("definition read from a nil org")
	}
	lazy := sourceSpreadLazyOrg(nil)
	if _, ok := StandardSpreadObjectDefinition(&lazy, "NoSuchObject"); ok {
		t.Fatal("definition read for an unknown object")
	}
}
