package vm

import "testing"

func TestNestedEnumLiteralResolvesBeforeSameNamedOuterStaticField(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "RepositorySortOrder",
		StaticFields: map[string]Field{
			"ASCENDING": {Name: "ASCENDING", Type: "RepositorySortOrder", Static: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "RepositorySortOrder.SortOrder",
		EnumValues: []string{"ASCENDING", "DESCENDING"},
	}); err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "RepositorySortOrder"
	machine.currentMethod = Method{
		ClassName: "RepositorySortOrder",
		Name:      "RepositorySortOrder.<static_field_init>.ASCENDING",
	}

	value, err := machine.lookup("SortOrder.ASCENDING")
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if value.Kind != ValueObject || value.Type != "RepositorySortOrder.SortOrder" || value.Text != "ASCENDING" {
		t.Fatalf("nested enum literal = %#v, want RepositorySortOrder.SortOrder.ASCENDING", value)
	}
	if _, err := machine.coerceAssignable("RepositorySortOrder.SortOrder", value); err != nil {
		t.Fatalf("nested enum literal should be assignable to its enum type: %v", err)
	}
}
