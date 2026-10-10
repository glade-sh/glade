package vm

import (
	"fmt"
	"testing"
)

func TestIterableViewPreservesConcreteCollectionType(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, collection := range []string{"List", "Set"} {
			t.Run(api+"/"+collection, func(t *testing.T) {
				source := fmt.Sprintf(`
%s<String> original = new %s<String>{'Alpha'};
Iterable<Object> view = original;
Object boxed = view;
System.assert(boxed instanceof %s<String>);
System.assert(!(boxed instanceof %s<Integer>));
Iterable<Object> emptyView = new %s<String>();
Object emptyBoxed = emptyView;
System.assert(emptyBoxed instanceof %s<String>);
`, collection, collection, collection, collection, collection, collection)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := New(nil).Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestIterableViewRejectsIncompatibleSet(t *testing.T) {
	program, err := CompileAnonymous(`
Iterable<Object> view = new Set<Integer>{1};
Object boxed = view;
try {
 Set<String> narrowed = (Set<String>)boxed;
 System.assert(false, 'Integer elements cannot become Strings');
} catch (TypeException expected) {
 System.assert(expected.getMessage().contains('Invalid conversion'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
