package automation

import (
	"math"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func flowCoreNumericLiteral(text string) (storage.Value, bool) {
	text = strings.TrimSpace(text)
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return storage.Value{}, false
	}
	if integer, err := strconv.ParseInt(text, 10, 64); err == nil {
		return storage.IntegerValue(integer), true
	}
	return storage.DecimalValue(text), true
}

func flowCoreAssignmentLiteralFormula(item flowAssignmentItemXML, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) string {
	variable, ok := variables[strings.ToLower(strings.TrimSpace(item.AssignToReference))]
	if !ok {
		name, field, dotted := strings.Cut(strings.TrimSpace(item.AssignToReference), ".")
		record, declared := variables[strings.ToLower(name)]
		recordVariable := declared && !record.IsCollection && strings.EqualFold(record.DataType, "SObject")
		recordVariable = recordVariable || strings.EqualFold(name, "$Record")
		lookupOutput := flowLookupFieldReference(item.AssignToReference, lookups) != ""
		operator := strings.ToLower(strings.TrimSpace(item.Operator))
		if dotted && field != "" && !strings.Contains(field, ".") && (recordVariable || lookupOutput) &&
			(operator == "assign" || operator == "add" || operator == "subtract") {
			if _, numeric := flowCoreNumericLiteral(item.Value.NumberValue); numeric {
				return strings.TrimSpace(item.Value.NumberValue)
			}
		}
		return ""
	}
	numeric := strings.EqualFold(variable.DataType, "Number") || strings.EqualFold(variable.DataType, "Currency")
	if !numeric {
		return ""
	}
	operator := strings.ToLower(strings.TrimSpace(item.Operator))
	if variable.IsCollection {
		if operator != "add" || !strings.EqualFold(variable.DataType, "Number") {
			return ""
		}
	} else if operator != "assign" && operator != "add" && operator != "subtract" {
		return ""
	}
	if _, ok := flowCoreNumericLiteral(item.Value.NumberValue); !ok {
		return ""
	}
	return strings.TrimSpace(item.Value.NumberValue)
}

func flowCoreAssignmentSourceField(item flowAssignmentItemXML, variables map[string]flowVariableXML, lookups []flowRecordLookupXML, fallback string) string {
	target := strings.TrimSpace(item.AssignToReference)
	variable, ok := variables[strings.ToLower(target)]
	if !ok || !(strings.EqualFold(variable.DataType, "Number") || strings.EqualFold(variable.DataType, "Currency")) {
		// Record variables and automatically stored lookup records resolve
		// assignment sources through the frame, which needs the $Record prefix.
		name, field, direct := strings.Cut(target, ".")
		variable, ok = variables[strings.ToLower(name)]
		recordVariable := ok && strings.EqualFold(variable.DataType, "SObject") && strings.TrimSpace(variable.ObjectType) != ""
		recordVariable = recordVariable || strings.EqualFold(name, "$Record")
		if !direct || field == "" || strings.Contains(field, ".") ||
			(!recordVariable && flowLookupFieldReference(target, lookups) == "") {
			return fallback
		}
	}
	if variable.IsCollection || !strings.EqualFold(item.Operator, "Assign") {
		return fallback
	}
	reference := strings.TrimSpace(item.Value.ElementReference)
	if len(reference) > len("$Record.") && strings.EqualFold(reference[:len("$Record.")], "$Record.") && !strings.Contains(reference[len("$Record."):], ".") {
		return reference
	}
	return fallback
}
