package vm

import (
	"strings"
	"testing"
)

func assertHeapMatchesFullWalk(t *testing.T, machine *VM, step string) int {
	t.Helper()
	got := machine.currentHeapSize()
	if want := machine.currentHeapSizeFullWalk(); got != want {
		t.Fatalf("%s: cached heap = %d, full walk = %d", step, got, want)
	}
	return got
}

// The cached static-root list must agree with the full class walk across
// grow/shrink, nested mutation, static reassignment and class replacement.
func TestHeapCachedStaticRootsMatchFullWalkAcrossMutations(t *testing.T) {
	for _, frozen := range []bool{false, true} {
		name := "plain"
		if frozen {
			name = "frozenShared"
		}
		t.Run(name, func(t *testing.T) {
			machine := New(nil)
			items := List(String("a"))
			mapping := Map()
			record := Object("Account")
			record.Fields["Name"] = String("n")
			statics := map[string]Field{
				"items":  {Name: "items", Static: true, Value: items},
				"mapped": {Name: "mapped", Static: true, Value: mapping},
				"record": {Name: "record", Static: true, Value: record},
				"count":  {Name: "count", Static: true, Value: Int(1)},
			}
			if err := machine.RegisterClass(Class{Name: "HeapHolder", StaticFields: statics}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterClass(Class{Name: "HeapOther", StaticFields: map[string]Field{}}); err != nil {
				t.Fatal(err)
			}
			if frozen {
				machine.FreezeClassLookup()
				machine = machine.CloneRuntimeFrozenShared(nil)
			}
			base := assertHeapMatchesFullWalk(t, machine, "initial")

			current := func() (Class, map[string]Field) {
				class, ok := machine.classEntry("HeapHolder")
				if !ok {
					t.Fatal("HeapHolder missing")
				}
				return class, class.StaticFields
			}

			// Grow and shrink a list held by a static (shared backing mutation).
			_, fields := current()
			list := fields["items"].Value
			for i := 0; i < 50; i++ {
				list.List = append(list.List, String(strings.Repeat("x", i+1)))
				field := fields["items"]
				field.Value = list
				fields["items"] = field
				assertHeapMatchesFullWalk(t, machine, "list grow")
			}
			grown := machine.currentHeapSize()
			if grown <= base {
				t.Fatalf("heap did not grow: %d <= %d", grown, base)
			}
			list.List = list.List[:3]
			field := fields["items"]
			field.Value = list
			fields["items"] = field
			if shrunk := assertHeapMatchesFullWalk(t, machine, "list shrink"); shrunk >= grown {
				t.Fatalf("heap did not shrink: %d >= %d", shrunk, grown)
			}

			// Grow and shrink a map.
			m := fields["mapped"].Value
			for i := 0; i < 20; i++ {
				m.Map[strings.Repeat("k", i+1)] = String("v")
			}
			assertHeapMatchesFullWalk(t, machine, "map grow")
			for key := range m.Map {
				delete(m.Map, key)
			}
			assertHeapMatchesFullWalk(t, machine, "map shrink")

			// Mutate a nested SObject field.
			rec := fields["record"].Value
			rec.Fields["Name"] = String(strings.Repeat("z", 500))
			assertHeapMatchesFullWalk(t, machine, "sobject mutate")

			// Reassign a static through the class-write path (new map identity).
			class, _ := machine.ensureMutableClass("HeapHolder")
			class.StaticFields = copyFieldMap(class.StaticFields)
			class.StaticFields["count"] = Field{Name: "count", Static: true, Value: String(strings.Repeat("c", 1000))}
			machine.storeMutableClassAtAlias("HeapHolder", class)
			assertHeapMatchesFullWalk(t, machine, "static reassign new map")

			// Reassign in place (same map identity), and add a new static.
			class, _ = machine.ensureMutableClass("HeapHolder")
			class.StaticFields["count"] = Field{Name: "count", Static: true, Value: Int(2)}
			class.StaticFields["late"] = Field{Name: "late", Static: true, Value: String("late")}
			machine.storeMutableClassAtAlias("HeapHolder", class)
			assertHeapMatchesFullWalk(t, machine, "static in place")

			// A static on a class that started with none.
			other, _ := machine.ensureMutableClass("HeapOther")
			if other.StaticFields == nil {
				other.StaticFields = map[string]Field{}
			}
			other.StaticFields["x"] = Field{Name: "x", Static: true, Value: String("other")}
			machine.storeMutableClassAtAlias("HeapOther", other)
			assertHeapMatchesFullWalk(t, machine, "empty class gains static")

			// Register a new class after the cache exists.
			if err := machine.RegisterClass(Class{Name: "HeapLate", StaticFields: map[string]Field{"v": {Name: "v", Static: true, Value: String("late class")}}}); err != nil {
				t.Fatal(err)
			}
			assertHeapMatchesFullWalk(t, machine, "late class")

			machine.Globals["g"] = List(String("global"))
			assertHeapMatchesFullWalk(t, machine, "global")
		})
	}
}
