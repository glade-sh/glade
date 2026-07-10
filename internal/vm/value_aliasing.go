package vm

import (
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

type sObjectFieldAliasLookupKey struct {
	Namespace string
	FieldName string
}

type mapKeyAliasKindCacheKey struct {
	MapType  string
	Kind     ValueKind
	TypeName string
}

type sObjectFieldAliasLookupCache struct {
	mu      sync.RWMutex
	entries map[sObjectFieldAliasLookupKey][]string
}

var mapKeyAliasKindCache sync.Map

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
	vm.recordCollectionMutation(updated.Ref)
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
	vm.recordCollectionMutation(updated.Ref)
	vm.propagateTopLevelCollectionAliases(vm.Globals, updated)
	localOnly := vm.localOnlyCollectionAlias(previous)
	if !localOnly {
		vm.propagateAliasSnapshotToScope(vm.Globals, previous, updated)
	}
	if !localOnly {
		vm.propagateAliasSnapshotToStatics(previous, updated)
	}
}
func (vm *VM) rememberLocalOnlyCollection(value Value) {
	if vm == nil || value.Ref == 0 || !mutableCollectionKind(value.Kind) {
		return
	}
	if vm.localOnlyCollectionRefs == nil {
		vm.localOnlyCollectionRefs = make(map[uint64]bool)
	}
	vm.localOnlyCollectionRefs[value.Ref] = true
}
func (vm *VM) markCollectionRefsEscaped(values ...Value) {
	if vm == nil || vm.localOnlyCollectionRefs == nil {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	for _, value := range values {
		vm.markCollectionRefsEscapedSeen(value, seen)
	}
}
func (vm *VM) markRootCollectionRefsEscaped(values ...Value) {
	if vm == nil || vm.localOnlyCollectionRefs == nil {
		return
	}
	for _, value := range values {
		if value.Ref != 0 && mutableCollectionKind(value.Kind) {
			delete(vm.localOnlyCollectionRefs, value.Ref)
		}
	}
}
func (vm *VM) markCollectionRefsEscapedSeen(value Value, seen map[uint64]bool) {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		if mutableCollectionKind(value.Kind) {
			delete(vm.localOnlyCollectionRefs, value.Ref)
		}
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			vm.markCollectionRefsEscapedSeen(child, seen)
		}
	case ValueMap:
		for _, child := range value.Map {
			vm.markCollectionRefsEscapedSeen(child, seen)
		}
		for _, child := range value.MapKeys {
			vm.markCollectionRefsEscapedSeen(child, seen)
		}
	case ValueList:
		for _, child := range value.List {
			vm.markCollectionRefsEscapedSeen(child, seen)
		}
	case ValueSet:
		for _, child := range value.Set {
			vm.markCollectionRefsEscapedSeen(child, seen)
		}
	}
}
func (vm *VM) localOnlyCollectionAlias(snapshot aliasSnapshot) bool {
	return vm != nil && snapshot.ref != 0 && mutableCollectionKind(snapshot.kind) && vm.localOnlyCollectionRefs != nil && vm.localOnlyCollectionRefs[snapshot.ref]
}
func mutableCollectionKind(kind ValueKind) bool {
	switch kind {
	case ValueList, ValueSet, ValueMap:
		return true
	default:
		return false
	}
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

func cloneValueDetachedPreserveRefs(value Value) Value {
	return cloneValueDetachedPreserveRefsSeen(value, make(map[uint64]Value))
}

func cloneValueDetachedPreserveRefsSeen(value Value, memo map[uint64]Value) Value {
	out := value
	if value.Ref != 0 && cloneDetachedRefKind(value.Kind) {
		if existing, ok := memo[value.Ref]; ok {
			return existing
		}
		out.Ref = newValueRef()
		out.Fields = nil
		out.Map = nil
		out.MapKeys = nil
		out.MapOrder = nil
		out.List = nil
		out.Set = nil
		if value.Fields != nil {
			out.Fields = make(map[string]Value, len(value.Fields))
		}
		if value.Map != nil {
			out.Map = make(map[string]Value, len(value.Map))
		}
		if value.MapKeys != nil {
			out.MapKeys = make(map[string]Value, len(value.MapKeys))
		}
		if value.MapOrder != nil {
			out.MapOrder = append([]string(nil), value.MapOrder...)
		}
		if value.List != nil {
			out.List = make([]Value, len(value.List))
		}
		if value.Set != nil {
			out.Set = make([]Value, len(value.Set))
		}
		memo[value.Ref] = out
	} else {
		if value.Fields != nil {
			out.Fields = make(map[string]Value, len(value.Fields))
		}
		if value.Map != nil {
			out.Map = make(map[string]Value, len(value.Map))
		}
		if value.MapKeys != nil {
			out.MapKeys = make(map[string]Value, len(value.MapKeys))
		}
		if value.MapOrder != nil {
			out.MapOrder = append([]string(nil), value.MapOrder...)
		}
		if value.List != nil {
			out.List = make([]Value, len(value.List))
		}
		if value.Set != nil {
			out.Set = make([]Value, len(value.Set))
		}
	}
	if value.Fields != nil {
		for key, child := range value.Fields {
			out.Fields[key] = cloneValueDetachedPreserveRefsSeen(child, memo)
		}
	}
	if value.Map != nil {
		for key, child := range value.Map {
			out.Map[key] = cloneValueDetachedPreserveRefsSeen(child, memo)
		}
	}
	if value.MapKeys != nil {
		for key, child := range value.MapKeys {
			out.MapKeys[key] = cloneValueDetachedPreserveRefsSeen(child, memo)
		}
	}
	if value.List != nil {
		for i, child := range value.List {
			out.List[i] = cloneValueDetachedPreserveRefsSeen(child, memo)
		}
	}
	if value.Set != nil {
		for i, child := range value.Set {
			out.Set[i] = cloneValueDetachedPreserveRefsSeen(child, memo)
		}
	}
	return out
}

func cloneDetachedRefKind(kind ValueKind) bool {
	switch kind {
	case ValueObject, ValueList, ValueSet, ValueMap:
		return true
	default:
		return false
	}
}

func snapshotAlias(value Value) aliasSnapshot {
	return aliasSnapshot{ref: value.Ref, kind: value.Kind, typeName: value.Type}
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
	if locations.empty() {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	locations.forEach(func(location staticFieldRef) {
		class, ok := vm.Classes[location.ClassName]
		if !ok || class.StaticFields == nil {
			return
		}
		field, ok := class.StaticFields[location.FieldName]
		if !ok {
			return
		}
		previousAlias := snapshotAlias(previous)
		if replaced, hint, ok := vm.replaceStaticAliasUsingDirectChildIndex(field.Value, location, previousAlias, updated); ok {
			field.Value = replaced
			class.StaticFields[location.FieldName] = field
			vm.Classes[location.ClassName] = class
			vm.rememberStaticAliasUpdateRefs(previousAlias, updated, location)
			vm.rememberStaticAliasDirectChildHint(previousAlias, updated, location, hint)
			return
		}
		if replaced, hint, ok := vm.replaceStaticAliasUsingChildHint(field.Value, location, previousAlias, updated); ok {
			field.Value = replaced
			class.StaticFields[location.FieldName] = field
			vm.Classes[location.ClassName] = class
			vm.rememberStaticAliasUpdateRefs(previousAlias, updated, location)
			vm.rememberStaticAliasChildHint(previousAlias, updated, location, hint)
			return
		}
		clearRefSeen(seen)
		replaced, changed, hint, hintOK := replaceAliasSnapshotWithStaticChildHint(field.Value, previousAlias, updated, seen)
		if !changed {
			vm.forgetStaticValueRefInField(previous.Ref, location)
			return
		}
		wasTopLevelAlias := field.Value.Ref != 0 && field.Value.Ref == previousAlias.ref && field.Value.Kind == previousAlias.kind
		previousFieldValue := field.Value
		field.Value = replaced
		class.StaticFields[location.FieldName] = field
		vm.Classes[location.ClassName] = class
		if wasTopLevelAlias {
			vm.rememberAdditionalStaticValueRefsInField(previousFieldValue, updated, location)
		} else {
			vm.rememberStaticAliasUpdateRefs(previousAlias, updated, location)
		}
		if hintOK {
			vm.rememberStaticAliasChildHint(previousAlias, updated, location, hint)
			vm.rememberStaticAliasDirectChildHint(previousAlias, updated, location, hint)
		}
	})
}
func (vm *VM) propagateAliasSnapshotToScope(scope map[string]Value, previous aliasSnapshot, updated Value) {
	recorder := vm.perfRecorder
	perfOn := recorder != nil
	var probe scopeAliasProbe
	var probePtr *scopeAliasProbe
	var perfStarted time.Time
	if perfOn {
		probePtr = &probe
		perfStarted = time.Now()
	}
	if !previous.valid() {
		if perfOn {
			recorder.recordScopeAliasProbe(probe, time.Since(perfStarted))
		}
		return
	}
	if vm.localOnlyCollectionAlias(previous) {
		if perfOn {
			probe.roots = uint64(len(scope))
			started := time.Now()
			probe.replacedRoots = uint64(propagateTopLevelAliasSnapshotToScopeCount(scope, previous, updated))
			probe.replacementDuration = time.Since(started)
		} else {
			propagateTopLevelAliasSnapshotToScope(scope, previous, updated)
		}
		if perfOn {
			recorder.recordScopeAliasProbe(probe, time.Since(perfStarted))
		}
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	for name, value := range scope {
		if perfOn {
			probe.roots++
		}
		var containmentStarted time.Time
		if perfOn {
			containmentStarted = time.Now()
		}
		if valueCannotContainAliasRef(value, previous.ref, previous.kind) {
			if perfOn {
				probe.containmentDuration += time.Since(containmentStarted)
			}
			continue
		}
		clearRefSeen(seen)
		contains := false
		if perfOn {
			contains = vm.valueContainsAliasRefCachedWithProbe(value, previous, seen, probePtr)
		} else {
			contains = vm.valueContainsAliasRefCached(value, previous, seen)
		}
		if perfOn {
			probe.containmentDuration += time.Since(containmentStarted)
		}
		if !contains {
			continue
		}
		clearRefSeen(seen)
		var replacementStarted time.Time
		if perfOn {
			replacementStarted = time.Now()
		}
		replaced, changed := replaceAliasSnapshot(value, previous, updated, seen)
		if perfOn {
			probe.replacementDuration += time.Since(replacementStarted)
		}
		if changed {
			scope[name] = replaced
			if perfOn {
				probe.replacedRoots++
			}
		}
	}
	if perfOn {
		recorder.recordScopeAliasProbe(probe, time.Since(perfStarted))
	}
}
func (vm *VM) propagateAliasSnapshotToStatics(previous aliasSnapshot, updated Value) {
	if !previous.valid() {
		return
	}
	if vm.localOnlyCollectionAlias(previous) {
		return
	}
	recorder := vm.perfRecorder
	perfOn := recorder != nil
	var started time.Time
	var locationVisits uint64
	var changedAny bool
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		var collectStarted time.Time
		if perfOn {
			collectStarted = time.Now()
		}
		vm.staticValueRefs, vm.staticValueRefFields = vm.collectStaticValueRefs()
		if perfOn {
			recorder.recordStaticAliasCollectPerf(time.Since(collectStarted))
		}
	}
	if !vm.staticValueRefs[previous.ref] {
		if perfOn {
			recorder.recordStaticAliasPerf(0, true, 0, false)
		}
		return
	}
	if perfOn {
		started = time.Now()
		defer func() {
			recorder.recordStaticAliasPerf(time.Since(started), false, locationVisits, changedAny)
		}()
	}
	locations := vm.staticValueRefFields[previous.ref]
	if locations.empty() {
		return
	}
	var seenPtr *map[uint64]bool
	var seen map[uint64]bool
	defer func() {
		if seenPtr != nil {
			aliasRefSetPool.Put(seenPtr)
		}
	}()
	locations.forEach(func(location staticFieldRef) {
		class, ok := vm.Classes[location.ClassName]
		if !ok || class.StaticFields == nil {
			return
		}
		field, ok := class.StaticFields[location.FieldName]
		if !ok {
			return
		}
		var fieldPerfName string
		var fieldPerfKind string
		var fieldPerfChildren int
		var fieldPerfStarted time.Time
		if perfOn {
			locationVisits++
			fieldPerfName = location.ClassName + "." + location.FieldName
			fieldPerfKind, fieldPerfChildren = staticAliasPerfValueShape(field.Value)
			fieldPerfStarted = time.Now()
		}
		recordFieldPerf := func(changed bool) {
			if perfOn {
				recorder.recordStaticAliasFieldPerf(fieldPerfName, fieldPerfKind, fieldPerfChildren, time.Since(fieldPerfStarted), changed)
			}
		}
		if field.Value.Ref != 0 && field.Value.Ref == previous.ref && field.Value.Kind == previous.kind {
			previousFieldValue := field.Value
			field.Value = updated
			class.StaticFields[location.FieldName] = field
			vm.Classes[location.ClassName] = class
			vm.rememberAdditionalStaticValueRefsInField(previousFieldValue, updated, location)
			recordFieldPerf(true)
			changedAny = true
			return
		}
		if replaced, hint, ok := vm.replaceStaticAliasUsingDirectChildIndex(field.Value, location, previous, updated); ok {
			field.Value = replaced
			class.StaticFields[location.FieldName] = field
			vm.Classes[location.ClassName] = class
			vm.rememberStaticAliasUpdateRefs(previous, updated, location)
			vm.rememberStaticAliasDirectChildHint(previous, updated, location, hint)
			recordFieldPerf(true)
			changedAny = true
			return
		}
		if replaced, hint, ok := vm.replaceStaticAliasUsingChildHint(field.Value, location, previous, updated); ok {
			field.Value = replaced
			class.StaticFields[location.FieldName] = field
			vm.Classes[location.ClassName] = class
			vm.rememberStaticAliasUpdateRefs(previous, updated, location)
			vm.rememberStaticAliasChildHint(previous, updated, location, hint)
			recordFieldPerf(true)
			changedAny = true
			return
		}
		if seenPtr == nil {
			seenPtr = aliasRefSetPool.Get().(*map[uint64]bool)
			seen = *seenPtr
		}
		clearRefSeen(seen)
		replaced, changed, hint, hintOK := replaceAliasSnapshotWithStaticChildHint(field.Value, previous, updated, seen)
		if !changed {
			vm.forgetStaticValueRefInField(previous.ref, location)
			recordFieldPerf(false)
			return
		}
		field.Value = replaced
		class.StaticFields[location.FieldName] = field
		vm.Classes[location.ClassName] = class
		vm.rememberStaticAliasUpdateRefs(previous, updated, location)
		if hintOK {
			vm.rememberStaticAliasChildHint(previous, updated, location, hint)
			vm.rememberStaticAliasDirectChildHint(previous, updated, location, hint)
		}
		recordFieldPerf(true)
		changedAny = true
	})
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
	return propagateTopLevelAliasSnapshotToScopeCount(scope, previous, updated) > 0
}

func propagateTopLevelAliasSnapshotToScopeCount(scope map[string]Value, previous aliasSnapshot, updated Value) int {
	if !previous.valid() || updated.Ref == 0 {
		return 0
	}
	changed := 0
	for name, value := range scope {
		if value.Ref == 0 || value.Ref != previous.ref || value.Kind != previous.kind {
			continue
		}
		replacement := updated
		replacement.Type = value.Type
		replacement.Static = value.Static
		replacement.Runtime = value.Runtime
		scope[name] = replacement
		changed++
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
	refsPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	refs := *refsPtr
	clear(refs)
	defer aliasRefSetPool.Put(refsPtr)
	for _, value := range scope {
		if value.Ref == 0 || value.Ref == updated.Ref || !sameBackingAliasRefreshKind(value.Kind) {
			continue
		}
		refs[value.Ref] = true
	}
	if len(refs) == 0 {
		return false
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	return valueContainsNestedRefInSet(updated, updated.Ref, refs, seen)
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
	vm.forgetStaticValueRefsInField(location)
	vm.collectStaticFieldValueRefsInField(value, location)
}
func (vm *VM) replaceStaticValueRefsInField(previous, value Value, location staticFieldRef) {
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		return
	}
	if sameStaticCollectionWriteback(previous, value) {
		vm.forgetStaticAliasDirectChildrenInField(location)
		return
	}
	vm.forgetStaticAliasChildHintsInField(location)
	vm.forgetStaticAliasDirectChildrenInField(location)
	vm.forgetStaticValueRefsFromValue(previous, location)
	vm.collectStaticFieldValueRefsInField(value, location)
}
func (vm *VM) rememberAdditionalStaticValueRefsInField(previous, value Value, location staticFieldRef) {
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		return
	}
	if sameStaticCollectionWriteback(previous, value) {
		vm.collectAdditionalStaticFieldValueRefsInField(value, location)
		return
	}
	if sameStaticCollectionRefSurface(previous, value) {
		return
	}
	vm.forgetStaticAliasChildHintsInField(location)
	vm.forgetStaticAliasDirectChildrenInField(location)
	vm.collectStaticFieldValueRefsInField(value, location)
}
func (vm *VM) rememberStaticAliasUpdateRefs(previous aliasSnapshot, updated Value, location staticFieldRef) {
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		return
	}
	vm.forgetStaticAliasChildHintsInField(location)
	if previous.valid() && updated.Ref == previous.ref && updated.Kind == previous.kind {
		seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
		seen := *seenPtr
		clear(seen)
		hasNested := valueHasNestedAliasRef(updated, updated.Ref, seen)
		aliasRefSetPool.Put(seenPtr)
		if !hasNested {
			return
		}
	}
	vm.collectStaticFieldValueRefsInField(updated, location)
}
func (vm *VM) collectAdditionalStaticFieldValueRefsInField(value Value, location staticFieldRef) {
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	collectAdditionalStaticFieldValueRefs(value, vm.staticValueRefs, vm.staticValueRefFields, location, seen, true)
}
func (vm *VM) collectStaticFieldValueRefsInField(value Value, location staticFieldRef) {
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	collectStaticFieldValueRefs(value, vm.staticValueRefs, vm.staticValueRefFields, location, seen)
}
func (vm *VM) forgetStaticValueRefsInField(location staticFieldRef) {
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		return
	}
	vm.forgetStaticAliasChildHintsInField(location)
	vm.forgetStaticAliasDirectChildrenInField(location)
	for ref := range vm.staticValueRefFields {
		vm.forgetStaticValueRefInField(ref, location)
	}
}
func (vm *VM) forgetStaticValueRefsFromValue(value Value, location staticFieldRef) {
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	vm.forgetStaticFieldValueRefs(value, location, seen)
}
func (vm *VM) forgetStaticFieldValueRefs(value Value, location staticFieldRef, seen map[uint64]bool) {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		vm.forgetStaticValueRefInField(value.Ref, location)
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			vm.forgetStaticFieldValueRefs(child, location, seen)
		}
	case ValueMap:
		for _, child := range value.Map {
			vm.forgetStaticFieldValueRefs(child, location, seen)
		}
		for _, child := range value.MapKeys {
			vm.forgetStaticFieldValueRefs(child, location, seen)
		}
	case ValueList:
		for _, child := range value.List {
			vm.forgetStaticFieldValueRefs(child, location, seen)
		}
	case ValueSet:
		for _, child := range value.Set {
			vm.forgetStaticFieldValueRefs(child, location, seen)
		}
	}
}
func (vm *VM) forgetStaticValueRefInField(ref uint64, location staticFieldRef) {
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil || ref == 0 {
		return
	}
	locations := vm.staticValueRefFields[ref]
	if !locations.remove(location) {
		return
	}
	if locations.empty() {
		delete(vm.staticValueRefFields, ref)
		delete(vm.staticValueRefs, ref)
		return
	}
	vm.staticValueRefFields[ref] = locations
}
func (vm *VM) invalidateStaticValueRefs() {
	vm.staticValueRefs = nil
	vm.staticValueRefFields = nil
	vm.staticAliasChildHints = nil
	vm.staticAliasDirectChildren = nil
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
func (vm *VM) collectStaticValueRefs() (map[uint64]bool, map[uint64]staticFieldRefSet) {
	refs := make(map[uint64]bool)
	fields := make(map[uint64]staticFieldRefSet)
	seen := make(map[uint64]bool)
	for className, class := range vm.Classes {
		for fieldName, field := range class.StaticFields {
			location := staticFieldRef{ClassName: className, FieldName: fieldName}
			clearRefSeen(seen)
			collectStaticFieldValueRefs(field.Value, refs, fields, location, seen)
		}
	}
	return refs, fields
}

type aliasContainmentCacheKey struct {
	ValueRef     uint64
	ValueKind    ValueKind
	ValueType    string
	PreviousRef  uint64
	PreviousKind ValueKind
	MutationSeq  uint64
}

func (vm *VM) recordCollectionMutation(ref uint64) {
	if vm == nil || ref == 0 {
		return
	}
	if vm.collectionRefMutationSeq == nil {
		vm.collectionRefMutationSeq = make(map[uint64]uint64)
	}
	vm.collectionRefMutationSeq[ref] = vm.collectionMutationSeq
	recorder := vm.perfRecorder
	if recorder != nil {
		recorder.recordScopeAliasMutationEpoch(vm.collectionMutationSeq)
	}
	if len(vm.aliasContainmentCache) > 16384 {
		if recorder != nil {
			recorder.recordScopeAliasContainmentCacheClear(len(vm.aliasContainmentCache))
		}
		clear(vm.aliasContainmentCache)
	}
}

func (vm *VM) collectionRefMutationVersion(ref uint64) uint64 {
	if vm == nil || ref == 0 || vm.collectionRefMutationSeq == nil {
		return 0
	}
	return vm.collectionRefMutationSeq[ref]
}

func (vm *VM) valueContainsAliasRefCached(value Value, previous aliasSnapshot, seen map[uint64]bool) bool {
	if !previous.valid() {
		return false
	}
	var cacheKey aliasContainmentCacheKey
	cacheable := false
	if value.Ref != 0 {
		if value.Ref == previous.ref && value.Kind == previous.kind {
			return true
		}
		if seen[value.Ref] {
			return false
		}
		seen[value.Ref] = true
		if cacheableAliasContainmentKind(value.Kind) {
			cacheable = true
			cacheKey = aliasContainmentCacheKey{
				ValueRef:     value.Ref,
				ValueKind:    value.Kind,
				ValueType:    firstAliasContainmentType(value),
				PreviousRef:  previous.ref,
				PreviousKind: previous.kind,
				MutationSeq:  vm.collectionRefMutationVersion(value.Ref),
			}
			if vm != nil && vm.aliasContainmentCache != nil && vm.aliasContainmentCache[cacheKey] {
				return false
			}
		}
	}
	if valueCannotContainAliasRef(value, previous.ref, previous.kind) {
		if cacheable {
			vm.rememberAliasContainmentMiss(cacheKey)
		}
		return false
	}
	found := false
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			if vm.valueContainsAliasRefCached(child, previous, seen) {
				found = true
				break
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if vm.valueContainsAliasRefCached(child, previous, seen) {
				found = true
				break
			}
		}
		if !found {
			for _, child := range value.MapKeys {
				if vm.valueContainsAliasRefCached(child, previous, seen) {
					found = true
					break
				}
			}
		}
	case ValueList:
		for _, child := range value.List {
			if vm.valueContainsAliasRefCached(child, previous, seen) {
				found = true
				break
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if vm.valueContainsAliasRefCached(child, previous, seen) {
				found = true
				break
			}
		}
	}
	if cacheable && !found {
		vm.rememberAliasContainmentMiss(cacheKey)
	}
	return found
}

func (vm *VM) valueContainsAliasRefCachedWithProbe(value Value, previous aliasSnapshot, seen map[uint64]bool, probe *scopeAliasProbe) bool {
	if probe != nil {
		probe.recursiveVisits++
	}
	if !previous.valid() {
		return false
	}
	var cacheKey aliasContainmentCacheKey
	cacheable := false
	if value.Ref != 0 {
		if value.Ref == previous.ref && value.Kind == previous.kind {
			return true
		}
		if seen[value.Ref] {
			return false
		}
		seen[value.Ref] = true
		if cacheableAliasContainmentKind(value.Kind) {
			cacheable = true
			cacheKey = aliasContainmentCacheKey{
				ValueRef:     value.Ref,
				ValueKind:    value.Kind,
				ValueType:    firstAliasContainmentType(value),
				PreviousRef:  previous.ref,
				PreviousKind: previous.kind,
				MutationSeq:  vm.collectionRefMutationVersion(value.Ref),
			}
			if vm != nil && vm.aliasContainmentCache != nil && vm.aliasContainmentCache[cacheKey] {
				if probe != nil {
					probe.containmentCacheHits++
				}
				return false
			}
			if probe != nil {
				probe.containmentCacheMisses++
			}
		}
	}
	if valueCannotContainAliasRef(value, previous.ref, previous.kind) {
		if cacheable {
			vm.rememberAliasContainmentMiss(cacheKey)
		}
		return false
	}
	found := false
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			if vm.valueContainsAliasRefCachedWithProbe(child, previous, seen, probe) {
				found = true
				break
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if vm.valueContainsAliasRefCachedWithProbe(child, previous, seen, probe) {
				found = true
				break
			}
		}
		if !found {
			for _, child := range value.MapKeys {
				if vm.valueContainsAliasRefCachedWithProbe(child, previous, seen, probe) {
					found = true
					break
				}
			}
		}
	case ValueList:
		for _, child := range value.List {
			if vm.valueContainsAliasRefCachedWithProbe(child, previous, seen, probe) {
				found = true
				break
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if vm.valueContainsAliasRefCachedWithProbe(child, previous, seen, probe) {
				found = true
				break
			}
		}
	}
	if cacheable && !found {
		vm.rememberAliasContainmentMiss(cacheKey)
	}
	return found
}

func firstAliasContainmentType(value Value) string {
	if strings.TrimSpace(value.Type) != "" {
		return value.Type
	}
	if strings.TrimSpace(value.Static) != "" {
		return value.Static
	}
	return value.Runtime
}

func (vm *VM) rememberAliasContainmentMiss(key aliasContainmentCacheKey) {
	if vm == nil || key.ValueRef == 0 || key.PreviousRef == 0 {
		return
	}
	if strings.TrimSpace(key.ValueType) == "" {
		return
	}
	if vm.aliasContainmentCache == nil {
		vm.aliasContainmentCache = make(map[aliasContainmentCacheKey]bool)
	}
	vm.aliasContainmentCache[key] = true
}

func cacheableAliasContainmentKind(kind ValueKind) bool {
	switch kind {
	case ValueList, ValueSet, ValueMap:
		return true
	default:
		return false
	}
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
func collectStaticFieldValueRefs(value Value, refs map[uint64]bool, fields map[uint64]staticFieldRefSet, location staticFieldRef, seen map[uint64]bool) {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		refs[value.Ref] = true
		locations := fields[value.Ref]
		locations.add(location)
		fields[value.Ref] = locations
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
func collectAdditionalStaticFieldValueRefs(value Value, refs map[uint64]bool, fields map[uint64]staticFieldRefSet, location staticFieldRef, seen map[uint64]bool, scanKnownChildren bool) {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		locations := fields[value.Ref]
		knownInField := refs[value.Ref] && locations.contains(location)
		if !knownInField {
			refs[value.Ref] = true
			locations.add(location)
			fields[value.Ref] = locations
		} else if !scanKnownChildren {
			return
		}
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			collectAdditionalStaticFieldValueRefs(child, refs, fields, location, seen, false)
		}
	case ValueMap:
		for _, child := range value.Map {
			collectAdditionalStaticFieldValueRefs(child, refs, fields, location, seen, false)
		}
		for _, child := range value.MapKeys {
			collectAdditionalStaticFieldValueRefs(child, refs, fields, location, seen, false)
		}
	case ValueList:
		for _, child := range value.List {
			collectAdditionalStaticFieldValueRefs(child, refs, fields, location, seen, false)
		}
	case ValueSet:
		for _, child := range value.Set {
			collectAdditionalStaticFieldValueRefs(child, refs, fields, location, seen, false)
		}
	}
}

type staticFieldRefSet struct {
	single    staticFieldRef
	hasSingle bool
	many      map[staticFieldRef]struct{}
}

func (s *staticFieldRefSet) add(location staticFieldRef) {
	if s.hasSingle {
		if s.single == location {
			return
		}
		if s.many == nil {
			s.many = make(map[staticFieldRef]struct{}, 1)
		}
		s.many[location] = struct{}{}
		return
	}
	if len(s.many) == 0 {
		s.single = location
		s.hasSingle = true
		return
	}
	s.many[location] = struct{}{}
}

func (s *staticFieldRefSet) remove(location staticFieldRef) bool {
	if s.hasSingle && s.single == location {
		s.single = staticFieldRef{}
		s.hasSingle = false
		return true
	}
	if len(s.many) == 0 {
		return false
	}
	if _, ok := s.many[location]; !ok {
		return false
	}
	delete(s.many, location)
	if len(s.many) == 0 {
		s.many = nil
	}
	return true
}

func (s staticFieldRefSet) contains(location staticFieldRef) bool {
	if s.hasSingle && s.single == location {
		return true
	}
	if len(s.many) == 0 {
		return false
	}
	_, ok := s.many[location]
	return ok
}

func (s staticFieldRefSet) empty() bool {
	return !s.hasSingle && len(s.many) == 0
}

func (s staticFieldRefSet) len() int {
	n := len(s.many)
	if s.hasSingle {
		n++
	}
	return n
}

func (s staticFieldRefSet) forEach(fn func(staticFieldRef)) {
	if s.hasSingle {
		fn(s.single)
	}
	for location := range s.many {
		fn(location)
	}
}

func (s staticFieldRefSet) locations() []staticFieldRef {
	out := make([]staticFieldRef, 0, s.len())
	s.forEach(func(location staticFieldRef) {
		out = append(out, location)
	})
	return out
}

type staticAliasChildHintKind string

const (
	staticAliasDirectMapChildIndexMinChildren                          = 64
	staticAliasChildHintObjectField           staticAliasChildHintKind = "objectField"
	staticAliasChildHintMapValue              staticAliasChildHintKind = "mapValue"
	staticAliasChildHintMapKey                staticAliasChildHintKind = "mapKey"
	staticAliasChildHintListIndex             staticAliasChildHintKind = "listIndex"
	staticAliasChildHintSetIndex              staticAliasChildHintKind = "setIndex"
)

type staticAliasChildHintKey struct {
	Ref      uint64
	Kind     ValueKind
	Location staticFieldRef
}

type staticAliasChildHint struct {
	Kind  staticAliasChildHintKind
	Name  string
	Key   string
	Index int
}

type staticAliasDirectChildKey struct {
	Ref  uint64
	Kind ValueKind
}

type staticAliasDirectChildIndex struct {
	RootRef    uint64
	RootKind   ValueKind
	ChildCount int
	Children   map[staticAliasDirectChildKey]staticAliasChildHint
}

func (vm *VM) staticAliasChildHintKey(previous aliasSnapshot, location staticFieldRef) staticAliasChildHintKey {
	return staticAliasChildHintKey{Ref: previous.ref, Kind: previous.kind, Location: location}
}

func (vm *VM) replaceStaticAliasUsingDirectChildIndex(value Value, location staticFieldRef, previous aliasSnapshot, updated Value) (Value, staticAliasChildHint, bool) {
	if vm == nil || !previous.valid() {
		return value, staticAliasChildHint{}, false
	}
	index, ok := vm.staticAliasDirectChildIndex(value, location)
	if !ok {
		return value, staticAliasChildHint{}, false
	}
	key := staticAliasDirectChildKey{Ref: previous.ref, Kind: previous.kind}
	hint, ok := index.Children[key]
	if !ok {
		return value, staticAliasChildHint{}, false
	}
	if hint.Kind == staticAliasChildHintMapKey && mapKeyTypeCannotContainAlias(value.Type, previous) {
		delete(index.Children, key)
		vm.staticAliasDirectChildren[location] = index
		return value, staticAliasChildHint{}, false
	}
	child, ok := staticAliasChildHintValue(value, hint)
	if !ok || child.Ref != previous.ref || child.Kind != previous.kind {
		delete(index.Children, key)
		vm.staticAliasDirectChildren[location] = index
		return value, staticAliasChildHint{}, false
	}
	if vm.perfRecorder != nil {
		vm.perfRecorder.recordStaticAliasDirectChildHit()
	}
	return replaceStaticAliasChildHintValue(value, hint, updated), hint, true
}

func (vm *VM) staticAliasDirectChildIndex(value Value, location staticFieldRef) (staticAliasDirectChildIndex, bool) {
	childCount := staticAliasDirectChildCount(value)
	if childCount == 0 {
		return staticAliasDirectChildIndex{}, false
	}
	if vm.staticAliasDirectChildren != nil {
		if index, ok := vm.staticAliasDirectChildren[location]; ok &&
			index.RootRef == value.Ref &&
			index.RootKind == value.Kind &&
			index.ChildCount == childCount {
			return index, true
		}
	}
	index := staticAliasDirectChildIndex{
		RootRef:    value.Ref,
		RootKind:   value.Kind,
		ChildCount: childCount,
		Children:   make(map[staticAliasDirectChildKey]staticAliasChildHint),
	}
	duplicates := make(map[staticAliasDirectChildKey]bool)
	add := func(child Value, hint staticAliasChildHint) {
		if child.Ref == 0 {
			return
		}
		key := staticAliasDirectChildKey{Ref: child.Ref, Kind: child.Kind}
		if duplicates[key] {
			return
		}
		if _, exists := index.Children[key]; exists {
			delete(index.Children, key)
			duplicates[key] = true
			return
		}
		index.Children[key] = hint
	}
	switch value.Kind {
	case ValueObject:
		for name, child := range value.Fields {
			add(child, staticAliasChildHint{Kind: staticAliasChildHintObjectField, Name: name})
		}
	case ValueMap:
		for key, child := range value.Map {
			add(child, staticAliasChildHint{Kind: staticAliasChildHintMapValue, Key: key})
		}
		for key, child := range value.MapKeys {
			add(child, staticAliasChildHint{Kind: staticAliasChildHintMapKey, Key: key})
		}
	default:
		return staticAliasDirectChildIndex{}, false
	}
	if vm.staticAliasDirectChildren == nil {
		vm.staticAliasDirectChildren = make(map[staticFieldRef]staticAliasDirectChildIndex)
	}
	vm.staticAliasDirectChildren[location] = index
	return index, true
}

func staticAliasDirectChildCount(value Value) int {
	switch value.Kind {
	case ValueObject:
		return len(value.Fields)
	case ValueMap:
		childCount := len(value.Map) + len(value.MapKeys)
		if childCount < staticAliasDirectMapChildIndexMinChildren {
			return 0
		}
		return childCount
	default:
		return 0
	}
}

func (vm *VM) rememberStaticAliasDirectChildHint(previous aliasSnapshot, updated Value, location staticFieldRef, hint staticAliasChildHint) {
	if vm == nil || !previous.valid() || vm.staticAliasDirectChildren == nil {
		return
	}
	if !staticAliasDirectChildHintKind(hint.Kind) {
		return
	}
	index, ok := vm.staticAliasDirectChildren[location]
	if !ok {
		return
	}
	previousKey := staticAliasDirectChildKey{Ref: previous.ref, Kind: previous.kind}
	if updated.Ref == 0 {
		delete(index.Children, previousKey)
		vm.staticAliasDirectChildren[location] = index
		return
	}
	updatedKey := staticAliasDirectChildKey{Ref: updated.Ref, Kind: updated.Kind}
	if updatedKey != previousKey {
		delete(vm.staticAliasDirectChildren, location)
		return
	}
	index.Children[updatedKey] = hint
	vm.staticAliasDirectChildren[location] = index
}

func staticAliasDirectChildHintKind(kind staticAliasChildHintKind) bool {
	switch kind {
	case staticAliasChildHintObjectField, staticAliasChildHintMapValue, staticAliasChildHintMapKey:
		return true
	default:
		return false
	}
}

func (vm *VM) replaceStaticAliasUsingChildHint(value Value, location staticFieldRef, previous aliasSnapshot, updated Value) (Value, staticAliasChildHint, bool) {
	if vm == nil || len(vm.staticAliasChildHints) == 0 || !previous.valid() {
		return value, staticAliasChildHint{}, false
	}
	key := vm.staticAliasChildHintKey(previous, location)
	hint, ok := vm.staticAliasChildHints[key]
	if !ok {
		return value, staticAliasChildHint{}, false
	}
	child, ok := staticAliasChildHintValue(value, hint)
	if !ok {
		delete(vm.staticAliasChildHints, key)
		return value, staticAliasChildHint{}, false
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	if !valueContainsAliasRef(child, previous.ref, previous.kind, seen) {
		delete(vm.staticAliasChildHints, key)
		return value, staticAliasChildHint{}, false
	}
	clearRefSeen(seen)
	replacedChild, changed := replaceAliasSnapshot(child, previous, updated, seen)
	if !changed {
		delete(vm.staticAliasChildHints, key)
		return value, staticAliasChildHint{}, false
	}
	if vm.perfRecorder != nil {
		vm.perfRecorder.recordStaticAliasChildHintHit()
	}
	return replaceStaticAliasChildHintValue(value, hint, replacedChild), hint, true
}

func (vm *VM) rememberStaticAliasChildHint(previous aliasSnapshot, updated Value, location staticFieldRef, hint staticAliasChildHint) {
	if vm == nil || !previous.valid() {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	key := vm.staticAliasChildHintKey(previous, location)
	if !valueContainsAliasRef(updated, previous.ref, previous.kind, seen) {
		if vm.staticAliasChildHints != nil {
			delete(vm.staticAliasChildHints, key)
		}
		return
	}
	if vm.staticAliasChildHints == nil {
		vm.staticAliasChildHints = make(map[staticAliasChildHintKey]staticAliasChildHint)
	}
	vm.staticAliasChildHints[key] = hint
}

func (vm *VM) forgetStaticAliasChildHintsInField(location staticFieldRef) {
	if vm == nil || len(vm.staticAliasChildHints) == 0 {
		return
	}
	for key := range vm.staticAliasChildHints {
		if key.Location == location {
			delete(vm.staticAliasChildHints, key)
		}
	}
}

func (vm *VM) forgetStaticAliasDirectChildrenInField(location staticFieldRef) {
	if vm == nil || len(vm.staticAliasDirectChildren) == 0 {
		return
	}
	delete(vm.staticAliasDirectChildren, location)
}

func staticAliasChildHintValue(value Value, hint staticAliasChildHint) (Value, bool) {
	switch hint.Kind {
	case staticAliasChildHintObjectField:
		if value.Kind != ValueObject || value.Fields == nil {
			return Value{}, false
		}
		child, ok := value.Fields[hint.Name]
		return child, ok
	case staticAliasChildHintMapValue:
		if value.Kind != ValueMap || value.Map == nil {
			return Value{}, false
		}
		child, ok := value.Map[hint.Key]
		return child, ok
	case staticAliasChildHintMapKey:
		if value.Kind != ValueMap || value.MapKeys == nil {
			return Value{}, false
		}
		child, ok := value.MapKeys[hint.Key]
		return child, ok
	case staticAliasChildHintListIndex:
		if value.Kind != ValueList || hint.Index < 0 || hint.Index >= len(value.List) {
			return Value{}, false
		}
		return value.List[hint.Index], true
	case staticAliasChildHintSetIndex:
		if value.Kind != ValueSet || hint.Index < 0 || hint.Index >= len(value.Set) {
			return Value{}, false
		}
		return value.Set[hint.Index], true
	default:
		return Value{}, false
	}
}

func replaceStaticAliasChildHintValue(value Value, hint staticAliasChildHint, child Value) Value {
	switch hint.Kind {
	case staticAliasChildHintObjectField:
		if value.Fields != nil {
			value.Fields[hint.Name] = child
		}
	case staticAliasChildHintMapValue:
		if value.Map != nil {
			value.Map[hint.Key] = child
		}
	case staticAliasChildHintMapKey:
		if value.MapKeys != nil {
			value.MapKeys[hint.Key] = child
		}
	case staticAliasChildHintListIndex:
		if hint.Index >= 0 && hint.Index < len(value.List) {
			value.List[hint.Index] = child
		}
	case staticAliasChildHintSetIndex:
		if hint.Index >= 0 && hint.Index < len(value.Set) {
			value.Set[hint.Index] = child
		}
	}
	return value
}

func replaceAliasSnapshotWithStaticChildHint(value Value, previous aliasSnapshot, updated Value, seen map[uint64]bool) (Value, bool, staticAliasChildHint, bool) {
	if !previous.valid() {
		return value, false, staticAliasChildHint{}, false
	}
	if value.Ref != 0 && value.Ref == previous.ref && value.Kind == previous.kind {
		return updated, true, staticAliasChildHint{}, false
	}
	changedCount := 0
	var hint staticAliasChildHint
	recordHint := func(next staticAliasChildHint) {
		changedCount++
		if changedCount == 1 {
			hint = next
		}
	}
	switch value.Kind {
	case ValueObject:
		for name, child := range value.Fields {
			if valueCannotContainAliasRef(child, previous.ref, previous.kind) {
				continue
			}
			replaced, childChanged := replaceValueAliasRef(child, previous, updated, seen)
			if childChanged {
				value.Fields[name] = replaced
				recordHint(staticAliasChildHint{Kind: staticAliasChildHintObjectField, Name: name})
			}
		}
	case ValueMap:
		for key, child := range value.Map {
			if valueCannotContainAliasRef(child, previous.ref, previous.kind) {
				continue
			}
			replaced, childChanged := replaceValueAliasRef(child, previous, updated, seen)
			if childChanged {
				value.Map[key] = replaced
				recordHint(staticAliasChildHint{Kind: staticAliasChildHintMapValue, Key: key})
			}
		}
		if !mapKeyTypeCannotContainAlias(value.Type, previous) {
			for key, child := range value.MapKeys {
				if valueCannotContainAliasRef(child, previous.ref, previous.kind) {
					continue
				}
				replaced, childChanged := replaceValueAliasRef(child, previous, updated, seen)
				if childChanged {
					value.MapKeys[key] = replaced
					recordHint(staticAliasChildHint{Kind: staticAliasChildHintMapKey, Key: key})
				}
			}
		}
	case ValueList:
		if listCannotContainAliasRef(value.List, previous.ref, previous.kind) {
			return value, false, staticAliasChildHint{}, false
		}
		for i, child := range value.List {
			replaced, childChanged := replaceValueAliasRef(child, previous, updated, seen)
			if childChanged {
				value.List[i] = replaced
				recordHint(staticAliasChildHint{Kind: staticAliasChildHintListIndex, Index: i})
			}
		}
	case ValueSet:
		if listCannotContainAliasRef(value.Set, previous.ref, previous.kind) {
			return value, false, staticAliasChildHint{}, false
		}
		for i, child := range value.Set {
			replaced, childChanged := replaceValueAliasRef(child, previous, updated, seen)
			if childChanged {
				value.Set[i] = replaced
				recordHint(staticAliasChildHint{Kind: staticAliasChildHintSetIndex, Index: i})
			}
		}
	default:
		replaced, changed := replaceAliasSnapshot(value, previous, updated, seen)
		return replaced, changed, staticAliasChildHint{}, false
	}
	return value, changedCount > 0, hint, changedCount == 1
}

func sameStaticCollectionWriteback(previous, value Value) bool {
	return previous.Ref != 0 &&
		previous.Ref == value.Ref &&
		previous.Kind == value.Kind &&
		mutableCollectionKind(value.Kind) &&
		len(previous.Fields) == 0 &&
		len(value.Fields) == 0
}

func sameStaticCollectionRefSurface(previous, value Value) bool {
	if previous.Ref == 0 || previous.Ref != value.Ref || previous.Kind != value.Kind || !mutableCollectionKind(value.Kind) {
		return false
	}
	switch value.Kind {
	case ValueList:
		if len(previous.List) != len(value.List) {
			return false
		}
		for i := range value.List {
			if !sameAliasRefKind(previous.List[i], value.List[i]) {
				return false
			}
		}
		return true
	case ValueSet:
		if len(previous.Set) != len(value.Set) {
			return false
		}
		for i := range value.Set {
			if !sameAliasRefKind(previous.Set[i], value.Set[i]) {
				return false
			}
		}
		return true
	case ValueMap:
		if len(previous.Map) != len(value.Map) || len(previous.MapKeys) != len(value.MapKeys) {
			return false
		}
		for key, child := range value.Map {
			previousChild, ok := previous.Map[key]
			if !ok || !sameAliasRefKind(previousChild, child) {
				return false
			}
		}
		for key, child := range value.MapKeys {
			previousChild, ok := previous.MapKeys[key]
			if !ok || !sameAliasRefKind(previousChild, child) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func sameAliasRefKind(left, right Value) bool {
	return left.Ref == right.Ref && left.Kind == right.Kind
}

func replaceCollectionAlias(value, previous, updated Value, seen map[uint64]bool) (Value, bool) {
	return replaceValueAlias(value, previous, updated, seen)
}
func replaceValueAlias(value, previous, updated Value, seen map[uint64]bool) (Value, bool) {
	if previous.Ref == 0 {
		return value, false
	}
	return replaceValueAliasRef(value, snapshotAlias(previous), updated, seen)
}
func replaceAliasSnapshot(value Value, previous aliasSnapshot, updated Value, seen map[uint64]bool) (Value, bool) {
	if !previous.valid() {
		return value, false
	}
	return replaceValueAliasRef(value, previous, updated, seen)
}
func replaceValueAliasRef(value Value, previous aliasSnapshot, updated Value, seen map[uint64]bool) (Value, bool) {
	if value.Ref != 0 {
		if value.Ref == previous.ref && value.Kind == previous.kind {
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
		for name, child := range value.Fields {
			if valueCannotContainAliasRef(child, previous.ref, previous.kind) {
				continue
			}
			replaced, childChanged := replaceValueAliasRef(child, previous, updated, seen)
			if childChanged {
				value.Fields[name] = replaced
				changed = true
			}
		}
	case ValueMap:
		for key, child := range value.Map {
			if valueCannotContainAliasRef(child, previous.ref, previous.kind) {
				continue
			}
			replaced, childChanged := replaceValueAliasRef(child, previous, updated, seen)
			if childChanged {
				value.Map[key] = replaced
				changed = true
			}
		}
		if mapKeyTypeCannotContainAlias(value.Type, previous) {
			return value, changed
		}
		for key, child := range value.MapKeys {
			if valueCannotContainAliasRef(child, previous.ref, previous.kind) {
				continue
			}
			replaced, childChanged := replaceValueAliasRef(child, previous, updated, seen)
			if childChanged {
				value.MapKeys[key] = replaced
				changed = true
			}
		}
	case ValueList:
		if listCannotContainAliasRef(value.List, previous.ref, previous.kind) {
			return value, false
		}
		for i, child := range value.List {
			replaced, childChanged := replaceValueAliasRef(child, previous, updated, seen)
			if childChanged {
				value.List[i] = replaced
				changed = true
			}
		}
	case ValueSet:
		if listCannotContainAliasRef(value.Set, previous.ref, previous.kind) {
			return value, false
		}
		for i, child := range value.Set {
			replaced, childChanged := replaceValueAliasRef(child, previous, updated, seen)
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
		for _, child := range value.Fields {
			if valueCannotContainAliasRef(child, previousRef, previousKind) {
				continue
			}
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if valueCannotContainAliasRef(child, previousRef, previousKind) {
				continue
			}
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
		if mapKeyTypeCannotContainAlias(value.Type, aliasSnapshot{ref: previousRef, kind: previousKind}) {
			return false
		}
		for _, child := range value.MapKeys {
			if valueCannotContainAliasRef(child, previousRef, previousKind) {
				continue
			}
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
func mapKeyTypeCannotContainAlias(mapType string, previous aliasSnapshot) bool {
	switch previous.kind {
	case ValueObject:
	case ValueList, ValueSet, ValueMap:
	default:
		return false
	}
	mapType = strings.TrimSpace(mapType)
	if mapType == "" {
		return false
	}
	typeName := ""
	if previous.kind == ValueObject {
		typeName = strings.TrimSpace(previous.typeName)
	}
	cacheKey := mapKeyAliasKindCacheKey{MapType: mapType, Kind: previous.kind, TypeName: typeName}
	if cached, ok := mapKeyAliasKindCache.Load(cacheKey); ok {
		return cached.(bool)
	}
	keyType, _, ok := mapTypeArgs(mapType)
	if !ok {
		mapKeyAliasKindCache.Store(cacheKey, false)
		return false
	}
	out := !declaredTypeCanContainAlias(keyType, previous)
	mapKeyAliasKindCache.Store(cacheKey, out)
	return out
}
func declaredTypeCanContainAlias(typeName string, previous aliasSnapshot) bool {
	if previous.kind == ValueObject {
		return declaredTypeCanContainObjectAlias(typeName, previous.typeName)
	}
	return declaredTypeCanContainAliasKind(typeName, previous.kind)
}
func declaredTypeCanContainAliasKind(typeName string, kind ValueKind) bool {
	typeName = strings.TrimSpace(typeName)
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	if typeName == "" || strings.EqualFold(typeName, "Object") || strings.EqualFold(typeName, "sObject") {
		return true
	}
	switch kind {
	case ValueList, ValueSet, ValueMap:
	default:
		return true
	}
	if collectionKindMatchesType(kind, typeName) {
		return true
	}
	if elementType, ok := collectionElementType(typeName); ok {
		return declaredTypeCanContainAliasKind(elementType, kind)
	}
	if keyType, valueType, ok := mapTypeArgs(typeName); ok {
		return declaredTypeCanContainAliasKind(keyType, kind) || declaredTypeCanContainAliasKind(valueType, kind)
	}
	return !scalarTypeCannotContainCollectionAlias(typeName)
}
func declaredTypeCanContainObjectAlias(typeName, previousType string) bool {
	typeName = strings.TrimSpace(typeName)
	previousType = strings.TrimSpace(previousType)
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	if rest, ok := stripLeadingSystemNamespace(previousType); ok {
		previousType = rest
	}
	if typeName == "" || previousType == "" || strings.EqualFold(typeName, "Object") || strings.EqualFold(typeName, "sObject") {
		return true
	}
	if strings.EqualFold(typeName, previousType) {
		return true
	}
	if elementType, ok := collectionElementType(typeName); ok {
		return declaredTypeCanContainObjectAlias(elementType, previousType)
	}
	if keyType, valueType, ok := mapTypeArgs(typeName); ok {
		return declaredTypeCanContainObjectAlias(keyType, previousType) || declaredTypeCanContainObjectAlias(valueType, previousType)
	}
	if scalarTypeCannotContainCollectionAlias(typeName) {
		return false
	}
	return true
}
func collectionKindMatchesType(kind ValueKind, typeName string) bool {
	switch kind {
	case ValueList:
		return collectionBase(typeName) == "List"
	case ValueSet:
		return collectionBase(typeName) == "Set"
	case ValueMap:
		return isMapType(typeName)
	default:
		return false
	}
}
func scalarTypeCannotContainCollectionAlias(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	switch strings.ToLower(typeName) {
	case "blob", "boolean", "bool", "currency", "date", "datetime", "decimal", "double", "id", "integer", "int", "long", "string", "time", "type", "url", "uuid":
		return true
	}
	return strings.HasPrefix(typeName, "Schema.")
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
	}
	if valueDeclaredTypesCannotContainAliasKind(value, previousKind) {
		return true
	}
	switch value.Kind {
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

func valueDeclaredTypesCannotContainAliasKind(value Value, previousKind ValueKind) bool {
	switch previousKind {
	case ValueList, ValueSet, ValueMap:
	default:
		return false
	}
	sawType := false
	sawType, canContain := declaredValueTypeCanContainAliasKind(value.Type, previousKind, sawType)
	if canContain {
		return false
	}
	sawType, canContain = declaredValueTypeCanContainAliasKind(value.Static, previousKind, sawType)
	if canContain {
		return false
	}
	sawType, canContain = declaredValueTypeCanContainAliasKind(value.Runtime, previousKind, sawType)
	return sawType && !canContain
}

func declaredValueTypeCanContainAliasKind(typeName string, previousKind ValueKind, sawType bool) (bool, bool) {
	if strings.TrimSpace(typeName) == "" {
		return sawType, false
	}
	return true, declaredTypeCanContainAliasKind(typeName, previousKind)
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
// large profiles showed findValueByRef at 184 s cum / 15 % CPU when this
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
	locations := vm.staticValueRefFields[ref]
	found := Null
	foundOK := false
	locations.forEach(func(location staticFieldRef) {
		if foundOK {
			return
		}
		class, ok := vm.Classes[location.ClassName]
		if !ok {
			return
		}
		field, ok := class.StaticFields[location.FieldName]
		if !ok {
			return
		}
		if field.Value.Ref == ref {
			found = field.Value
			foundOK = true
			return
		}
		if value, ok := findValueByRef(field.Value, ref, make(map[uint64]bool)); ok {
			found = value
			foundOK = true
			return
		}
	})
	return found, foundOK
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
