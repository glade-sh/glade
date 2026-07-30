package vm

import "testing"

func TestNamespaceAliasStaticLocationsUseOneCanonicalOwner(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(namespaceAliasLocationClass("Registry", "pkg", true, "managed")); err != nil {
		t.Fatal(err)
	}

	target := machine.Classes["pkg.Registry"].StaticFields["Items"].Value
	machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
	locations := machine.staticValueRefFields[target.Ref].locations()
	if len(locations) != 1 || locations[0] != (staticFieldRef{ClassName: "pkg.Registry", FieldName: "Items"}) {
		t.Fatalf("managed static locations = %#v, want one canonical qualified owner", locations)
	}

	updated := target
	updated.List = append(append([]Value(nil), target.List...), String("updated"))
	machine.propagateAliasSnapshotToStatics(snapshotAlias(target), updated)
	for _, alias := range []string{"Registry", "pkg.Registry"} {
		got := machine.Classes[alias].StaticFields["Items"].Value
		if got.Ref != target.Ref || len(got.List) != 2 || got.List[1].Text != "updated" {
			t.Fatalf("%s static alias = %#v, want current shared value", alias, got)
		}
	}
}

func TestNamespaceAliasCanonicalOwnersPreserveCollisionsAndPostFreezeOverlay(t *testing.T) {
	template := New(nil)
	if err := template.RegisterClass(namespaceAliasLocationClass("Registry", "pkg", true, "managed")); err != nil {
		t.Fatal(err)
	}
	template.FreezeClassLookup()
	machine := template.CloneRuntime(nil)
	if err := machine.RegisterClass(namespaceAliasLocationClass("Registry", "", false, "local")); err != nil {
		t.Fatal(err)
	}
	if machine.frozenClassLookup != nil {
		t.Fatal("post-freeze registration did not create a private lookup overlay")
	}
	if template.frozenClassLookup == nil {
		t.Fatal("post-freeze clone registration modified the frozen template lookup")
	}

	local, ok := machine.lookupClass("Registry")
	if !ok || local.Namespace != "" {
		t.Fatalf("local collision lookup = (%#v, %v), want local class", local, ok)
	}
	managed, ok := machine.lookupClass("pkg.Registry")
	if !ok || managed.Namespace != "pkg" {
		t.Fatalf("qualified managed lookup = (%#v, %v), want pkg class", managed, ok)
	}

	localTarget := machine.Classes["Registry"].StaticFields["Items"].Value
	managedTarget := machine.Classes["pkg.Registry"].StaticFields["Items"].Value
	_, fields := machine.collectStaticValueRefs()
	assertCanonicalStaticLocation(t, fields[localTarget.Ref], staticFieldRef{ClassName: "Registry", FieldName: "Items"})
	assertCanonicalStaticLocation(t, fields[managedTarget.Ref], staticFieldRef{ClassName: "pkg.Registry", FieldName: "Items"})
}

func TestNamespaceAliasCanonicalOwnersDoNotCollapseManagedNamespacesOrInstances(t *testing.T) {
	machine := New(nil)
	for _, namespace := range []string{"alpha", "beta"} {
		class := namespaceAliasLocationClass("Registry", namespace, true, namespace)
		class.Access = "private"
		class.Fields = map[string]Field{
			"Instance": {Name: "Instance", Type: "String", Value: String(namespace)},
		}
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}

	_, fields := machine.collectStaticValueRefs()
	for _, namespace := range []string{"alpha", "beta"} {
		alias := namespace + ".Registry"
		class := machine.Classes[alias]
		target := class.StaticFields["Items"].Value
		assertCanonicalStaticLocation(t, fields[target.Ref], staticFieldRef{ClassName: alias, FieldName: "Items"})
		if got := class.Fields["Instance"].Value.Text; got != namespace {
			t.Fatalf("%s instance field = %q, want %q", alias, got, namespace)
		}
		resolved, ok := machine.lookupClassInNamespace(namespace, "Registry")
		if !ok || resolved.Namespace != namespace {
			t.Fatalf("%s namespace lookup = (%#v, %v)", namespace, resolved, ok)
		}
	}
}

func namespaceAliasLocationClass(name, namespace string, dependency bool, marker string) Class {
	items := testTypedList("List<String>", String(marker))
	return Class{
		Name:       name,
		Namespace:  namespace,
		Dependency: dependency,
		StaticFields: map[string]Field{
			"Items": {
				Name:         "Items",
				Type:         "List<String>",
				Static:       true,
				Value:        items,
				InitialValue: items,
			},
		},
	}
}

func assertCanonicalStaticLocation(t *testing.T, locations staticFieldRefSet, want staticFieldRef) {
	t.Helper()
	got := locations.locations()
	if len(got) != 1 || got[0] != want {
		t.Fatalf("static locations = %#v, want [%#v]", got, want)
	}
}
