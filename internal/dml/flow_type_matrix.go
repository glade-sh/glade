package dml

import (
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func flowCapturedTransformVariable(frame *flowFrame, reference string) bool {
	return flowCapturedVariable(frame, reference, false)
}

func flowCapturedVariable(frame *flowFrame, reference string, collection bool) bool {
	if frame == nil {
		return false
	}
	variable, ok := frame.variables[flowFrameKey(reference)]
	if !ok || variable.IsCollection != collection {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(variable.DataType)) {
	case "string", "number", "boolean", "date", "datetime":
		return true
	default:
		return false
	}
}

type flowLookupCollectionFault struct{ cause error }

func (e *flowLookupCollectionFault) Error() string { return e.cause.Error() }
func (e *flowLookupCollectionFault) Unwrap() error { return e.cause }

// IsFlowLookupCollectionFault identifies the captured, catchable interview
// failures. Unsupported engine failures outside this shape keep their route.
func IsFlowLookupCollectionFault(err error) bool {
	var fault *flowLookupCollectionFault
	return errors.As(err, &fault)
}

func flowLookupCollectionHasCapturedFault(field storage.Field, value storage.Value) bool {
	if value.Kind != storage.ValueList {
		return false
	}
	for _, item := range value.List {
		if field.Type == storage.FieldString && field.Filterable != nil && !*field.Filterable &&
			(item.Kind == storage.ValueInteger || item.Kind == storage.ValueDecimal || item.Kind == storage.ValueBoolean) {
			return true
		}
		switch field.Type {
		case storage.FieldString, storage.FieldPicklist:
			if item.Kind == storage.ValueDate || item.Kind == storage.ValueDateTime {
				return true
			}
		case storage.FieldReference:
			switch item.Kind {
			case storage.ValueInteger, storage.ValueDecimal, storage.ValueBoolean, storage.ValueDate, storage.ValueDateTime:
				return true
			}
		}
	}
	return false
}

// The collection cases are bounded by the Flow lookup captures at API 62/67.
// An empty or unassigned list is handled by the caller without inspecting an element.
func flowLookupCollectionValuesEqual(field storage.Field, target, element storage.Value) (bool, bool) {
	switch field.Type {
	case storage.FieldString, storage.FieldPicklist:
		if field.Type == storage.FieldString && field.Filterable != nil && !*field.Filterable {
			return false, false
		}
		switch element.Kind {
		case storage.ValueString:
			return storageValuesEqual(field, target, element), true
		case storage.ValueInteger, storage.ValueDecimal, storage.ValueBoolean:
			var text string
			switch element.Kind {
			case storage.ValueInteger:
				text = strconv.FormatInt(element.Integer, 10)
			case storage.ValueDecimal:
				text = element.Decimal
			case storage.ValueBoolean:
				text = strconv.FormatBool(element.Boolean)
			}
			return storageValuesEqual(field, target, storage.StringValue(text)), true
		default:
			return false, false
		}
	case storage.FieldID, storage.FieldReference:
		if element.Kind != storage.ValueString {
			return false, false
		}
		left, right := idFromStorageValue(target), idFromStorageValue(element)
		if storage.ValidateID(left) == nil && storage.ValidateID(right) == nil {
			return storage.IDsEqual(left, right), true
		}
		return storageValuesEqual(field, target, element), true
	case storage.FieldInteger, storage.FieldDecimal:
		if element.Kind != storage.ValueInteger && element.Kind != storage.ValueDecimal {
			return false, false
		}
		if target.Kind == storage.ValueNull {
			return false, true
		}
		left, leftOK := flowTypeMatrixNumber(target)
		right, rightOK := flowTypeMatrixNumber(element)
		if !leftOK || !rightOK {
			return false, false
		}
		return left.Cmp(right) == 0, true
	case storage.FieldBoolean:
		if element.Kind != storage.ValueBoolean {
			return false, false
		}
		return storageValuesEqual(field, target, element), true
	case storage.FieldDate, storage.FieldDateTime:
		// The captured populated Date and DateTime lists fault on these fields.
		return false, false
	default:
		return false, false
	}
}

func flowTypeMatrixNumber(v storage.Value) (*big.Rat, bool) {
	var s string
	switch v.Kind {
	case storage.ValueInteger:
		s = strconv.FormatInt(v.Integer, 10)
	case storage.ValueDecimal:
		s = v.Decimal
	default:
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

// Coerce values before storing the transient record so the interview output
// has the field's captured runtime type, not just its source variable's type.
func (e *Engine) flowTransformTargetValue(field storage.Field, value storage.Value) (storage.Value, bool) {
	if value.Kind == storage.ValueNull {
		return value, true
	}
	switch field.Type {
	case storage.FieldString, storage.FieldPicklist:
		switch value.Kind {
		case storage.ValueString:
			return value, true
		case storage.ValueBoolean:
			return storage.StringValue(strconv.FormatBool(value.Boolean)), true
		case storage.ValueInteger:
			return storage.StringValue(strconv.FormatInt(value.Integer, 10)), true
		case storage.ValueDecimal:
			return storage.StringValue(value.Decimal), value.Decimal != ""
		case storage.ValueDate:
			if field.Type == storage.FieldPicklist {
				return storage.StringValue(value.String), true
			}
			if !e.flowMergeLocaleIsEnUS() {
				return storage.Value{}, false
			}
			s, ok := flowMergeFieldText(value)
			return storage.StringValue(s), ok
		case storage.ValueDateTime:
			parsed, err := time.Parse(time.RFC3339Nano, value.String)
			if err != nil {
				return storage.Value{}, false
			}
			if field.Type == storage.FieldPicklist {
				return storage.StringValue(parsed.UTC().Format(time.RFC3339)), true
			}
			if e == nil || e.Org == nil || !e.flowMergeLocaleIsEnUS() {
				return storage.Value{}, false
			}
			users := e.Org.Objects["User"]
			user, ok := users.Records[e.systemUserID()]
			if !ok || user.Fields["TimeZoneSidKey"].String != "America/Los_Angeles" {
				return storage.Value{}, false
			}
			zone, err := time.LoadLocation("America/Los_Angeles")
			if err != nil {
				return storage.Value{}, false
			}
			return storage.StringValue(parsed.In(zone).Format("1/2/2006, 3:04 PM")), true
		}
	case storage.FieldInteger, storage.FieldDecimal:
		return value, value.Kind == storage.ValueInteger || value.Kind == storage.ValueDecimal
	case storage.FieldBoolean:
		return value, value.Kind == storage.ValueBoolean
	case storage.FieldDate:
		if value.Kind == storage.ValueDate {
			return value, true
		}
		if value.Kind == storage.ValueDateTime {
			parsed, err := time.Parse(time.RFC3339Nano, value.String)
			if err != nil {
				return storage.Value{}, false
			}
			return storage.DateValue(parsed.UTC().Format("2006-01-02")), true
		}
	case storage.FieldDateTime:
		if value.Kind == storage.ValueDateTime {
			return value, true
		}
		if value.Kind == storage.ValueDate {
			parsed, err := time.Parse("2006-01-02", value.String)
			if err != nil {
				return storage.Value{}, false
			}
			return storage.DateTimeValue(parsed.UTC().Format(time.RFC3339)), true
		}
	case storage.FieldID, storage.FieldReference:
		// The captures establish non-null output for these sources, but not
		// the precise representation of the resulting reference.
		switch value.Kind {
		case storage.ValueString, storage.ValueID, storage.ValueBoolean, storage.ValueInteger, storage.ValueDecimal, storage.ValueDate, storage.ValueDateTime:
			return value, true
		}
	}
	return storage.Value{}, false
}
