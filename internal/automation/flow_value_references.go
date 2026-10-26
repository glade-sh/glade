package automation

import (
	"fmt"
	"strings"
)

func flowRecordUpdateAssignmentSource(value flowValueXML, formula string, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) string {
	source := flowLookupSourceFieldValueUnlessFormula(value, formula, variables, lookups)
	if source != "" || strings.TrimSpace(formula) != "" {
		return source
	}
	reference := strings.TrimSpace(value.ElementReference)
	if variable, ok := variables[strings.ToLower(reference)]; ok && !variable.IsCollection {
		return reference
	}
	return ""
}

func flowInvalidValueReference(raw flowXML, variables map[string]flowVariableXML) string {
	invalidGlobal := func(value flowValueXML) string {
		reference := strings.TrimSpace(value.ElementReference)
		switch reference {
		case "$GlobalConstant.True", "$GlobalConstant.False", "$GlobalConstant.Null", "$GlobalConstant.EmptyString":
			return fmt.Sprintf("elementReference %q does not identify a Flow resource", reference)
		}
		return ""
	}
	for _, assignment := range raw.Assignments {
		for _, item := range assignment.Items {
			if problem := invalidGlobal(item.Value); problem != "" {
				return problem
			}
		}
	}
	for _, update := range raw.RecordUpdates {
		for _, item := range update.Fields {
			if problem := invalidGlobal(item.Value); problem != "" {
				return problem
			}
		}
		root, path, nested := strings.Cut(strings.TrimSpace(update.InputReference), ".")
		variable, declared := variables[strings.ToLower(root)]
		if raw.Start.Object == "" && nested && !strings.Contains(path, ".") && declared && strings.EqualFold(variable.DataType, "SObject") && len(update.Fields) > 0 {
			return fmt.Sprintf("record update %q cannot combine an input record relationship with input assignments", update.Name)
		}
	}
	return ""
}
