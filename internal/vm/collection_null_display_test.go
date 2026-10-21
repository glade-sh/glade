package vm

import "testing"

// Owned native string-conversion rows at API 62/67/68,
// anonymous and @IsTest: display renders null, while String.valueOf keeps it.
func TestExecListDisplayNullToString(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for name, body := range map[string]string{
			"unset":     "return probeOutput;",
			"converted": "Object value = null; return String.valueOf(value);",
		} {
			t.Run(api+"/"+name, func(t *testing.T) {
				method, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				if err := machine.RegisterClass(Class{
					Name:   "DisplaySeed",
					Fields: map[string]Field{"probeOutput": {Name: "probeOutput", Type: "String"}},
					Methods: map[string]Method{"toString": {
						Name: "DisplaySeed.toString", ClassName: "DisplaySeed", ReturnType: "String", Program: method,
					}},
				}); err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymousWithOptions(`
DisplaySeed seed = new DisplaySeed();
System.assertEquals(null, seed.toString());
String observed = new List<Object>{seed} + '';
System.assertEquals('(null)', observed);
Object boxed = seed;
System.assertEquals('null', boxed + '');
System.assertEquals(null, String.valueOf(seed));
System.assertEquals('null', String.format('{0}', new List<Object>{seed}));
`, CompileOptions{APIVersion: api})
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

func TestExecListDisplayToStringControls(t *testing.T) {
	method, err := CompileAnonymous("return probeOutput;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:   "DisplaySeed",
		Fields: map[string]Field{"probeOutput": {Name: "probeOutput", Type: "String"}},
		Methods: map[string]Method{"toString": {
			Name: "DisplaySeed.toString", ClassName: "DisplaySeed", ReturnType: "String", Program: method,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
DisplaySeed seed = new DisplaySeed();
seed.probeOutput = '';
System.assertEquals('()', new List<Object>{seed} + '');
seed.probeOutput = 'amber';
System.assertEquals('(amber)', new List<Object>{seed} + '');
System.assertEquals('(null)', new List<Object>{null} + '');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
