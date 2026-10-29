package vm

import "reflect"

// Ref identifies an Apex value, but detached snapshots can retain that Ref
// with different child backings. A uniqueness proof must inspect every such
// snapshot, including sidecar fields, rather than treating Ref as a visit key.
type mapPutProofBacking struct {
	kind                  ValueKind
	ref                   uint64
	fields, entries, keys uintptr
	list, set             uintptr
	listLen, setLen       int
}

func mapPutProofBackingOf(value Value) mapPutProofBacking {
	return mapPutProofBacking{
		kind: value.Kind, ref: value.Ref,
		fields:  reflect.ValueOf(value.Fields).Pointer(),
		entries: reflect.ValueOf(value.Map).Pointer(),
		keys:    reflect.ValueOf(value.MapKeys).Pointer(),
		list:    reflect.ValueOf(value.List).Pointer(), listLen: len(value.List),
		set: reflect.ValueOf(value.Set).Pointer(), setLen: len(value.Set),
	}
}

func walkMapPutProofValues(value Value, seen map[mapPutProofBacking]bool, visit func(Value) bool) bool {
	// Inspect identity before the visit guard: another snapshot of this exact
	// backing still provides a positive alias match.
	if visit(value) {
		return true
	}
	key := mapPutProofBackingOf(value)
	if seen[key] {
		return false
	}
	seen[key] = true
	for _, child := range value.Fields {
		if walkMapPutProofValues(child, seen, visit) {
			return true
		}
	}
	for _, child := range value.Map {
		if walkMapPutProofValues(child, seen, visit) {
			return true
		}
	}
	for _, child := range value.MapKeys {
		if walkMapPutProofValues(child, seen, visit) {
			return true
		}
	}
	for _, child := range value.List {
		if walkMapPutProofValues(child, seen, visit) {
			return true
		}
	}
	for _, child := range value.Set {
		if walkMapPutProofValues(child, seen, visit) {
			return true
		}
	}
	return false
}

func mapPutProofContainsAliasRef(value Value, ref uint64, kind ValueKind, seen map[mapPutProofBacking]bool) bool {
	return ref != 0 && walkMapPutProofValues(value, seen, func(candidate Value) bool {
		return candidate.Ref == ref && candidate.Kind == kind
	})
}

func staticAliasMapPutPathKind(kind staticAliasChildHintKind) bool {
	return kind == staticAliasChildHintObjectMapField || kind == staticAliasChildHintMapValueMapField
}

func (vm *VM) forgetStaticMapPutPathsForRef(ref uint64) {
	if len(vm.staticAliasChildHints) == 0 {
		return
	}
	if vm.staticValueRefFields == nil {
		for key, hint := range vm.staticAliasChildHints {
			if staticAliasMapPutPathKind(hint.Kind) {
				delete(vm.staticAliasChildHints, key)
			}
		}
		return
	}
	vm.staticValueRefFields[ref].forEach(func(location staticFieldRef) { vm.forgetStaticMapPutPathsInField(location) })
}

func (vm *VM) forgetStaticMapPutPathsInField(location staticFieldRef) {
	if !vm.staticMapPutPathHints {
		return
	}
	remaining := false
	for key, hint := range vm.staticAliasChildHints {
		if !staticAliasMapPutPathKind(hint.Kind) {
			continue
		}
		if key.Location == location {
			delete(vm.staticAliasChildHints, key)
			continue
		}
		remaining = true
	}
	vm.staticMapPutPathHints = remaining
}

// rememberStaticAliasChildHintEntry stores one hint and notes whether a map-put
// path hint now exists, so forgetting them can skip an empty scan.
func (vm *VM) rememberStaticAliasChildHintEntry(key staticAliasChildHintKey, hint staticAliasChildHint) {
	if vm.staticAliasChildHints == nil {
		vm.staticAliasChildHints = make(map[staticAliasChildHintKey]staticAliasChildHint)
	}
	if staticAliasMapPutPathKind(hint.Kind) {
		vm.staticMapPutPathHints = true
	}
	vm.staticAliasChildHints[key] = hint
}

// Delta writes preserve unrelated paths. Any reference reached by the complete
// inserted-subtree scan can have changed or introduced another route to a map.
func (vm *VM) invalidateStaticHintsForDelta(location staticFieldRef, delta *staticAliasIndexDelta, seen map[uint64]bool) {
	for key, hint := range vm.staticAliasChildHints {
		if key.Location == location && (seen[key.Ref] || staticAliasMapPutPathKind(hint.Kind) && seen[hint.ParentRef]) {
			delete(vm.staticAliasChildHints, key)
		}
	}
	if index, exists := vm.staticAliasDirectChildren[location]; exists {
		if index.RootRef == delta.root.Ref {
			delete(vm.staticAliasDirectChildren, location)
		} else {
			for child := range index.Children {
				if seen[child.Ref] {
					delete(vm.staticAliasDirectChildren, location)
					break
				}
			}
		}
	}
	if seen[delta.root.Ref] {
		delta.backReferences = true
	}
	delta.childrenScanned = true
}

func (vm *VM) mapPutDeltaBackReferences(delta *staticAliasIndexDelta) bool {
	vm.scanStaticAliasIndexDeltaChildren(delta)
	return delta.backReferences
}

func mapPutPathParent(value Value, hint staticAliasChildHint) (Value, bool) {
	if value.Ref == 0 || value.Ref != hint.RootRef {
		return Value{}, false
	}
	switch hint.Kind {
	case staticAliasChildHintObjectMapField:
		return value, value.Kind == ValueObject && value.Ref == hint.ParentRef
	case staticAliasChildHintMapValueMapField:
		if value.Kind != ValueMap || value.Runtime != "" || len(value.Fields) != 0 {
			return Value{}, false
		}
		parent, found := value.Map[hint.Key]
		return parent, found && parent.Kind == ValueObject && parent.Ref == hint.ParentRef
	default:
		return Value{}, false
	}
}

// This cache covers exactly an Apex field or a map-value followed by that
// field. Learning proves uniqueness; later exact delta writes retain it only
// while inserted subtrees cannot introduce another route to the mutated map.
func (vm *VM) replaceStaticAliasUsingMapPutPath(value Value, location staticFieldRef, previous aliasSnapshot, updated Value, delta *staticAliasIndexDelta) (Value, bool) {
	if delta == nil || delta.parentRef == 0 || delta.fieldName == "" || previous.ref != delta.root.Ref || previous.kind != ValueMap ||
		!staticAliasIndexDeltaMatchesMap(updated, delta) || value.Ref == 0 || vm.mapPutDeltaBackReferences(delta) {
		return value, false
	}
	key := vm.staticAliasChildHintKey(previous, location)
	hint, cached := vm.staticAliasChildHints[key]
	if cached && staticAliasMapPutPathKind(hint.Kind) {
		parent, valid := mapPutPathParent(value, hint)
		field, found := parent.Fields[hint.Name]
		if valid && found && hint.ParentRef == delta.parentRef && hint.Name == delta.fieldName && field.Kind == ValueMap && field.Ref == previous.ref {
			parent.Fields[hint.Name] = updated
			if hint.Kind == staticAliasChildHintMapValueMapField {
				value.Map[hint.Key] = parent
			} else {
				value = parent
			}
			if vm.perfRecorder != nil {
				vm.perfRecorder.recordStaticAliasChildHintHit()
			}
			return value, true
		}
		delete(vm.staticAliasChildHints, key)
	}
	hint = staticAliasChildHint{RootRef: value.Ref, ParentRef: delta.parentRef, Name: delta.fieldName}
	parent := Value{}
	switch value.Kind {
	case ValueObject:
		if value.Ref != delta.parentRef {
			return value, false
		}
		hint.Kind = staticAliasChildHintObjectMapField
		parent = value
	case ValueMap:
		if value.Runtime != "" || len(value.Fields) != 0 {
			return value, false
		}
		hint.Kind = staticAliasChildHintMapValueMapField
		found := false
		for entry, candidate := range value.Map {
			if candidate.Kind == ValueObject && candidate.Ref == delta.parentRef {
				if found {
					return value, false
				}
				found, hint.Key, parent = true, entry, candidate
			}
		}
		if !found {
			return value, false
		}
	default:
		return value, false
	}
	field, found := parent.Fields[delta.fieldName]
	if !found || field.Kind != ValueMap || field.Ref != previous.ref {
		return value, false
	}
	seen := make(map[mapPutProofBacking]bool)
	for name, sibling := range parent.Fields {
		if name != delta.fieldName && mapPutProofContainsAliasRef(sibling, previous.ref, previous.kind, seen) {
			return value, false
		}
	}
	if value.Kind == ValueMap {
		for entry, sibling := range value.Map {
			if entry != hint.Key && mapPutProofContainsAliasRef(sibling, previous.ref, previous.kind, seen) {
				return value, false
			}
		}
		for _, mapKey := range value.MapKeys {
			if mapPutProofContainsAliasRef(mapKey, previous.ref, previous.kind, seen) {
				return value, false
			}
		}
	}
	parent.Fields[delta.fieldName] = updated
	if value.Kind == ValueMap {
		value.Map[hint.Key] = parent
	} else {
		value = parent
	}
	vm.rememberStaticAliasChildHintEntry(key, hint)
	return value, true
}
