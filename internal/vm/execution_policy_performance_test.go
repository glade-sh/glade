package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexversion"
)

// classSharingModeBaseline preserves the parent implementation as an outcome oracle.
func classSharingModeBaseline(vm *VM, className string) (string, bool) {
	seen := map[string]bool{}
	secureDefault := false
	for strings.TrimSpace(className) != "" && !seen[strings.ToLower(className)] {
		seen[strings.ToLower(className)] = true
		class, ok := vm.lookupClass(className)
		if !ok {
			break
		}
		secureDefault = secureDefault || apexversion.Enabled(class.APIVersion, apexversion.SecureDefaults)
		switch {
		case methodHasModifier(class.Modifiers, "with sharing"):
			return "with sharing", true
		case methodHasModifier(class.Modifiers, "without sharing"):
			return "without sharing", true
		case methodHasModifier(class.Modifiers, "inherited sharing"):
			return "inherited sharing", true
		}
		className = vm.resolvedSuperClassName(class)
	}
	if secureDefault {
		return "with sharing", true
	}
	return "", false
}

func sharingModeFixture() *VM {
	machine := New(nil)
	for _, class := range []Class{
		{Name: "Legacy", APIVersion: "62.0"},
		{Name: "Modern", APIVersion: "67.0"},
		{Name: "Explicit", Modifiers: []string{"without sharing"}},
		{Name: "Child", SuperClass: "Explicit"},
		{Name: "LoopA", SuperClass: "LoopB"},
		{Name: "LoopB", SuperClass: "LoopA"},
		{Name: "Ωmega", APIVersion: "67.0"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			panic(err)
		}
	}
	return machine
}

func TestClassSharingModeMatchesParent(t *testing.T) {
	machine := sharingModeFixture()
	for _, name := range []string{"", "  ", "missing", "Legacy", "Modern", "Explicit", "Child", "LoopA", "ωMEGA"} {
		got, ok := machine.classSharingMode(name)
		want, wantOK := classSharingModeBaseline(machine, name)
		if got != want || ok != wantOK {
			t.Fatalf("%q: got %q %t, want %q %t", name, got, ok, want, wantOK)
		}
	}
}

func BenchmarkClassSharingMode(b *testing.B) {
	machine := sharingModeFixture()
	names := []string{"Legacy", "Modern", "Explicit", "Child", "LoopA", "ωMEGA", "missing"}
	for _, name := range names {
		got, ok := machine.classSharingMode(name)
		want, wantOK := classSharingModeBaseline(machine, name)
		if got != want || ok != wantOK {
			b.Fatal("sharing mode changed")
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = machine.classSharingMode(names[i%len(names)])
	}
}
