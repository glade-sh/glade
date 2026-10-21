package vm

import "testing"

func TestCollectionPropertyReceiverUsesStoredRuntimeParent(t *testing.T) {
	for _, propertyOnChild := range []bool{false, true} {
		name := "childField"
		if propertyOnChild {
			name = "childProperty"
		}
		t.Run(name, func(t *testing.T) {
			machine := New(nil)
			property := Field{Name: "Items", Type: "Map<String,Object>", Property: true, Getter: &Method{Name: "Base.Items.get", ClassName: "Base"}}
			plain := Field{Name: "Items", Type: "Map<String,Object>"}
			base, derived := property, plain
			if propertyOnChild {
				base, derived = plain, property
				derived.Getter = &Method{Name: "Derived.Items.get", ClassName: "Derived"}
			}
			for _, class := range []Class{
				{Name: "Base", Fields: map[string]Field{"Items": base}},
				{Name: "Derived", SuperClass: "Base", Fields: map[string]Field{"Items": derived}},
				{Name: "Holder", Fields: map[string]Field{"Child": {Name: "Child", Type: "Base"}}},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			child := Object("Derived")
			holder := Object("Holder")
			holder.Fields["Child"] = child
			machine.Globals["holder"] = holder
			machine.VarTypes["holder"] = "Holder"
			if got := machine.sourceCollectionPropertyReceiver("holder.Child.Items"); got != propertyOnChild {
				t.Fatalf("getter metadata=%v; runtime parent getter=%v", got, propertyOnChild)
			}
		})
	}
}
