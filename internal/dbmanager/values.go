package dbmanager

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type FieldInput struct {
	State  string   `json:"state,omitempty"`
	Value  any      `json:"value,omitempty"`
	ID     string   `json:"id,omitempty"`
	Values []string `json:"values,omitempty"`
	Null   bool     `json:"null,omitempty"`
}

type MutationPayload struct {
	Fields map[string]FieldInput `json:"fields"`
}

type MutationResult struct {
	Success    bool      `json:"success"`
	ID         string    `json:"id,omitempty"`
	Created    bool      `json:"created,omitempty"`
	StatusCode string    `json:"statusCode,omitempty"`
	Message    string    `json:"message,omitempty"`
	Fields     []string  `json:"fields,omitempty"`
	Record     RecordRow `json:"record,omitempty"`
}

func FieldInputToStorageValue(field storage.Field, input FieldInput) (storage.Value, bool, error) {
	if input.Null || strings.EqualFold(input.State, "null") {
		return storage.NullValue(), true, nil
	}
	switch field.Type {
	case storage.FieldBoolean:
		value, ok := inputBool(input.Value)
		if !ok {
			return storage.Value{}, false, fmt.Errorf("expected boolean")
		}
		return storage.BooleanValue(value), false, nil
	case storage.FieldInteger:
		value, ok := inputInteger(input.Value)
		if !ok {
			return storage.Value{}, false, fmt.Errorf("expected integer")
		}
		return storage.IntegerValue(value), false, nil
	case storage.FieldDecimal:
		value, ok := inputDecimal(input.Value)
		if !ok {
			return storage.Value{}, false, fmt.Errorf("expected number")
		}
		return storage.DecimalValue(value), false, nil
	case storage.FieldID:
		value := strings.TrimSpace(firstNonEmpty(input.ID, inputString(input.Value)))
		if value == "" {
			return storage.Value{}, false, fmt.Errorf("expected id")
		}
		return storage.IDValue(storage.ID(value)), false, nil
	case storage.FieldReference:
		value := strings.TrimSpace(firstNonEmpty(input.ID, inputString(input.Value)))
		if value == "" {
			return storage.Value{}, false, fmt.Errorf("expected lookup id")
		}
		return storage.IDValue(storage.ID(value)), false, nil
	case storage.FieldDate:
		return storage.DateValue(inputString(input.Value)), false, nil
	case storage.FieldDateTime:
		return storage.DateTimeValue(inputString(input.Value)), false, nil
	case storage.FieldBlob:
		return storage.BlobValue(inputString(input.Value)), false, nil
	case storage.FieldMultiPicklist:
		values := input.Values
		if len(values) == 0 {
			text := inputString(input.Value)
			if text != "" {
				values = strings.Split(text, ";")
			}
		}
		clean := make([]string, 0, len(values))
		for _, value := range values {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				clean = append(clean, trimmed)
			}
		}
		return storage.StringValue(strings.Join(clean, ";")), false, nil
	default:
		return storage.StringValue(inputString(input.Value)), false, nil
	}
}

func FieldInputFromStorageValue(field storage.Field, value storage.Value) FieldInput {
	if value.Kind == storage.ValueNull {
		return FieldInput{Null: true}
	}
	if field.Type == storage.FieldMultiPicklist {
		text := storageValueString(value)
		if text == "" {
			return FieldInput{Values: []string{}}
		}
		return FieldInput{Value: text, Values: strings.Split(text, ";")}
	}
	return FieldInput{Value: StorageValueJSON(value)}
}

func StorageValueJSON(value storage.Value) any {
	switch value.Kind {
	case storage.ValueNull:
		return nil
	case storage.ValueInteger:
		return value.Integer
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueList:
		out := make([]any, 0, len(value.List))
		for _, item := range value.List {
			out = append(out, StorageValueJSON(item))
		}
		return out
	default:
		return value.String
	}
}

func storageValueString(value storage.Value) string {
	switch value.Kind {
	case storage.ValueNull:
		return ""
	case storage.ValueInteger:
		return strconv.FormatInt(value.Integer, 10)
	case storage.ValueBoolean:
		if value.Boolean {
			return "true"
		}
		return "false"
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueList:
		parts := make([]string, 0, len(value.List))
		for _, item := range value.List {
			parts = append(parts, storageValueString(item))
		}
		return strings.Join(parts, ";")
	default:
		return value.String
	}
}

func inputString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		if math.Trunc(typed) == typed {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return fmt.Sprint(value)
	}
}

func inputBool(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		return false, false
	}
}

func inputInteger(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		if math.Trunc(typed) != typed {
			return 0, false
		}
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func inputDecimal(value any) (string, bool) {
	switch typed := value.(type) {
	case int:
		return strconv.Itoa(typed), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return "", false
		}
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return "", false
		}
		return typed.String(), true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return "", false
		}
		parsed, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return "", false
		}
		return text, true
	default:
		return "", false
	}
}
