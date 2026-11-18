package vm

import "maps"

// staticRefIdx is the view of a VM's static reference index that the collect
// and forget paths read and write. A VM owns its maps outright (base nil), or
// it layers private overlay maps over a frozen base that sibling VMs share.
//
// Overlay rules, which keep a write costing O(changes) instead of O(index):
//   - refs: an overlay entry wins over the base. true marks a ref known, false
//     is a tombstone that hides a base entry.
//   - fields: an overlay entry wins over the base. An empty set is a tombstone.
//     A set read from the base shares its many map with every sibling, so it is
//     copied (per key) before any in-place change.
//
// The base maps are never written after they are shared.
type staticRefIdx struct {
	refs       map[uint64]bool
	fields     map[uint64]staticFieldRefSet
	baseRefs   map[uint64]bool
	baseFields map[uint64]staticFieldRefSet
}

func (x *staticRefIdx) known(ref uint64) bool {
	if known, ok := x.refs[ref]; ok {
		return known
	}
	return x.baseRefs[ref]
}

// locations returns the set for ref. When the VM is layered the set may share
// its many map with the base: callers must not change it in place.
func (x *staticRefIdx) locations(ref uint64) staticFieldRefSet {
	if set, ok := x.fields[ref]; ok {
		return set
	}
	return x.baseFields[ref]
}

// ownedLocations returns the set for ref with a private many map.
func (x *staticRefIdx) ownedLocations(ref uint64) staticFieldRefSet {
	if set, ok := x.fields[ref]; ok {
		return set
	}
	set := x.baseFields[ref]
	set.many = maps.Clone(set.many)
	return set
}

func (x *staticRefIdx) store(ref uint64, set staticFieldRefSet) {
	x.fields[ref] = set
	x.refs[ref] = true
}

// add records ref as present at location.
func (x *staticRefIdx) add(ref uint64, location staticFieldRef) {
	if x.baseFields != nil && x.known(ref) && x.locations(ref).contains(location) {
		return
	}
	set := x.ownedLocations(ref)
	set.add(location)
	x.store(ref, set)
}

// markKnown records ref without a location.
func (x *staticRefIdx) markKnown(ref uint64) {
	if !x.known(ref) {
		x.refs[ref] = true
	}
}

// remove drops location from ref and reports whether it was present.
func (x *staticRefIdx) remove(ref uint64, location staticFieldRef) bool {
	if !x.locations(ref).contains(location) {
		return false
	}
	set := x.ownedLocations(ref)
	set.remove(location)
	if !set.empty() {
		x.fields[ref] = set
		return true
	}
	if _, inBase := x.baseFields[ref]; inBase {
		x.fields[ref] = staticFieldRefSet{}
		x.refs[ref] = false
		return true
	}
	delete(x.fields, ref)
	delete(x.refs, ref)
	return true
}

// each visits every ref that has a location set. fn may call remove.
func (x *staticRefIdx) each(fn func(ref uint64)) {
	for ref, set := range x.fields {
		if x.baseFields != nil && set.empty() {
			continue
		}
		fn(ref)
	}
	for ref := range x.baseFields {
		if _, shadowed := x.fields[ref]; !shadowed {
			fn(ref)
		}
	}
}

// staticRefIndex returns the VM's current index view.
func (vm *VM) staticRefIndex() staticRefIdx {
	return staticRefIdx{refs: vm.staticValueRefs, fields: vm.staticValueRefFields, baseRefs: vm.staticRefBaseRefs, baseFields: vm.staticRefBaseFields}
}

// staticRefIndexSnapshot returns the effective index as plain maps. The result
// shares location sets with the VM and must be treated as read-only.
func (vm *VM) staticRefIndexSnapshot() (map[uint64]bool, map[uint64]staticFieldRefSet) {
	if vm.staticRefBaseRefs == nil && vm.staticRefBaseFields == nil {
		return vm.staticValueRefs, vm.staticValueRefFields
	}
	refs := maps.Clone(vm.staticRefBaseRefs)
	if refs == nil {
		refs = make(map[uint64]bool)
	}
	for ref, known := range vm.staticValueRefs {
		if known {
			refs[ref] = true
		} else {
			delete(refs, ref)
		}
	}
	fields := maps.Clone(vm.staticRefBaseFields)
	if fields == nil {
		fields = make(map[uint64]staticFieldRefSet)
	}
	for ref, set := range vm.staticValueRefFields {
		if set.empty() {
			delete(fields, ref)
		} else {
			fields[ref] = set
		}
	}
	return refs, fields
}

// buildStaticValueRefs replaces the index with a fresh full collection.
func (vm *VM) buildStaticValueRefs() {
	vm.staticRefBaseRefs, vm.staticRefBaseFields = nil, nil
	vm.staticValueRefs, vm.staticValueRefFields = vm.collectStaticValueRefs()
	vm.staticValueRefsShared = false
}

// ensureStaticValueRefsOwned prepares the index for a write. A shared index
// becomes the frozen base under empty private overlays; nothing is copied.
func (vm *VM) ensureStaticValueRefsOwned() {
	if !vm.staticValueRefsShared {
		return
	}
	vm.staticRefBaseRefs, vm.staticRefBaseFields = vm.staticValueRefs, vm.staticValueRefFields
	vm.staticValueRefs = make(map[uint64]bool)
	vm.staticValueRefFields = make(map[uint64]staticFieldRefSet)
	vm.staticValueRefsShared = false
}

// flattenStaticValueRefs folds a layered index into plain maps so it can be
// handed to clones as a new frozen base.
func (vm *VM) flattenStaticValueRefs() {
	if vm.staticRefBaseRefs == nil && vm.staticRefBaseFields == nil {
		return
	}
	vm.staticValueRefs, vm.staticValueRefFields = vm.staticRefIndexSnapshot()
	vm.staticRefBaseRefs, vm.staticRefBaseFields = nil, nil
}
