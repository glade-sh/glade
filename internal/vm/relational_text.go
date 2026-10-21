package vm

import "strings"

func relationalTextValue(value Value) bool {
	return value.Kind == ValueString || value.Kind == ValueObject && strings.EqualFold(value.Type, "Id")
}
