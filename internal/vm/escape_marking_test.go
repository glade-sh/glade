package vm

import "testing"

// A writeback of the collection already stored in a field marks only the
// elements that differ from the stored ones; elements kept at their slot were
// marked when they were inserted.
func TestSetObjectFieldValueWritebackMarksOnlyChangedElements(t *testing.T) {
	machine := New(nil)
	holder := List()
	kept := Object("RecordInput")
	plantedLocal := Object("Planted")
	kept.Fields["nested"] = plantedLocal
	holder.List = append(holder.List, kept)
	owner := Object("TriggerContext")
	owner.Fields["records"] = holder

	// The planted object stands in for a value that escaped through another
	// path; its local-only bit shows whether the kept element was walked.
	machine.rememberLocalOnlyObject(plantedLocal)
	appended := Object("RecordInput")
	appendedNested := Object("Appended")
	appended.Fields["nested"] = appendedNested
	machine.rememberLocalOnlyObject(appended)
	machine.rememberLocalOnlyObject(appendedNested)
	machine.rememberLocalOnlyCollection(holder)

	written := holder
	written.List = append(append([]Value(nil), holder.List...), appended)
	machine.setObjectFieldValue(&owner, "records", written)

	if machine.localOnlyAlias(snapshotAlias(appended)) || machine.localOnlyAlias(snapshotAlias(appendedNested)) {
		t.Fatalf("appended element must escape on writeback")
	}
	if machine.localOnlyAlias(snapshotAlias(holder)) {
		t.Fatalf("the written collection must escape")
	}
	if !machine.localOnlyAlias(snapshotAlias(plantedLocal)) {
		t.Fatalf("kept element was walked again on writeback")
	}
	if got := len(owner.Fields["records"].List); got != 2 {
		t.Fatalf("stored list length = %d, want 2", got)
	}
}

func TestSetObjectFieldValueNewCollectionMarksEveryElement(t *testing.T) {
	machine := New(nil)
	stored := List()
	stored.List = append(stored.List, Object("RecordInput"))
	owner := Object("TriggerContext")
	owner.Fields["records"] = stored

	replacement := List()
	element := Object("RecordInput")
	nested := Object("Nested")
	element.Fields["nested"] = nested
	replacement.List = append(replacement.List, element)
	machine.rememberLocalOnlyCollection(replacement)
	machine.rememberLocalOnlyObject(element)
	machine.rememberLocalOnlyObject(nested)

	machine.setObjectFieldValue(&owner, "records", replacement)

	for _, value := range []Value{replacement, element, nested} {
		if machine.localOnlyAlias(snapshotAlias(value)) {
			t.Fatalf("%s stayed local-only after a new collection was stored", value.Type)
		}
	}
}

func TestSetObjectFieldValueWritebackMarksReplacedSlot(t *testing.T) {
	machine := New(nil)
	holder := List()
	holder.List = append(holder.List, Object("RecordInput"), Object("RecordInput"))
	owner := Object("TriggerContext")
	owner.Fields["records"] = holder

	replacement := Object("RecordInput")
	nested := Object("Nested")
	replacement.Fields["nested"] = nested
	machine.rememberLocalOnlyObject(replacement)
	machine.rememberLocalOnlyObject(nested)

	written := holder
	written.List = append([]Value(nil), holder.List...)
	written.List[0] = replacement
	machine.setObjectFieldValue(&owner, "records", written)

	for _, value := range []Value{replacement, nested} {
		if machine.localOnlyAlias(snapshotAlias(value)) {
			t.Fatalf("%s stayed local-only after replacing a slot", value.Type)
		}
	}
}
