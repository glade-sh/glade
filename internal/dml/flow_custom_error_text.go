package dml

import (
	"html"
	"math/big"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// renderFlowCustomErrorText resolves {!reference} merge fields in a Custom
// Error message. Native rows (flow_custom_error_merge) show en_US text: a Date
// as "March 9, 2031", a currency or number without a currency symbol and with
// grouping ("1,234.5", "5"), a Boolean as "true" and a null as empty text. A
// reference or value type that is not captured keeps the authored message.
func (e *Engine) renderFlowCustomErrorText(message string, frame *flowFrame, formulaMaps ...map[string]string) string {
	var formulas map[string]string
	if len(formulaMaps) > 0 {
		formulas = formulaMaps[0]
	}
	return e.renderFlowCustomErrorTextWithConstants(message, frame, nil, formulas)
}

func (e *Engine) renderFlowCustomErrorTextWithConstants(message string, frame *flowFrame, constants map[string]string, formulaMaps ...map[string]string) string {
	return e.renderFlowCustomErrorMessage(message, frame, constants, firstFlowFormulaMap(formulaMaps), nil)
}

func firstFlowFormulaMap(maps []map[string]string) map[string]string {
	if len(maps) > 0 {
		return maps[0]
	}
	return nil
}

// renderFlowCustomErrorMessage also finishes a merge field that names a flow
// formula resource outside the captured frame-bound set (resources): native
// rows (flow_assign_routes, API 62.0 and 67.0) render its result.
func (e *Engine) renderFlowCustomErrorMessage(message string, frame *flowFrame, constants, formulas, resources map[string]string) string {
	text, _ := e.renderFlowCustomErrorMessageForAPI(message, frame, constants, formulas, resources, "")
	return text
}

// flowMergeUnhandledFault reports a merge field whose native flow run fails
// with an unhandled fault instead of rendering text (flow_record_merge: a
// DateTime or Time record field at API 62.0).
type flowMergeUnhandledFault struct{}

func (flowMergeUnhandledFault) Error() string {
	return "flow merge field raised an unhandled fault"
}

func (e *Engine) renderFlowCustomErrorMessageForAPI(message string, frame *flowFrame, constants, formulas, resources map[string]string, apiVersion string) (string, bool) {
	if frame == nil || !strings.Contains(message, "{!") {
		return message, false
	}
	fault := false
	capturedFormula := flowCapturedCustomErrorFormula(message, formulas, frame)
	rendered, ok := renderFlowTextTemplate(message, frame, func(reference string) (storage.Value, bool) {
		if number, ok := constants[strings.ToLower(reference)]; ok {
			if !e.flowMergeLocaleIsEnUS() {
				return storage.Value{}, false
			}
			text, supported := flowMergeFieldText(storage.DecimalValue(number))
			if !supported {
				return storage.Value{}, false
			}
			return storage.StringValue(text), true
		}
		if strings.EqualFold(reference, "$Flow.CurrentDateTime") {
			if text, ok := e.flowCurrentDateTimeMergeText(); ok {
				return storage.StringValue(text), true
			}
			return storage.Value{}, false
		}
		value, found := e.flowFrameReferenceValue(reference, frame)
		if recordField, kind := e.flowRecordMergeField(reference, frame); recordField && found && (apiVersion == "62.0" || apiVersion == "67.0") {
			switch kind {
			case storage.FieldDecimal:
				if value.Kind == storage.ValueDecimal || value.Kind == storage.ValueInteger {
					return storage.StringValue(flowRecordMergeNumber(value)), true
				}
			case storage.FieldDateTime, storage.FieldTime:
				if value.Kind == storage.ValueNull {
					return storage.StringValue(""), true
				}
				switch apiVersion {
				case "62.0":
					fault = true
					return storage.Value{}, false
				case "67.0":
					if text, ok := e.flowRecordMergeTemporal(kind, value); ok {
						return storage.StringValue(text), true
					}
				}
			}
		}
		if expression, formula := formulas[flowFrameKey(reference)]; formula && capturedFormula && flowCoreFormulaUsesScalars(expression, frame, true) {
			value, found = e.flowContextFormulaValueWithCapturedBindings(expression, frame, true)
		} else if expression, formula := resources[strings.ToLower(strings.TrimSpace(reference))]; formula && !found {
			value, found = e.flowContextFormulaValue(expression, frame)
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
	if fault {
		return message, true
	}
	if !ok {
		return message, false
	}
	return rendered.String, false
}

func (e *Engine) flowMergeLocaleIsEnUS() bool {
	if e == nil || e.Org == nil {
		return false
	}
	users, ok := e.Org.Objects["User"]
	if !ok {
		return false
	}
	user, ok := users.Records[e.systemUserID()]
	if !ok {
		return false
	}
	return user.Fields["LocaleSidKey"].String == "en_US"
}

// Only the captured user locale and time zone are modeled here. Other user
// settings keep the authored message until their formatting is captured.
func (e *Engine) flowCurrentDateTimeMergeText() (string, bool) {
	if e == nil || e.Org == nil {
		return "", false
	}
	users, ok := e.Org.Objects["User"]
	if !ok {
		return "", false
	}
	user, ok := users.Records[e.systemUserID()]
	if !ok {
		return "", false
	}
	if user.Fields["LocaleSidKey"].String != "en_US" || user.Fields["TimeZoneSidKey"].String != "America/Los_Angeles" {
		return "", false
	}
	zone, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return "", false
	}
	now := time.Now()
	if e.Now != nil {
		now = e.Now()
	}
	return now.In(zone).Format("1/2/2006, 3:04 PM"), true
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

// flowRecordMergeField resolves a `$Record.Field` merge reference to the
// triggering record's field type.
func (e *Engine) flowRecordMergeField(reference string, frame *flowFrame) (bool, storage.FieldType) {
	if e == nil || e.Org == nil || frame == nil {
		return false, ""
	}
	name, field, ok := strings.Cut(strings.TrimSpace(reference), ".")
	if !ok || !strings.EqualFold(name, "$Record") || strings.Contains(field, ".") {
		return false, ""
	}
	record, present := frame.records[flowFrameKey(name)]
	if !present {
		return false, ""
	}
	objectName, modeled := storage.ResolveObjectName(*e.Org, record.Object)
	if !modeled {
		return false, ""
	}
	definition := e.Org.Objects[objectName].Definition
	resolved, found := storage.ResolveFieldName(definition, e.Org.Namespace, field)
	if !found {
		return false, ""
	}
	return true, definition.Fields[resolved].Type
}

// flowRecordMergeNumber renders a Number, Currency or Percent record field the
// way the native Custom Error does: at most three fraction digits, rounded half
// to even, without trailing zeros. A negative value that rounds to zero keeps
// its sign; an exact zero does not (flow_record_merge).
func flowRecordMergeNumber(value storage.Value) string {
	text := value.Decimal
	if text == "" {
		text = value.String
	}
	if value.Kind == storage.ValueInteger {
		return groupFlowMergeNumber(workflowValueString(value))
	}
	number, ok := new(big.Rat).SetString(strings.TrimSpace(text))
	if !ok {
		return groupFlowMergeNumber(text)
	}
	negative := number.Sign() < 0
	scaled := new(big.Rat).Mul(new(big.Rat).Abs(number), big.NewRat(1000, 1))
	whole := new(big.Int).Quo(scaled.Num(), scaled.Denom())
	remainder := new(big.Rat).Sub(scaled, new(big.Rat).SetInt(whole))
	switch remainder.Cmp(big.NewRat(1, 2)) {
	case 1:
		whole.Add(whole, big.NewInt(1))
	case 0:
		if whole.Bit(0) == 1 {
			whole.Add(whole, big.NewInt(1))
		}
	}
	digits := whole.String()
	for len(digits) < 4 {
		digits = "0" + digits
	}
	rounded := digits[:len(digits)-3] + "." + digits[len(digits)-3:]
	grouped := groupFlowMergeNumber(rounded)
	if negative && number.Sign() != 0 && !strings.HasPrefix(grouped, "-") {
		grouped = "-" + grouped
	}
	return grouped
}

// flowRecordMergeTemporal renders a DateTime or Time record field at API 67.0
// for the captured en_US user: DateTime in the user's time zone as
// "3/9/2031, 8:04 AM", Time as "3:04 PM" with no zone shift.
func (e *Engine) flowRecordMergeTemporal(kind storage.FieldType, value storage.Value) (string, bool) {
	if !e.flowMergeLocaleIsEnUS() {
		return "", false
	}
	text := strings.TrimSpace(value.String)
	if kind == storage.FieldTime {
		for _, layout := range []string{"15:04:05.000Z", "15:04:05.000", "15:04:05Z", "15:04:05", "15:04"} {
			if parsed, err := time.Parse(layout, text); err == nil {
				return parsed.Format("3:04 PM"), true
			}
		}
		return "", false
	}
	zone, ok := e.flowMergeUserLocation()
	if !ok {
		return "", false
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.000Z0700", "2006-01-02T15:04:05Z"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.In(zone).Format("1/2/2006, 3:04 PM"), true
		}
	}
	return "", false
}

// flowMergeUserLocation is the captured user time zone (America/Los_Angeles).
func (e *Engine) flowMergeUserLocation() (*time.Location, bool) {
	if e == nil || e.Org == nil {
		return nil, false
	}
	users, ok := e.Org.Objects["User"]
	if !ok {
		return nil, false
	}
	user, ok := users.Records[e.systemUserID()]
	if !ok || user.Fields["TimeZoneSidKey"].String != "America/Los_Angeles" {
		return nil, false
	}
	zone, err := time.LoadLocation("America/Los_Angeles")
	return zone, err == nil
}
