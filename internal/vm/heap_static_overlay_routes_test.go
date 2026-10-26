package vm

import (
	"strings"
	"testing"
)

// Native heap_throw_point List/Map rows retain payloads in Apex statics.
// The project runner reaches those statics through cloned class views.
func TestHeapStaticPayloadDirectEagerFrozenAndNestedCloneRoutes(t *testing.T) {
	for _, route := range []string{"direct", "eager_clone", "frozen_clone", "nested_frozen_clone", "materialized_clone"} {
		t.Run(route, func(t *testing.T) {
			template := New(nil)
			if err := template.RegisterClass(Class{Name: "HeapOverlayRegistry", Namespace: "pkg", StaticFields: map[string]Field{
				"Held": {Name: "Held", Type: "List<String>", Static: true, Value: Null, InitialValue: Null},
			}}); err != nil {
				t.Fatal(err)
			}
			machine := template
			if route != "direct" {
				template.FreezeClassLookup()
				if route == "eager_clone" {
					machine = template.CloneRuntime(nil)
				} else {
					machine = template.CloneRuntimeFrozenShared(nil)
				}
				if route == "nested_frozen_clone" {
					machine = machine.CloneRuntimeFrozenShared(nil)
				}
			}
			class, ok := machine.lookupClass("pkg.HeapOverlayRegistry")
			if !ok {
				t.Fatal("class missing")
			}
			payload := List(String(strings.Repeat("x", 100000)))
			machine.writeStaticFieldValue("pkg.HeapOverlayRegistry", "Held", class, class.StaticFields["Held"], payload)
			if route == "materialized_clone" {
				machine.prepareClassMapWrite()
			}
			current, ok := machine.lookupClass("pkg.HeapOverlayRegistry")
			if !ok || len(current.StaticFields["Held"].Value.List) != 1 {
				t.Fatal("static write did not reach the runtime class view")
			}
			if strings.Contains(route, "frozen") && machine.classValueOverlay == nil {
				t.Fatal("test missed the shared class-value overlay route")
			}
			if got := machine.currentHeapSize(); got < 100000 || got > 101000 {
				t.Fatalf("retained static heap=%d; want one payload across class aliases", got)
			}
			if route != "direct" {
				if got := template.currentHeapSize(); got > 1024 {
					t.Fatalf("clone changed template heap: %d", got)
				}
				sibling := template.CloneRuntimeFrozenShared(nil)
				if got := sibling.currentHeapSize(); got > 1024 {
					t.Fatalf("clone changed sibling heap: %d", got)
				}
			}
		})
	}
}
