package automation

import "strings"

func flowParentTextAssignmentSource(_ flowXML, item flowAssignmentItemXML, variables map[string]flowVariableXML, fallback string) string {
	operator := strings.TrimSpace(item.Operator)
	if !strings.EqualFold(operator, "Assign") && !strings.EqualFold(operator, "Add") {
		return fallback
	}
	variable, ok := variables[strings.ToLower(strings.TrimSpace(item.AssignToReference))]
	if !ok || (variable.IsCollection && !strings.EqualFold(operator, "Add")) || !strings.EqualFold(strings.TrimSpace(variable.DataType), "String") {
		return fallback
	}
	reference := strings.TrimSpace(item.Value.ElementReference)
	parts := strings.Split(reference, ".")
	if (len(parts) != 2 && len(parts) != 3) || !strings.EqualFold(parts[0], "$Record") {
		return fallback
	}
	for _, part := range parts[1:] {
		if strings.TrimSpace(part) == "" {
			return fallback
		}
	}
	// String assignments need the triggering record's identity in the frame.
	// The runtime validates the parent path and terminal Text field.
	return reference
}
