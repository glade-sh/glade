package vm

import "testing"

func TestRangeRegisteredMethodsIncludesOverlay(t *testing.T) {
	base := New(nil)
	if err := base.RegisterMethod(Method{Name: "OverlayOwner.first"}); err != nil {
		t.Fatal(err)
	}
	clone := base.CloneRuntime(nil)
	if err := clone.RegisterMethod(Method{Name: "OverlayOwner.first", ReturnType: "String"}); err != nil {
		t.Fatal(err)
	}
	if err := clone.RegisterMethod(Method{Name: "OverlayOwner.second", ReturnType: "Integer"}); err != nil {
		t.Fatal(err)
	}
	got := map[string]Method{}
	clone.RangeRegisteredMethods(func(m Method) { got[m.Name] = m })
	if got["OverlayOwner.first"].ReturnType != "String" || got["OverlayOwner.second"].ReturnType != "Integer" {
		t.Fatalf("effective methods: %#v", got)
	}
	base.RangeRegisteredMethods(func(m Method) {
		if m.Name == "OverlayOwner.second" {
			t.Error("overlay leaked to base")
		}
	})
}
