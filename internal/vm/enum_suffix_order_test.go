package vm

import "testing"

// A bare enum name can be the suffix of a project nested enum and of a platform
// enum. The suffix memo used the first class in map order, so the answer
// changed from run to run. Project enums win; ties break by class name.
func TestEnumSuffixLookupPrefersProjectEnumDeterministically(t *testing.T) {
	for i := 0; i < 60; i++ {
		machine := New(nil)
		for _, class := range []Class{
			{Name: "Aaa.State", PlatformType: true, EnumValues: []string{"Platform"}},
			{Name: "Outer.State", Nested: true, EnumValues: []string{"Ready"}},
			{Name: "Zzz.State", PlatformType: true, EnumValues: []string{"Platform"}},
		} {
			if err := machine.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
		}
		class, ok := machine.resolveEnumClass("State")
		if !ok || class.Name != "Outer.State" || class.PlatformType {
			t.Fatalf("iteration %d: resolved %q (platform=%v, ok=%v), want the project enum Outer.State", i, class.Name, class.PlatformType, ok)
		}
	}
}

func TestEnumSuffixLookupTieBreakIsStable(t *testing.T) {
	for _, platform := range []bool{false, true} {
		var first string
		for i := 0; i < 60; i++ {
			machine := New(nil)
			for _, name := range []string{"Mmm.State", "Aaa.State", "Zzz.State", "Ccc.State"} {
				if err := machine.RegisterClass(Class{Name: name, PlatformType: platform, EnumValues: []string{"V"}}); err != nil {
					t.Fatal(err)
				}
			}
			class, ok := machine.resolveEnumClass("State")
			if !ok {
				t.Fatal("State did not resolve")
			}
			if i == 0 {
				first = class.Name
			} else if class.Name != first {
				t.Fatalf("platform=%v: iteration %d resolved %q, earlier %q", platform, i, class.Name, first)
			}
		}
		if first != "Aaa.State" {
			t.Fatalf("platform=%v: resolved %q, want the first class name Aaa.State", platform, first)
		}
	}
}
