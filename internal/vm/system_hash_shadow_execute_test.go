package vm

import "testing"

// Exercise the anonymous System.hashCode entry point with source class identity.
func TestSystemHashSourceClassShadowAnonymousRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			if err := machine.RegisterClass(Class{Name: "HashParent"}); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name   string
				result string
			}{{"Account", "71"}, {"Contact", "72"}} {
				method, err := CompileAnonymous("return " + tc.result + ";")
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: tc.name, SuperClass: "HashParent", Methods: map[string]Method{"hashCode": {Name: tc.name + ".hashCode", ClassName: tc.name, ReturnType: "Integer", Access: "public", APIVersion: api, Program: method}}}); err != nil {
					t.Fatal(err)
				}
			}
			program, err := CompileAnonymousWithOptions("System.assertEquals(71,System.hashCode(new Account())); System.assertEquals(72,System.hashCode(new Contact()));", CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine.currentMethod = Method{APIVersion: api}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
