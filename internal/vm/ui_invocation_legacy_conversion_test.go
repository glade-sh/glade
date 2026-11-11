package vm

import "testing"

func TestHistoricalLightningSelectedOverloadConversion(t *testing.T) {
	short, err := CompileAnonymous(`return 'short';`)
	if err != nil {
		t.Fatal(err)
	}
	long, err := CompileAnonymous(`return 'long:' + String.valueOf(count);`)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"45.0", "54.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			for _, method := range []Method{
				{Name: "LegacyController.load", ClassName: "LegacyController", ReturnType: "String", IsStatic: true, APIVersion: api, Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: "name", Type: "String"}}, Program: short},
				{Name: "LegacyController.load", ClassName: "LegacyController", ReturnType: "String", IsStatic: true, APIVersion: api, Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: "name", Type: "String"}, {Name: "count", Type: "Integer"}}, Program: long},
			} {
				if err := machine.RegisterMethod(method); err != nil {
					t.Fatal(err)
				}
			}
			for _, tc := range []struct {
				name    string
				count   any
				want    string
				invalid bool
			}{
				{name: "valid", count: float64(7), want: "long:7"},
				{name: "null", count: nil, want: "long:null"},
				{name: "malformed", count: "bad", invalid: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					result, err := machine.InvokeLWCMethod("LegacyController", "load", map[string]any{"name": "A", "count": tc.count})
					if err != nil {
						t.Fatal(err)
					}
					if tc.invalid {
						if result.Success || result.Error == nil || result.Error.Type != "InvalidActionParameter" {
							t.Fatalf("expected invalid action parameter, got %#v", result)
						}
					} else if !result.Success || result.ReturnValue != tc.want {
						t.Fatalf("result=%#v want=%s", result, tc.want)
					}
				})
			}
		})
	}
}
