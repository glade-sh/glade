package vm

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func classIndexReference(classes map[string]Class) map[string]Class {
	result := make(map[string]Class)
	ranks := make(map[string]int)
	namespaces := make(map[string]string)
	put := func(name string, class Class, exact bool) {
		if strings.TrimSpace(name) == "" {
			return
		}
		key := canonicalClassLookupKey(name)
		rank := classLookupKeyRank(class, exact)
		if current, ok := ranks[key]; ok && !classLookupKeyWins(rank, class.Namespace, current, namespaces[key]) {
			return
		}
		result[key], ranks[key], namespaces[key] = class, rank, class.Namespace
	}
	for alias, class := range classes {
		put(alias, class, true)
		put(class.Name, class, false)
		if class.Namespace != "" {
			put(class.Namespace+"."+class.Name, class, true)
		}
	}
	return result
}

func TestClassIndexMatchesReference(t *testing.T) {
	classes := map[string]Class{
		"Widget":     {Name: "Widget"},
		"pkg.Widget": {Name: "Widget", Namespace: "pkg", Dependency: true},
		"Alias":      {Name: "Other"},
		" ":          {Name: "Named"},
		"ONE.Gadget": {Name: "Gadget", Namespace: "one", Dependency: true},
		"TWO.Gadget": {Name: "Gadget", Namespace: "two", Dependency: true},
		"Ωwner":      {Name: "Ωwner"},
	}
	machine := New(nil)
	machine.Classes = classes
	machine.rebuildClassLookup()
	want := classIndexReference(classes)
	if !reflect.DeepEqual(machine.classLookup, want) {
		t.Fatalf("rebuilt index differs from reference: got %v, want %v", machine.classLookup, want)
	}
}

func BenchmarkClassIndexRebuild(b *testing.B) {
	machine := New(nil)
	machine.Classes = make(map[string]Class, 1500)
	for i := 0; i < 1500; i++ {
		name := fmt.Sprintf("Model%04d", i)
		machine.Classes[name] = Class{Name: name}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		machine.rebuildClassLookup()
	}
}
