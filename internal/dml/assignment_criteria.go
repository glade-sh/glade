package dml

import (
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// evaluateAssignmentCriteria evaluates one assignment rule criteria item. It
// models the operators and value rules that the native rows in
// automation_residual_paths prove for Lead criteria, and hands every other
// combination to the shared workflow criteria evaluator unchanged.
//
// Proven by the rows (API 62.0 and 67.0 alike):
//   - Text and picklist operators equals, notEqual, startsWith, contains and
//     notContain compare without case. The rule value is a comma separated
//     list: equals, startsWith and contains match any element; notEqual and
//     notContain match when no element matches.
//   - A comma separated rule value is trimmed element by element.
//   - Number operators equals (list), notEqual, greaterThan, greaterOrEqual,
//     lessThan and lessOrEqual compare numerically; a blank number sorts below
//     every bound, equals nothing and passes notEqual.
//   - Date rule values are month/day/year; greaterThan, greaterOrEqual,
//     equals and lessThan compare days; a blank date sorts below every bound.
//   - Multi-select picklist includes matches any listed value, excludes
//     matches a blank value or one that lacks the listed value, and equals
//     matches a selection that equals one listed choice as a whole.
//   - A checkbox that is not set reads as false.
func evaluateAssignmentCriteria(item storage.WorkflowCriteriaItem, record storage.Record, definition storage.ObjectDefinition, namespace string) (bool, bool) {
	fieldName, ok := storage.ResolveFieldName(definition, namespace, strings.TrimSpace(item.Field))
	if !ok || fieldName == "" {
		return evaluateWorkflowCriteria(item, record, definition, namespace)
	}
	field := definition.Fields[fieldName]
	operation := strings.ToLower(strings.TrimSpace(item.Operation))
	want := trimFormulaLiteral(item.Value)
	value, present := record.GetField(fieldName)
	blank := !present || value.Kind == storage.ValueNull || record.ExplicitNulls[fieldName]
	switch field.Type {
	case storage.FieldString, storage.FieldPicklist:
		switch operation {
		case "equals", "notequal", "startswith", "contains", "notcontain":
			return assignmentTextCriteria(operation, blank, workflowValueString(value), want), true
		}
	case storage.FieldInteger, storage.FieldDecimal:
		if matched, handled, supported := assignmentNumberCriteria(operation, blank, value, want); handled {
			return matched, supported
		}
	case storage.FieldDate:
		if matched, handled, supported := assignmentDateCriteria(operation, blank, value, want); handled {
			return matched, supported
		}
	case storage.FieldBoolean:
		switch operation {
		case "equals", "notequal":
			// An unset checkbox reads as false (rows B1 to B3).
			actual := present && value.Kind == storage.ValueBoolean && value.Boolean
			matched := strings.EqualFold(want, "true") == actual
			if operation == "notequal" {
				matched = !matched
			}
			return matched, true
		}
	case storage.FieldMultiPicklist:
		switch operation {
		case "equals":
			// Row M4: the rule value is a list of single choices, and the
			// selection must equal one of them as a whole.
			text := strings.ToLower(strings.TrimSpace(workflowValueString(value)))
			for _, element := range strings.FieldsFunc(want, func(r rune) bool { return r == ',' || r == ';' }) {
				if !blank && text == strings.ToLower(strings.TrimSpace(element)) {
					return true, true
				}
			}
			return false, true
		case "includes":
			selected := assignmentSelected(blank, value)
			for _, part := range strings.Split(want, ",") {
				if selected[strings.ToLower(part)] {
					return true, true
				}
			}
			return false, true
		case "excludes":
			if strings.Contains(want, ",") {
				break
			}
			return !assignmentSelected(blank, value)[strings.ToLower(want)], true
		}
	}
	return evaluateWorkflowCriteria(item, record, definition, namespace)
}

func assignmentSelected(blank bool, value storage.Value) map[string]bool {
	selected := map[string]bool{}
	if blank {
		return selected
	}
	for _, part := range strings.Split(workflowValueString(value), ";") {
		selected[strings.ToLower(part)] = true
	}
	return selected
}

func assignmentTextCriteria(operation string, blank bool, text, want string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	any := false
	for _, element := range strings.Split(strings.ToLower(want), ",") {
		element = strings.TrimSpace(element)
		switch operation {
		case "equals", "notequal":
			any = any || (!blank && text == element)
		case "startswith":
			any = any || (!blank && strings.HasPrefix(text, element))
		default:
			any = any || (!blank && strings.Contains(text, element))
		}
	}
	if operation == "notequal" || operation == "notcontain" {
		return !any
	}
	return any
}

// assignmentNumberCriteria reports handled=false for combinations the rows do
// not prove, so the shared evaluator keeps its behavior for them.
func assignmentNumberCriteria(operation string, blank bool, value storage.Value, want string) (matched, handled, supported bool) {
	switch operation {
	case "equals":
		if blank {
			return false, true, true
		}
		actual, ok := assignmentNumber(value)
		if !ok {
			return false, false, false
		}
		for _, part := range strings.Split(want, ",") {
			if bound, err := strconv.ParseFloat(strings.TrimSpace(part), 64); err == nil && bound == actual {
				return true, true, true
			}
		}
		return false, true, true
	case "notequal", "greaterthan", "greaterorequal", "lessthan", "lessorequal":
		if strings.Contains(want, ",") {
			return false, false, false
		}
		bound, err := strconv.ParseFloat(strings.TrimSpace(want), 64)
		if err != nil {
			return false, false, false
		}
		if blank {
			return assignmentBlankNumber(operation), true, true
		}
		actual, ok := assignmentNumber(value)
		if !ok {
			return false, false, false
		}
		switch operation {
		case "notequal":
			return actual != bound, true, true
		case "greaterthan":
			return actual > bound, true, true
		case "greaterorequal":
			return actual >= bound, true, true
		case "lessthan":
			return actual < bound, true, true
		default:
			return actual <= bound, true, true
		}
	}
	return false, false, false
}

func assignmentNumber(value storage.Value) (float64, bool) {
	switch value.Kind {
	case storage.ValueInteger:
		return float64(value.Integer), true
	case storage.ValueDecimal:
		number, err := strconv.ParseFloat(strings.TrimSpace(value.Decimal), 64)
		if err != nil {
			number, err = strconv.ParseFloat(strings.TrimSpace(value.String), 64)
		}
		return number, err == nil
	}
	return 0, false
}

func assignmentDateCriteria(operation string, blank bool, value storage.Value, want string) (matched, handled, supported bool) {
	switch operation {
	case "greaterthan", "greaterorequal", "equals", "lessthan":
	default:
		return false, false, false
	}
	bound, err := time.Parse("1/2/2006", strings.TrimSpace(want))
	if err != nil {
		return false, false, false
	}
	if blank {
		// A blank date sorts below every bound (rows D1, D5, D6, D7).
		return operation == "lessthan", true, true
	}
	actual, err := time.Parse("2006-01-02", value.String)
	if err != nil {
		return false, false, false
	}
	switch operation {
	case "greaterthan":
		return actual.After(bound), true, true
	case "greaterorequal":
		return !actual.Before(bound), true, true
	case "equals":
		return actual.Equal(bound), true, true
	default:
		return actual.Before(bound), true, true
	}
}

// assignmentBlankNumber is the result of a blank number against a bound. A
// blank value sorts below every bound (rows N3, N9, N13 lessThan and
// lessOrEqual match; N1, N8, N12 greaterThan and greaterOrEqual do not) and is
// not equal to a bound (rows N10, N11).
func assignmentBlankNumber(operation string) bool {
	switch operation {
	case "notequal", "lessthan", "lessorequal":
		return true
	}
	return false
}
