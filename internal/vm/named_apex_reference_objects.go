package vm

import (
	"errors"
	"maps"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// These reference targets are available inside named Apex source without being
// installed org objects. Keep this lookup outside the object and describe-map
// caches: an anonymous caller regains its own visibility when a helper returns.
func (vm *VM) namedApexReferenceObjectName(name string) (string, bool) {
	canonical, ok := storage.ResolveNamedApexReferenceObjectName(name)
	if !ok || vm == nil || vm.currentClass == "" {
		return "", false
	}
	// A trigger body is named source: native sees the same reference targets
	// there as in a class. The trigger is not a class, so skip the class lookup.
	if vm.currentTrigger {
		return canonical, true
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

// Winter 27 hides these types through reflection at API 62-66 as well as 67+.
// The gate is scoped to the captured API range; other versions retain the
// previous behavior, including Tooling executeAnonymous transactions.
func (vm *VM) namedApexReferenceReflectionAvailable() bool {
	// Native trigger bodies resolve these targets through reflection at every
	// API (Type.forName and Schema.getGlobalDescribe are non-null).
	if vm != nil && vm.currentTrigger {
		return true
	}
	if vm != nil && apexversion.AtLeast(vm.currentMethod.APIVersion, 62) && apexversion.Before(vm.currentMethod.APIVersion, 67) {
		return false
	}
	return vm != nil && !vm.toolingExecuteAnonymous &&
		apexversion.AtLeast(vm.currentMethod.APIVersion, 62) && apexversion.Before(vm.currentMethod.APIVersion, 67)
}

// Native static SOQL over these reference objects compiles only in named source.
// API 62-67 and Tooling executeAnonymous transactions throw on Winter 27.
// Only the failed static query is
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
	if vm.currentTrigger {
		// A trigger body compiles the query and fails it at run time: the
		// scalar count() form with SecurityException, every other form with
		// the long unsupported-sObject QueryException.
		if query.Count && len(query.GroupBy) == 0 && query.Having == nil {
			return Null, newExceptionError("SecurityException", "Access to entity '"+canonical+"' denied")
		}
		return Null, newExceptionError("QueryException", soql.MissingObjectMessage(canonical))
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

// isNamedApexReferenceDescribe reports a describe of a named-source reference
// object that no org or standard definition replaces.
func (vm *VM) isNamedApexReferenceDescribe(objectName string) bool {
	if _, ok := storage.ResolveNamedApexReferenceObjectName(objectName); !ok {
		return false
	}
	if _, ok := storage.StandardObjectDefinition(objectName); ok {
		return false
	}
	if vm != nil && vm.Org != nil {
		if _, defined := vm.resolveObjectName(objectName); defined {
			return false
		}
	}
	return true
}

// namedGlobalDescribeExtension memoizes the extended global describe map for
// one base map. A raw write to the base cache replaces its backing map (the
// cache entry is cloned before the write), so the base map's identity and
// length validate the entry; the name list covers a change of caller.
type namedGlobalDescribeExtension struct {
	base     map[string]Value
	baseLen  int
	names    string
	extended map[string]Value
	// shared: extended is the generation's map, read by other VMs too.
	shared bool
}

// withNamedApexReferenceGlobalEntries adds the reference objects to the global
// describe map seen by named source (keySet, values and containsKey agree with
// get). Anonymous code keeps the base map. The extended map is shared across
// calls while the base map is unchanged, so it is marked shared: any write
// detaches before mutating. The caller fills the stored keys through the
// shared projection key cache.
func (vm *VM) withNamedApexReferenceGlobalEntries(base Value) (Value, bool) {
	var names []string
	for _, name := range storage.NamedApexReferenceObjectNames() {
		if canonical, ok := vm.namedApexReferenceObjectName(name); ok {
			if _, present := base.Map[mapKey(String(strings.ToLower(canonical)))]; !present {
				names = append(names, canonical)
			}
		}
	}
	if len(names) == 0 {
		return base, false
	}
	joined := strings.Join(names, ",")
	extended := base
	extended.mapBackingShared = true
	if cached := vm.namedGlobalDescribe; cached != nil && sameMapBacking(cached.base, base.Map) &&
		len(base.Map) == cached.baseLen && cached.names == joined {
		extended.Map = cached.extended
		extended.mapEntriesShared = cached.shared
		return extended, true
	}
	build := func() map[string]Value {
		built := maps.Clone(base.Map)
		for _, name := range names {
			built[mapKey(String(strings.ToLower(name)))] = sObjectTypeToken(name)
		}
		return built
	}
	shared, ok := vm.sharedGlobalDescribeExtension(base.Map, "named", joined, build)
	if !ok {
		shared = build()
	}
	extended.Map = shared
	extended.mapEntriesShared = ok
	vm.namedGlobalDescribe = &namedGlobalDescribeExtension{base: base.Map, baseLen: len(base.Map), names: joined, extended: extended.Map, shared: ok}
	return extended, true
}

// namedApexReferenceDynamicQueryFault rewrites the unknown-object QueryException
// of a dynamic query over a reference object. Named source reports the short
// message with the canonical name (every API 62-67); anonymous code keeps the
// long message that echoes the typed name.
func (vm *VM) namedApexReferenceDynamicQueryFault(value Value, err error) (Value, error) {
	if err == nil {
		return value, nil
	}
	var thrown *apexThrowError
	if !errors.As(err, &thrown) || !strings.EqualFold(strings.TrimPrefix(thrown.value.Type, "System."), "QueryException") {
		return value, err
	}
	const prefix = "sObject type '"
	message := ""
	if m, ok := thrown.value.Fields["message"]; ok {
		message = m.Text
	}
	if !strings.HasPrefix(message, prefix) {
		return value, err
	}
	end := strings.Index(message[len(prefix):], "'")
	if end < 0 {
		return value, err
	}
	canonical, ok := vm.namedApexReferenceObjectName(message[len(prefix) : len(prefix)+end])
	if !ok || vm.currentTrigger {
		return value, err
	}
	return Null, newExceptionError("QueryException", prefix+canonical+"' is not supported.")
}
