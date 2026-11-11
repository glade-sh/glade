package vm

import "testing"

// The native declaration boundary rejects overloads at API 55 and 56. Their
// runtime behavior cannot be observed; the last two cases retain the local
// unsupported boundary when such methods are registered directly.
func TestHistoricalLightningOverloadBoundary(t *testing.T) {
	one, err := CompileAnonymous(`return 'one:' + String.valueOf(first);`)
	if err != nil {
		t.Fatal(err)
	}
	two, err := CompileAnonymous(`return 'two:' + String.valueOf(first) + ':' + String.valueOf(second);`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		api    string
		accept bool
	}{
		{"45.0", true}, {"53.0", true}, {"54.0", true},
		{"55.0", false}, {"56.0", false},
	} {
		t.Run(tc.api, func(t *testing.T) {
			m := New(nil)
			for _, method := range []Method{
				{Name: "BoundaryAction.load", ClassName: "BoundaryAction", ReturnType: "String", IsStatic: true, APIVersion: tc.api, Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: "first", Type: "String"}}, Program: one},
				{Name: "BoundaryAction.load", ClassName: "BoundaryAction", ReturnType: "String", IsStatic: true, APIVersion: tc.api, Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: "first", Type: "String"}, {Name: "second", Type: "String"}}, Program: two},
			} {
				if err := m.RegisterMethod(method); err != nil {
					t.Fatal(err)
				}
			}
			for _, v := range []struct {
				name string
				args map[string]any
				want string
			}{
				{"empty", map[string]any{}, "two:null:null"},
				{"one", map[string]any{"first": "A"}, "two:A:null"},
				{"both", map[string]any{"first": "A", "second": "B"}, "two:A:B"},
				{"extra", map[string]any{"second": "B", "extra": "C"}, "two:null:B"},
			} {
				t.Run(v.name, func(t *testing.T) {
					got, err := m.InvokeLWCMethod("BoundaryAction", "load", v.args)
					if err != nil {
						t.Fatal(err)
					}
					if tc.accept {
						if !got.Success || got.ReturnValue != v.want {
							t.Fatalf("result=%#v want=%s", got, v.want)
						}
					} else if got.Success || got.Error == nil || got.Error.Code != "GLADELWC013" {
						t.Fatalf("expected unsupported overload, result=%#v", got)
					}
				})
			}
		})
	}
}
