package vm

import (
	"reflect"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/storage"
)

type sObjectFieldAliasLookupKey struct {
	Namespace string
	FieldName string
}

type sObjectFieldAliasLookupCache struct {
	mu      sync.RWMutex
	entries map[sObjectFieldAliasLookupKey][]string
}

func newSObjectFieldAliasLookupCache() *sObjectFieldAliasLookupCache {
	return &sObjectFieldAliasLookupCache{entries: make(map[sObjectFieldAliasLookupKey][]string)}
}

func (c *sObjectFieldAliasLookupCache) load(key sObjectFieldAliasLookupKey) ([]string, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	aliases, ok := c.entries[key]
	return aliases, ok
}

func (c *sObjectFieldAliasLookupCache) store(key sObjectFieldAliasLookupKey, aliases []string) []string {
	if c == nil {
		return aliases
	}
	c.mu.Lock()
	c.entries[key] = aliases
	c.mu.Unlock()
	return aliases
}

func (vm *VM) populatedFieldsMapAliasLookup(receiver, key Value) (Value, bool) {
	if receiver.Kind != ValueMap || key.Kind != ValueString || !strings.HasPrefix(receiver.Runtime, "sobject-populated-fields:") {
		return Null, false
	}
	namespace := ""
	if vm != nil && vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	if namespace == "" {
		return Null, false
	}
	for rawKey, value := range receiver.Map {
		candidate := valueFromMapKey(rawKey)
		if candidate.Kind == ValueString && schemaDescribeMapKeyMatches(namespace, candidate.Text, key.Text) {
			return value, true
		}
	}
	return Null, false
}
func populatedFieldsMapAllowsAliasContains(receiver Value) bool {
	if receiver.Kind != ValueMap || receiver.Fields == nil {
		return false
	}
	marker, ok := receiver.Fields[sobjectPopulatedFieldsAliasContainsField]
	return ok && marker.Kind == ValueBool && marker.Bool
}
func (vm *VM) sObjectFieldMapLookupAliases(fieldName string) []string {
	namespace := ""
	if vm != nil && vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	cacheKey := sObjectFieldAliasLookupKey{Namespace: namespace, FieldName: fieldName}
	if vm != nil {
		if vm.sObjectFieldAliasCache == nil {
			vm.sObjectFieldAliasCache = newSObjectFieldAliasLookupCache()
		}
		if aliases, ok := vm.sObjectFieldAliasCache.load(cacheKey); ok {
			return aliases
		}
	}
	seen := map[string]bool{}
	var aliases []string
	add := func(alias string) {
		alias = strings.TrimSpace(alias)
		if alias == "" || seen[alias] {
			return
		}
		seen[alias] = true
		aliases = append(aliases, alias)
		if lowered := strings.ToLower(alias); lowered != alias && !seen[lowered] {
			seen[lowered] = true
			aliases = append(aliases, lowered)
		}
	}
	add(fieldName)
	if dot := strings.LastIndex(fieldName, "."); dot >= 0 && dot+1 < len(fieldName) {
		add(fieldName[dot+1:])
	}
	if namespace != "" {
		for _, alias := range append([]string(nil), aliases...) {
			add(storage.NamespaceTokenName(namespace, alias))
			add(storage.StripNamespaceToken(namespace, alias))
		}
	}
	for _, alias := range append([]string(nil), aliases...) {
		add(stripAnyNamespaceToken(alias))
	}
	if vm != nil {
		return vm.sObjectFieldAliasCache.store(cacheKey, aliases)
	}
	return aliases
}
func (vm *VM) propagateCollectionMutation(previous, updated Value) {
	if !sameCollectionType(previous, updated) {
		return
	}
	vm.collectionMutationSeq++
	vm.propagateValueMutationToScope(vm.Globals, previous, updated)
	vm.propagateValueMutationToStatics(previous, updated)
}
func (vm *VM) propagateCollectionMutationFromSnapshot(previous aliasSnapshot, updated Value) {
	if !previous.valid() {
		return
	}
	switch updated.Kind {
	case ValueList, ValueSet, ValueMap:
	default:
		return
	}
	vm.collectionMutationSeq++
	vm.propagateTopLevelCollectionAliases(vm.Globals, updated)
	vm.propagateAliasSnapshotToScope(vm.Globals, previous, updated)
	vm.propagateAliasSnapshotToStatics(previous, updated)
}
func (vm *VM) propagateCollectionMutationToScope(scope map[string]Value, previous, updated Value) {
	vm.propagateValueMutationToScope(scope, previous, updated)
}
func (vm *VM) propagateValueMutationToScope(scope map[string]Value, previous, updated Value) {
	if sameAliasValue(previous, updated) {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	for name, value := range scope {
		clearRefSeen(seen)
		replaced, changed := replaceValueAlias(value, previous, updated, seen)
		if changed {
			scope[name] = replaced
		}
	}
}
func (vm *VM) propagateUpdatedValueAliases(scope map[string]Value, updated Value) {
	if len(scope) == 0 {
		return
	}
	topLevelAliasesPtr := aliasRefSliceMapPool.Get().(*map[uint64][]string)
	topLevelAliases := *topLevelAliasesPtr
	clear(topLevelAliases)
	defer func() {
		// Drop large slices so the pool entry stays compact.
		for k, v := range topLevelAliases {
			if cap(v) > 16 {
				delete(topLevelAliases, k)
			}
		}
		aliasRefSliceMapPool.Put(topLevelAliasesPtr)
	}()
	for name, value := range scope {
		if value.Ref != 0 && value.Ref != updated.Ref {
			topLevelAliases[value.Ref] = append(topLevelAliases[value.Ref], name)
		}
	}
	if len(topLevelAliases) == 0 {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	var walk func(Value, bool)
	walk = func(value Value, root bool) {
		if value.Ref != 0 {
			if seen[value.Ref] {
				return
			}
			seen[value.Ref] = true
			if !root {
				for _, name := range topLevelAliases[value.Ref] {
					if merged, ok := mergeRicherApexMocksAlias(scope[name], value); ok {
						scope[name] = merged
						continue
					}
					scope[name] = value
				}
			}
		}
		switch value.Kind {
		case ValueObject:
			for _, child := range value.Fields {
				walk(child, false)
			}
		case ValueMap:
			for _, child := range value.Map {
				walk(child, false)
			}
			for _, child := range value.MapKeys {
				walk(child, false)
			}
		case ValueList:
			for _, child := range value.List {
				walk(child, false)
			}
		case ValueSet:
			for _, child := range value.Set {
				walk(child, false)
			}
		}
	}
	walk(updated, true)
}
func valueHasNestedAlias(value Value) bool {
	if value.Ref == 0 {
		seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
		seen := *seenPtr
		clear(seen)
		defer aliasRefSetPool.Put(seenPtr)
		return valueHasRef(value, seen)
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	return valueHasNestedAliasRef(value, value.Ref, seen)
}
func mergeRicherApexMocksAlias(existing, replacement Value) (Value, bool) {
	if existing.Kind != ValueObject || replacement.Kind != ValueObject || existing.Ref == 0 || existing.Ref != replacement.Ref {
		return Null, false
	}
	if !strings.EqualFold(frameworkMockSupportType(existing.Type), "ApexMocks") ||
		!strings.EqualFold(frameworkMockSupportType(replacement.Type), "ApexMocks") {
		return Null, false
	}
	if len(existing.Fields) <= len(replacement.Fields) {
		return Null, false
	}
	merged := existing
	if replacement.Type != "" {
		merged.Type = replacement.Type
	}
	if replacement.Static != "" {
		merged.Static = replacement.Static
	}
	if replacement.Runtime != "" {
		merged.Runtime = replacement.Runtime
	}
	if merged.Fields == nil {
		merged.Fields = make(map[string]Value, len(replacement.Fields))
	}
	for key, value := range replacement.Fields {
		merged.Fields[key] = value
	}
	return merged, true
}
func cloneValuePreserveRefs(value Value) Value {
	return cloneValuePreserveRefsSeen(value, make(map[uint64]bool))
}
func snapshotAlias(value Value) aliasSnapshot {
	return aliasSnapshot{ref: value.Ref, kind: value.Kind}
}
func cloneValuePreserveRefsSeen(value Value, seen map[uint64]bool) Value {
	out := value
	if value.Ref != 0 {
		if seen[value.Ref] {
			return out
		}
		seen[value.Ref] = true
		defer delete(seen, value.Ref)
	}
	if value.Fields != nil {
		out.Fields = make(map[string]Value, len(value.Fields))
		for key, child := range value.Fields {
			out.Fields[key] = cloneValuePreserveRefsSeen(child, seen)
		}
	}
	if value.Map != nil {
		out.Map = make(map[string]Value, len(value.Map))
		for key, child := range value.Map {
			out.Map[key] = cloneValuePreserveRefsSeen(child, seen)
		}
	}
	if value.MapKeys != nil {
		out.MapKeys = make(map[string]Value, len(value.MapKeys))
		for key, child := range value.MapKeys {
			out.MapKeys[key] = cloneValuePreserveRefsSeen(child, seen)
		}
	}
	if value.MapOrder != nil {
		out.MapOrder = append([]string(nil), value.MapOrder...)
	}
	if value.List != nil {
		out.List = make([]Value, len(value.List))
		for i, child := range value.List {
			out.List[i] = cloneValuePreserveRefsSeen(child, seen)
		}
	}
	if value.Set != nil {
		out.Set = make([]Value, len(value.Set))
		for i, child := range value.Set {
			out.Set[i] = cloneValuePreserveRefsSeen(child, seen)
		}
	}
	return out
}
func (vm *VM) propagateCollectionMutationToStatics(previous, updated Value) {
	vm.propagateValueMutationToStatics(previous, updated)
}
func (vm *VM) propagateValueMutationToStatics(previous, updated Value) {
	if previous.Ref == 0 {
		return
	}
	if sameAliasValue(previous, updated) {
		return
	}
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		vm.staticValueRefs, vm.staticValueRefFields = vm.collectStaticValueRefs()
	}
	if !vm.staticValueRefs[previous.Ref] {
		return
	}
	locations := vm.staticValueRefFields[previous.Ref]
	if len(locations) == 0 {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	for _, location := range locations {
		class, ok := vm.Classes[location.ClassName]
		if !ok || class.StaticFields == nil {
			continue
		}
		field, ok := class.StaticFields[location.FieldName]
		if !ok {
			continue
		}
		clearRefSeen(seen)
		replaced, changed := replaceValueAlias(field.Value, previous, updated, seen)
		if !changed {
			continue
		}
		field.Value = replaced
		class.StaticFields[location.FieldName] = field
		vm.Classes[location.ClassName] = class
		vm.rememberStaticValueRefsInField(updated, location)
	}
}
func (vm *VM) propagateAliasSnapshotToScope(scope map[string]Value, previous aliasSnapshot, updated Value) {
	if !previous.valid() {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	for name, value := range scope {
		clearRefSeen(seen)
		replaced, changed := replaceAliasSnapshot(value, previous, updated, seen)
		if changed {
			scope[name] = replaced
		}
	}
}
func (vm *VM) propagateAliasSnapshotToStatics(previous aliasSnapshot, updated Value) {
	if !previous.valid() {
		return
	}
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		vm.staticValueRefs, vm.staticValueRefFields = vm.collectStaticValueRefs()
	}
	if !vm.staticValueRefs[previous.ref] {
		return
	}
	locations := vm.staticValueRefFields[previous.ref]
	if len(locations) == 0 {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	for _, location := range locations {
		class, ok := vm.Classes[location.ClassName]
		if !ok || class.StaticFields == nil {
			continue
		}
		field, ok := class.StaticFields[location.FieldName]
		if !ok {
			continue
		}
		clearRefSeen(seen)
		replaced, changed := replaceAliasSnapshot(field.Value, previous, updated, seen)
		if !changed {
			continue
		}
		field.Value = replaced
		class.StaticFields[location.FieldName] = field
		vm.Classes[location.ClassName] = class
		vm.rememberStaticValueRefsInField(updated, location)
	}
}
func (vm *VM) propagateAliasSnapshotMutationToScope(scope map[string]Value, previous aliasSnapshot, original, updated Value, refreshNestedCollections bool) bool {
	if !scopeHasAnyRef(scope) {
		return false
	}
	if sameAliasListCollectionViewOnly(original, updated) {
		return false
	}
	if sameAliasRuntimeBacking(original, updated) {
		if refreshNestedCollections && scopeHasNestedCollectionAliasNeedingRefresh(scope, updated) {
			vm.propagateUpdatedValueAliases(scope, updated)
		}
		if propagateStaleTopLevelAliasSnapshotToScope(scope, previous, updated) {
			return true
		}
		return false
	}
	if sameAliasRuntimeData(original, updated) || sameAliasRuntimeDataWithCallerCollectionView(original, updated) {
		if valueHasNestedAliasRef(original, previous.ref, make(map[uint64]bool)) {
			vm.propagateUpdatedValueAliases(scope, updated)
		}
		return false
	}
	if refreshNestedCollections && sameBackingAliasRefreshKind(updated.Kind) && vm.propagateTopLevelCollectionAliases(scope, updated) {
		return true
	}
	if refreshNestedCollections && sameBackingAliasRefreshKind(updated.Kind) && vm.propagateCollectionValueAliasToScope(scope, original, updated) {
		return true
	}
	if propagateTopLevelAliasSnapshotToScope(scope, previous, updated) {
		return true
	}
	vm.propagateAliasSnapshotToScope(scope, previous, updated)
	return true
}
func propagateTopLevelAliasSnapshotToScope(scope map[string]Value, previous aliasSnapshot, updated Value) bool {
	if !previous.valid() || updated.Ref == 0 {
		return false
	}
	changed := false
	for name, value := range scope {
		if value.Ref == 0 || value.Ref != previous.ref || value.Kind != previous.kind {
			continue
		}
		replacement := updated
		replacement.Type = value.Type
		replacement.Static = value.Static
		replacement.Runtime = value.Runtime
		scope[name] = replacement
		changed = true
	}
	return changed
}

func propagateStaleTopLevelAliasSnapshotToScope(scope map[string]Value, previous aliasSnapshot, updated Value) bool {
	if !previous.valid() || updated.Ref == 0 {
		return false
	}
	changed := false
	for name, value := range scope {
		if value.Ref == 0 || value.Ref != previous.ref || value.Kind != previous.kind {
			continue
		}
		if sameAliasRuntimeBacking(value, updated) || sameAliasRuntimeData(value, updated) || sameAliasRuntimeDataWithCallerCollectionView(value, updated) {
			continue
		}
		replacement := updated
		replacement.Type = value.Type
		replacement.Static = value.Static
		replacement.Runtime = value.Runtime
		scope[name] = replacement
		changed = true
	}
	return changed
}

func (vm *VM) propagateTopLevelCollectionAliases(scope map[string]Value, updated Value) bool {
	if updated.Ref == 0 {
		return false
	}
	changed := false
	for name, value := range scope {
		if value.Ref == 0 || value.Ref != updated.Ref || value.Kind != updated.Kind {
			continue
		}
		replacement := updated
		replacement.Type = value.Type
		replacement.Static = value.Static
		replacement.Runtime = value.Runtime
		scope[name] = replacement
		changed = true
	}
	return changed
}
func (vm *VM) propagateCollectionValueAliasToScope(scope map[string]Value, original, updated Value) bool {
	if original.Ref == 0 || original.Ref != updated.Ref || original.Kind != updated.Kind {
		return false
	}
	if sameAliasValue(original, updated) {
		return false
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	changed := false
	for name, value := range scope {
		clearRefSeen(seen)
		replaced, replacedValue := replaceValueAlias(value, original, updated, seen)
		if replacedValue {
			scope[name] = replaced
			changed = true
		}
	}
	return changed
}
func sameAliasRuntimeBacking(original, updated Value) bool {
	if original.Ref == 0 || original.Ref != updated.Ref || original.Kind != updated.Kind ||
		original.Type != updated.Type || original.Text != updated.Text ||
		original.Static != updated.Static || original.Runtime != updated.Runtime {
		return false
	}
	switch original.Kind {
	case ValueObject:
		return sameMapBacking(original.Fields, updated.Fields)
	case ValueMap:
		return sameMapBacking(original.Map, updated.Map) &&
			sameMapBacking(original.MapKeys, updated.MapKeys) &&
			sameSliceBacking(original.MapOrder, updated.MapOrder)
	case ValueList:
		return sameSliceBacking(original.List, updated.List)
	case ValueSet:
		return sameSliceBacking(original.Set, updated.Set)
	default:
		return false
	}
}
func sameMapBacking[K comparable, V any](left, right map[K]V) bool {
	if len(left) != len(right) {
		return false
	}
	if len(left) == 0 {
		return (left == nil) == (right == nil)
	}
	return reflect.ValueOf(left).Pointer() == reflect.ValueOf(right).Pointer()
}
func sameSliceBacking[T any](left, right []T) bool {
	if len(left) != len(right) {
		return false
	}
	if len(left) == 0 {
		return (left == nil) == (right == nil)
	}
	return reflect.ValueOf(left).Pointer() == reflect.ValueOf(right).Pointer()
}
func scopeHasNestedCollectionAliasNeedingRefresh(scope map[string]Value, updated Value) bool {
	if updated.Ref == 0 || len(scope) == 0 {
		return false
	}
	refs := make(map[uint64]bool)
	for _, value := range scope {
		if value.Ref == 0 || value.Ref == updated.Ref || !sameBackingAliasRefreshKind(value.Kind) {
			continue
		}
		refs[value.Ref] = true
	}
	if len(refs) == 0 {
		return false
	}
	return valueContainsNestedRefInSet(updated, updated.Ref, refs, make(map[uint64]bool))
}
func sameBackingAliasRefreshKind(kind ValueKind) bool {
	switch kind {
	case ValueList, ValueSet, ValueMap:
		return true
	default:
		return false
	}
}
func valueContainsNestedRefInSet(value Value, rootRef uint64, refs map[uint64]bool, seen map[uint64]bool) bool {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return false
		}
		seen[value.Ref] = true
		if value.Ref != rootRef && refs[value.Ref] {
			return true
		}
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			if valueContainsNestedRefInSet(child, rootRef, refs, seen) {
				return true
			}
		}
	case ValueList:
		for _, child := range value.List {
			if valueContainsNestedRefInSet(child, rootRef, refs, seen) {
				return true
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if valueContainsNestedRefInSet(child, rootRef, refs, seen) {
				return true
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if valueContainsNestedRefInSet(child, rootRef, refs, seen) {
				return true
			}
		}
		for _, child := range value.MapKeys {
			if valueContainsNestedRefInSet(child, rootRef, refs, seen) {
				return true
			}
		}
	}
	return false
}

// scopeHasAnyRef returns true if any binding in scope carries a non-zero
// reference id. Most scopes during expression evaluation contain only
// primitives or null; this lets us bail before invoking the deep
// sameAliasRuntimeData comparison.
func scopeHasAnyRef(scope map[string]Value) bool {
	for _, value := range scope {
		if value.Ref != 0 {
			return true
		}
	}
	return false
}
func sameAliasListCollectionViewOnly(original, updated Value) bool {
	if original.Ref == 0 || original.Ref != updated.Ref || original.Kind != ValueList || updated.Kind != ValueList {
		return false
	}
	if strings.EqualFold(original.Type, updated.Type) || collectionBase(original.Type) != "List" || collectionBase(updated.Type) != "List" {
		return false
	}
	if len(original.List) != len(updated.List) {
		return false
	}
	return true
}
func sameAliasRuntimeDataWithCallerCollectionView(original, updated Value) bool {
	if original.Ref == 0 || original.Ref != updated.Ref || original.Kind != updated.Kind {
		return false
	}
	switch original.Kind {
	case ValueList, ValueSet, ValueMap:
	default:
		return false
	}
	callerView := updated
	callerView.Type = original.Type
	return sameAliasRuntimeData(original, callerView)
}
func (vm *VM) rememberStaticValueRefs(value Value) {
	if vm.staticValueRefs == nil {
		return
	}
	collectValueRefs(value, vm.staticValueRefs, make(map[uint64]bool))
}
func (vm *VM) rememberStaticValueRefsInField(value Value, location staticFieldRef) {
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		return
	}
	collectStaticFieldValueRefs(value, vm.staticValueRefs, vm.staticValueRefFields, location, make(map[uint64]bool))
}
func (vm *VM) invalidateStaticValueRefs() {
	vm.staticValueRefs = nil
	vm.staticValueRefFields = nil
}
func (vm *VM) invalidateStaticValueRefsForChange(previous, updated Value) {
	if vm.staticValueRefs == nil {
		return
	}
	vm.invalidateStaticValueRefs()
}
func valueHasRef(value Value, seen map[uint64]bool) bool {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return false
		}
		return true
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			if valueHasRef(child, seen) {
				return true
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if valueHasRef(child, seen) {
				return true
			}
		}
		for _, child := range value.MapKeys {
			if valueHasRef(child, seen) {
				return true
			}
		}
	case ValueList:
		for _, child := range value.List {
			if valueHasRef(child, seen) {
				return true
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if valueHasRef(child, seen) {
				return true
			}
		}
	}
	return false
}
func (vm *VM) collectStaticValueRefs() (map[uint64]bool, map[uint64][]staticFieldRef) {
	refs := make(map[uint64]bool)
	fields := make(map[uint64][]staticFieldRef)
	seen := make(map[uint64]bool)
	for className, class := range vm.Classes {
		for fieldName, field := range class.StaticFields {
			clearRefSeen(seen)
			collectStaticFieldValueRefs(field.Value, refs, fields, staticFieldRef{ClassName: className, FieldName: fieldName}, seen)
		}
	}
	return refs, fields
}
func collectValueRefs(value Value, refs, seen map[uint64]bool) {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		refs[value.Ref] = true
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			collectValueRefs(child, refs, seen)
		}
	case ValueMap:
		for _, child := range value.Map {
			collectValueRefs(child, refs, seen)
		}
		for _, child := range value.MapKeys {
			collectValueRefs(child, refs, seen)
		}
	case ValueList:
		for _, child := range value.List {
			collectValueRefs(child, refs, seen)
		}
	case ValueSet:
		for _, child := range value.Set {
			collectValueRefs(child, refs, seen)
		}
	}
}
func collectStaticFieldValueRefs(value Value, refs map[uint64]bool, fields map[uint64][]staticFieldRef, location staticFieldRef, seen map[uint64]bool) {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		refs[value.Ref] = true
		fields[value.Ref] = appendStaticFieldRef(fields[value.Ref], location)
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			collectStaticFieldValueRefs(child, refs, fields, location, seen)
		}
	case ValueMap:
		for _, child := range value.Map {
			collectStaticFieldValueRefs(child, refs, fields, location, seen)
		}
		for _, child := range value.MapKeys {
			collectStaticFieldValueRefs(child, refs, fields, location, seen)
		}
	case ValueList:
		for _, child := range value.List {
			collectStaticFieldValueRefs(child, refs, fields, location, seen)
		}
	case ValueSet:
		for _, child := range value.Set {
			collectStaticFieldValueRefs(child, refs, fields, location, seen)
		}
	}
}
func appendStaticFieldRef(locations []staticFieldRef, location staticFieldRef) []staticFieldRef {
	for _, existing := range locations {
		if existing == location {
			return locations
		}
	}
	return append(locations, location)
}
func replaceCollectionAlias(value, previous, updated Value, seen map[uint64]bool) (Value, bool) {
	return replaceValueAlias(value, previous, updated, seen)
}
func replaceValueAlias(value, previous, updated Value, seen map[uint64]bool) (Value, bool) {
	if previous.Ref == 0 {
		return value, false
	}
	return replaceValueAliasRef(value, previous.Ref, previous.Kind, updated, seen)
}
func replaceAliasSnapshot(value Value, previous aliasSnapshot, updated Value, seen map[uint64]bool) (Value, bool) {
	if !previous.valid() {
		return value, false
	}
	return replaceValueAliasRef(value, previous.ref, previous.kind, updated, seen)
}
func replaceValueAliasRef(value Value, previousRef uint64, previousKind ValueKind, updated Value, seen map[uint64]bool) (Value, bool) {
	if value.Ref != 0 {
		if value.Ref == previousRef && value.Kind == previousKind {
			return updated, true
		}
		if seen[value.Ref] {
			return value, false
		}
		seen[value.Ref] = true
	}
	changed := false
	switch value.Kind {
	case ValueObject:
		if objectFieldsCannotContainAliasRef(value.Fields, previousRef, previousKind) {
			return value, false
		}
		for name, child := range value.Fields {
			replaced, childChanged := replaceValueAliasRef(child, previousRef, previousKind, updated, seen)
			if childChanged {
				value.Fields[name] = replaced
				changed = true
			}
		}
	case ValueMap:
		if mapCannotContainAliasRef(value, previousRef, previousKind) {
			return value, false
		}
		for key, child := range value.Map {
			replaced, childChanged := replaceValueAliasRef(child, previousRef, previousKind, updated, seen)
			if childChanged {
				value.Map[key] = replaced
				changed = true
			}
		}
		for key, child := range value.MapKeys {
			replaced, childChanged := replaceValueAliasRef(child, previousRef, previousKind, updated, seen)
			if childChanged {
				value.MapKeys[key] = replaced
				changed = true
			}
		}
	case ValueList:
		if listCannotContainAliasRef(value.List, previousRef, previousKind) {
			return value, false
		}
		for i, child := range value.List {
			replaced, childChanged := replaceValueAliasRef(child, previousRef, previousKind, updated, seen)
			if childChanged {
				value.List[i] = replaced
				changed = true
			}
		}
	case ValueSet:
		if listCannotContainAliasRef(value.Set, previousRef, previousKind) {
			return value, false
		}
		for i, child := range value.Set {
			replaced, childChanged := replaceValueAliasRef(child, previousRef, previousKind, updated, seen)
			if childChanged {
				value.Set[i] = replaced
				changed = true
			}
		}
	}
	return value, changed
}
func valueContainsAliasRef(value Value, previousRef uint64, previousKind ValueKind, seen map[uint64]bool) bool {
	if previousRef == 0 {
		return false
	}
	if value.Ref != 0 {
		if value.Ref == previousRef && value.Kind == previousKind {
			return true
		}
		if seen[value.Ref] {
			return false
		}
		seen[value.Ref] = true
	}
	switch value.Kind {
	case ValueObject:
		if objectFieldsCannotContainAliasRef(value.Fields, previousRef, previousKind) {
			return false
		}
		for _, child := range value.Fields {
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
	case ValueMap:
		if mapCannotContainAliasRef(value, previousRef, previousKind) {
			return false
		}
		for _, child := range value.Map {
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
		for _, child := range value.MapKeys {
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
	case ValueList:
		if listCannotContainAliasRef(value.List, previousRef, previousKind) {
			return false
		}
		for _, child := range value.List {
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
	case ValueSet:
		if listCannotContainAliasRef(value.Set, previousRef, previousKind) {
			return false
		}
		for _, child := range value.Set {
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
	}
	return false
}
func listCannotContainObjectRef(values []Value, previousRef uint64) bool {
	return listCannotContainAliasRef(values, previousRef, ValueObject)
}
func listCannotContainAliasRef(values []Value, previousRef uint64, previousKind ValueKind) bool {
	if previousRef == 0 || len(values) == 0 {
		return false
	}
	for _, value := range values {
		if valueCannotContainAliasRef(value, previousRef, previousKind) {
			continue
		}
		if value.Kind != previousKind || value.Ref == 0 {
			return false
		}
		if value.Ref == previousRef {
			return false
		}
	}
	return true
}
func objectFieldsCannotContainObjectRef(fields map[string]Value, previousRef uint64) bool {
	return objectFieldsCannotContainAliasRef(fields, previousRef, ValueObject)
}
func objectFieldsCannotContainAliasRef(fields map[string]Value, previousRef uint64, previousKind ValueKind) bool {
	if previousRef == 0 || len(fields) == 0 {
		return false
	}
	for _, value := range fields {
		if !valueCannotContainAliasRef(value, previousRef, previousKind) {
			return false
		}
	}
	return true
}
func mapCannotContainObjectRef(value Value, previousRef uint64) bool {
	return mapCannotContainAliasRef(value, previousRef, ValueObject)
}
func mapCannotContainAliasRef(value Value, previousRef uint64, previousKind ValueKind) bool {
	if previousRef == 0 || len(value.Map)+len(value.MapKeys) == 0 {
		return false
	}
	for _, child := range value.Map {
		if !valueCannotContainAliasRef(child, previousRef, previousKind) {
			return false
		}
	}
	for _, child := range value.MapKeys {
		if !valueCannotContainAliasRef(child, previousRef, previousKind) {
			return false
		}
	}
	return true
}
func valueCannotContainObjectRef(value Value, previousRef uint64) bool {
	return valueCannotContainAliasRef(value, previousRef, ValueObject)
}
func valueCannotContainAliasRef(value Value, previousRef uint64, previousKind ValueKind) bool {
	if value.Ref == previousRef && value.Kind == previousKind {
		return false
	}
	switch value.Kind {
	case ValueNull, ValueInt, ValueDecimal, ValueBool, ValueString:
		return true
	case ValueObject:
		return value.Ref != previousRef && len(value.Fields) == 0
	case ValueList:
		return value.Ref != previousRef && len(value.List) == 0
	case ValueSet:
		return value.Ref != previousRef && len(value.Set) == 0
	case ValueMap:
		return value.Ref != previousRef && len(value.Map)+len(value.MapKeys) == 0
	default:
		return false
	}
}
func collectionAliasMatch(left, right Value) bool {
	return valueAliasMatch(left, right)
}
func sameAliasValue(left, right Value) bool {
	if left.Ref == 0 || left.Ref != right.Ref || left.Kind != right.Kind {
		return false
	}
	seenPtr := aliasPairSetPool.Get().(*map[[2]uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasPairSetPool.Put(seenPtr)
	return sameAliasContent(left, right, seen)
}
func sameAliasContent(left, right Value, seen map[[2]uint64]bool) bool {
	if left.Kind != right.Kind || left.Type != right.Type || left.Text != right.Text || left.Static != right.Static || left.Runtime != right.Runtime {
		return false
	}
	if left.Ref != 0 && right.Ref != 0 {
		key := [2]uint64{left.Ref, right.Ref}
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	switch left.Kind {
	case ValueObject:
		if len(left.Fields) != len(right.Fields) {
			return false
		}
		for name, leftValue := range left.Fields {
			rightValue, ok := right.Fields[name]
			if !ok || !sameAliasContent(leftValue, rightValue, seen) {
				return false
			}
		}
		return true
	case ValueList:
		if len(left.List) != len(right.List) {
			return false
		}
		for i := range left.List {
			if !sameAliasContent(left.List[i], right.List[i], seen) {
				return false
			}
		}
		return true
	case ValueSet:
		if len(left.Set) != len(right.Set) {
			return false
		}
		rightValues := append([]Value(nil), right.Set...)
		for _, leftValue := range left.Set {
			match := -1
			for i, rightValue := range rightValues {
				if sameAliasContent(leftValue, rightValue, seen) {
					match = i
					break
				}
			}
			if match < 0 {
				return false
			}
			rightValues = append(rightValues[:match], rightValues[match+1:]...)
		}
		return true
	case ValueMap:
		if len(left.Map) != len(right.Map) || len(left.MapKeys) != len(right.MapKeys) {
			return false
		}
		for key, leftValue := range left.Map {
			rightValue, ok := right.Map[key]
			if !ok || !sameAliasContent(leftValue, rightValue, seen) {
				return false
			}
		}
		for key, leftValue := range left.MapKeys {
			rightValue, ok := right.MapKeys[key]
			if !ok || !sameAliasContent(leftValue, rightValue, seen) {
				return false
			}
		}
		return true
	default:
		return left.Equal(right)
	}
}
func sameAliasRuntimeData(left, right Value) bool {
	seenPtr := aliasPairSetPool.Get().(*map[[2]uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasPairSetPool.Put(seenPtr)
	return sameAliasRuntimeContent(left, right, seen)
}
func sameAliasRuntimeContent(left, right Value, seen map[[2]uint64]bool) bool {
	if left.Kind != right.Kind || left.Type != right.Type || left.Text != right.Text ||
		left.Int != right.Int || left.Decimal != right.Decimal || left.Bool != right.Bool {
		return false
	}
	if left.Ref != 0 && right.Ref != 0 && left.Ref != right.Ref {
		return false
	}
	if left.Ref != 0 && right.Ref != 0 {
		key := [2]uint64{left.Ref, right.Ref}
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	switch left.Kind {
	case ValueObject:
		if len(left.Fields) != len(right.Fields) {
			return false
		}
		for name, leftValue := range left.Fields {
			rightValue, ok := right.Fields[name]
			if !ok || !sameAliasRuntimeContent(leftValue, rightValue, seen) {
				return false
			}
		}
		return true
	case ValueList:
		if len(left.List) != len(right.List) {
			return false
		}
		for i := range left.List {
			if !sameAliasRuntimeContent(left.List[i], right.List[i], seen) {
				return false
			}
		}
		return true
	case ValueSet:
		if len(left.Set) != len(right.Set) {
			return false
		}
		rightValues := append([]Value(nil), right.Set...)
		for _, leftValue := range left.Set {
			match := -1
			for i, rightValue := range rightValues {
				if sameAliasRuntimeContent(leftValue, rightValue, seen) {
					match = i
					break
				}
			}
			if match < 0 {
				return false
			}
			rightValues = append(rightValues[:match], rightValues[match+1:]...)
		}
		return true
	case ValueMap:
		if len(left.Map) != len(right.Map) || len(left.MapKeys) != len(right.MapKeys) {
			return false
		}
		for key, leftValue := range left.Map {
			rightValue, ok := right.Map[key]
			if !ok || !sameAliasRuntimeContent(leftValue, rightValue, seen) {
				return false
			}
		}
		for key, leftValue := range left.MapKeys {
			rightValue, ok := right.MapKeys[key]
			if !ok || !sameAliasRuntimeContent(leftValue, rightValue, seen) {
				return false
			}
		}
		return true
	default:
		return left.Equal(right)
	}
}
func valueHasNestedAliasRef(value Value, rootRef uint64, seen map[uint64]bool) bool {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return false
		}
		seen[value.Ref] = true
		if value.Ref != rootRef {
			return true
		}
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			if valueHasNestedAliasRef(child, rootRef, seen) {
				return true
			}
		}
	case ValueList:
		for _, child := range value.List {
			if valueHasNestedAliasRef(child, rootRef, seen) {
				return true
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if valueHasNestedAliasRef(child, rootRef, seen) {
				return true
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if valueHasNestedAliasRef(child, rootRef, seen) {
				return true
			}
		}
		for _, child := range value.MapKeys {
			if valueHasNestedAliasRef(child, rootRef, seen) {
				return true
			}
		}
	}
	return false
}
func clearRefSeen(seen map[uint64]bool) {
	for ref := range seen {
		delete(seen, ref)
	}
}
func valueAliasMatch(left, right Value) bool {
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case ValueObject, ValueList, ValueSet, ValueMap:
		return left.Ref != 0 && left.Ref == right.Ref
	default:
		return false
	}
}
func valueAliasSnapshotMatch(left aliasSnapshot, right Value) bool {
	return left.valid() && left.kind == right.Kind && left.ref == right.Ref
}
func sameCollectionType(left, right Value) bool {
	if left.Kind != right.Kind || !strings.EqualFold(left.Type, right.Type) {
		return false
	}
	switch left.Kind {
	case ValueList, ValueSet, ValueMap:
		return true
	default:
		return false
	}
}

// staticFieldValueByRef consults the per-VM reverse index built by
// collectStaticValueRefs. It avoids scanning every class for every lookup;
// nams profiles showed findValueByRef at 184 s cum / 15 % CPU when this
// walked all classes × all static fields per ref.
func (vm *VM) staticFieldValueByRef(ref uint64) (Value, bool) {
	if ref == 0 {
		return Null, false
	}
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		vm.staticValueRefs, vm.staticValueRefFields = vm.collectStaticValueRefs()
	}
	if !vm.staticValueRefs[ref] {
		return Null, false
	}
	for _, location := range vm.staticValueRefFields[ref] {
		class, ok := vm.Classes[location.ClassName]
		if !ok {
			continue
		}
		field, ok := class.StaticFields[location.FieldName]
		if !ok {
			continue
		}
		if field.Value.Ref == ref {
			return field.Value, true
		}
		if value, ok := findValueByRef(field.Value, ref, make(map[uint64]bool)); ok {
			return value, true
		}
	}
	return Null, false
}
func (vm *VM) scanStaticFieldValueByRef(ref uint64) (Value, bool) {
	for _, class := range vm.Classes {
		for _, field := range class.StaticFields {
			if field.Value.Ref == ref {
				return field.Value, true
			}
			if value, ok := findValueByRef(field.Value, ref, make(map[uint64]bool)); ok {
				return value, true
			}
		}
	}
	return Null, false
}
func (vm *VM) liveScopes() []map[string]Value {
	scopes := make([]map[string]Value, 0, len(vm.scopeStack)+1)
	scopes = append(scopes, vm.Globals)
	for i := len(vm.scopeStack) - 1; i >= 0; i-- {
		scopes = append(scopes, vm.scopeStack[i])
	}
	return scopes
}
func directValueByRefInScope(scope map[string]Value, ref uint64) (Value, bool) {
	for _, value := range scope {
		if value.Ref == ref {
			return value, true
		}
	}
	return Null, false
}
func findValueByRefInScope(scope map[string]Value, ref uint64, seen map[uint64]bool) (Value, bool) {
	for _, value := range scope {
		if found, ok := findValueByRef(value, ref, seen); ok {
			return found, true
		}
	}
	return Null, false
}
