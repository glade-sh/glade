package dml

import (
	"html"

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
			frame.scalars[flowFrameKey(variable.Name)] = variable.DefaultValue.Clone()
		}
	}
}

// Only formulas that reference an interview scalar need the frame evaluator.
// Ordinary literals and triggering-record expressions retain their old path.
func flowCoreFormulaUsesScalars(expression string, frame *flowFrame) bool {
	for _, token := range tokenizeFormula(html.UnescapeString(expression)) {
		if token.typ == formulaTokenIdent {
			if _, ok := frame.scalars[flowFrameKey(token.text)]; ok {
				return true
			}
		}
	}
	return false
}

func (e *Engine) flowCoreRecordCreateWithFrame(create storage.FlowRecordCreate, source storage.Record, definition storage.ObjectDefinition, frame *flowFrame) (storage.ID, error) {
	assignments := append([]storage.WorkflowFieldUpdate(nil), create.InputAssignments...)
	for i := range assignments {
		assignment := &assignments[i]
		var value storage.Value
		resolved := false
		if reference, ok := e.flowFrameReferenceValue(assignment.SourceField, frame); assignment.SourceField != "" && ok {
			value, resolved = reference, true
		} else if flowCoreFormulaUsesScalars(assignment.Formula, frame) || e.InvocableFlow && assignment.Formula != "" {
			var ok bool
			if e.InvocableFlow {
				value, ok = e.flowContextFormulaValue(assignment.Formula, frame)
			} else {
				value, ok = flowFrameFormulaValue(assignment.Formula, frame)
			}
			if !ok {
				return "", dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{assignment.Field}, "dml: flow record create %s has unsupported scalar expression", create.Name)
			}
			resolved = true
		}
		if resolved {
			assignment.SourceField, assignment.Formula = "", ""
			assignment.LiteralValue = workflowValueString(value)
			if value.Kind == storage.ValueDecimal {
				assignment.LiteralValue = value.Decimal
			}
		}
	}
	create.InputAssignments = assignments
	return e.executeFlowRecordCreate(create, source, definition, frame.lookupOutputs)
}
