package vm

import (
	"errors"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

// These reference targets are available inside named Apex source without being
// installed org objects. Keep this lookup outside the object and describe-map
// caches: an anonymous caller regains its own visibility when a helper returns.
func (vm *VM) namedApexReferenceObjectName(name string) (string, bool) {
	canonical, ok := storage.ResolveNamedApexReferenceObjectName(name)
	if !ok || vm == nil || vm.currentClass == "" || vm.currentTrigger {
		return "", false
	}
	class, ok := vm.lookupClass(vm.currentClass)
	if !ok || class.PlatformType || methodHasModifier(class.Modifiers, AnonymousClassModifier) {
		return "", false
	}
	return canonical, true
}

// Only an exact source declaration shadows the new reference-object spelling;
// a short alias of an unrelated class must not suppress the Schema record.
func (vm *VM) namedApexReferenceClassShadow(name string) bool {
	class, ok := vm.lookupClass(name)
	return ok && !class.PlatformType && strings.EqualFold(runtimeClassName(class), name)
}

// Native API 62-66 exposes these types through reflection in named transactions.
// API 67 and Tooling executeAnonymous transactions return null, including calls
// made by named helpers within an anonymous transaction.
func (vm *VM) namedApexReferenceReflectionAvailable() bool {
	return vm != nil && !vm.toolingExecuteAnonymous &&
		apexversion.AtLeast(vm.currentMethod.APIVersion, 62) && apexversion.Before(vm.currentMethod.APIVersion, 67)
}

// Native static SOQL over these reference objects compiles only in named source.
// Named transactions at API 62-66 return no rows; API 67 and Tooling
// executeAnonymous transactions throw. Only the failed static query is
// reinterpreted, so ordinary queries keep their existing path and cost.
func (vm *VM) namedApexReferenceStaticQuery(raw string, err error) (Value, error) {
	var thrown *apexThrowError
	if !errors.As(err, &thrown) || !strings.EqualFold(strings.TrimPrefix(thrown.value.Type, "System."), "QueryException") {
		return Null, err
	}
	query, parseErr := vm.parseSOQLAt(raw)
	if parseErr != nil {
		expanded, expandErr := vm.expandSOQLBinds(raw)
		if expandErr != nil {
			return Null, err
		}
		if query, parseErr = vm.parseSOQLAt(expanded); parseErr != nil {
			return Null, err
		}
	}
	canonical, ok := storage.ResolveNamedApexReferenceObjectName(query.Object)
	if !ok {
		return Null, err
	}
	if _, defined := vm.resolveObjectName(query.Object); defined {
		return Null, err
	}
	if !vm.namedApexReferenceReflectionAvailable() {
		return Null, newExceptionError("QueryException", "sObject type '"+canonical+"' is not supported.")
	}
	if query.Count && len(query.GroupBy) == 0 && query.Having == nil {
		row := Object("AggregateResult")
		count := Int(0)
		count.Static = "Object"
		row.Fields["expr0"] = count
		out := List(row)
		out.Type = "List<AggregateResult>"
		tagSOQLQueryList(&out, raw)
		return out, nil
	}
	out := List()
	out.Type = "List<" + canonical + ">"
	tagSOQLQueryList(&out, raw)
	return out, nil
}
