package vm

import "testing"

func TestCloneRuntimeIsolatesNestedStaticStateAcrossClassAliases(t *testing.T) {
	value := Object("StaticState")
	node := Object("StaticNode")
	node.Fields["labels"] = Set(String("value"))
	value.Fields["items"] = List(node)

	initial := Map()
	initialEntry := Map()
	initialEntry.Map[mapKey(String("leaf"))] = String("initial")
	initialEntry.MapKeys[mapKey(String("leaf"))] = String("leaf")
	initialEntry.MapOrder = append(initialEntry.MapOrder, mapKey(String("leaf")))
	initial.Map[mapKey(String("seed"))] = List(initialEntry)
	initial.MapKeys[mapKey(String("seed"))] = String("seed")
	initial.MapOrder = append(initial.MapOrder, mapKey(String("seed")))

	template := New(nil)
	if err := template.RegisterClass(Class{
		Name:      "StaticRegistry",
		Namespace: "pkg",
		StaticFields: map[string]Field{
			"State": {
				Name:         "State",
				Type:         "StaticState",
				Static:       true,
				Value:        value,
				InitialValue: initial,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	templateClass := template.Classes["StaticRegistry"]
	templateField := templateClass.StaticFields["State"]
	templateField.Value = value
	templateField.InitialValue = initial
	templateClass.StaticFields["State"] = templateField
	template.Classes["StaticRegistry"] = templateClass
	template.FreezeClassLookup()
	first := template.CloneRuntime(nil)
	second := template.CloneRuntime(nil)

	firstClass := first.Classes["StaticRegistry"]
	firstField := firstClass.StaticFields["State"]
	firstValue := firstField.Value
	firstItems := firstValue.Fields["items"]
	firstNode := firstItems.List[0]
	firstLabels := firstNode.Fields["labels"]
	firstLabels.Set[0] = String("changed-value")
	firstNode.Fields["labels"] = firstLabels
	firstItems.List[0] = firstNode
	firstValue.Fields["items"] = firstItems
	firstField.Value = firstValue

	firstInitial := firstField.InitialValue
	firstInitialList := firstInitial.Map[mapKey(String("seed"))]
	firstInitialEntry := firstInitialList.List[0]
	firstInitialEntry.Map[mapKey(String("leaf"))] = String("changed-initial")
	firstInitialList.List[0] = firstInitialEntry
	firstInitial.Map[mapKey(String("seed"))] = firstInitialList
	firstInitial.MapKeys[mapKey(String("seed"))] = String("changed-seed-key")
	firstInitial.MapOrder[0] = "changed-order"
	firstField.InitialValue = firstInitial
	firstClass.StaticFields["State"] = firstField
	first.Classes["StaticRegistry"] = firstClass

	aliasField := first.Classes["pkg.StaticRegistry"].StaticFields["State"]
	if got := aliasField.Value.Fields["items"].List[0].Fields["labels"].Set[0].Text; got != "changed-value" {
		t.Fatalf("clone class alias value = %q, want changed-value", got)
	}
	if got := aliasField.InitialValue.Map[mapKey(String("seed"))].List[0].Map[mapKey(String("leaf"))].Text; got != "changed-initial" {
		t.Fatalf("clone class alias initial value = %q, want changed-initial", got)
	}

	assertNestedStaticStateUnchanged(t, "template short alias", template.Classes["StaticRegistry"])
	assertNestedStaticStateUnchanged(t, "template qualified alias", template.Classes["pkg.StaticRegistry"])
	assertNestedStaticStateUnchanged(t, "sibling short alias", second.Classes["StaticRegistry"])
	assertNestedStaticStateUnchanged(t, "sibling qualified alias", second.Classes["pkg.StaticRegistry"])
}

func assertNestedStaticStateUnchanged(t *testing.T, name string, class Class) {
	t.Helper()
	field := class.StaticFields["State"]
	if got := field.Value.Fields["items"].List[0].Fields["labels"].Set[0].Text; got != "value" {
		t.Fatalf("%s nested static value = %q, want value", name, got)
	}
	seedKey := mapKey(String("seed"))
	leafKey := mapKey(String("leaf"))
	if got := field.InitialValue.Map[seedKey].List[0].Map[leafKey].Text; got != "initial" {
		t.Fatalf("%s nested static initial value = %q, want initial", name, got)
	}
	if got := field.InitialValue.MapKeys[seedKey].Text; got != "seed" {
		t.Fatalf("%s static initial map key = %q, want seed", name, got)
	}
	if got := field.InitialValue.MapOrder[0]; got != seedKey {
		t.Fatalf("%s static initial map order = %q, want %q", name, got, seedKey)
	}
}
