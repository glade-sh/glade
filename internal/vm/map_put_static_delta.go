package vm

import "strings"

// staticAliasIndexDelta describes one completed Map.put. Only the map identity
// and these inserted key/value subtrees can add references during its backing
// field writeback. It is local to this operation and never retained by the VM.
type staticAliasIndexDelta struct {
	root                   Value
	children               []Value
	parentRef              uint64
	fieldName              string
	childrenScanned        bool
	backReferences         bool
	childRefs              map[uint64]bool
	parentContainment      *mapPutParentContainmentCache
	parentContainmentEpoch uint64
	parentContainmentStage uint8
	parentUntrackedBacking bool
}

func (vm *VM) scanStaticAliasIndexDeltaChildren(delta *staticAliasIndexDelta) {
	if delta.childrenScanned {
		return
	}
	delta.childRefs = make(map[uint64]bool)
	seen := make(map[mapPutProofBacking]bool)
	var visits uint64
	for _, child := range delta.children {
		walkMapPutProofValues(child, seen, func(value Value) bool {
			visits++
			if mapPutUntrackedBacking(value) {
				delta.parentUntrackedBacking = true
			}
			if value.Ref != 0 {
				delta.childRefs[value.Ref] = true
			}
			if value.Ref == delta.root.Ref && value.Kind == ValueMap {
				delta.backReferences = true
			}
			return false
		})
	}
	delta.childrenScanned = true
	if vm.perfRecorder != nil {
		vm.perfRecorder.recordStaticAliasRefScanVisits(visits)
	}
}

func staticAliasIndexDeltaMatchesMap(value Value, delta *staticAliasIndexDelta) bool {
	return delta != nil && delta.root.Ref != 0 && value.Kind == ValueMap && delta.root.Kind == ValueMap &&
		len(value.Fields) == 0 && value.Runtime == delta.root.Runtime &&
		value.Ref == delta.root.Ref && sameMapBacking(value.Map, delta.root.Map) && sameMapBacking(value.MapKeys, delta.root.MapKeys)
}

// Only a direct Apex backing field has a writeback whose complete mutation is
// known here. Other stores keep their original accessor/coercion behavior.
func (vm *VM) storeMapPutReceiverWithStaticIndexDelta(receiverName string, receiver Value, key, item Value) (*staticAliasIndexDelta, error) {
	fallback := func() (*staticAliasIndexDelta, error) { return nil, vm.storeReceiver(receiverName, receiver) }
	parts := strings.Split(receiverName, ".")
	if len(parts) != 2 || parts[0] != "this" || parts[1] == "" || receiver.Kind != ValueMap || receiver.Ref == 0 || receiver.Runtime != "" || len(receiver.Fields) != 0 ||
		vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		return fallback()
	}
	root, ok := vm.Globals["this"]
	if !ok || root.Kind != ValueObject || root.Ref == 0 || vm.isSObjectLikeType(root.Type) {
		return fallback()
	}
	class, loaded := vm.lookupClass(root.Type)
	if !loaded || class.PlatformType {
		return fallback()
	}
	field, owner, known := vm.lookupReceiverField(root.Type, parts[1])
	if !known || field.Setter != nil || field.Getter != nil && !vm.isCurrentGetter(field.Getter) {
		return fallback()
	}
	if declared := vm.resolveTypeNameInClass(owner, field.Type); !strings.EqualFold(declared, receiver.Type) {
		return fallback()
	}
	name := field.Name
	if name == "" {
		name = parts[1]
	}
	stored, found := root.Fields[name]
	delta := &staticAliasIndexDelta{root: receiver, children: []Value{key, item}, parentRef: root.Ref, fieldName: name}
	if !found || !staticAliasIndexDeltaMatchesMap(stored, delta) {
		return fallback()
	}
	vm.prepareMapPutParentContainment(delta)
	if err := vm.assignPathWithStaticIndexDelta(root, []string{parts[1]}, receiver, delta); err != nil {
		return nil, err
	}
	// Coercion can fall back inside assignment. The operation's final map
	// propagation must also fall back if that store changed the backing value.
	storedRoot := vm.Globals["this"]
	if stored, exists := storedRoot.Fields[name]; !exists || !staticAliasIndexDeltaMatchesMap(stored, delta) {
		return nil, nil
	}
	return delta, nil
}

// A map already indexed at this location has no newly reachable ancestors.
// Fully visit each inserted subtree, including children of known references;
// their contents may have changed since their identity was first indexed.
func (vm *VM) rememberStaticIndexDeltaInField(delta *staticAliasIndexDelta, location staticFieldRef) bool {
	if delta == nil || delta.root.Kind != ValueMap || delta.root.Ref == 0 || vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		return false
	}
	location = vm.canonicalStaticFieldLocation(location)
	if !vm.staticValueRefs[delta.root.Ref] || !vm.staticValueRefFields[delta.root.Ref].contains(location) {
		return false
	}
	vm.ensureStaticValueRefsOwned()
	vm.scanStaticAliasIndexDeltaChildren(delta)
	// Register the map identity without marking it visited. If an inserted
	// subtree links back to this map, that entire reachable graph must still
	// be scanned: known identities may have gained unindexed descendants.
	vm.staticValueRefs[delta.root.Ref] = true
	locations := vm.staticValueRefFields[delta.root.Ref]
	locations.add(location)
	vm.staticValueRefFields[delta.root.Ref] = locations
	if vm.perfRecorder != nil {
		vm.perfRecorder.recordStaticAliasRefScanVisits(1)
	}
	for ref := range delta.childRefs {
		vm.staticValueRefs[ref] = true
		locations := vm.staticValueRefFields[ref]
		locations.add(location)
		vm.staticValueRefFields[ref] = locations
	}
	vm.invalidateStaticHintsForDelta(location, delta, delta.childRefs)
	return true
}

func (vm *VM) propagateAliasSnapshotToCacheEntryWithIndexDelta(location staticFieldRef, previous aliasSnapshot, updated Value, delta *staticAliasIndexDelta) bool {
	if delta == nil {
		return vm.propagateAliasSnapshotToCacheEntry(location, previous, updated)
	}
	old, ok := vm.cacheAliasRoot(location)
	if !ok {
		return false
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	replaced, changed := replaceCacheAliasSnapshot(old, previous, updated, seen)
	aliasRefSetPool.Put(seenPtr)
	if !changed {
		vm.forgetStaticValueRefInField(previous.ref, location)
		return true
	}
	vm.storeCacheAliasRoot(location, replaced)
	if vm.rememberStaticIndexDeltaInField(delta, location) {
		return true
	}
	if old.Ref == previous.ref && old.Kind == previous.kind {
		vm.rememberAdditionalStaticValueRefsInField(old, replaced, location)
	} else {
		vm.rememberStaticAliasUpdateRefs(previous, updated, location)
	}
	return true
}
