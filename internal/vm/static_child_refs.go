package vm

// Field writes can publish a new subtree even when the parent retains its
// identity and backing map. Register that subtree at each containing static
// location before another mutation needs the reference index.
func (vm *VM) rememberPublishedObjectFieldRefs(parent, previous, value Value) {
	if parent.Kind != ValueObject || parent.Ref == 0 || vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		return
	}
	if value.Ref == 0 && value.Kind != ValueObject && !mutableCollectionKind(value.Kind) {
		return
	}
	// A map method writes the same backing maps back to their field. Its
	// collection propagation registers inserted children; rescanning all of
	// the existing entries here repeats that work.
	if previous.Kind == ValueMap && value.Kind == ValueMap && previous.Ref != 0 && previous.Ref == value.Ref &&
		len(previous.Fields) == 0 && len(value.Fields) == 0 &&
		sameMapBacking(previous.Map, value.Map) && sameMapBacking(previous.MapKeys, value.MapKeys) {
		return
	}
	if vm.isSObjectLikeType(parent.Type) {
		return
	}
	locations := vm.staticValueRefFields[parent.Ref]
	locations.forEach(func(location staticFieldRef) {
		// The write publishes exactly this subtree. Only refs inside it can gain
		// another occurrence in the field; hints and direct-child entries for
		// untouched refs keep their verified-unique slot, and removals self-heal
		// because replay re-verifies containment before replacing.
		vm.collectStaticFieldValueRefsInFieldForgettingHints(value, location, staticAliasChildHint{}, false)
	})
}
