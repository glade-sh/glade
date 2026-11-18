package vm

import (
	"errors"
	"testing"
)

// Native DataWeave observations: dataweave_getter_routes/org.tsv:2-21.
// This tests the VM's typed input conversion; it does not run a DataWeave script.
func TestDataWeaveGetterNativeInputConversion(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, tc := range []struct {
				name, getter, want string
				visits             int64
				recursive          bool
			}{
				{"direct", `visits++; return 'direct';`, "direct", 1, false},
				{"finite", `visits++; if (visits == 1) return helper(); return 'finite';`, "finite", 2, false},
				{"recursive", `visits++; return helper();`, "", 0, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					machine := New(nil)
					compile := func(source string) Method {
						t.Helper()
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						return Method{ClassName: "DataWeaveGetterProbe", APIVersion: api, Program: program}
					}
					getter := compile(tc.getter)
					getter.Name, getter.ReturnType = "DataWeaveGetterProbe.value.get", "String"
					helper := compile(`return value;`)
					helper.Name, helper.ReturnType, helper.Access = "DataWeaveGetterProbe.helper", "String", "public"
					if err := machine.RegisterClass(Class{
						Name: "DataWeaveGetterProbe", APIVersion: api,
						Fields: map[string]Field{
							"visits": {Name: "visits", Type: "Integer", Access: "public"},
							"value":  {Name: "value", Type: "String", Access: "public", Property: true, HasGetter: true, Getter: &getter},
						},
						Methods: map[string]Method{"helper": helper},
					}); err != nil {
						t.Fatal(err)
					}
					receiver := Object("DataWeaveGetterProbe")
					receiver.Static = "DataWeaveGetterProbe"
					receiver.classInstance, receiver.projectClass = true, true
					receiver.Fields["visits"] = Int(0)
					receiver.Fields["value"] = Null
					got, err := machine.dataWeaveInputValue(receiver, make(map[uint64]bool), 0)
					if tc.recursive {
						var limit *RuntimeError
						if !errors.As(err, &limit) || limit.Type != "System.LimitException" || limit.Message != "Maximum stack depth reached: 1001" {
							t.Fatalf("conversion error=%v, want captured stack limit", err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if got.Kind != "object" {
						t.Fatalf("converted kind=%s, want object", got.Kind)
					}
					found := false
					for _, field := range got.Fields {
						if field.Name == "value" {
							found = true
							if field.Value.Kind != "string" || field.Value.Text != tc.want {
								t.Errorf("converted value=%+v, want %q", field.Value, tc.want)
							}
						}
					}
					if !found {
						t.Fatal("converted object is missing value")
					}
					if visits := receiver.Fields["visits"]; visits.Kind != ValueInt || visits.Int != tc.visits {
						t.Fatalf("getter visits=%+v, want %d", visits, tc.visits)
					}
				})
			}
		})
	}
}
