package apextest

import (
	"reflect"
	"testing"
)

// Constructor declarations carry their declared access just like methods,
// fields, and source-free artifact constructors. Omitted access stays omitted.
func TestCompileProjectConstructorsRetainDeclaredModifiers(t *testing.T) {
	project := newSalesforceSurfaceProject(t, `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"66.0"}`)
	project.writeClass("ConstructorProbe", `global virtual class ConstructorProbe {
  protected ConstructorProbe() {}
  public ConstructorProbe(String value) {}
  private ConstructorProbe(Integer value) {}
  ConstructorProbe(Boolean value) {}
  global ConstructorProbe(Long value) {}
}`)
	index := loadTestIndex(t, project.root)
	for _, class := range compileProjectClasses(index, nil) {
		if class.Name != "ConstructorProbe" {
			continue
		}
		if len(class.Constructors) != 5 {
			t.Fatalf("constructor count = %d, want 5", len(class.Constructors))
		}
		for _, ctor := range class.Constructors {
			parameter := ""
			if len(ctor.Params) > 0 {
				parameter = ctor.Params[0].Type
			}
			wantAccess := map[string]string{"": "protected", "String": "public", "Integer": "private", "Boolean": "", "Long": "global"}[parameter]
			var wantModifiers []string
			if wantAccess != "" {
				wantModifiers = []string{wantAccess}
			}
			if ctor.Access != wantAccess || !reflect.DeepEqual(ctor.Modifiers, wantModifiers) {
				t.Errorf("constructor(%s) access/modifiers = %q/%v, want %q/%v", parameter, ctor.Access, ctor.Modifiers, wantAccess, wantModifiers)
			}
		}
		return
	}
	t.Fatal("source class was not compiled")
}
