package soql

import (
	"errors"
	"fmt"
)

// Bind syntax keeps dynamic QueryException text separate from the captured
// native Apex compiler's primary token diagnostic.
type soqlBindCompileError struct {
	err     error
	message string
}

func (e *soqlBindCompileError) Error() string { return e.err.Error() }
func (e *soqlBindCompileError) Unwrap() error { return e.err }
func soqlBindSyntaxError(token string) error {
	return &soqlBindCompileError{err: queryError("unexpected token: '"+token+"'", "invalid SOQL bind syntax"), message: fmt.Sprintf("Unexpected token '%s'.", token)}
}
func BindCompileDiagnosticMessage(err error) string {
	var bind *soqlBindCompileError
	if errors.As(err, &bind) {
		return bind.message
	}
	return ""
}
