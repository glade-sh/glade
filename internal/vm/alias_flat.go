package vm

const (
	flatObjectAliasIndexMaxEntries   = 64
	flatObjectAliasIndexMaxSlots     = 1 << 16
	flatObjectAliasIndexMaxListSize  = 1 << 14
	flatObjectAliasIndexMaxTypeBytes = 256
)

type flatObjectAliasIndexKey struct {
	ref      uint64
	first    *Value
	length   int
	typeName string
}

type flatObjectAliasIndex struct {
	positionsByRef map[uint64]int
	// Only duplicated identities allocate a positions slice.
	additionalPositions map[uint64][]int
	slots               int
}

func flatObjectAliasKey(value Value) (flatObjectAliasIndexKey, bool) {
	if value.Kind != ValueList || value.Ref == 0 || len(value.List) == 0 || len(value.List) > flatObjectAliasIndexMaxListSize || len(value.Type) > flatObjectAliasIndexMaxTypeBytes {
		return flatObjectAliasIndexKey{}, false
	}
	return flatObjectAliasIndexKey{ref: value.Ref, first: &value.List[0], length: len(value.List), typeName: value.Type}, true
}

func (vm *VM) tryIndexedFlatObjectAliasRoot(value Value, previous aliasSnapshot, updated Value, probe *scopeAliasProbe) (handled bool, changed bool) {
	if vm == nil || previous.kind != ValueObject || !previous.valid() {
		return false, false
	}
	key, ok := flatObjectAliasKey(value)
	if !ok {
		return false, false
	}
	if updated.Kind != ValueObject {
		vm.clearFlatObjectAliasIndexes()
		return false, false
	}
	if updated.Ref != previous.ref {
		vm.clearFlatObjectAliasIndexes()
	}
	if probe != nil {
		probe.shortcutVisits++
	}
	index := vm.flatAliasIndex[key]
	if index == nil {
		index, ok = buildFlatObjectAliasIndex(value, probe)
		if !ok || !vm.storeFlatObjectAliasIndex(key, index) {
			return false, false
		}
	}
	if !flatObjectAliasValueSafe(updated, probe) {
		vm.clearFlatObjectAliasIndexes()
		return false, false
	}
	first, found := index.positionsByRef[previous.ref]
	other := index.additionalPositions[previous.ref]
	if probe != nil {
		probe.shortcutVisits++
	}
	if !found {
		return true, false
	}
	count := 1 + len(other)
	for n := 0; n < count; n++ {
		position := first
		if n > 0 {
			position = other[n-1]
		}
		if probe != nil {
			probe.shortcutVisits++
		}
		if position < 0 || position >= len(value.List) || value.List[position].Kind != ValueObject || value.List[position].Ref != previous.ref {
			vm.deleteFlatObjectAliasIndex(key)
			return false, false
		}
	}
	for n := 0; n < count; n++ {
		position := first
		if n > 0 {
			position = other[n-1]
		}
		value.List[position] = updated
		if probe != nil {
			probe.shortcutVisits++
		}
	}
	if updated.Ref != previous.ref {
		delete(index.positionsByRef, previous.ref)
		delete(index.additionalPositions, previous.ref)
		if updated.Ref != 0 {
			if _, exists := index.positionsByRef[updated.Ref]; !exists {
				index.positionsByRef[updated.Ref] = first
				if len(other) > 0 {
					if index.additionalPositions == nil {
						index.additionalPositions = make(map[uint64][]int)
					}
					index.additionalPositions[updated.Ref] = other
				}
			} else {
				if index.additionalPositions == nil {
					index.additionalPositions = make(map[uint64][]int)
				}
				positions := append(index.additionalPositions[updated.Ref], first)
				index.additionalPositions[updated.Ref] = append(positions, other...)
			}
		}
		if probe != nil {
			probe.shortcutVisits += uint64(count)
		}
	}
	return true, true
}

func buildFlatObjectAliasIndex(value Value, probe *scopeAliasProbe) (*flatObjectAliasIndex, bool) {
	// Nested/otherwise ineligible roots are common on fallback paths. Prove
	// eligibility before allocating an index that would immediately be dropped.
	unknownTarget := aliasSnapshot{ref: newValueRef(), kind: ValueObject}
	for position := range value.List {
		child := &value.List[position]
		safe, _ := flatObjectAliasChild(child, unknownTarget, probe)
		if !safe {
			return nil, false
		}
	}
	index := &flatObjectAliasIndex{positionsByRef: make(map[uint64]int, len(value.List)), slots: len(value.List)}
	for position := range value.List {
		child := &value.List[position]
		if child.Kind == ValueObject && child.Ref != 0 {
			if _, exists := index.positionsByRef[child.Ref]; !exists {
				index.positionsByRef[child.Ref] = position
			} else {
				if index.additionalPositions == nil {
					index.additionalPositions = make(map[uint64][]int)
				}
				index.additionalPositions[child.Ref] = append(index.additionalPositions[child.Ref], position)
			}
			if probe != nil {
				probe.shortcutVisits++
			}
		}
	}
	return index, true
}

func flatObjectAliasValueSafe(value Value, probe *scopeAliasProbe) bool {
	unknownTarget := aliasSnapshot{ref: newValueRef(), kind: ValueObject}
	safe, _ := flatObjectAliasChild(&value, unknownTarget, probe)
	return safe
}

func (vm *VM) storeFlatObjectAliasIndex(key flatObjectAliasIndexKey, index *flatObjectAliasIndex) bool {
	if vm == nil || index == nil || index.slots > flatObjectAliasIndexMaxSlots {
		return false
	}
	if vm.flatAliasIndex == nil {
		vm.flatAliasIndex = make(map[flatObjectAliasIndexKey]*flatObjectAliasIndex)
	}
	if len(vm.flatAliasIndex) >= flatObjectAliasIndexMaxEntries || vm.flatAliasIndexSlots+index.slots > flatObjectAliasIndexMaxSlots {
		var visits uint64
		for _, existing := range vm.flatAliasIndex {
			visits += uint64(1 + existing.slots)
		}
		if vm.perfRecorder != nil {
			vm.perfRecorder.recordScopeAliasShortcutVisits(visits)
		}
		clear(vm.flatAliasIndex)
		vm.flatAliasIndexSlots = 0
	}
	vm.flatAliasIndex[key] = index
	vm.flatAliasIndexSlots += index.slots
	return true
}

func (vm *VM) deleteFlatObjectAliasIndex(key flatObjectAliasIndexKey) {
	if vm == nil || vm.flatAliasIndex == nil {
		return
	}
	if index := vm.flatAliasIndex[key]; index != nil {
		vm.flatAliasIndexSlots -= index.slots
		delete(vm.flatAliasIndex, key)
		if vm.perfRecorder != nil {
			vm.perfRecorder.recordScopeAliasShortcutVisits(uint64(1 + index.slots))
		}
	}
}

func (vm *VM) invalidateFlatObjectAliasIndexes(_ uint64) {
	// Refs do not identify physical backing: another ref or overlapping slice
	// can expose the same slots. Structural mutations discard the bounded cache.
	vm.clearFlatObjectAliasIndexes()
}

func (vm *VM) setListElement(list Value, index int, child Value) {
	vm.clearFlatObjectAliasIndexes()
	vm.clearNestedReceiverCache()
	list.List[index] = child
}

func (vm *VM) prepareFlatObjectAliasReplacement(previous aliasSnapshot, updated Value) {
	if vm == nil || len(vm.flatAliasIndex) == 0 {
		return
	}
	var probe scopeAliasProbe
	safe := previous.kind == ValueObject && updated.Kind == ValueObject &&
		previous.ref == updated.Ref && flatObjectAliasValueSafe(updated, &probe)
	if vm.perfRecorder != nil {
		vm.perfRecorder.recordScopeAliasShortcutVisits(probe.shortcutVisits)
	}
	// Legacy scope/static walks also write shared list slots. They may retain
	// indexes only when both identity and flat eligibility survive replacement.
	if !safe {
		vm.clearFlatObjectAliasIndexes()
	}
}

func (vm *VM) clearFlatObjectAliasIndexes() {
	if vm == nil || len(vm.flatAliasIndex) == 0 {
		return
	}
	visits := uint64(len(vm.flatAliasIndex) + vm.flatAliasIndexSlots)
	if vm.perfRecorder != nil {
		vm.perfRecorder.recordScopeAliasShortcutVisits(visits)
	}
	clear(vm.flatAliasIndex)
	vm.flatAliasIndex = nil
	vm.flatAliasIndexSlots = 0
}

func (vm *VM) invalidateFlatObjectAliasIndexesForValue(value Value) {
	switch value.Kind {
	case ValueNull, ValueInt, ValueDecimal, ValueBool, ValueString:
		return
	default:
		vm.clearFlatObjectAliasIndexes()
	}
}

// tryFlatObjectAliasRoot replaces direct object aliases in a List or Set after
// a complete preflight proves that no non-target child needs recursive work.
// It preserves legacy declared-type pruning; otherwise, current field values
// must be scalar or one collection layer of primitive values. A failed preflight
// leaves the root untouched for the legacy walk. Nothing persists between calls,
// and a handled result applies only to this root, including its current backing.
func tryFlatObjectAliasRoot(value Value, previous aliasSnapshot, updated Value, probe *scopeAliasProbe) (handled bool, changed bool) {
	if previous.kind != ValueObject || !previous.valid() {
		return false, false
	}
	var elements []Value
	switch value.Kind {
	case ValueList:
		elements = value.List
	case ValueSet:
		elements = value.Set
	default:
		return false, false
	}
	if valueCannotContainAliasRef(value, previous.ref, previous.kind) {
		return false, false
	}
	if probe != nil {
		probe.shortcutVisits++
	}
	found := false
	for i := range elements {
		safe, target := flatObjectAliasChild(&elements[i], previous, probe)
		if !safe {
			return false, false
		}
		found = found || target
	}
	if !found {
		return true, false
	}
	// Update every direct occurrence, including duplicate refs with independent
	// Fields backings. Legacy replacement also stops at a matching target, so
	// the target's own fields need no scalar check.
	for i := range elements {
		child := &elements[i]
		if probe != nil {
			probe.shortcutVisits++
		}
		if child.Kind == previous.kind && child.Ref == previous.ref {
			*child = updated
		}
	}
	return true, true
}

func flatObjectAliasChild(child *Value, previous aliasSnapshot, probe *scopeAliasProbe) (safe bool, target bool) {
	if probe != nil {
		probe.shortcutVisits++
	}
	if child.Kind == previous.kind && child.Ref == previous.ref {
		return true, true
	}
	switch child.Kind {
	case ValueNull, ValueInt, ValueDecimal, ValueBool, ValueString:
		return true, false
	case ValueObject:
		for _, field := range child.Fields {
			// Preserve the replacement walk's existing field pruning, including
			// scalar-typed bookkeeping collections. Do not scan their contents.
			if valueCannotContainAliasRef(field, previous.ref, previous.kind) {
				if probe != nil {
					probe.shortcutVisits++
				}
				continue
			}
			if !aliasScalarField(field, probe) {
				return false, false
			}
		}
		return true, false
	default:
		return false, false
	}
}

// aliasScalarField checks actual values in unpruned fields. Only primitive
// leaves and one collection layer are accepted; objects, deeper collections,
// and cycles fall back to the recursive walk.
func aliasScalarField(value Value, probe *scopeAliasProbe) bool {
	if probe != nil {
		probe.shortcutVisits++
	}
	switch value.Kind {
	case ValueNull, ValueInt, ValueDecimal, ValueBool, ValueString:
		return true
	case ValueList:
		for _, child := range value.List {
			if !aliasPrimitiveLeaf(child, probe) {
				return false
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if !aliasPrimitiveLeaf(child, probe) {
				return false
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if !aliasPrimitiveLeaf(child, probe) {
				return false
			}
		}
		for _, child := range value.MapKeys {
			if !aliasPrimitiveLeaf(child, probe) {
				return false
			}
		}
	default:
		return false
	}
	return true
}

func aliasPrimitiveLeaf(value Value, probe *scopeAliasProbe) bool {
	if probe != nil {
		probe.shortcutVisits++
	}
	switch value.Kind {
	case ValueNull, ValueInt, ValueDecimal, ValueBool, ValueString:
		return true
	default:
		return false
	}
}
