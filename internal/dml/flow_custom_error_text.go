package dml

import (
	"html"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// renderFlowCustomErrorText resolves {!reference} merge fields in a Custom
// Error message. Native rows (flow_custom_error_merge) show en_US text: a Date
// as "March 9, 2031", a currency or number without a currency symbol and with
// grouping ("1,234.5", "5"), a Boolean as "true" and a null as empty text. A
// reference or value type that is not captured keeps the authored message.
func (e *Engine) renderFlowCustomErrorText(message string, frame *flowFrame, formulas map[string]string) string {
	if frame == nil || !strings.Contains(message, "{!") {
		return message
	}
	capturedFormula := flowCapturedCustomErrorFormula(message, formulas, frame)
	rendered, ok := renderFlowTextTemplate(message, frame, func(reference string) (storage.Value, bool) {
		value, found := e.flowFrameReferenceValue(reference, frame)
		if expression, formula := formulas[flowFrameKey(reference)]; formula && capturedFormula && flowCoreFormulaUsesScalars(expression, frame, true) {
			value, found = e.flowContextFormulaValueWithCapturedBindings(expression, frame, true)
		}
		if !found {
			return storage.Value{}, false
		}
		text, supported := flowMergeFieldText(value)
		if !supported {
			return storage.Value{}, false
		}
		return storage.StringValue(text), true
	})
	if !ok {
		return message
	}
	return rendered.String
}

func flowMergeFieldText(value storage.Value) (string, bool) {
	switch value.Kind {
	case storage.ValueNull:
		return "", true
	case storage.ValueString:
		return value.String, true
	case storage.ValueID:
		return string(value.ID), true
	case storage.ValueBoolean:
		if value.Boolean {
			return "true", true
		}
		return "false", true
	case storage.ValueInteger:
		return groupFlowMergeNumber(workflowValueString(value)), true
	case storage.ValueDecimal:
		text := value.Decimal
		if text == "" {
			text = value.String
		}
		return groupFlowMergeNumber(text), true
	case storage.ValueDate:
		date, err := time.Parse("2006-01-02", value.String)
		if err != nil {
			return "", false
		}
		return date.Format("January 2, 2006"), true
	}
	return "", false
}

func groupFlowMergeNumber(text string) string {
	text = strings.TrimSpace(text)
	sign := ""
	if strings.HasPrefix(text, "-") {
		sign, text = "-", text[1:]
	}
	whole, fraction, _ := strings.Cut(text, ".")
	fraction = strings.TrimRight(fraction, "0")
	if whole == "0" && fraction == "" {
		// Native Custom Error text normalizes -0 and -0.00 to 0.
		sign = ""
	}
	var grouped strings.Builder
	for i, digit := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(digit)
	}
	if fraction != "" {
		return sign + grouped.String() + "." + fraction
	}
	return sign + grouped.String()
}

// A partial-insert capture covers a formula merge that reads a declared String
// in a record-triggered Create interview, not arbitrary Custom Error failures.
func flowCapturedCustomErrorFormula(message string, formulas map[string]string, frame *flowFrame) bool {
	if frame == nil || !frame.capturedBeforeCreate {
		return false
	}
	lower := strings.ToLower(message)
	for name, expression := range formulas {
		if !strings.Contains(lower, "{!"+strings.ToLower(name)+"}") {
			continue
		}
		for _, token := range tokenizeFormula(html.UnescapeString(expression)) {
			if token.typ != formulaTokenIdent {
				continue
			}
			reference := strings.ToLower(token.text)
			if strings.Contains(reference, ".") {
				continue
			}
			variable, declared := frame.variables[flowFrameKey(reference)]
			if declared && !variable.IsCollection && strings.EqualFold(variable.DataType, "String") {
				return true
			}
		}
	}
	return false
}
