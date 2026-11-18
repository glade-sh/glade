package vm

import "testing"

func TestLightningHistoricalOverloadNative(t *testing.T) {
	one, err := CompileAnonymous(`return 'one';`)
	if err != nil {
		t.Fatal(err)
	}
	two, err := CompileAnonymous(`return 'two:' + String.valueOf(recordId) + ':' + String.valueOf(groupId);`)
	if err != nil {
		t.Fatal(err)
	}
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "forward", true: "reverse"}[reverse], func(t *testing.T) {
			m := New(nil)
			methods := []Method{
				{Name: "CaptureAura45.load", ClassName: "CaptureAura45", ReturnType: "String", IsStatic: true, APIVersion: "45.0", Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: "recordId", Type: "String"}}, Program: one},
				{Name: "CaptureAura45.load", ClassName: "CaptureAura45", ReturnType: "String", IsStatic: true, APIVersion: "45.0", Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: "recordId", Type: "String"}, {Name: "groupId", Type: "String"}}, Program: two},
			}
			if reverse {
				methods[0], methods[1] = methods[1], methods[0]
			}
			for _, method := range methods {
				if err := m.RegisterMethod(method); err != nil {
					t.Fatal(err)
				}
			}
			for _, tc := range []struct {
				name string
				args map[string]any
				want string
			}{
				{"empty", map[string]any{}, "two:null:null"},
				{"one", map[string]any{"recordId": "A"}, "two:A:null"},
				{"two", map[string]any{"recordId": "A", "groupId": "B"}, "two:A:B"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					result, err := m.InvokeLWCMethod("CaptureAura45", "load", tc.args)
					if err != nil || !result.Success || result.ReturnValue != tc.want {
						t.Fatalf("result=%#v error=%v want=%s", result, err, tc.want)
					}
				})
			}
		})
	}
}
