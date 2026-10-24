package dml

import "strings"

func flowFormulaUsesRecordVariables(expression string, frame *flowFrame) bool {
	if frame == nil {
		return false
	}
	for _, token := range tokenizeFormula(expression) {
		if token.typ != formulaTokenIdent {
			continue
		}
		root, _, dotted := strings.Cut(token.text, ".")
		if !dotted {
			continue
		}
		variable, declared := frame.variables[flowFrameKey(root)]
		if declared && !variable.IsCollection && strings.EqualFold(variable.DataType, "SObject") {
			return true
		}
	}
	return false
}
