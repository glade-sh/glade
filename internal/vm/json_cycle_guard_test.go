package vm

import (
	"bytes"
	"testing"
)

func TestJSONSerializeCycleTerminatesForAliasedSObject(t *testing.T) {
	account := Object("Account")
	account.Fields["Name"] = String("cycle")
	account.Fields["Parent"] = account

	got, err := jsonMarshalNoEscape((&VM{}).jsonFromValueForSerialize(account, false))
	if err != nil {
		t.Fatalf("serialize cyclic SObject: %v", err)
	}
	want := []byte(`{"attributes":{"type":"Account"},"Name":"cycle","Parent":null}`)
	if !bytes.Equal(got, want) {
		t.Fatalf("serialized cyclic SObject = %s, want %s", got, want)
	}
}

func TestJSONSerializeCycleTerminatesForAliasedMap(t *testing.T) {
	value := Map()
	value.Map["self"] = value

	got, err := jsonMarshalNoEscape((&VM{}).jsonFromValueForSerialize(value, false))
	if err != nil {
		t.Fatalf("serialize cyclic map: %v", err)
	}
	want := []byte(`{"self":null}`)
	if !bytes.Equal(got, want) {
		t.Fatalf("serialized cyclic map = %s, want %s", got, want)
	}
}

func TestJSONSerializeCycleTerminatesForSharedBackingWithDifferentObjectRefs(t *testing.T) {
	value := Object("Account")
	alias := value
	alias.Ref = newValueRef()
	value.Fields["Parent"] = alias

	got, err := jsonMarshalNoEscape((&VM{}).jsonFromValueForSerialize(value, false))
	if err != nil {
		t.Fatalf("serialize differently-referenced cyclic SObject: %v", err)
	}
	want := []byte(`{"attributes":{"type":"Account"},"Parent":null}`)
	if !bytes.Equal(got, want) {
		t.Fatalf("serialized differently-referenced cyclic SObject = %s, want %s", got, want)
	}
}

func TestJSONSerializeCycleTerminatesForSharedBackingWithDifferentMapRefs(t *testing.T) {
	value := Map()
	alias := value
	alias.Ref = newValueRef()
	value.Map["self"] = alias

	got, err := jsonMarshalNoEscape((&VM{}).jsonFromValueForSerialize(value, false))
	if err != nil {
		t.Fatalf("serialize differently-referenced cyclic map: %v", err)
	}
	want := []byte(`{"self":null}`)
	if !bytes.Equal(got, want) {
		t.Fatalf("serialized differently-referenced cyclic map = %s, want %s", got, want)
	}
}

func TestJSONSerializeEmptyCollectionsAvoidCycleTracking(t *testing.T) {
	vm := &VM{}
	for name, value := range map[string]Value{
		"list": List(),
		"set":  Set(),
		"map":  Map(),
	} {
		got, err := jsonMarshalNoEscape(vm.jsonFromValueForSerialize(value, false))
		if err != nil {
			t.Fatalf("serialize empty %s: %v", name, err)
		}
		if string(got) != "[]" && name != "map" {
			t.Fatalf("serialized empty %s = %s, want []", name, got)
		}
		if name == "map" && string(got) != "{}" {
			t.Fatalf("serialized empty map = %s, want {}", got)
		}
	}
}

func BenchmarkJSONSerializeCycleGuard(b *testing.B) {
	value := Object("Payload")
	value.Fields["Name"] = String("payload")
	items := List()
	for i := 0; i < 20; i++ {
		item := Object("Item")
		item.Fields["Index"] = Int(int64(i))
		item.Fields["Value"] = String("value")
		items.List = append(items.List, item)
	}
	value.Fields["Items"] = items

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := jsonMarshalNoEscape((&VM{}).jsonFromValueForSerialize(value, false)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONSerializeCycleGuardScalars(b *testing.B) {
	values := []Value{Null, Int(1), Decimal(1.5), Bool(true), String("value")}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := jsonMarshalNoEscape((&VM{}).jsonFromValueForSerialize(values[i%len(values)], false)); err != nil {
			b.Fatal(err)
		}
	}
}
