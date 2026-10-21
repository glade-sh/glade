package vm

import (
	"fmt"
	"testing"
)

func TestGenericInterfaceCastCapturedStringImplementors(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, implementation := range []struct {
			name  string
			class string
		}{
			{name: "top-level", class: "TextOrder"},
			{name: "qualified-inner", class: "CastHolder.TextOrder"},
		} {
			for _, lookup := range []string{"fresh", "frozen"} {
				t.Run(api+"/"+implementation.name+"/"+lookup, func(t *testing.T) {
					compare, err := CompileAnonymousWithOptions("return 0;", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := New(nil)
					if err := machine.RegisterClass(Class{Name: "CastHolder", APIVersion: api, Access: "public"}); err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterClass(Class{
						Name:       implementation.class,
						APIVersion: api,
						Access:     "public",
						Interfaces: []string{"System.Comparator<String>"},
						Methods: map[string]Method{
							"compare": {
								Name:       implementation.class + ".compare",
								ClassName:  implementation.class,
								APIVersion: api,
								Access:     "public",
								ReturnType: "Integer",
								Params:     []Param{{Name: "left", Type: "String"}, {Name: "right", Type: "String"}},
								Program:    compare,
							},
						},
					}); err != nil {
						t.Fatal(err)
					}
					if lookup == "frozen" {
						machine.FreezeClassLookup()
					}
					source := fmt.Sprintf(`Object held = new %s();
System.Comparator<Object> castValue = (System.Comparator<Object>)held;
System.assert(castValue != null);`, implementation.class)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
