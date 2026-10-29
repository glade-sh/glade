package vm

import "weak"

// Key-set values own their backing. The VM's weak index lets map writes find
// surviving views without retaining temporary maps after those views disappear.
type mapKeySetBacking struct {
	value          Value
	version        uint64
	members        []Value
	membersVersion uint64
	// frozen is set only on field-map holder descriptors (see
	// describe_field_map_holder.go); key-set backings leave it nil.
	frozen map[string]Value
}

type mapKeySetSourceRef = weak.Pointer[mapKeySetBacking]

type mapKeySetHashCache struct {
	version uint64
	hashes  []setInsertionHash
}

func (vm *VM) bindMapKeySet(source Value, keys *Value) {
	if source.Ref == 0 {
		return
	}
	if vm.mapKeySetBackings == nil {
		vm.mapKeySetBackings = make(map[uint64]mapKeySetSourceRef)
	}
	// Weak entries do not retain payloads; periodically discard their dead IDs.
	if vm.mapKeySetBinds%64 == 0 {
		for ref, entry := range vm.mapKeySetBackings {
			if entry.Value() == nil {
				delete(vm.mapKeySetBackings, ref)
			}
		}
	}
	vm.mapKeySetBinds++
	backing := vm.mapKeySetBackings[source.Ref].Value()
	if backing == nil {
		backing = &mapKeySetBacking{value: source, members: keys.Set}
		vm.mapKeySetBackings[source.Ref] = weak.Make(backing)
	}
	keys.mapKeySetSource = backing
	keys.mapKeySetHashes = &mapKeySetHashCache{version: backing.version, hashes: keys.setInsertionHashes}
}

func (vm *VM) updateMapKeySetBacking(value Value) {
	if value.Kind == ValueMap {
		if backing := vm.mapKeySetBackings[value.Ref].Value(); backing != nil {
			backing.value = value
			backing.version++
		}
	}
}

// Normalize at collection read boundaries, including previously evaluated
// arguments and nested serialization values. This does not invoke Apex code.
func (keys Value) currentMapKeySet() Value {
	backing := keys.mapKeySetSource
	if backing == nil {
		return keys
	}
	if keys.Kind == ValueMap {
		return keys.currentFieldMapHolder()
	}
	if keys.Kind != ValueSet {
		return keys
	}
	if backing.membersVersion != backing.version {
		backing.members = nil
		for _, raw := range orderedValueMapKeys(backing.value) {
			backing.members = append(backing.members, mapStoredKey(backing.value, raw))
		}
		backing.membersVersion = backing.version
	}
	keys.Set = backing.members
	if cache := keys.mapKeySetHashes; cache != nil && cache.version == backing.version {
		keys.setInsertionHashes = cache.hashes
	}
	return keys
}

func (vm *VM) refreshMapKeySet(keys Value) (Value, error) {
	keys = keys.currentMapKeySet()
	backing, cache := keys.mapKeySetSource, keys.mapKeySetHashes
	if backing == nil || cache == nil || cache.version == backing.version {
		return keys, nil
	}
	version := backing.version
	hashes := make([]setInsertionHash, 0, len(keys.Set))
	for _, key := range keys.Set {
		hash, err := vm.captureSetInsertionHash(key)
		if err != nil {
			return Null, err
		}
		hashes = append(hashes, hash)
	}
	cache.hashes, cache.version = hashes, version
	keys.setInsertionHashes = hashes
	return keys, nil
}

func (vm *VM) writeMapKeySetRemovals(keys Value) error {
	if keys.Kind != ValueSet || keys.mapKeySetSource == nil {
		return nil
	}
	source := keys.mapKeySetSource.value
	previous := snapshotAlias(source)
	// A key-set removal is a write to its source map. Apply the same
	// ownership barrier as direct Map members before touching either map.
	vm.prepareSharedMapMutation(&source)
	changed := false
	for _, raw := range orderedValueMapKeys(source) {
		keep, err := vm.setContainsValue(keys, mapStoredKey(source, raw), &Result{})
		if err != nil {
			return err
		}
		if !keep {
			delete(source.Map, raw)
			delete(source.MapKeys, raw)
			changed = true
		}
	}
	if changed {
		vm.storeGlobalDescribeCacheEntry(source)
		vm.propagateCollectionMutationFromSnapshot(previous, source)
	}
	return nil
}
