package dml

import (
	"html"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func flowCoreSeedScalarDefaults(variables []storage.FlowVariable, frame *flowFrame) {
	for _, variable := range variables {
		key := flowFrameKey(variable.Name)
		frame.variables[key] = variable
		if flowCorePrimitiveCollection(variable) {
			frame.scalars[key] = storage.ListValue()
		}
		if !variable.IsCollection && variable.DefaultValue != nil {
			frame.scalars[key] = variable.DefaultValue.Clone()
		}
	}
}

// Only formulas that reference an interview scalar need the frame evaluator.
// Ordinary literals and triggering-record expressions retain their old path.
func flowCoreFormulaUsesScalars(expression string, frame *flowFrame, captured bool) bool {
	for _, token := range tokenizeFormula(html.UnescapeString(expression)) {
		if token.typ == formulaTokenIdent {
			if _, ok := frame.scalars[flowFrameKey(token.text)]; ok {
				return true
			}
			if captured && frame.capturedFormulaBindings {
				reference := strings.TrimPrefix(strings.ToLower(token.text), "$record.")
				variable, declared := frame.variables[flowFrameKey(reference)]
				if declared && !variable.IsCollection && strings.EqualFold(variable.DataType, "String") {
					return true
				}
			}
		}
	}
	return false
}

func (e *Engine) flowCoreRecordCreateWithFrame(create storage.FlowRecordCreate, source storage.Record, definition storage.ObjectDefinition, frame *flowFrame) (storage.ID, error) {
	assignments := append([]storage.WorkflowFieldUpdate(nil), create.InputAssignments...)
	kept := assignments[:0]
	for _, assignment := range assignments {
		if e.flowCreateUnsetSObjectReference(create, assignment, frame) {
			continue
		}
		kept = append(kept, assignment)
	}
	assignments = kept
	var nullFields map[string]bool
	for i := range assignments {
		assignment := &assignments[i]
		var value storage.Value
		resolved := false
		if reference, ok := e.flowFrameReferenceValue(assignment.SourceField, frame); assignment.SourceField != "" && ok {
			value, resolved = reference, true
		} else if flowCoreFormulaUsesScalars(assignment.Formula, frame, assignment.FlowFrameFormula) || e.InvocableFlow && assignment.Formula != "" {
			var ok bool
			if e.InvocableFlow {
				value, ok = e.flowContextFormulaValue(assignment.Formula, frame)
			} else {
				value, ok = e.flowFrameFormulaValue(assignment.Formula, frame, assignment.FlowFrameFormula)
			}
			if !ok {
				return "", dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{assignment.Field}, "dml: flow record create %s has unsupported scalar expression", create.Name)
			}
			resolved = true
		}
		if resolved {
			// An unset record variable field is null, not an empty string
			// (flow_create_unset_variable_reference/org.tsv: a null Pricebook2Id
			// reports REQUIRED_FIELD_MISSING, API 62.0 and 67.0).
			if value.Kind == storage.ValueNull || value.Kind == storage.ValueID && value.ID == "" {
				if nullFields == nil {
					nullFields = make(map[string]bool)
				}
				nullFields[strings.ToLower(assignment.Field)] = true
			}
			assignment.SourceField, assignment.Formula = "", ""
			assignment.LiteralValue = workflowValueString(value)
			if value.Kind == storage.ValueDecimal {
				assignment.LiteralValue = value.Decimal
			}
		}
	}
	create.InputAssignments = assignments
	return e.executeFlowRecordCreate(create, source, definition, frame.lookupOutputs, nullFields)
}

// flowCreateUnsetSObjectReference reports a Create Records lookup-field input
// read from a field of a declared SObject variable that the interview never
// assigned. The value is null, so the field is left unset (lookup fields have
// no default value): flow_criteria_shapes/org.tsv pb_test rows, 62.0 and 67.0,
// where Salesforce reports REQUIRED_FIELD_MISSING for the create.
func (e *Engine) flowCreateUnsetSObjectReference(create storage.FlowRecordCreate, assignment storage.WorkflowFieldUpdate, frame *flowFrame) bool {
	if frame == nil || e.Org == nil || strings.TrimSpace(assignment.Formula) != "" || assignment.LiteralValue != "" {
		return false
	}
	variable, field, nested := strings.Cut(strings.TrimSpace(assignment.SourceField), ".")
	if !nested || strings.Contains(field, ".") || !flowFrameSObjectVariable(frame, flowFrameKey(variable)) {
		return false
	}
	key := flowFrameKey(variable)
	if _, assigned := frame.records[key]; assigned {
		return false
	}
	if _, scalar := frame.scalars[key]; scalar {
		return false
	}
	if _, lookup := frame.lookupOutputs[key]; lookup {
		return false
	}
	target, _, err := e.object(create.ObjectName)
	if err != nil {
		return false
	}
	name, ok := storage.ResolveFieldName(target.Definition, e.Org.Namespace, assignment.Field)
	return ok && target.Definition.Fields[name].Type == storage.FieldReference
}
