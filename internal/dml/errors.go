package dml

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func storageValuesEqual(field storage.Field, left, right storage.Value) bool {
	if left.Kind == storage.ValueString && right.Kind == storage.ValueString && !field.CaseSensitive {
		return strings.EqualFold(left.String, right.String)
	}
	if left.Kind != right.Kind {
		if left.Kind == storage.ValueID && right.Kind == storage.ValueString {
			return string(left.ID) == right.String
		}
		if left.Kind == storage.ValueString && right.Kind == storage.ValueID {
			return left.String == string(right.ID)
		}
		return false
	}
	switch left.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return left.String == right.String
	case storage.ValueDecimal:
		return left.Decimal == right.Decimal
	case storage.ValueInteger:
		return left.Integer == right.Integer
	case storage.ValueBoolean:
		return left.Boolean == right.Boolean
	case storage.ValueID:
		return left.ID == right.ID
	default:
		return false
	}
}

type dmlError struct {
	code    string
	fields  []string
	message string
}

func (e dmlError) Error() string {
	return e.message
}

func dmlErrorf(code string, fields []string, format string, args ...any) error {
	return dmlError{code: code, fields: append([]string(nil), fields...), message: fmt.Sprintf(format, args...)}
}

func customMetadataReadOnlyError(objectName string) error {
	return dmlErrorf("INVALID_TYPE", nil, "dml: custom metadata object %s is read-only", objectName)
}

func resultFromError(id storage.ID, err error) Result {
	var typed dmlError
	if errors.As(err, &typed) {
		return failedResult(id, typed.message, typed.code, typed.fields)
	}
	msg := err.Error()
	code := "FIELD_CUSTOM_VALIDATION_EXCEPTION"
	var fields []string
	switch {
	case contains(msg, "unknown object"):
		code = "INVALID_TYPE"
	case contains(msg, "unknown field"):
		code = "INVALID_FIELD_FOR_INSERT_UPDATE"
		fields = extractField(msg)
	case contains(msg, "required field"):
		code = "REQUIRED_FIELD_MISSING"
		fields = extractField(msg)
	case contains(msg, "duplicate id"):
		code = "DUPLICATE_VALUE"
		fields = []string{"Id"}
	case contains(msg, "deleted"):
		code = "ENTITY_IS_DELETED"
	case contains(msg, "update requires id") || contains(msg, "delete requires id") || contains(msg, "undelete requires id"):
		code = "MISSING_ARGUMENT"
		fields = []string{"Id"}
		if contains(msg, "update requires id") {
			msg = "Id not specified in an update call:"
		}
	case contains(msg, "does not exist"):
		code = "ENTITY_IS_DELETED"
	}
	return failedResult(id, msg, code, fields)
}

func failedResult(id storage.ID, message, statusCode string, fields []string) Result {
	copiedFields := append([]string(nil), fields...)
	return Result{
		ID:         id,
		Success:    false,
		Error:      message,
		StatusCode: statusCode,
		Fields:     copiedFields,
		Errors: []Error{{
			Message:    message,
			StatusCode: statusCode,
			Fields:     append([]string(nil), copiedFields...),
		}},
	}
}

func extractField(msg string) []string {
	// Extract field name from "dml: ... field Object.Field" or "dml: ... field Object.Field is null"
	parts := strings.Split(msg, ".")
	if len(parts) >= 2 {
		field := strings.TrimSpace(parts[len(parts)-1])
		field = strings.TrimSuffix(field, " is null")
		if field != "" {
			return []string{field}
		}
	}
	return nil
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), substr)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func copySequences(in map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
func formulaDefaultShouldEvaluate(field storage.Field, rawDefault string) bool {
	if rawDefault == "" {
		return false
	}
	switch field.Type {
	case storage.FieldDate, storage.FieldDateTime:
		return strings.ContainsAny(rawDefault, "()")
	default:
		return false
	}
}
