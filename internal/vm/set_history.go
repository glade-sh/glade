package vm

import "strings"

// A missing/untracked tag preserves the baseline of producers without VM
// insertion context (plain Set and JSON collection producers).
type setInsertionHash struct {
	key     string
	tracked bool
}

func (value Value) setInsertionHashAt(index int) setInsertionHash {
	if index < len(value.setInsertionHashes) {
		return value.setInsertionHashes[index]
	}
	return setInsertionHash{}
}

func cloneSetInsertionHashes(value Value) []setInsertionHash {
	if value.setInsertionHashes == nil {
		return nil
	}
	out := make([]setInsertionHash, len(value.Set))
	copy(out, value.setInsertionHashes)
	return out
}

func (vm *VM) captureSetInsertionHash(value Value) (setInsertionHash, error) {
	if value.Kind == ValueInt || value.Kind == ValueDecimal {
		return setInsertionHash{key: mapKey(value), tracked: true}, nil
	}
	key, handled, err := vm.apexObjectHashMapKeyChecked(value)
	if err != nil {
		return setInsertionHash{}, err
	}
	// Identity-reference and framework fallbacks retain their equality baseline.
	// Custom equality may span types, so history stores only the numeric hash,
	// leaving same-hash comparisons to the existing Apex equality path.
	prefix := string(ValueObject) + ":" + value.Type + ":hash:"
	if !handled || !strings.HasPrefix(key, prefix) {
		return setInsertionHash{}, nil
	}
	return setInsertionHash{key: strings.TrimPrefix(key, prefix), tracked: true}, nil
}

func (vm *VM) setIndexOfValueWithHash(set Value, needle Value, hash setInsertionHash, result *Result) (int, error) {
	for index, value := range set.Set {
		stored := set.setInsertionHashAt(index)
		if stored.tracked && (!hash.tracked || stored.key != hash.key) {
			continue
		}
		equal, err := vm.apexCollectionElementEquals(value, needle, result)
		if err != nil {
			return -1, err
		}
		if equal {
			return index, nil
		}
	}
	return -1, nil
}

func (vm *VM) setIndexOfValue(set Value, needle Value, result *Result) (int, error) {
	for index := range set.Set {
		if set.setInsertionHashAt(index).tracked {
			hash, err := vm.captureSetInsertionHash(needle)
			if err != nil {
				return -1, err
			}
			return vm.setIndexOfValueWithHash(set, needle, hash, result)
		}
	}
	for i, value := range set.Set {
		if value.Kind == ValueInt || value.Kind == ValueDecimal || needle.Kind == ValueInt || needle.Kind == ValueDecimal {
			if setPrimitiveValuesEqual(value, needle) {
				return i, nil
			}
			continue
		}
		equal, err := vm.apexCollectionElementEquals(value, needle, result)
		if err != nil {
			return -1, err
		}
		if equal {
			return i, nil
		}
	}
	return -1, nil
}

// Set numeric keys preserve the boxed numeric type and Decimal scale.
func setPrimitiveValuesEqual(left, right Value) bool {
	if left.Kind == ValueInt || left.Kind == ValueDecimal || right.Kind == ValueInt || right.Kind == ValueDecimal {
		return mapKey(left) == mapKey(right)
	}
	return left.Equal(right)
}

func (vm *VM) setContainsValue(set Value, needle Value, result *Result) (bool, error) {
	index, err := vm.setIndexOfValue(set, needle, result)
	return index >= 0, err
}

// Insertion callers capture once after coercion and reuse that hash for the
// duplicate decision. Copy the slices before mutation so errors cannot leave
// aliases with a changed element slice and stale history.
func appendSetEntry(set Value, value Value, hash setInsertionHash) Value {
	hashes := make([]setInsertionHash, len(set.Set), len(set.Set)+1)
	copy(hashes, set.setInsertionHashes)
	set.Set = append(append([]Value(nil), set.Set...), value)
	set.setInsertionHashes = append(hashes, hash)
	return set
}

func removeSetEntry(set Value, index int) Value {
	values := make([]Value, 0, len(set.Set)-1)
	hashes := make([]setInsertionHash, 0, len(set.Set)-1)
	for i, value := range set.Set {
		if i != index {
			values = append(values, value)
			hashes = append(hashes, set.setInsertionHashAt(i))
		}
	}
	set.Set = values
	set.setInsertionHashes = hashes
	return set
}

func (vm *VM) constructTrackedSet(typeName string, values []Value, result *Result) (Value, error) {
	set := Set()
	set.Type = typeName
	for _, value := range values {
		item, err := vm.coerceCollectionElement(typeName, value)
		if err != nil {
			return Null, err
		}
		hash, err := vm.captureSetInsertionHash(item)
		if err != nil {
			return Null, err
		}
		index, err := vm.setIndexOfValueWithHash(set, item, hash, result)
		if err != nil {
			return Null, err
		}
		if index < 0 {
			set = appendSetEntry(set, item, hash)
		}
	}
	return set, nil
}

// Coercion transports history by surviving index without invoking callbacks or
// replacing insertion history with the elements' current hashes.
func setTransportContains(values []Value, hashes []setInsertionHash, needle Value, hash setInsertionHash) bool {
	for i, value := range values {
		if hashes[i].tracked && hash.tracked && hashes[i].key != hash.key {
			continue
		}
		if setPrimitiveValuesEqual(value, needle) {
			return true
		}
	}
	return false
}

func sameSetInsertionHistory(left, right Value) bool {
	if len(left.Set) != len(right.Set) {
		return false
	}
	for index := range left.Set {
		if left.setInsertionHashAt(index) != right.setInsertionHashAt(index) {
			return false
		}
	}
	return true
}
