package vm

import "testing"

// Native Lightning Out observations: aura_overload_ties/org.tsv:2-105.
func TestHistoricalLightningEqualArityNative(t *testing.T) {
	for _, api := range []string{"45.0", "54.0"} {
		t.Run(api, func(t *testing.T) {
			for _, order := range []string{"forward", "reverse"} {
				t.Run(order, func(t *testing.T) {
					for _, shape := range []struct {
						name, textParam, integerParam string
					}{
						{"same", "value", "value"},
						{"cross", "alpha", "beta"},
						{"swapped", "beta", "alpha"},
					} {
						t.Run(shape.name, func(t *testing.T) {
							text, err := CompileAnonymous(`return 'string:' + String.valueOf(` + shape.textParam + `);`)
							if err != nil {
								t.Fatal(err)
							}
							integer, err := CompileAnonymous(`return 'integer:' + String.valueOf(` + shape.integerParam + `);`)
							if err != nil {
								t.Fatal(err)
							}
							methods := []Method{
								{Name: "LegacyTieController.load", ClassName: "LegacyTieController", ReturnType: "String", IsStatic: true, APIVersion: api, Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: shape.textParam, Type: "String"}}, Program: text},
								{Name: "LegacyTieController.load", ClassName: "LegacyTieController", ReturnType: "String", IsStatic: true, APIVersion: api, Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: shape.integerParam, Type: "Integer"}}, Program: integer},
							}
							if order == "reverse" {
								methods[0], methods[1] = methods[1], methods[0]
							}
							machine := New(nil)
							for _, method := range methods {
								if err := machine.RegisterMethod(method); err != nil {
									t.Fatal(err)
								}
							}
							for _, tc := range []struct {
								name string
								args map[string]any
								want string
							}{
								{"missing", map[string]any{}, "string:null"},
								{"null", map[string]any{"value": nil, "alpha": nil, "beta": nil}, "string:null"},
								{"numeric", map[string]any{"value": float64(42), "alpha": float64(42), "beta": float64(42)}, "string:42"},
								{"string", map[string]any{"value": "bad", "alpha": "bad", "beta": "bad"}, "string:bad"},
							} {
								t.Run(tc.name, func(t *testing.T) {
									got, err := machine.InvokeLWCMethod("LegacyTieController", "load", tc.args)
									if err != nil || !got.Success || got.ReturnValue != tc.want {
										t.Fatalf("result=%#v error=%v want=%s", got, err, tc.want)
									}
								})
							}
						})
					}
				})
			}
		})
	}
}
