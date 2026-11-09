package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowScalarFormulaPermissionPrincipal(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, granted := range []bool{false, true} {
			for _, captured := range []bool{false, true} {
				org := testOrg()
				org.APIVersion = api
				grants := permissionRegexOrg(granted, "Bypass")
				for _, name := range []string{"User", "PermissionSet", "PermissionSetAssignment", "SetupEntityAccess", "CustomPermission"} {
					org.Objects[name] = grants.Objects[name]
				}
				engine := NewEngine(&org)
				engine.UserID = permTestUser
				frame := newFlowFrame()
				frame.scalars[flowFrameKey("Suffix")] = storage.StringValue("yes")
				frame.capturedFormulaBindings = captured
				formula := `IF($Permission.Bypass, Suffix, "no")`
				want := "no"
				if granted {
					want = "yes"
				}
				update := storage.WorkflowFieldUpdate{Formula: formula, FlowFrameFormula: captured}
				value, ok := engine.flowTextScalarFieldUpdateValue(update, storage.Field{Type: storage.FieldString}, org.Objects["Account"].Definition, frame)
				if !ok || value.String != want {
					t.Errorf("api=%s granted=%t captured=%t update=%+v ok=%t want=%s", api, granted, captured, value, ok, want)
				}
				create := storage.FlowRecordCreate{Name: "Probe", ObjectName: "Account", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Name", Formula: formula, FlowFrameFormula: captured}}}
				id, err := engine.flowCoreRecordCreateWithFrame(create, storage.Record{Object: "Account"}, org.Objects["Account"].Definition, frame)
				if err != nil {
					t.Fatal(err)
				}
				if got := org.Objects["Account"].Records[id].Fields["Name"].String; got != want {
					t.Errorf("api=%s granted=%t captured=%t create=%q want=%q", api, granted, captured, got, want)
				}
			}
		}
	}
}
