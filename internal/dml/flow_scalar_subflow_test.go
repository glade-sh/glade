package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowSubflowScalarFormulaTriggerFieldUpdate(t *testing.T) {
	// The project capture observes these values after a subflow returns its scalar.
	for _, tc := range []struct {
		name, formula, want string
		value               storage.Value
	}{
		{"boolean", `IF(Result, "true", "false")`, "true", storage.BooleanValue(true)},
		{"fraction", "TEXT(Result)", "-2.5", storage.DecimalValue("-2.5")},
		{"zero_boundary", "TEXT(Result)", "0", storage.IntegerValue(0)},
	} {
		t.Run(tc.name+"/trigger_subflow_field_update", func(t *testing.T) {
			org := testOrg()
			object := org.Objects["Account"]
			object.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
			org.Objects["Account"] = object
			engine := NewEngine(&org)
			frame := newFlowFrame()
			frame.scalars[flowFrameKey("Result")] = tc.value
			record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Cedar")}, ExplicitNulls: map[string]bool{}}
			step := storage.FlowStep{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Name: "WriteResult", Field: "Description", Formula: tc.formula}}}
			changed, err := engine.applyFlowStep("CedarParent", "Account", &record, object.Definition, step, frame)
			if err != nil || !changed || !storage.ValuesEqual(record.Fields["Description"], storage.StringValue(tc.want)) {
				t.Fatalf("native description=%q; Glade=%+v changed=%v err=%v", tc.want, record.Fields["Description"], changed, err)
			}
		})
	}
}
