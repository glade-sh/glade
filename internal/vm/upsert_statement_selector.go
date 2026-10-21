package vm

import "strings"

// Resolve qualified statement selectors through the same metadata-backed
// token as Database.upsert.
func (vm *VM) upsertStatementSelectorField(value Value, selector string) string {
	parts := strings.Split(selector, ".")
	if len(parts) != 2 && (len(parts) != 3 || !strings.EqualFold(parts[1], "Fields")) {
		return selector
	}
	if value.Kind == ValueList && len(value.List) == 0 {
		return selector
	}
	token, ok := vm.lookupSObjectFieldToken(parts)
	if !ok {
		return selector
	}
	targets := []Value{value}
	if value.Kind == ValueList {
		targets = value.List
	}
	for _, target := range targets {
		owner, known := vm.describeObjectTokenName(vm.canonicalSObjectValueType(target))
		if !known || !strings.EqualFold(owner, token.Fields["object"].Text) {
			return selector
		}
	}
	return token.Fields["field"].Text
}
