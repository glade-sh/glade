package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func flowDeclaredNumericScalar(name string, frame *flowFrame) (storage.Value, bool) {
	if frame == nil {
		return storage.Value{}, false
	}
	variable := frame.variables[flowFrameKey(name)]
	if !strings.EqualFold(variable.DataType, "Number") && !strings.EqualFold(variable.DataType, "Currency") {
		return storage.Value{}, false
	}
	value, ok := flowDeclaredScalar(name, frame)
	return value, ok && (value.Kind == storage.ValueNull || value.Kind == storage.ValueInteger || value.Kind == storage.ValueDecimal)
}

// Numeric Flow Decisions order null below every number, including negative
// numbers; two nulls compare equal. This is distinct from formula null handling.
// Native: flow_null_comparisons N01-N12, N19-N30 and their Currency twins,
// at APIs 62.0 and 67.0, in autolaunched and record-triggered flows.
func flowNumericVariableOperandCriterion(item storage.WorkflowCriteriaItem, frame *flowFrame) (bool, bool, bool) {
	left, ok := flowDeclaredNumericScalar(item.Field, frame)
	if !ok {
		return false, false, false
	}
	var symbol string
	switch strings.ToLower(strings.TrimSpace(item.Operation)) {
	case "equals":
		symbol = "="
	case "notequal", "notequals":
		symbol = "!="
	case "greaterthan":
		symbol = ">"
	case "greaterthanorequalto":
		symbol = ">="
	case "lessthan":
		symbol = "<"
	case "lessthanorequalto":
		symbol = "<="
	default:
		return false, false, false
	}
	reference := strings.TrimSpace(item.SourceField)
	literal := strings.TrimSpace(item.Value)
	if reference == "" && strings.HasPrefix(literal, "{!") && strings.HasSuffix(literal, "}") {
		reference = strings.TrimSpace(literal[2 : len(literal)-1])
	}
	var right storage.Value
	if reference != "" {
		right, ok = flowDeclaredNumericScalar(reference, frame)
		if !ok {
			return false, false, false
		}
	} else {
		// Only numeric literals and declared numeric resources belong here.
		// Other operand shapes retain their existing decision readers.
		literal = trimFormulaLiteral(literal)
		if literal == "" {
			return false, false, false
		}
		if _, ok := formulaNumberValueFromText(literal); !ok {
			return false, false, false
		}
		right = storage.DecimalValue(literal)
	}
	// Non-null equality keeps the existing scalar evaluator, including its
	// representation and precision behavior. Only null equality is owned here.
	if left.Kind != storage.ValueNull && right.Kind != storage.ValueNull && (symbol == "=" || symbol == "!=") {
		return false, false, false
	}
	lhs, rhs := flowFormulaValue(left), flowFormulaValue(right)
	if left.Kind == storage.ValueNull || right.Kind == storage.ValueNull {
		order := 0.0
		if left.Kind != storage.ValueNull {
			order = 1
		} else if right.Kind != storage.ValueNull {
			order = -1
		}
		lhs, rhs = formulaNumberValue(order), formulaNumberValue(0)
	}
	return compareFormulaValues(lhs, rhs, symbol), true, true
}
