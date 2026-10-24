package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

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
	// Custom equality may span types, so history stores only the numeric hash,
	// leaving same-hash comparisons to the existing Apex equality path.
	prefix := string(ValueObject) + ":" + value.Type + ":hash:"
	if !handled || !strings.HasPrefix(key, prefix) {
		return setInsertionHash{}, nil
	}
	return setInsertionHash{key: strings.TrimPrefix(key, prefix), tracked: true}, nil
}

// Concrete typed Set literals and add capture identity history. Map.keySet
// keeps captureSetInsertionHash's baseline.
func (vm *VM) captureTypedSetInsertionHash(setType string, value Value) (setInsertionHash, error) {
	element, typed := collectionElementType(vm.resolveAssignableTargetType(setType))
	if value.Kind == ValueObject && collectionBase(setType) == "Set" && typed {
		// Resolve short nested names and namespace-qualified runtime spellings
		// to the same class for both literal and add identity-hash capture.
		if !strings.EqualFold(element, value.Type) {
			if class, ok := vm.lookupClass(vm.resolveRuntimeTypeName(element)); ok {
				element = runtimeClassName(class)
			}
		}
		if vm.sameSetClassIdentity(element, value.Type) && vm.setEqualsOnlyIdentity(value) {
			key, handled, err := vm.apexObjectHashMapKeyChecked(value)
			if err != nil {
				return setInsertionHash{}, err
			}
			if handled && strings.HasPrefix(key, string(ValueObject)+":"+value.Type+":ref:") {
				return setInsertionHash{key: key, tracked: true}, nil
			}
		}
	}
	return vm.captureSetInsertionHash(value)
}

// Resolve aliases only within the existing concrete typed Set identity guard.
// Equal local, nested and namespace-qualified class names keep the same hash
// history; subclasses and unrelated classes remain separate identities.
func (vm *VM) sameSetClassIdentity(left, right string) bool {
	if strings.EqualFold(left, right) {
		return true
	}
	leftName, leftOK := vm.resolveClassName(left)
	rightName, rightOK := vm.resolveClassName(right)
	if !leftOK || !rightOK {
		return false
	}
	leftClass, leftOK := vm.lookupClass(leftName)
	rightClass, rightOK := vm.lookupClass(rightName)
	return leftOK && rightOK && strings.EqualFold(runtimeClassName(leftClass), runtimeClassName(rightClass))
}

// The observed plain bulk/copy shapes retain distinct equals-only identities.
// Other producers, APIs and namespace contexts keep their existing capture.
func (vm *VM) captureBulkSetInsertionHash(setType string, value Value) (setInsertionHash, error) {
	version := vm.currentMethod.APIVersion
	if version == "" && vm.Org != nil {
		version = vm.Org.APIVersion
	}
	major, known := apexversion.Major(version)
	if known && (major == 62 || major == 67 || major == 68) && vm.currentMethod.Namespace == "" &&
		(vm.Org == nil || vm.Org.Namespace == "") {
		return vm.captureTypedSetInsertionHash(setType, value)
	}
	return vm.captureSetInsertionHash(value)
}

func (vm *VM) setEqualsOnlyIdentity(value Value) bool {
	hash, hasHash, hashAmbiguous := vm.resolveInstanceMethodForArgs(value.Type, "hashCode", nil)
	if hashAmbiguous || (hasHash && !strings.EqualFold(hash.ClassName, "Object")) {
		return false
	}
	equals, hasEquals, equalsAmbiguous := vm.resolveInstanceMethodForArgs(value.Type, "equals", []Value{value})
	return hasEquals && !equalsAmbiguous && vm.sameSetClassIdentity(equals.ClassName, value.Type) &&
		!equals.IsStatic && strings.EqualFold(equals.ReturnType, "Boolean") &&
		len(equals.Params) == 1 && strings.EqualFold(equals.Params[0].Type, "Object")
}

func (vm *VM) setIndexOfValueWithHash(set Value, needle Value, hash setInsertionHash, result *Result) (int, error) {
	if !hash.tracked {
		if index, ok := primitiveSetIndex(set, needle); ok {
			return index, nil
		}
	}
	for index, value := range set.Set {
		stored := set.setInsertionHashAt(index)
		if stored.tracked && (!hash.tracked || stored.key != hash.key) {
			// Untracked inputs retain the baseline equality scan
			// against the new literal/add identity history. Numeric/custom
			// hash history keeps its existing bucket restriction.
			identityPrefix := string(ValueObject) + ":" + value.Type + ":ref:"
			if hash.tracked || !strings.HasPrefix(stored.key, identityPrefix) {
				continue
			}
		}
		equal, err := vm.equalsUserSetKey(value, needle, result)
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
	if index, ok := primitiveSetIndex(set, needle); ok {
		return index, nil
	}
	for index := range set.Set {
		if set.setInsertionHashAt(index).tracked {
			hash, err := vm.captureTypedSetInsertionHash(set.Type, needle)
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
		equal, err := vm.equalsUserSetKey(value, needle, result)
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
// duplicate decision. Primitive append ownership protects surviving prefixes;
// other sets copy both slices before mutation.
func appendSetEntry(set Value, value Value, hash setInsertionHash) Value {
	if updated, ok := appendPrimitiveSetEntry(set, value, hash); ok {
		return updated
	}
	hashes := make([]setInsertionHash, len(set.Set), len(set.Set)+1)
	copy(hashes, set.setInsertionHashes)
	values := make([]Value, len(set.Set)+1)
	copy(values, set.Set)
	values[len(set.Set)] = value
	set.Set = values
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
	set.primitiveSetState = &primitiveSetState{items: set.Set, hashes: hashes}
	return set
}

func (vm *VM) constructTrackedSet(typeName string, values []Value, literal bool, result *Result) (Value, error) {
	set := Set()
	set.Type = typeName
	for _, value := range values {
		item, err := vm.coerceCollectionElement(typeName, value)
		if err != nil {
			return Null, err
		}
		var hash setInsertionHash
		if literal {
			hash, err = vm.captureTypedSetInsertionHash(typeName, item)
		} else {
			hash, err = vm.captureBulkSetInsertionHash(typeName, item)
		}
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
