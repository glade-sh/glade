package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// Missing HAVING variables are resolved before countQuery rejects a grouped
// COUNT(field) query. This only establishes name availability; value-type and
// query-shape diagnostics retain their existing execution paths.
func (vm *VM) validateDynamicSOQLCountHavingNames(raw string) error {
	query, err := vm.parseSOQLAt(raw)
	if err != nil || query.Having == nil {
		return nil
	}
	var check func(*soql.Condition) error
	check = func(condition *soql.Condition) error {
		if condition == nil {
			return nil
		}
		for i := range condition.And {
			if err := check(&condition.And[i]); err != nil {
				return err
			}
		}
		for i := range condition.Or {
			if err := check(&condition.Or[i]); err != nil {
				return err
			}
		}
		values := append([]storage.Value{condition.Value, condition.Value2}, condition.Values...)
		for _, value := range values {
			if value.Kind != storage.ValueID || !strings.HasPrefix(string(value.ID), ":") {
				continue
			}
			name := string(value.ID)[1:]
			if name == "" || !isIdentStart(name[0]) {
				continue
			}
			plainName := true
			for i := 1; i < len(name); i++ {
				if !isIdentPart(name[i]) {
					plainName = false
					break
				}
			}
			if !plainName || isSOQLLiteralBind(name) {
				continue
			}
			if _, err := vm.lookupSOQLBind(name); err != nil && strings.HasPrefix(err.Error(), "unknown variable ") {
				return newExceptionError("QueryException", "Variable does not exist: "+name)
			}
		}
		return nil
	}
	return check(query.Having)
}
