package vm

// This cache is separate from ordinary containment misses: every entry is a
// complete, backing-specific proof that its graph lacks this parent object.
// Values keep their keyed backings alive for the lifetime of the proof.
type mapPutParentContainmentCache struct {
	parentRef uint64
	epoch     uint64
	misses    map[mapPutProofBacking]Value
}

const (
	mapPutParentPrepared uint8 = iota + 1
	mapPutParentAssigned
	mapPutParentFinished
)

func mapPutUntrackedBacking(value Value) bool {
	return value.Ref == 0 && (value.Fields != nil || value.Map != nil || value.MapKeys != nil || value.List != nil || value.Set != nil)
}

func (cache *mapPutParentContainmentCache) contains(value Value, probe *scopeAliasProbe, retainRoot bool) bool {
	seen := make(map[mapPutProofBacking]bool)
	type proof struct {
		key   mapPutProofBacking
		value Value
	}
	var pending []proof
	tracked := true
	var walk func(Value, bool) bool
	walk = func(current Value, root bool) bool {
		if probe != nil {
			probe.recursiveVisits++
		}
		if current.Ref == cache.parentRef && current.Kind == ValueObject {
			return true
		}
		if mapPutUntrackedBacking(current) {
			tracked = false
		}
		key := mapPutProofBackingOf(current)
		if _, certified := cache.misses[key]; certified {
			if probe != nil {
				probe.containmentCacheHits++
			}
			return false
		}
		if seen[key] {
			return false
		}
		seen[key] = true
		if current.Ref != 0 && len(pending)+len(cache.misses) < aliasContainmentCacheMaxEntries &&
			(root && retainRoot || len(current.Fields)+len(current.Map)+len(current.MapKeys)+len(current.List)+len(current.Set) > aliasContainmentCacheMinEntries) {
			pending = append(pending, proof{key, current})
		}
		for _, child := range current.Fields {
			if walk(child, false) {
				return true
			}
		}
		for _, child := range current.Map {
			if walk(child, false) {
				return true
			}
		}
		for _, child := range current.MapKeys {
			if walk(child, false) {
				return true
			}
		}
		for _, child := range current.List {
			if walk(child, false) {
				return true
			}
		}
		for _, child := range current.Set {
			if walk(child, false) {
				return true
			}
		}
		return false
	}
	if walk(value, true) {
		return true
	}
	// Unidentified mutable carriers can change without a reference mutation.
	// Their ancestors cannot retain a proof either.
	if !tracked {
		return false
	}
	// A partial DFS can return to an ancestor that later reaches the parent.
	// Publish no partial misses; only a negative whole walk proves every node.
	for _, entry := range pending {
		if len(cache.misses) >= aliasContainmentCacheMaxEntries {
			break
		}
		cache.misses[entry.key] = entry.value
	}
	return false
}

func (vm *VM) prepareMapPutParentContainment(delta *staticAliasIndexDelta) {
	if delta == nil || delta.parentRef == 0 {
		return
	}
	vm.scanStaticAliasIndexDeltaChildren(delta)
	if delta.childRefs[delta.parentRef] || delta.parentUntrackedBacking {
		vm.mapPutParentContainment = nil
		return
	}
	cache := vm.mapPutParentContainment
	if cache == nil || cache.parentRef != delta.parentRef || cache.epoch != vm.aliasContainmentMutationSeq {
		cache = &mapPutParentContainmentCache{parentRef: delta.parentRef, epoch: vm.aliasContainmentMutationSeq, misses: make(map[mapPutProofBacking]Value)}
		vm.mapPutParentContainment = cache
	}
	// Final alias propagation also replaces detached snapshots of this map.
	// Its existing entries must lack the parent, not just the inserted entry.
	// A prior full proof amortizes this check across same-backing safe puts.
	if cache.contains(delta.root, nil, true) {
		vm.mapPutParentContainment = nil
		return
	}
	delta.parentContainment = cache
	delta.parentContainmentEpoch = vm.aliasContainmentMutationSeq
	delta.parentContainmentStage = mapPutParentPrepared
}

func (vm *VM) invalidateMapPutParentContainment(delta *staticAliasIndexDelta) {
	if delta == nil {
		return
	}
	delta.parentContainment = nil
	delta.parentContainmentStage = 0
}

func (vm *VM) advanceMapPutParentContainment(delta *staticAliasIndexDelta, updatedParent Value) {
	if delta == nil || delta.parentContainmentStage != mapPutParentPrepared {
		return
	}
	cache := delta.parentContainment
	field, exists := updatedParent.Fields[delta.fieldName]
	if cache == nil || vm.mapPutParentContainment != cache || cache.epoch != delta.parentContainmentEpoch ||
		delta.parentContainmentEpoch == ^uint64(0) || vm.aliasContainmentMutationSeq != delta.parentContainmentEpoch+1 ||
		updatedParent.Kind != ValueObject || updatedParent.Ref != delta.parentRef || !exists || !staticAliasIndexDeltaMatchesMap(field, delta) {
		vm.invalidateMapPutParentContainment(delta)
		return
	}
	cache.epoch = vm.aliasContainmentMutationSeq
	delta.parentContainmentEpoch = cache.epoch
	delta.parentContainmentStage = mapPutParentAssigned
}

func (vm *VM) finishMapPutParentContainment(delta *staticAliasIndexDelta) {
	if delta == nil || delta.parentContainmentStage != mapPutParentAssigned {
		return
	}
	cache := delta.parentContainment
	if cache == nil || vm.mapPutParentContainment != cache || cache.epoch != delta.parentContainmentEpoch ||
		delta.parentContainmentEpoch == ^uint64(0) || vm.aliasContainmentMutationSeq != delta.parentContainmentEpoch+1 {
		vm.invalidateMapPutParentContainment(delta)
		return
	}
	cache.epoch = vm.aliasContainmentMutationSeq
	delta.parentContainmentEpoch = cache.epoch
	delta.parentContainmentStage = mapPutParentFinished
}

func (vm *VM) mapPutParentContainsAliasRef(value Value, previous aliasSnapshot, delta *staticAliasIndexDelta, probe *scopeAliasProbe) (bool, bool) {
	if delta == nil || delta.parentContainmentStage != mapPutParentAssigned || previous.kind != ValueObject || previous.ref != delta.parentRef {
		return false, false
	}
	cache := delta.parentContainment
	if cache == nil || vm.mapPutParentContainment != cache || cache.parentRef != previous.ref || cache.epoch != vm.aliasContainmentMutationSeq || cache.epoch != delta.parentContainmentEpoch {
		return false, false
	}
	return cache.contains(value, probe, false), true
}
