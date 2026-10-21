package vm

import "testing"

func TestSwitchCaseLocalLeavesScopeBeforeNextIteration(t *testing.T) {
	for _, api := range []string{"49.0", "61.0", "67.0"} {
		for _, seedCase := range []string{"Seed", "Fallback"} {
			t.Run(api+"/"+seedCase, func(t *testing.T) {
				machine := New(nil)
				if err := machine.RegisterClass(Class{Name: "Palette"}); err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: "Palette.Shade", EnumValues: []string{"Light", "Dark"}}); err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymousWithOptions(`
Integer count = 0;
for (String phase : new List<String>{'`+seedCase+`', 'Read'}) {
    switch on phase {
        when 'Seed' { Integer shade = 3; }
        when 'Read' {
            for (Shade shade : Shade.values()) { count++; }
        }
        when else { Integer shade = 4; }
    }
}
System.assertEquals(2, count);
System.assertEquals(2, Shade.values().size());
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine.currentClass = "Palette"
				machine.currentMethod = Method{ClassName: "Palette", Name: "Palette.observe"}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
