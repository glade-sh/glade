package automation

import (
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type flowSubflowOutputXML struct {
	Name              string `xml:"name"`
	AssignToReference string `xml:"assignToReference"`
}

func flowCoreSubflowOutput(output flowSubflowOutputXML) storage.FlowAssignment {
	name, target := strings.TrimSpace(output.Name), strings.TrimSpace(output.AssignToReference)
	return storage.FlowAssignment{Name: name, Target: target, SourceField: name}
}

// Scalar defaults belong to their interview.
func flowCoreScalarDefault(variable flowVariableXML) *storage.Value {
	if variable.IsCollection {
		return nil
	}
	var value storage.Value
	switch strings.ToLower(strings.TrimSpace(variable.DataType)) {
	case "string":
		value = storage.StringValue(variable.Value.StringValue)
		if variable.Value.StringValue == "" {
			value = storage.NullValue()
		}
	case "number", "currency":
		var ok bool
		value, ok = flowCoreNumericLiteral(variable.Value.NumberValue)
		if !ok {
			return nil
		}
	case "boolean":
		switch strings.ToLower(strings.TrimSpace(variable.Value.BooleanValue)) {
		case "true":
			value = storage.BooleanValue(true)
		case "false":
			value = storage.BooleanValue(false)
		default:
			return nil
		}
	default:
		return nil
	}
	if strings.TrimSpace(variable.Value.ElementReference) != "" || variable.Value.Formula != "" || variable.Value.Expression != "" {
		return nil
	}
	return &value
}

func flowCoreNumberAssignment(item flowAssignmentItemXML, variables map[string]flowVariableXML) string {
	if !strings.EqualFold(strings.TrimSpace(item.Operator), "Assign") {
		return ""
	}
	variable, ok := variables[strings.ToLower(strings.TrimSpace(item.AssignToReference))]
	if !ok || variable.IsCollection || !strings.EqualFold(variable.DataType, "Number") {
		return ""
	}
	number := strings.TrimSpace(item.Value.NumberValue)
	if _, err := strconv.ParseInt(number, 10, 64); err != nil {
		return ""
	}
	return number
}

// A declared scalar resource is resolved in its interview, not the object schema.
func flowCoreScalarVariableReference(reference string, variables map[string]flowVariableXML) bool {
	variable, ok := variables[strings.ToLower(strings.TrimSpace(reference))]
	if !ok || variable.IsCollection {
		return false
	}
	return strings.EqualFold(variable.DataType, "String") || strings.EqualFold(variable.DataType, "Number") || strings.EqualFold(variable.DataType, "Currency")
}

// Legacy autolaunched starts are admitted here only for a terminal scalar Assign.
func flowCoreScalarAssignmentStart(raw flowXML, variables map[string]flowVariableXML) string {
	if strings.TrimSpace(raw.Start.Object) != "" || !flowActive(raw.Status) || !strings.EqualFold(raw.ProcessType, "AutoLaunchedFlow") || len(raw.Assignments) != 1 {
		return ""
	}
	target := strings.TrimSpace(raw.StartElementReference)
	assignment := raw.Assignments[0]
	if target == "" || !strings.EqualFold(target, firstNonBlank(assignment.Name, assignment.Label)) || len(assignment.Items) != 1 || strings.TrimSpace(assignment.Connector.TargetReference) != "" || strings.TrimSpace(assignment.FaultConnector.TargetReference) != "" {
		return ""
	}
	item := assignment.Items[0]
	if !strings.EqualFold(item.Operator, "Assign") || !flowCoreScalarVariableReference(item.AssignToReference, variables) {
		return ""
	}
	value := item.Value
	if value.ElementReference != "" || value.Formula != "" || value.Expression != "" || value.BooleanValue != "" || value.DateValue != "" || value.DateTimeValue != "" {
		return ""
	}
	variable := variables[strings.ToLower(strings.TrimSpace(item.AssignToReference))]
	if strings.EqualFold(variable.DataType, "Currency") {
		return ""
	}
	if strings.EqualFold(variable.DataType, "Number") {
		if flowCoreNumberAssignment(item, variables) == "" {
			return ""
		}
	} else if value.NumberValue != "" {
		return ""
	}
	return target
}
