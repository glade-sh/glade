package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// String put rejects an invalid numeric string and Boolean, and the token
// overload rejects String input. Numeric/blank strings on the string-name
// overload were not captured here.
func scaleZeroNumberDynamicPutError(definition storage.ObjectDefinition, field storage.Field, fieldArg, input Value) error {
	if !scaleZeroNumberCapturedField(definition, field) {
		return nil
	}
	if fieldArg.Kind == ValueObject && isSObjectFieldTokenType(fieldArg.Type) && input.Kind == ValueString {
		return sObjectIllegalAssignmentError(input, "Decimal")
	}
	if fieldArg.Kind != ValueString {
		return nil
	}
	if input.Kind == ValueBool {
		return sObjectIllegalAssignmentError(input, "Decimal")
	}
	if input.Kind == ValueString && strings.TrimSpace(input.Text) != "" {
		if _, err := decimalFromText(input.Text); err != nil {
			return sObjectIllegalAssignmentError(input, "Decimal")
		}
	}
	return nil
}
