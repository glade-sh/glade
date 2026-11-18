package vm

import (
	"reflect"
	"testing"
)

// The scalar-only shortcut in collectAdditionalStaticFieldValueRefsInFieldWithChildHints
// walks only the collection itself. For a List or Set of scalars that must leave
// the same index and count the same visits as the full walk, whether or not the
// collection is already indexed in the field.
func TestStaticScalarCollectionWalkMatchesFullWalk(t *testing.T) {
	location := staticFieldRef{ClassName: "Helper", FieldName: "log"}
	other := staticFieldRef{ClassName: "Helper", FieldName: "other"}
	children := []Value{Int(1), String("a"), Null, Decimal(2.5), Bool(true), Int(1)}
	for _, value := range []Value{List(children...), Set(children...)} {
		for _, known := range []staticFieldRef{{}, location, other} {
			fresh := func() staticRefIdx {
				index := staticRefIdx{refs: map[uint64]bool{}, fields: map[uint64]staticFieldRefSet{}}
				if known != (staticFieldRef{}) {
					index.add(value.Ref, known)
				}
				return index
			}
			full, short := fresh(), fresh()
			var fullVisits, shortVisits uint64
			collectAdditionalStaticFieldValueRefsWithVisits(value, &full, location, map[uint64]bool{}, true, &fullVisits, nil)
			scalars, ok := scalarOnlyCollectionChildren(value)
			if !ok || scalars != len(value.List)+len(value.Set) {
				t.Fatalf("%s: scalarOnlyCollectionChildren = %d %v", value.Kind, scalars, ok)
			}
			root := value
			root.List, root.Set = nil, nil
			shortVisits = uint64(scalars)
			collectAdditionalStaticFieldValueRefsWithVisits(root, &short, location, map[uint64]bool{}, true, &shortVisits, nil)
			if !reflect.DeepEqual(full, short) || fullVisits != shortVisits {
				t.Fatalf("%s known=%v: full %#v (%d visits), shortcut %#v (%d visits)", value.Kind, known, full, fullVisits, short, shortVisits)
			}
		}
	}
	for _, child := range []Value{List(Int(1)), Set(Int(1)), Map(), Object("Account"), {Kind: ValueInt, Int: 1, Ref: 9}} {
		if _, ok := scalarOnlyCollectionChildren(List(Int(1), child)); ok {
			t.Fatalf("child %s with ref %d counted as scalar", child.Kind, child.Ref)
		}
	}
	if _, ok := scalarOnlyCollectionChildren(Map()); ok {
		t.Fatal("a Map is not a scalar-only List or Set")
	}
}
