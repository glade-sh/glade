package vm

import (
	"sort"
	"strings"
	"testing"
)

// The unqualified nested-enum lookup previously scanned every class entry. The
// index must answer exactly the classes that scan admitted, follow later
// registrations, and leave lookup results unchanged.
func TestEnumSuffixClassIndexMatchesClassScan(t *testing.T) {
	machine := New(nil)
	for _, class := range []Class{
		{Name: "Outer.State", EnumValues: []string{"Ready", "Done"}},
		{Name: "Other.State", EnumValues: []string{"Idle"}},
		{Name: "Outer.Plain"},
		{Name: "Alone", EnumValues: []string{"Single"}},
		{Name: "Nested.Deep.State", EnumValues: []string{"Deep"}},
		{Name: "Shape", Namespace: "pkg", EnumValues: []string{"Round"}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	scan := func(suffix string) []string {
		var names []string
		for _, class := range machine.ClassEntries() {
			if strings.HasSuffix(class.Name, "."+suffix) && len(class.EnumValues) > 0 {
				names = append(names, class.Name)
			}
		}
		sort.Strings(names)
		return slicesCompactSorted(names)
	}
	indexed := func(suffix string) []string {
		var names []string
		for _, key := range machine.enumSuffixClassCandidates(suffix) {
			class, ok := machine.classEntry(key)
			if !ok {
				t.Fatalf("index key %q has no class entry", key)
			}
			names = append(names, class.Name)
		}
		sort.Strings(names)
		return names
	}
	for _, suffix := range []string{"State", "Plain", "Alone", "Shape", "state", "Missing", ""} {
		want, got := scan(suffix), indexed(suffix)
		if strings.Join(want, ",") != strings.Join(got, ",") {
			t.Fatalf("suffix %q: index %v, scan %v", suffix, got, want)
		}
	}
	// A registration after the first lookup must be visible to the next one.
	if err := machine.RegisterClass(Class{Name: "Late.State", EnumValues: []string{"Late"}}); err != nil {
		t.Fatal(err)
	}
	if want, got := scan("State"), indexed("State"); strings.Join(want, ",") != strings.Join(got, ",") {
		t.Fatalf("after registration: index %v, scan %v", got, want)
	}
	if len(scan("State")) != 4 {
		t.Fatalf("expected four State enums after registration, scan found %v", scan("State"))
	}
}

func slicesCompactSorted(names []string) []string {
	out := names[:0]
	for i, name := range names {
		if i == 0 || names[i-1] != name {
			out = append(out, name)
		}
	}
	return out
}

func TestExecUnqualifiedNestedEnumLookupUsesIndex(t *testing.T) {
	machine := New(nil)
	// "Label" is not a registered class, so the lookup falls through to the
	// nested-enum suffix candidates, as the former scan over every class did.
	if err := machine.RegisterClass(Class{Name: "Outer.Label", EnumValues: []string{"Some_Label", "Other"}}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Object first = Label.Some_Label;
System.assertEquals('Some_Label', String.valueOf(first));
Object second = Label.other;
System.assertEquals('Other', String.valueOf(second));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if machine.enumSuffixClasses == nil {
		t.Fatal("unqualified nested enum lookup did not build the suffix index")
	}
	missing, err := CompileAnonymous(`Object v = Phase.Open; System.assertEquals('Open', String.valueOf(v));`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(missing); err == nil {
		t.Fatal("unregistered nested enum resolved")
	}
	if err := machine.RegisterClass(Class{Name: "Outer.Phase", EnumValues: []string{"Open"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(missing); err != nil {
		t.Fatalf("enum registered after the index was built did not resolve: %v", err)
	}
	unknown, err := CompileAnonymous(`Object v = Label.Missing;`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(unknown); err == nil {
		t.Fatal("unknown enum member resolved through the suffix index")
	}
}
