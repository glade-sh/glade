package vm

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCloneRuntimeSharesStaticTemplatesUntilFirstWrite(t *testing.T) {
	template := New(nil)
	if err := template.RegisterClass(Class{
		Name:      "Registry",
		Namespace: "pkg",
		StaticFields: map[string]Field{
			"Values": {
				Name:         "Values",
				Type:         "List<String>",
				Static:       true,
				Value:        List(String("seed")),
				InitialValue: List(String("seed")),
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	template.FreezeClassLookup()
	first := template.CloneRuntimeFrozenShared(nil)
	second := template.CloneRuntimeFrozenShared(nil)

	templatePointer := reflect.ValueOf(template.Classes["Registry"].StaticFields).Pointer()
	if got := reflect.ValueOf(first.Classes["Registry"].StaticFields).Pointer(); got != templatePointer {
		t.Fatalf("first clone static template pointer = %x, want shared %x", got, templatePointer)
	}
	if got := reflect.ValueOf(second.Classes["pkg.Registry"].StaticFields).Pointer(); got != templatePointer {
		t.Fatalf("second clone static template pointer = %x, want shared %x", got, templatePointer)
	}

	class, ok := first.lookupClass("pkg.Registry")
	if !ok {
		t.Fatal("class not found")
	}
	field := class.StaticFields["Values"]
	first.writeStaticFieldValue("pkg.Registry", "Values", class, field, List(String("changed")))

	firstPointer := reflect.ValueOf(first.Classes["Registry"].StaticFields).Pointer()
	if firstPointer == templatePointer {
		t.Fatal("first static write did not detach the shared template")
	}
	if got := reflect.ValueOf(first.Classes["pkg.Registry"].StaticFields).Pointer(); got != firstPointer {
		t.Fatalf("qualified alias pointer = %x, want detached %x", got, firstPointer)
	}
	if got := template.Classes["Registry"].StaticFields["Values"].Value.List[0].Text; got != "seed" {
		t.Fatalf("template value = %q, want seed", got)
	}
	if got := second.Classes["Registry"].StaticFields["Values"].Value.List[0].Text; got != "seed" {
		t.Fatalf("sibling value = %q, want seed", got)
	}
}

func TestCloneRuntimePostFreezeRegistrationKeepsStructuralOverlayPrivate(t *testing.T) {
	template := New(nil)
	if err := template.RegisterClass(Class{Name: "Existing"}); err != nil {
		t.Fatal(err)
	}
	template.FreezeClassLookup()
	clone := template.CloneRuntimeFrozenShared(nil)
	if err := clone.RegisterClass(Class{
		Name: "Added",
		StaticFields: map[string]Field{
			"Flag": {Name: "Flag", Type: "Boolean", Static: true, Value: Bool(true)},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := clone.lookupClass("Added"); !ok {
		t.Fatal("clone registration missing")
	}
	if _, ok := template.lookupClass("Added"); ok {
		t.Fatal("clone registration changed frozen template")
	}
}

func TestCloneRuntimeDetachesStaticCollectionBeforeMemberMutation(t *testing.T) {
	mutate, err := CompileAnonymous("Values.put('changed', 'yes');")
	if err != nil {
		t.Fatal(err)
	}
	initial := typedMap("Map<String,String>")
	initial.Map[mapKey(String("seed"))] = String("value")
	initial.MapKeys[mapKey(String("seed"))] = String("seed")
	template := New(nil)
	if err := template.RegisterClass(Class{
		Name: "MapRegistry",
		StaticFields: map[string]Field{
			"Values": {
				Name:         "Values",
				Type:         "Map<String,String>",
				Static:       true,
				Value:        initial,
				InitialValue: initial,
			},
		},
		Methods: map[string]Method{
			"mutate": {
				Name:      "MapRegistry.mutate",
				ClassName: "MapRegistry",
				IsStatic:  true,
				Program:   mutate,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	template.FreezeClassLookup()
	first := template.CloneRuntimeFrozenShared(nil)
	second := template.CloneRuntimeFrozenShared(nil)
	if _, err := first.CallStatic("MapRegistry.mutate", nil); err != nil {
		t.Fatal(err)
	}
	changedKey := mapKey(String("changed"))
	if _, ok := template.Classes["MapRegistry"].StaticFields["Values"].Value.Map[changedKey]; ok {
		t.Fatal("member mutation changed template static collection")
	}
	if _, ok := second.Classes["MapRegistry"].StaticFields["Values"].Value.Map[changedKey]; ok {
		t.Fatal("member mutation changed sibling static collection")
	}
	if _, ok := first.Classes["MapRegistry"].StaticFields["Values"].Value.Map[changedKey]; !ok {
		t.Fatal("member mutation missing from owning runtime")
	}
}

func BenchmarkRuntimeClassCloneDeep(b *testing.B) {
	classes, plan := staticOverlayBenchmarkClasses(b, 1500, 8)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloned := copyClassMapWithPlan(classes, plan)
		if len(cloned) != len(classes) {
			b.Fatal("class clone count mismatch")
		}
	}
}

func BenchmarkRuntimeClassCloneOverlay(b *testing.B) {
	classes, plan := staticOverlayBenchmarkClasses(b, 1500, 8)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloned := copyClassMapSharedStaticsWithPlan(classes, plan)
		if len(cloned) != len(classes) {
			b.Fatal("class clone count mismatch")
		}
	}
}

func staticOverlayBenchmarkClasses(tb testing.TB, classCount, staticCount int) (map[string]Class, *classCopyPlan) {
	tb.Helper()
	machine := New(nil)
	for i := 0; i < classCount; i++ {
		fields := make(map[string]Field, staticCount)
		for j := 0; j < staticCount; j++ {
			name := fmt.Sprintf("Field%02d", j)
			fields[name] = Field{
				Name:         name,
				Type:         "List<String>",
				Static:       true,
				Value:        List(String("alpha"), String("beta"), String("gamma")),
				InitialValue: List(String("alpha"), String("beta"), String("gamma")),
			}
		}
		if err := machine.RegisterClass(Class{Name: fmt.Sprintf("Class%04d", i), StaticFields: fields}); err != nil {
			tb.Fatal(err)
		}
	}
	return machine.Classes, buildClassCopyPlan(machine.Classes)
}
