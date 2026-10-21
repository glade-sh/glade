package automation

import "strings"

// Boolean output assignments retain their declared type.
// Use the existing typed formula path; String literals and record fields keep
// their previous lowering. The capture contains scalar Assign booleanValue only.
func invocableFlowActionBooleanAssignment(item flowAssignmentItemXML, variables map[string]flowVariableXML) string {
	if !strings.EqualFold(strings.TrimSpace(item.Operator), "Assign") {
		return ""
	}
	variable, ok := variables[strings.ToLower(strings.TrimSpace(item.AssignToReference))]
	if !ok || variable.IsCollection || !strings.EqualFold(variable.DataType, "Boolean") {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(item.Value.BooleanValue)) {
	case "true":
		return "TRUE"
	case "false":
		return "FALSE"
	default:
		return ""
	}
}

// A declared Account variable named Record takes precedence over the legacy
// triggering-record spelling. Its fields belong to the interview's own value.
func invocableFlowActionRecordAssignment(reference string, variables map[string]flowVariableXML) bool {
	name, field, found := strings.Cut(strings.TrimSpace(reference), ".")
	if !found || field == "" || name != "Record" {
		return false
	}
	variable, declared := variables[strings.ToLower(name)]
	return declared && variable.Name == name && !variable.IsCollection && strings.EqualFold(variable.DataType, "SObject") && strings.EqualFold(variable.ObjectType, "Account")
}

// The explicit global Null spelling is not a valid source for the input/output
// Account Description assignment. An empty assignment value remains valid.
func invocableFlowActionInvalidNullAssignment(raw flowXML, variables map[string]flowVariableXML) bool {
	if raw.Start.Object != "" || !strings.EqualFold(raw.ProcessType, "AutoLaunchedFlow") {
		return false
	}
	for _, assignment := range raw.Assignments {
		for _, item := range assignment.Items {
			name, field, found := strings.Cut(strings.TrimSpace(item.AssignToReference), ".")
			variable, declared := variables[strings.ToLower(name)]
			if found && field == "Description" && declared && variable.Name == name && variable.IsInput && variable.IsOutput && !variable.IsCollection && strings.EqualFold(variable.DataType, "SObject") && strings.EqualFold(variable.ObjectType, "Account") && item.Operator == "Assign" && item.Value.ElementReference == "$GlobalConstant.Null" {
				return true
			}
		}
	}
	return false
}

// Extend the terminal scalar legacy entry with the scalar Account field/null
// assignments and Boolean output marker used by the supported interview shape.
func invocableFlowActionRecordAssignmentStart(raw flowXML, variables map[string]flowVariableXML) string {
	if raw.Start.Object != "" || !flowActive(raw.Status) || !strings.EqualFold(raw.ProcessType, "AutoLaunchedFlow") || len(raw.Assignments) != 1 {
		return ""
	}
	assignment := raw.Assignments[0]
	target := strings.TrimSpace(raw.StartElementReference)
	if target == "" || target != firstNonBlank(assignment.Name, assignment.Label) || len(assignment.Items) != 3 || assignment.Connector.TargetReference != "" || assignment.FaultConnector.TargetReference != "" {
		return ""
	}
	record, hasRecord := variables["record"]
	marker, hasMarker := variables["fired"]
	if !hasRecord || record.Name != "record" || record.IsCollection || !record.IsInput || !record.IsOutput || !strings.EqualFold(record.DataType, "SObject") || !strings.EqualFold(record.ObjectType, "Account") || !hasMarker || marker.Name != "fired" || marker.IsCollection || marker.IsInput || !marker.IsOutput || !strings.EqualFold(marker.DataType, "Boolean") {
		return ""
	}
	seen := make(map[string]bool, 3)
	for _, item := range assignment.Items {
		if item.Operator != "Assign" || seen[item.AssignToReference] {
			return ""
		}
		seen[item.AssignToReference] = true
		switch item.AssignToReference {
		case "record.Name":
			if item.Value != (flowValueXML{StringValue: "Foo"}) {
				return ""
			}
		case "record.Description":
			if item.Value != (flowValueXML{}) {
				return ""
			}
		case "fired":
			if item.Value != (flowValueXML{BooleanValue: "true"}) {
				return ""
			}
		default:
			return ""
		}
	}
	return target
}
