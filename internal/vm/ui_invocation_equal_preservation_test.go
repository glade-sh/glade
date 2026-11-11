package vm

import "testing"

// Equal-arity String/Integer overloads before API 55: the String overload is
// selected in every declaration order (native rows in
// internal/apextest/testdata/conformance/aura_overload_determinism_scalar/org.tsv,
// PR #209). This replaces the interim "unsupported" expectation from #175, which
// predated those rows.
func TestHistoricalLightningEqualArityPreservesMain(t *testing.T) {
	text, err := CompileAnonymous(`return 'text:' + String.valueOf(value);`)
	if err != nil {
		t.Fatal(err)
	}
	integer, err := CompileAnonymous(`return 'integer:' + String.valueOf(value);`)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"45.0", "54.0"} {
		t.Run(api, func(t *testing.T) {
			for _, reverse := range []bool{false, true} {
				order := "forward"
				if reverse {
					order = "reverse"
				}
				t.Run(order, func(t *testing.T) {
					methods := []Method{
						{Name: "LocalController.load", ClassName: "LocalController", ReturnType: "String", IsStatic: true, APIVersion: api, Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: "value", Type: "String"}}, Program: text},
						{Name: "LocalController.load", ClassName: "LocalController", ReturnType: "String", IsStatic: true, APIVersion: api, Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: "value", Type: "Integer"}}, Program: integer},
					}
					if reverse {
						methods[0], methods[1] = methods[1], methods[0]
					}
					machine := New(nil)
					for _, method := range methods {
						if err := machine.RegisterMethod(method); err != nil {
							t.Fatal(err)
						}
					}
					for _, tc := range []struct {
						name        string
						params      map[string]any
						want        string
						unsupported bool
					}{
						{"missing", map[string]any{}, "", true},
						{"null", map[string]any{"value": nil}, "", true},
						{"numeric", map[string]any{"value": float64(42)}, "", true},
						{"text", map[string]any{"value": "not-an-integer"}, "text:not-an-integer", false},
					} {
						t.Run(tc.name, func(t *testing.T) {
							got, err := machine.InvokeLWCMethod("LocalController", "load", tc.params)
							if err != nil {
								t.Fatal(err)
							}
							if tc.unsupported {
								// String overload selected: it either returns its text or rejects the
								// argument; it never falls back to the Integer overload or reports the
								// overload as unsupported.
								stringChosen := (got.Success && len(got.ReturnValue.(string)) >= 5 && got.ReturnValue.(string)[:5] == "text:") ||
									(got.Error != nil && got.Error.Code != "GLADELWC013")
								if !stringChosen {
									t.Fatalf("expected the String overload, got %#v", got)
								}
							} else if !got.Success || got.ReturnValue != tc.want {
								t.Fatalf("result=%#v want=%q", got, tc.want)
							}
						})
					}
				})
			}
		})
	}
}
