package vm

import (
	"reflect"
	"sync"
	"testing"
)

func TestPrimitiveSetIndependentCloneRoutes(t *testing.T) {
	for _, element := range []string{"String", "Id"} {
		for _, shape := range []string{"nil", "empty", "populated"} {
			for route, clone := range primitiveSetCloneRoutes() {
				t.Run(element+"/"+shape+"/"+route, func(t *testing.T) {
					original := Set()
					original.Type = "Set<" + element + ">"
					if shape == "empty" {
						original.Set = make([]Value, 0, 4)
					}
					if shape == "populated" {
						original = appendSetEntry(original, primitiveSetCloneKey(element, "001000000000001"), setInsertionHash{})
					}
					left, right := clone(t, original), clone(t, original)
					if left.primitiveSetState != nil && (left.primitiveSetState == original.primitiveSetState || left.primitiveSetState == right.primitiveSetState) {
						t.Fatal("independent clones share mutable Set membership/append state")
					}
					left = appendSetEntry(left, primitiveSetCloneKey(element, "001000000000002"), setInsertionHash{})
					left = appendSetEntry(left, primitiveSetCloneKey(element, "001000000000003"), setInsertionHash{})
					right = appendSetEntry(right, primitiveSetCloneKey(element, "001000000000004"), setInsertionHash{})
					machine := New(nil)
					for _, check := range []struct {
						set  Value
						key  string
						want bool
					}{
						{left, "001000000000003", true}, {left, "001000000000004", false},
						{right, "001000000000004", true}, {right, "001000000000002", false},
						{original, "001000000000002", false},
					} {
						got, err := machine.setContainsValue(check.set, primitiveSetCloneKey(element, check.key), &Result{})
						if got != check.want || err != nil {
							t.Fatalf("contains %s = %v, %v; want %v", check.key, got, err, check.want)
						}
					}
				})
			}
		}
	}
}

func primitiveSetCloneKey(element, text string) Value {
	value := String(text)
	if element == "Id" {
		value.Type = "Id"
	}
	return value
}

func primitiveSetCloneRoutes() map[string]func(*testing.T, Value) Value {
	return map[string]func(*testing.T, Value) Value{
		"apex-clone": func(t *testing.T, v Value) Value {
			out, _, err := New(nil).callSetValueMember("", v, "clone", nil, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			return out
		},
		"apex-deep-clone": func(t *testing.T, v Value) Value {
			out, _, err := New(nil).callSetValueMember("", v, "deepClone", nil, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			return out
		},
		"debug-clone":       func(_ *testing.T, v Value) Value { return cloneValue(v) },
		"detached-clone":    func(_ *testing.T, v Value) Value { return cloneValueDetachedPreserveRefs(v) },
		"alias-snapshot":    func(_ *testing.T, v Value) Value { return cloneValuePreserveRefs(v) },
		"vm-alias-snapshot": func(_ *testing.T, v Value) Value { return New(nil).cloneValuePreserveRefs(v) },
		"static-value": func(_ *testing.T, v Value) Value {
			return copyFieldMap(map[string]Field{"keys": {Value: v}})["keys"].Value
		},
		"static-initial-value": func(_ *testing.T, v Value) Value {
			return copyFieldMap(map[string]Field{"keys": {InitialValue: v}})["keys"].InitialValue
		},
		"runtime-eager":               func(t *testing.T, v Value) Value { return primitiveSetRuntimeClone(t, v, false) },
		"runtime-frozen-first-access": func(t *testing.T, v Value) Value { return primitiveSetRuntimeClone(t, v, true) },
	}
}

func primitiveSetRuntimeClone(t *testing.T, v Value, frozen bool) Value {
	t.Helper()
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "CloneRegistry", StaticFields: map[string]Field{"keys": {Name: "keys", Static: true, Type: v.Type, Value: v, InitialValue: v}}}); err != nil {
		t.Fatal(err)
	}
	// Model initialized runtime state, after registration's type transport.
	class := machine.Classes["CloneRegistry"]
	field := class.StaticFields["keys"]
	field.Value, field.InitialValue = v, v
	class.StaticFields["keys"] = field
	machine.Classes["CloneRegistry"] = class
	machine.FreezeClassLookup()
	clone := machine.CloneRuntime(nil)
	if frozen {
		clone = machine.CloneRuntimeFrozenShared(nil)
	}
	class, ok := clone.ensureMutableClass("CloneRegistry")
	if !ok {
		t.Fatal("missing cloned class")
	}
	return class.StaticFields["keys"].Value
}

func TestPrimitiveSetConcurrentCloneAppendRoutes(t *testing.T) {
	for route, clone := range primitiveSetCloneRoutes() {
		t.Run(route, func(t *testing.T) {
			original := Set()
			original.Type = "Set<String>"
			values := []Value{clone(t, original), clone(t, original)}
			var group sync.WaitGroup
			start := make(chan struct{})
			for i := range values {
				group.Add(1)
				go func(i int) {
					defer group.Done()
					<-start
					values[i] = appendSetEntry(values[i], String("first"), setInsertionHash{})
				}(i)
			}
			close(start)
			group.Wait()
		})
	}
}

func TestPrimitiveSetAliasSnapshotStructuralRoutes(t *testing.T) {
	for _, element := range []string{"Object", "String", "Id"} {
		for _, populated := range []bool{false, true} {
			for route, clone := range map[string]func(Value) Value{"alias-snapshot": cloneValuePreserveRefs, "vm-alias-snapshot": New(nil).cloneValuePreserveRefs} {
				t.Run(element+"/"+route+"/"+map[bool]string{false: "empty", true: "populated"}[populated], func(t *testing.T) {
					original := Set()
					original.Type = "Set<" + element + ">"
					if populated {
						original = appendSetEntry(original, primitiveSetCloneKey(element, "001000000000001"), setInsertionHash{})
					}
					if snapshot := clone(original); !reflect.DeepEqual(original, snapshot) {
						t.Fatal("alias snapshot changes structural state")
					}
				})
			}
		}
	}
}

func TestPrimitiveSetCacheHeaderElisionRoute(t *testing.T) {
	before := Object("Account")
	after := before
	after.primitiveSetState = &primitiveSetState{}
	if valueSameHeaderAndBacking(before, after) || New(nil).shouldElideRecordScopeWalk(before, after) {
		t.Fatal("changed Set ownership token bypasses header audit")
	}
}

func TestPrimitiveSetPopulatedCloneIndexedContainsRoutes(t *testing.T) {
	for _, element := range []string{"String", "Id"} {
		for route, clone := range primitiveSetCloneRoutes() {
			t.Run(element+"/"+route, func(t *testing.T) {
				key := primitiveSetCloneKey(element, "001000000000001")
				original := Set()
				original.Type = "Set<" + element + ">"
				original = appendSetEntry(original, key, setInsertionHash{})
				copied := clone(t, original)
				if index, handled := primitiveSetIndex(copied, key); !handled || index != 0 {
					t.Fatalf("cloned contains lost indexed route: index=%d handled=%v", index, handled)
				}
			})
		}
	}
}
