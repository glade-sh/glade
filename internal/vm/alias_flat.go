package vm

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
