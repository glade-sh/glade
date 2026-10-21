package automation

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Explicit save entries share literal filter groups. Older trigger
// metadata and unsupported value expressions retain their existing projection.
func flowCoreEntryFormula(start flowStartXML, formulas map[string]string, variables map[string]flowVariableXML) string {
	trigger := strings.TrimSpace(start.TriggerType)
	recordTrigger := strings.TrimSpace(start.RecordTriggerType)
	changedAfterSaveUpdate := start.DoesRequireRecordChangedToMeetCriteria &&
		strings.EqualFold(trigger, "RecordAfterSave") && strings.EqualFold(recordTrigger, "Update")
	beforeSaveUpdate := strings.EqualFold(trigger, "RecordBeforeSave") && strings.EqualFold(recordTrigger, "Update")
	if (start.DoesRequireRecordChangedToMeetCriteria && !changedAfterSaveUpdate && !beforeSaveUpdate) ||
		(!strings.EqualFold(trigger, "RecordBeforeSave") && !strings.EqualFold(trigger, "RecordAfterSave")) ||
		(!changedAfterSaveUpdate && !beforeSaveUpdate && !strings.EqualFold(recordTrigger, "CreateAndUpdate")) {
		return ""
	}
	if strings.TrimSpace(start.FilterFormula) != "" {
		if !strings.EqualFold(trigger, "RecordAfterSave") {
			return ""
		}
		return flowCoreLiteralEntryFormula(start.FilterFormula)
	}
	logic := strings.TrimSpace(start.FilterLogic)
	if logic == "" || strings.EqualFold(logic, "and") {
		return ""
	}
	groups, ok := flowConditionLogicGroups(logic, len(start.Filters))
	if !ok {
		return ""
	}
	conditions := make([]flowConditionXML, 0, len(start.Filters))
	for _, filter := range start.Filters {
		if _, ok := flowCoreEntryFilterLiteral(filter.Value); !ok || strings.TrimSpace(filter.Value.Formula) != "" || strings.TrimSpace(filter.Value.Expression) != "" || strings.TrimSpace(filter.Value.ElementReference) != "" {
			return ""
		}
		switch strings.ToLower(strings.TrimSpace(filter.Operator)) {
		case "equalto", "greaterthan":
		default:
			return ""
		}
		conditions = append(conditions, flowConditionXML{LeftValueReference: "$Record." + trimObjectPrefix(strings.TrimSpace(filter.Field)), Operator: filter.Operator, RightValue: filter.Value})
	}
	return flowCoreEntryConditionsToFormula(groups, conditions, formulas, variables)
}

func flowCoreEntryConditionsToFormula(groups [][]int, conditions []flowConditionXML, formulas map[string]string, variables map[string]flowVariableXML) string {
	var orParts []string
	for _, group := range groups {
		var andParts []string
		for _, idx := range group {
			if idx >= len(conditions) {
				continue
			}
			condition := conditions[idx]
			left := flowConditionFormulaOperand(condition.LeftValueReference, formulas, variables)
			right, ok := flowCoreEntryFilterLiteral(condition.RightValue)
			if left == "" || !ok {
				return ""
			}
			operator := "="
			if strings.EqualFold(strings.TrimSpace(condition.Operator), "GreaterThan") {
				operator = ">"
			}
			andParts = append(andParts, left+" "+operator+" "+right)
		}
		if len(andParts) > 0 {
			if len(andParts) == 1 {
				orParts = append(orParts, andParts[0])
			} else {
				orParts = append(orParts, "AND("+strings.Join(andParts, ",")+")")
			}
		}
	}
	if len(orParts) == 0 {
		return ""
	}
	if len(orParts) == 1 {
		return orParts[0]
	}
	return "OR(" + strings.Join(orParts, ",") + ")"
}

func flowCoreEntryFilterLiteral(value flowValueXML) (string, bool) {
	values := 0
	for _, candidate := range []string{value.StringValue, value.BooleanValue, value.NumberValue, value.DateValue, value.DateTimeValue} {
		if strings.TrimSpace(candidate) != "" {
			values++
		}
	}
	if values != 1 || strings.TrimSpace(value.Formula) != "" || strings.TrimSpace(value.Expression) != "" || strings.TrimSpace(value.ElementReference) != "" {
		return "", false
	}
	switch {
	case strings.TrimSpace(value.StringValue) != "":
		return formulaQuote(strings.TrimSpace(value.StringValue)), true
	case strings.TrimSpace(value.BooleanValue) != "":
		boolean := strings.ToLower(strings.TrimSpace(value.BooleanValue))
		if boolean == "true" || boolean == "false" {
			return boolean, true
		}
	case strings.TrimSpace(value.NumberValue) != "":
		if _, ok := flowCoreNumericLiteral(value.NumberValue); ok {
			return strings.TrimSpace(value.NumberValue), true
		}
	case strings.TrimSpace(value.DateValue) != "":
		date, err := time.Parse("2006-01-02", strings.TrimSpace(value.DateValue))
		if err == nil {
			return "DATE(" + date.Format("2006,1,2") + ")", true
		}
	case strings.TrimSpace(value.DateTimeValue) != "":
		datetime, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value.DateTimeValue))
		if err == nil {
			return "DATETIMEVALUE(" + strconv.Quote(datetime.Format(time.RFC3339Nano)) + ")", true
		}
	}
	return "", false
}

var flowCoreLiteralEntryFormulaShape = regexp.MustCompile(`(?i)^AND\(\s*\{!\$Record\.[[:alnum:]_]+\}\s*,\s*BEGINS\(\s*\{!\$Record\.[[:alnum:]_]+\}\s*,\s*"(?:[^"\\]|\\.)*"\s*\)\s*\)$`)

func flowCoreLiteralEntryFormula(expression string) string {
	expression = strings.TrimSpace(expression)
	if !flowCoreLiteralEntryFormulaShape.MatchString(expression) {
		return ""
	}
	return normalizeFlowFormula(expression)
}
