package vm

import (
	"reflect"
	"testing"
)

func TestValueSameHeaderAndBacking(t *testing.T) {
	base := Object("Account")
	base.Ref = 41
	base.Fields["Name"] = String("before")
	base.loadedReferenceSnapshot = &sobjectLoadedReferenceSnapshot{}

	if !valueSameHeaderAndBacking(base, base) {
		t.Fatal("identical values should have the same header and backing")
	}

	changed := base
	changed.Fields = map[string]Value{"Name": String("before")}
	if valueSameHeaderAndBacking(base, changed) {
		t.Fatal("different Fields maps should not have the same header and backing")
	}

	allocated := Value{Kind: ValueObject, Ref: 41, Fields: make(map[string]Value)}
	if valueSameHeaderAndBacking(Value{Kind: ValueObject, Ref: 41}, allocated) {
		t.Fatal("nil-to-allocated Fields should not have the same header and backing")
	}

	for name, mutate := range map[string]func(*Value){
		"type":                   func(value *Value) { value.Type = "Contact" },
		"text":                   func(value *Value) { value.Text = "changed" },
		"hidden flag":            func(value *Value) { value.schemaIDConstructor = true },
		"loaded reference":       func(value *Value) { value.loadedReferenceSnapshot = &sobjectLoadedReferenceSnapshot{} },
		"list map null key flag": func(value *Value) { value.sobjectListNullMapKey = true },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if valueSameHeaderAndBacking(base, candidate) {
				t.Fatal("changed Value field should prevent elision")
			}
		})
	}

	emptyLeft := Object("Account")
	emptyRight := emptyLeft
	emptyRight.Fields = make(map[string]Value)
	if valueSameHeaderAndBacking(emptyLeft, emptyRight) {
		t.Fatal("distinct empty Fields maps should not have the same backing")
	}
}

func TestValueSameHeaderAndBackingGuardsValueFieldCount(t *testing.T) {
	if got, want := reflect.TypeOf(Value{}).NumField(), 32; got != want {
		t.Fatalf("valueSameHeaderAndBacking must be updated for Value fields: got %d, want %d", got, want)
	}
}

func TestListMapNullKeyFlagChangeDoesNotElideRecordPropagation(t *testing.T) {
	before := Object("Account")
	before.Ref = 41
	before.Fields["Name"] = String("before")
	after := before
	after.sobjectListNullMapKey = true

	if (&VM{}).shouldElideRecordScopeWalk(before, after) {
		t.Fatal("changing the list-based null key flag must retain record propagation")
	}
}
