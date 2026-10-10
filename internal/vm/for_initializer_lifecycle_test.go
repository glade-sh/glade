package vm

import "testing"

func TestClassicForInitializerBindingsLeaveScope(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		wantErr bool
	}{
		{"exhaustion", `for (Integer minute = 0, other = 2; minute < other; minute++) {}`, false},
		{"zero iterations", `for (Integer minute = 0; minute < 0; minute++) {}`, false},
		{"break", `for (Integer minute = 0; minute < 2; minute++) { break; }`, false},
		{"continue", `for (Integer minute = 0; minute < 2; minute++) { continue; }`, false},
		{"return", `for (Integer minute = 0; minute < 2; minute++) { return minute; }`, false},
		{"throw", `for (Integer minute = 0; minute < 2; minute++) { throw new IllegalArgumentException('owned'); }`, true},
		{"initializer failure", `for (Integer minute = 0, other = Integer.valueOf('invalid'); minute < 2; minute++) {}`, true},
	}
	for _, test := range cases {
		for _, restore := range []bool{false, true} {
			name := test.name + "/discard"
			if restore {
				name = test.name + "/restore"
			}
			t.Run(name, func(t *testing.T) {
				program, err := CompileAnonymous(test.source)
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				previous := Value{Kind: ValueString, Text: "preserved", Static: "String"}
				if restore {
					machine.Globals["minute"] = previous
					machine.VarTypes["minute"] = "String"
				}
				_, err = machine.Execute(program)
				if (err != nil) != test.wantErr {
					t.Fatalf("error = %v, want error %v", err, test.wantErr)
				}
				value, hadValue := machine.Globals["minute"]
				typeName, hadType := machine.VarTypes["minute"]
				if restore {
					if !hadValue || value.Kind != ValueString || value.Text != previous.Text || !hadType || typeName != "String" {
						t.Fatalf("old binding not restored: value=%#v, type=%q", value, typeName)
					}
				} else if hadValue || hadType {
					t.Fatalf("loop binding leaked: value=%#v, type=%q", value, typeName)
				}
				if _, ok := machine.Globals["other"]; ok {
					t.Fatal("second initializer value leaked")
				}
				if _, ok := machine.VarTypes["other"]; ok {
					t.Fatal("second initializer type leaked")
				}
			})
		}
	}
}

func TestClassicForAssignmentInitializerPreservesOuterMutations(t *testing.T) {
	program, err := CompileAnonymous(`
Integer counter = -1;
Integer total = 0;
for (counter = 0; counter < 3; counter++) { total += counter; }
System.assertEquals(3, counter);
System.assertEquals(3, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
