package vm

import "strings"

// The owned pagination observations cover the bind-map SYSTEM_MODE and USER_MODE overloads.
// Preserve the prior boundary for other pagination forms until observed.
func (vm *VM) scopeMinePaginationBoundary(raw string, capturedMode bool) error {
	query, err := vm.parseSOQLAt(raw)
	if err != nil || !strings.EqualFold(query.UsingScope, "mine") {
		return nil
	}
	if capturedMode && query.SecurityMode == "" {
		return nil
	}
	return &RuntimeError{Type: "UnsupportedFeature", Message: "soql: USING SCOPE " + query.UsingScope + " is not supported by the local SOQL runtime"}
}
