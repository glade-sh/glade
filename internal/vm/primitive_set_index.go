package vm

import (
	"maps"
	"strings"
)

// The slice headers are an append ownership token. A shorter surviving
// snapshot must copy before appending, even if it shares spare capacity.
// Membership follows the ordered element slice; map iteration is never exposed.
type primitiveSetState struct {
	items     []Value
	hashes    []setInsertionHash
	element   string
	positions map[string]int
}

func clonePrimitiveSetState(state *primitiveSetState) *primitiveSetState {
	if state == nil {
		return nil
	}
	cloned := *state
	cloned.positions = maps.Clone(state.positions)
	// Copying the token and map also isolates nil/empty Sets, whose matching
	// headers alone cannot establish exclusive ownership. Keep absent/stale
	// headers intact to preserve structural alias snapshots.
	return &cloned
}

func bindPrimitiveSetCloneState(original Value, cloned *Value) {
	// Only a valid primitive index can follow a copied payload. Rebinding an
	// absent or stale token would change structural snapshot contents.
	if original.Kind == ValueSet && original.primitiveSetState.matches(original) && primitiveSetElement(original) != "" {
		cloned.primitiveSetState.items = cloned.Set
		cloned.primitiveSetState.hashes = cloned.setInsertionHashes
	}
}

func (state *primitiveSetState) matches(set Value) bool {
	return state != nil && sameSliceBacking(state.items, set.Set) && sameSliceBacking(state.hashes, set.setInsertionHashes)
}

func primitiveSetElement(set Value) string {
	element, ok := collectionElementType(set.Type)
	if !ok || collectionBase(set.Type) != "Set" {
		return ""
	}
	switch {
	case strings.EqualFold(element, "String"):
		return "String"
	case strings.EqualFold(element, "Id"):
		return "Id"
	}
	return ""
}

func primitiveSetKey(element string, value Value) (string, bool) {
	if value.Kind == ValueNull {
		return "null", true
	}
	if element == "String" && value.Kind == ValueString && !strings.EqualFold(value.Type, "Id") {
		return "string:" + value.Text, true
	}
	if element == "Id" {
		if text, ok := typedIDValueText(value); ok {
			return "id:" + canonicalIDMapKey(text), true
		}
	}
	return "", false
}

func preparePrimitiveSetState(set Value, state *primitiveSetState, element string) bool {
	if state.positions != nil && state.element == element {
		return true
	}
	positions := make(map[string]int, len(set.Set))
	for i, value := range set.Set {
		key, ok := primitiveSetKey(element, value)
		if !ok || set.setInsertionHashAt(i).tracked {
			return false
		}
		if _, exists := positions[key]; !exists {
			positions[key] = i
		}
	}
	state.element, state.positions = element, positions
	return true
}

func primitiveSetIndex(set Value, needle Value) (int, bool) {
	element := primitiveSetElement(set)
	if element == "" {
		return -1, false
	}
	key, ok := primitiveSetKey(element, needle)
	if !ok {
		return -1, false
	}
	state := set.primitiveSetState
	if !state.matches(set) || !preparePrimitiveSetState(set, state, element) {
		return -1, false
	}
	if index, exists := state.positions[key]; exists {
		return index, true
	}
	return -1, true
}

func appendPrimitiveSetEntry(set Value, value Value, hash setInsertionHash) (Value, bool) {
	element := primitiveSetElement(set)
	key, ok := primitiveSetKey(element, value)
	if element == "" || !ok || hash.tracked {
		return set, false
	}
	state := set.primitiveSetState
	owned := state.matches(set)
	if !owned {
		state = &primitiveSetState{items: set.Set, hashes: set.setInsertionHashes}
	}
	if !preparePrimitiveSetState(set, state, element) {
		return set, false
	}
	if !owned {
		values := make([]Value, len(set.Set), max(4, 2*(len(set.Set)+1)))
		copy(values, set.Set)
		hashes := make([]setInsertionHash, len(set.Set), cap(values))
		copy(hashes, set.setInsertionHashes)
		set.Set, set.setInsertionHashes = values, hashes
	}
	set.Set = append(set.Set, value)
	// History can be absent on an untracked producer. Materialize its old slots
	// before appending so history and elements always have corresponding indices.
	if len(set.setInsertionHashes) < len(set.Set)-1 {
		hashes := make([]setInsertionHash, len(set.Set)-1, cap(set.Set))
		copy(hashes, set.setInsertionHashes)
		set.setInsertionHashes = hashes
	}
	set.setInsertionHashes = append(set.setInsertionHashes, hash)
	if _, exists := state.positions[key]; !exists {
		state.positions[key] = len(set.Set) - 1
	}
	state.items, state.hashes = set.Set, set.setInsertionHashes
	set.primitiveSetState = state
	return set, true
}
