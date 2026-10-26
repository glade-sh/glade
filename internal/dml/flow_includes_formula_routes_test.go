package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The decision-shape INC rows capture case-insensitive multi-select membership.
// Exercise each parser entry point used by Flow, workflow, validation and fields.
func TestFlowIncludesFormulaRecordValueAndFrameRoutes(t *testing.T) {
	org := testOrg()
	definition := org.Objects["Account"].Definition
	record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("First;Second")}}
	for _, tc := range []struct {
		name, expression string
		want             bool
	}{
		{"match", `INCLUDES(Name, "first")`, true},
		{"missing", `INCLUDES(Name, "Third")`, false},
		{"null", `INCLUDES(NULL, "First")`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			boolRoutes := map[string]func() (bool, bool){
				"record": func() (bool, bool) { return evaluateRecordFormula(tc.expression, record) },
				"org":    func() (bool, bool) { return evaluateRecordFormulaInOrg(tc.expression, &org, definition, record) },
				"validation_workflow": func() (bool, bool) {
					return evaluateValidationFormulaInOrg(tc.expression, &org, definition, record, nil, false)
				},
			}
			for route, run := range boolRoutes {
				t.Run(route, func(t *testing.T) {
					got, ok := run()
					if !ok || got != tc.want {
						t.Fatalf("got=%t supported=%t; native=%t", got, ok, tc.want)
					}
				})
			}
			for _, route := range []string{"field_value", "org_field_value", "flow_frame"} {
				t.Run(route, func(t *testing.T) {
					var got storage.Value
					var ok bool
					switch route {
					case "field_value":
						got, _, ok = evaluateRecordFormulaValue(tc.expression, storage.Field{Type: storage.FieldBoolean}, record)
					case "org_field_value":
						got, _, ok = EvaluateRecordFormulaValueInOrg(tc.expression, storage.Field{Type: storage.FieldBoolean}, &org, definition, record)
					default:
						frame := newFlowFrame()
						frame.scalars["name"] = record.Fields["Name"]
						got, ok = flowFrameFormulaValue(tc.expression, frame)
					}
					if !ok || got.Kind != storage.ValueBoolean || got.Boolean != tc.want {
						t.Fatalf("got=%#v supported=%t; native=%t", got, ok, tc.want)
					}
				})
			}
		})
	}
}
