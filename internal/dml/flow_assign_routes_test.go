package dml

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Org capture flow_assign_routes (API 62.0 and 67.0) unit boundaries.

func TestFlowCustomErrorRendersResourceFormulas(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	def := storage.ObjectDefinition{APIName: "Account", FlowRules: []storage.FlowRule{{Name: "Probe", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}}}
	rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Abc")}}
	frame := engine.newFlowStepFrame("Probe", &rec, def, nil, nil, nil)
	got := engine.renderFlowCustomErrorMessage("Bad {!FErr} [{!$Record.Name}]", frame, nil, nil, map[string]string{"ferr": "UPPER(Name) & '-' & TEXT(LEN(Name))"})
	if got != "Bad ABC-3 [Abc]" {
		t.Fatalf("message=%q", got)
	}
	got = engine.renderFlowCustomErrorMessage("E {!FErrBr} L{!FErrLen}", frame, nil, nil, map[string]string{"ferrbr": "'x' & BR() & 'y'", "ferrlen": "TEXT(LEN('x' & BR() & 'y'))"})
	if got != "E x\ny L14" {
		t.Fatalf("message=%q", got)
	}
}

func TestFlowCustomErrorFieldSelection(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	def := storage.ObjectDefinition{APIName: "Account"}
	rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{}}
	frame := engine.newFlowStepFrame("Probe", &rec, def, nil, nil, nil)
	step := storage.FlowStep{Kind: "customError", CustomError: storage.FlowCustomError{Name: "E", Messages: []storage.FlowCustomErrorMessage{{Message: "m", Field: "Fw1__c"}}}}
	_, err := engine.applyFlowStep("Probe", "Account", &rec, def, step, frame)
	var failure *flowCustomErrorFailure
	if !errors.As(err, &failure) {
		t.Fatalf("err=%v", err)
	}
	result, _ := FlowCustomErrorFailureResult(err)
	if len(result.Fields) != 1 || result.Fields[0] != "Fw1__c" {
		t.Fatalf("result=%+v", result)
	}
}

func TestFlowCriteriaReadAbsentFormulaFields(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	def := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
		"Name":    {APIName: "Name", Type: storage.FieldString},
		"Calc__c": {APIName: "Calc__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "'a' & BR() & 'b'"},
	}}
	rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("n")}}
	got := engine.flowRecordWithCriteriaFormulas(rec, def, []storage.WorkflowCriteriaItem{{Field: "Calc__c", Operation: "EqualTo"}})
	if value := got.Fields["Calc__c"]; value.String != "a\nb" {
		t.Fatalf("Calc__c=%#v", value)
	}
	if _, present := rec.Fields["Calc__c"]; present {
		t.Fatal("source record mutated")
	}
}

func TestFlowFormulaTextNullStaysText(t *testing.T) {
	field := storage.Field{APIName: "Cw4__c", Type: storage.FieldString}
	value, isNull, ok := materializedFormulaFieldValue(field, formulaValue{kind: formulaString, text: "null"})
	if !ok || isNull || value.String != "null" {
		t.Fatalf("value=%#v null=%v ok=%v", value, isNull, ok)
	}
}

func TestFormulaMissingRelationshipEvaluatesConstantParentFormula(t *testing.T) {
	org := testOrg()
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
		"Calc__c": {APIName: "Calc__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "'a' & BR() & 'b'"},
		"Name":    {APIName: "Name", Type: storage.FieldString},
		"Ref__c":  {APIName: "Ref__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "Name & 'x'"},
	}}}
	lookup := storage.Field{APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"}
	if got := formulaMissingRelationshipValue(org, lookup, []string{"Calc__c"}, ""); got.kind != formulaString || got.text != "a"+formulaBRText+"b" {
		t.Fatalf("constant=%#v", got)
	}
	// A parent formula that reads a field keeps the uncaptured null.
	if got := formulaMissingRelationshipValue(org, lookup, []string{"Ref__c"}, ""); got.kind != formulaNull && got.text != "" {
		t.Fatalf("field formula=%#v", got)
	}
}

func TestBeforeSaveTextCollectionLoopIsSafe(t *testing.T) {
	rule := storage.FlowRule{Variables: []storage.FlowVariable{
		{Name: "vColl", DataType: "String", IsCollection: true},
		{Name: "vItem", DataType: "String"},
		{Name: "vAcc", DataType: "String"},
	}}
	body := []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "vAcc", Operator: "Add", SourceField: "vItem"}}}
	steps := []storage.FlowStep{
		{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "vColl", Operator: "Add", LiteralValue: "p"}},
		{Kind: "loop", Loop: storage.FlowLoop{Name: "Lp", CollectionReference: "vColl", CurrentItemReference: "vItem", Steps: body}},
	}
	if !flowBeforeSaveAssignedRecordLoopsSafe(rule, steps) {
		t.Fatal("captured Text collection loop rejected")
	}
	other := []storage.FlowStep{steps[0], {Kind: "loop", Loop: storage.FlowLoop{Name: "Lp", CollectionReference: "vColl", CurrentItemReference: "vItem", Steps: []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "vAcc", Operator: "Assign", SourceField: "vItem"}}}}}}
	if flowBeforeSaveAssignedRecordLoopsSafe(rule, other) {
		t.Fatal("uncaptured loop body admitted")
	}
}
