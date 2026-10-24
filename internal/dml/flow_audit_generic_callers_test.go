package dml

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native rows (internal/apextest/testdata/conformance/flow_decision_shapes/
// org.tsv PARENT rows, API 62.0 and 67.0, anonymous and @IsTest) observe a
// system-only $Record.LastModifiedDate solely as a parent Update Records
// assignment value. Every other frame-reference reader has no native
// observation, so a source record whose LastModifiedDate exists only as a
// system timestamp must resolve exactly as it did before the audit fallback:
// the same result as an explicitly null LastModifiedDate, never the stamp.
const flowAuditGenericStamp = "2026-10-19T01:02:03.000Z"

type flowAuditGenericFixture struct {
	engine     *Engine
	org        *storage.OrgState
	source     storage.Record
	definition storage.ObjectDefinition
	account    storage.ObjectDefinition
	frame      *flowFrame
}

func newFlowAuditGenericFixture(api string, invocable, explicitNull bool) flowAuditGenericFixture {
	org := testOrg()
	org.APIVersion = api
	account := org.Objects["Account"]
	account.Definition.Fields["Observed__c"] = storage.Field{APIName: "Observed__c", Type: storage.FieldDateTime}
	account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
	org.Objects["Account"] = account
	engine := NewEngine(&org)
	engine.InvocableFlow = invocable
	definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"LastModifiedDate": {APIName: "LastModifiedDate", Type: storage.FieldDateTime}}}
	source := storage.Record{Object: "Contact", System: storage.SystemFields{LastModifiedDate: flowAuditGenericStamp}}
	if explicitNull {
		source.ExplicitNulls = map[string]bool{"LastModifiedDate": true}
	}
	frame := newFlowFrame()
	frame.saveDecisions = !invocable
	frame.autolaunched = invocable
	seedFlowFrame(&engine, &source, frame)
	frame.records[flowFrameKey("rec")] = source.Clone()
	return flowAuditGenericFixture{engine: &engine, org: &org, source: source, definition: definition, account: org.Objects["Account"].Definition, frame: frame}
}

func TestFlowAuditSystemTimestampGenericFrameCallers(t *testing.T) {
	type observe func(t *testing.T, f flowAuditGenericFixture, reference string) storage.Value
	assign := func(t *testing.T, f flowAuditGenericFixture, reference string) storage.Value {
		if err := f.engine.applyFlowAssignmentStep(storage.FlowAssignment{Name: "Observe", Target: "observed", Operator: "Assign", SourceField: reference}, f.frame); err != nil {
			t.Fatal(err)
		}
		value, ok := f.frame.scalars[flowFrameKey("observed")]
		if !ok {
			t.Fatal("assignment stored no scalar")
		}
		return value
	}
	for _, caller := range []struct {
		name      string
		invocable bool
		run       observe
	}{
		{name: "assignment/flowFrameValue", run: assign},
		{name: "assignment/flowFrameValue/invocable", invocable: true, run: assign},
		{name: "assignment_then_subflow_input", run: func(t *testing.T, f flowAuditGenericFixture, reference string) storage.Value {
			assign(t, f, reference)
			child := storage.FlowRule{Name: "AuditLeaf", Active: true, Variables: []storage.FlowVariable{{Name: "stamp", DataType: "DateTime", IsInput: true, IsOutput: true}}}
			if err := f.engine.runSubflow(child, f.definition, storage.FlowSubflow{
				Name: "CallChild", FlowName: child.Name,
				InputAssignments:  []storage.WorkflowFieldUpdate{{Field: "stamp", SourceField: "observed"}},
				OutputAssignments: []storage.FlowAssignment{{Target: "returned", SourceField: "stamp"}},
			}, f.source, f.definition, f.frame); err != nil {
				t.Fatal(err)
			}
			return f.frame.scalars[flowFrameKey("returned")]
		}},
		{name: "delete_records_criteria", run: func(t *testing.T, f flowAuditGenericFixture, reference string) storage.Value {
			candidate := storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Observed__c": storage.DateTimeValue(flowAuditGenericStamp)}}
			matches, ok := f.engine.evaluateFlowRecordDeleteCriteria(storage.FlowRecordDelete{Name: "Del", ObjectName: "Account", Criteria: []storage.WorkflowCriteriaItem{{Field: "Observed__c", Operation: "equals", SourceField: reference}}}, candidate, f.account, f.definition, f.source, f.frame)
			return storage.StringValue(fmt.Sprintf("matches=%t supported=%t", matches, ok))
		}},
		{name: "create_records_input", run: func(t *testing.T, f flowAuditGenericFixture, reference string) storage.Value {
			id, err := f.engine.flowCoreRecordCreateWithFrame(storage.FlowRecordCreate{Name: "Make", ObjectName: "Account", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Name", LiteralValue: "Probe"}, {Field: "Description", SourceField: reference}}}, f.source, f.definition, f.frame)
			if err != nil {
				t.Fatal(err)
			}
			value, ok := f.org.Objects["Account"].Records[id].Fields["Description"]
			if !ok {
				return storage.NullValue()
			}
			return value
		}},
		{name: "invocable_decision_left_operand", invocable: true, run: func(t *testing.T, f flowAuditGenericFixture, reference string) storage.Value {
			matches, supported, handled := f.engine.invocableFlowRecordCriterion(storage.WorkflowCriteriaItem{Field: reference, Operation: "equals", Value: "'" + flowAuditGenericStamp + "'"}, f.frame)
			return storage.StringValue(fmt.Sprintf("matches=%t supported=%t handled=%t", matches, supported, handled))
		}},
		{name: "invocable_decision_right_operand", invocable: true, run: func(t *testing.T, f flowAuditGenericFixture, reference string) storage.Value {
			f.frame.records[flowFrameKey("other")] = storage.Record{Object: "Contact", Fields: map[string]storage.Value{"Stamp": storage.StringValue(flowAuditGenericStamp)}}
			matches, supported, handled := f.engine.invocableFlowRecordCriterion(storage.WorkflowCriteriaItem{Field: "other.Stamp", Operation: "equals", Value: "{!" + reference + "}"}, f.frame)
			return storage.StringValue(fmt.Sprintf("matches=%t supported=%t handled=%t", matches, supported, handled))
		}},
		{name: "scalar_decision_right_operand", run: func(t *testing.T, f flowAuditGenericFixture, reference string) storage.Value {
			f.frame.variables[flowFrameKey("declared")] = storage.FlowVariable{Name: "declared", DataType: "DateTime"}
			matches, supported, handled := f.engine.flowVariableOperandCriterion(storage.WorkflowCriteriaItem{Field: "declared", Operation: "equals", SourceField: reference}, f.frame)
			return storage.StringValue(fmt.Sprintf("matches=%t supported=%t handled=%t", matches, supported, handled))
		}},
		{name: "invocable_formula", invocable: true, run: func(t *testing.T, f flowAuditGenericFixture, reference string) storage.Value {
			value, ok := f.engine.flowContextFormulaValue("BLANKVALUE("+reference+", \"blank\")", f.frame)
			if !ok {
				t.Fatal("formula unsupported")
			}
			return value
		}},
		{name: "text_template", run: func(t *testing.T, f flowAuditGenericFixture, reference string) storage.Value {
			f.frame.textTemplates[flowFrameKey("tpl")] = "at {!" + reference + "}"
			value, ok := f.engine.flowFrameReferenceValue("tpl", f.frame)
			if !ok {
				t.Fatal("template unresolved")
			}
			return value
		}},
		{name: "action_context_text_template", invocable: true, run: func(t *testing.T, f flowAuditGenericFixture, reference string) storage.Value {
			f.frame.textTemplates[flowFrameKey("tpl")] = "at {!" + reference + "}"
			return f.engine.flowActionContextFromFrame(f.frame).Scalars[flowFrameKey("tpl")]
		}},
	} {
		for _, api := range []string{"62.0", "67.0"} {
			for _, reference := range []string{"$Record.LastModifiedDate", "rec.LastModifiedDate"} {
				t.Run(fmt.Sprintf("%s/%s/%s", caller.name, api, reference), func(t *testing.T) {
					got := caller.run(t, newFlowAuditGenericFixture(api, caller.invocable, false), reference)
					base := caller.run(t, newFlowAuditGenericFixture(api, caller.invocable, true), reference)
					if strings.Contains(workflowValueString(got), "2026-10-19") {
						t.Fatalf("system-only LastModifiedDate leaked: %#v", got)
					}
					if !storage.ValuesEqual(got, base) || got.Kind != base.Kind {
						t.Fatalf("system-only=%#v; explicit-null control=%#v", got, base)
					}
				})
			}
		}
	}
}

// Positive control: the parent Update Records assignment from
// $Record.LastModifiedDate keeps the system timestamp (native PARENT:Open
// dt=set at API 62.0 and 67.0). A named record variable has no native row, so
// it keeps the earlier null.
func TestFlowAuditSystemTimestampParentAssignmentScope(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, control := range []struct {
			reference string
			want      storage.Value
		}{
			{"$Record.LastModifiedDate", storage.DateTimeValue(flowAuditGenericStamp)},
			{"LastModifiedDate", storage.DateTimeValue(flowAuditGenericStamp)},
			{"rec.LastModifiedDate", storage.NullValue()},
			{"$Record__Prior.LastModifiedDate", storage.NullValue()},
		} {
			t.Run(api+"/"+control.reference, func(t *testing.T) {
				f := newFlowAuditGenericFixture(api, false, false)
				f.frame.records[flowFrameKey("$Record__Prior")] = storage.Record{Object: "Contact", System: storage.SystemFields{LastModifiedDate: flowAuditGenericStamp}}
				value, null, ok := f.engine.flowRecordUpdateAssignmentValue(storage.Field{Type: storage.FieldDateTime}, f.source, storage.WorkflowFieldUpdate{SourceField: control.reference}, f.definition, f.frame, true)
				if !ok || null != (control.want.Kind == storage.ValueNull) || !storage.ValuesEqual(value, control.want) {
					t.Fatalf("value=%#v null=%t supported=%t; want %#v", value, null, ok, control.want)
				}
			})
		}
	}
}
