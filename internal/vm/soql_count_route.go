package vm

import (
	"errors"
	"fmt"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// Invalid filters win over the scalar COUNT route rejection. Use the normal
// bind, literal and schema checks without executing the query or spending limits.
// Keep the caller's bind source, independent of any downstream bookkeeping map.
func (vm *VM) scalarCountRouteError(raw string, expand func(string) (string, error)) error {
	if query, err := vm.parseSOQLAt(raw); err == nil {
		if err := vm.validateSOQLSelection(query, true); err != nil {
			return err
		}
	}
	text, err := expand(raw)
	if err != nil {
		return newExceptionError("QueryException", fmt.Sprintf("%s in query %q", err, raw))
	}
	query, err := vm.parseSOQLAt(text)
	if err == nil {
		err = vm.validateSOQLSelection(query, false)
		if err != nil {
			return err
		}
		if vm.Org != nil {
			name, ok := storage.ResolveObjectName(*vm.Org, query.Object)
			if !ok {
				return newExceptionError("QueryException", soql.MissingObjectMessage(query.Object))
			}
			err = soql.ValidateReferences(*vm.Org, vm.Org.Objects[name].Definition, query)
		}
	}
	if err != nil {
		var queryErr *soql.QueryError
		if errors.As(err, &queryErr) {
			return soqlQueryException(queryErr, text)
		}
		return newExceptionError("QueryException", err.Error())
	}
	return newExceptionError("QueryException", "use countQuery() for [select count()...] queries")
}
