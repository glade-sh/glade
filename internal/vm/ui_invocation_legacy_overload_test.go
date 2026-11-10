package vm

import "testing"

// Native Lightning Out arity rows use API 45 Apex and component APIs 59/67.
// The highest-arity declaration is unique in these cases. Equal-arity
// overload selection has no general rule and remains an explicit boundary.
func TestLightningHistoricalUniqueArityNative(t *testing.T) {
	programs := make([]Method, 0, 4)
	for _, tc := range []struct {
		name   string
		params []Param
		source string
	}{
		{"zero", nil, `return 'zero';`},
		{"one", []Param{{Name: "first", Type: "String"}}, `return 'one';`},
		{"two", []Param{{Name: "first", Type: "String"}, {Name: "second", Type: "String"}}, `return 'two';`},
		{"three", []Param{{Name: "first", Type: "String"}, {Name: "second", Type: "String"}, {Name: "third", Type: "String"}}, `return 'three:' + String.valueOf(first) + ':' + String.valueOf(second) + ':' + String.valueOf(third);`},
	} {
		p, err := CompileAnonymous(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		programs = append(programs, Method{Name: "NativeController.arity", ClassName: "NativeController", ReturnType: "String", IsStatic: true, APIVersion: "45.0", Modifiers: []string{"AuraEnabled"}, Params: tc.params, Program: p})
	}
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "forward", true: "reverse"}[reverse], func(t *testing.T) {
			m := New(nil)
			for i := range programs {
				j := i
				if reverse {
					j = len(programs) - 1 - i
				}
				if err := m.RegisterMethod(programs[j]); err != nil {
					t.Fatal(err)
				}
			}
			for _, tc := range []struct {
				name string
				args map[string]any
				want string
			}{
				{"empty", map[string]any{}, "three:null:null:null"},
				{"one", map[string]any{"first": "A"}, "three:A:null:null"},
				{"two", map[string]any{"first": "A", "second": "B"}, "three:A:B:null"},
				{"all", map[string]any{"first": "A", "second": "B", "third": "C"}, "three:A:B:C"},
				{"extra", map[string]any{"third": "C", "extra": "X"}, "three:null:null:C"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					got, err := m.InvokeLWCMethod("NativeController", "arity", tc.args)
					if err != nil || !got.Success || got.ReturnValue != tc.want {
						t.Fatalf("result=%#v error=%v want=%s", got, err, tc.want)
					}
				})
			}
		})
	}
}
