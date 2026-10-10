package vm

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ResolveVisualforceControllerConstructor shares runtime overload selection with
// the page compiler, which checks the selected constructor's visibility next.
func (vm *VM) ResolveVisualforceControllerConstructor(className string, args []Value) (Method, bool, bool) {
	class, ok := vm.lookupClass(className)
	if !ok {
		return Method{}, false, false
	}
	return vm.matchConstructor(class, args)
}

// Keep the UI error's JSON projection stable while retaining the exception
// needed by the Visualforce page-action boundary (native action_throw).
func (err *UIActionError) Error() string {
	return err.Message
}

func (err *UIActionError) Unwrap() error {
	return err.cause
}

func (err *UIActionError) StackTraceString() string {
	var runtime *RuntimeError
	if !errors.As(err.cause, &runtime) {
		return ""
	}
	lines := make([]string, 0, len(runtime.Stack))
	for _, frame := range runtime.Stack {
		line := apexStackFrameSymbol(frame.Symbol)
		if frame.Line > 0 {
			line += fmt.Sprintf(": line %d", frame.Line)
			if frame.Column > 0 {
				column := frame.Column
				if strings.HasPrefix(line, "Class.") {
					// Visualforce reports Apex class locations at column 1,
					// even when the failing statement is later on the line.
					// Keep the precise runtime frame for debugger consumers.
					column = 1
				}
				line += fmt.Sprintf(", column %d", column)
			}
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// PageReferenceURL supplies the existing getUrl projection to VF navigation.
func PageReferenceURL(page Value) Value {
	return pageReferenceURL(page)
}

// Message cases at API 59/67 reject explicit null text arguments before
// the message can enter the request queue.
func newApexPagesMessage(args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 4 {
		return Null, errors.New("ApexPages.Message constructor expects severity, summary[, detail[, componentLabel]]")
	}
	if args[1].Kind == ValueNull {
		return Null, newExceptionError("NullPointerException", "Argument 2 cannot be null")
	}
	if len(args) >= 3 && args[2].Kind == ValueNull {
		return Null, newExceptionError("NullPointerException", "Argument 3 cannot be null")
	}
	message := Object("ApexPages.Message")
	message.Fields["severity"] = args[0]
	message.Fields["summary"] = args[1]
	if len(args) >= 3 {
		message.Fields["detail"] = args[2]
	}
	if len(args) == 4 {
		message.Fields["componentLabel"] = args[3]
	}
	return message, nil
}

// The regular serialization traversal still controls getter evaluation,
// transient fields, aliases and cycles. Only a reached ApexPages.Message
// contributes this failure; the non-serializing trace projection is unchanged.
type apexPagesMessageJSONFailure struct{}

func (apexPagesMessageJSONFailure) Error() string {
	return "Apex Type unsupported in JSON: ApexPages.Message"
}

func (failure apexPagesMessageJSONFailure) MarshalJSON() ([]byte, error) {
	return nil, failure
}

func apexPagesSerializationError(err error) error {
	var failure apexPagesMessageJSONFailure
	if errors.As(err, &failure) {
		return jsonDeserializeException("%s", failure.Error())
	}
	return jsonDeserializeException("%s", err.Error())
}

// Re-encode existing query values with the same form encoding used after a
// parameter-map mutation. Keep their original order, multiplicity and presence
// of '='; decoding a space or Unicode value does not reorder unrelated keys.
func canonicalPageReferenceQuery(query string) string {
	parts := strings.Split(query, "&")
	for i, part := range parts {
		key, value, assigned := strings.Cut(part, "=")
		decodedKey, keyErr := url.QueryUnescape(key)
		decodedValue, valueErr := url.QueryUnescape(value)
		if keyErr != nil || valueErr != nil {
			continue
		}
		parts[i] = url.QueryEscape(decodedKey)
		if assigned {
			parts[i] += "=" + url.QueryEscape(decodedValue)
		}
	}
	return strings.Join(parts, "&")
}
