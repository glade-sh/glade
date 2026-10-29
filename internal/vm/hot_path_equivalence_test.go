package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The ASCII fast paths must agree with the Unicode folding they replace for
// every name shape the runtime sees, including non-ASCII input that keeps the
// original comparison.
func TestBuiltinExceptionTypeFastPathMatchesFolding(t *testing.T) {
	reference := func(typeName string) bool {
		typeName = exceptionTypeName(typeName)
		for known := range builtinExceptionParents {
			if strings.EqualFold(typeName, known) {
				return true
			}
		}
		return false
	}
	names := []string{"", "Exception", "exception", "DMLEXCEPTION", "DmlException", "System.DmlException", "system.dmlexception",
		"IllegalStateException", "PatternSyntaxException", "QueryException ", "NullPointerException", "MyException",
		"Custom.DmlException", "Dmlİxception", "DmlExceptıon", "KelvinException", "ListException", "LISTEXCEPTION"}
	for known := range builtinExceptionParents {
		names = append(names, known, strings.ToUpper(known), strings.ToLower(known), "System."+known)
	}
	for _, name := range names {
		if got, want := isBuiltinExceptionType(name), reference(name); got != want {
			t.Fatalf("isBuiltinExceptionType(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestFieldIsReferenceFastPathMatchesLowering(t *testing.T) {
	reference := func(field storage.Field) bool {
		if field.Type == storage.FieldReference || len(field.ReferenceTo) > 0 {
			return true
		}
		switch strings.ToLower(strings.TrimSpace(string(field.Type))) {
		case "lookup", "masterdetail", "metadatarelationship":
			return true
		default:
			return false
		}
	}
	types := []string{"", "Lookup", "LOOKUP", " lookup ", "MasterDetail", "masterdetail", "MetadataRelationship", "METADATARELATIONSHIP",
		"Text", "Reference", "Lookups", "Look", "Picklist", "İookup", "lookup ", string(storage.FieldReference), "lookUp\t"}
	for _, typeName := range types {
		field := storage.Field{Type: storage.FieldType(typeName)}
		if got, want := vmFieldIsReference(field), reference(field); got != want {
			t.Fatalf("vmFieldIsReference(%q) = %v, want %v", typeName, got, want)
		}
		field.ReferenceTo = []string{"Account"}
		if !vmFieldIsReference(field) {
			t.Fatalf("vmFieldIsReference(%q with ReferenceTo) = false", typeName)
		}
	}
}

// Forgetting map-put path hints scans every retained hint. The presence flag
// lets a VM without such hints skip the scan and must track every insertion
// and removal exactly.
func TestStaticMapPutPathHintPresenceTracksHints(t *testing.T) {
	machine := New(nil)
	first := staticFieldRef{ClassName: "Registry", FieldName: "First"}
	second := staticFieldRef{ClassName: "Registry", FieldName: "Second"}
	if machine.staticMapPutPathHints {
		t.Fatal("fresh VM reports map-put path hints")
	}
	machine.forgetStaticMapPutPathsInField(first)
	machine.rememberStaticAliasChildHintEntry(staticAliasChildHintKey{Ref: 1, Kind: ValueObject, Location: first}, staticAliasChildHint{Kind: staticAliasChildHintObjectField})
	if machine.staticMapPutPathHints {
		t.Fatal("an ordinary child hint set the map-put presence flag")
	}
	machine.rememberStaticAliasChildHintEntry(staticAliasChildHintKey{Ref: 2, Kind: ValueObject, Location: first}, staticAliasChildHint{Kind: staticAliasChildHintObjectMapField})
	machine.rememberStaticAliasChildHintEntry(staticAliasChildHintKey{Ref: 3, Kind: ValueMap, Location: second}, staticAliasChildHint{Kind: staticAliasChildHintMapValueMapField})
	if !machine.staticMapPutPathHints {
		t.Fatal("map-put path hints did not set the presence flag")
	}
	machine.forgetStaticMapPutPathsInField(first)
	if _, ok := machine.staticAliasChildHints[staticAliasChildHintKey{Ref: 2, Kind: ValueObject, Location: first}]; ok {
		t.Fatal("map-put hint in the forgotten field remained")
	}
	if _, ok := machine.staticAliasChildHints[staticAliasChildHintKey{Ref: 1, Kind: ValueObject, Location: first}]; !ok {
		t.Fatal("ordinary child hint was removed with the map-put hints")
	}
	if !machine.staticMapPutPathHints {
		t.Fatal("presence flag cleared while another field still holds a map-put hint")
	}
	machine.forgetStaticMapPutPathsInField(second)
	if machine.staticMapPutPathHints {
		t.Fatal("presence flag remained after the last map-put hint was forgotten")
	}
	for key, hint := range machine.staticAliasChildHints {
		if staticAliasMapPutPathKind(hint.Kind) {
			t.Fatalf("map-put hint %v remained", key)
		}
	}
	machine.invalidateStaticValueRefs()
	if machine.staticMapPutPathHints || machine.staticAliasChildHints != nil {
		t.Fatal("static index invalidation left hint state behind")
	}
}

func TestStaticMapPutPathHintPresenceFollowsFixtureHints(t *testing.T) {
	machine, _ := mapPutStaticIndexFixture(t, 8, false, false)
	for i := range 4 {
		mapPutStaticIndexPut(t, machine, String("entry_"+string(rune('a'+i))), mapPutStaticIndexNode("item"))
	}
	present := false
	for _, hint := range machine.staticAliasChildHints {
		if staticAliasMapPutPathKind(hint.Kind) {
			present = true
		}
	}
	if present && !machine.staticMapPutPathHints {
		t.Fatal("fixture created map-put hints without setting the presence flag")
	}
	for key := range machine.staticAliasChildHints {
		machine.forgetStaticMapPutPathsInField(key.Location)
	}
	for key, hint := range machine.staticAliasChildHints {
		if staticAliasMapPutPathKind(hint.Kind) {
			t.Fatalf("map-put hint %v survived forgetting its field", key)
		}
	}
	if machine.staticMapPutPathHints {
		t.Fatal("presence flag remained after every field was forgotten")
	}
}
