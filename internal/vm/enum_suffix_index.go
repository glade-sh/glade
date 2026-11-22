package vm

import (
	"sort"
	"strings"
)

// enumSuffixClassIndex lists the class entries whose declared name ends with
// ".<suffix>" and that declare enum values. It replaces the scan over every
// class entry that an unqualified `Suffix.MEMBER` lookup previously performed
// when no class or static field resolved first. The index is rebuilt whenever
// the class lookup generation or class count changes; candidates are re-read
// and re-checked at lookup time, so a stale value can never be returned.
type enumSuffixClassIndex struct {
	generation uint64
	classCount int
	bySuffix   map[string][]string
	partial    bool
}

// enumSuffixClassCandidates returns the class entry keys whose current class
// name ends with "."+suffix and that have enum values, in sorted key order.
func (vm *VM) enumSuffixClassCandidates(suffix string) []string {
	// Gated stub enums of this simple name were entries before the gate.
	vm.bindGatedPlatformClassesNamed(suffix)
	index := vm.enumSuffixClasses
	root := vm.enumSuffixFrozenRoot()
	if index == nil || index.generation != vm.classLookupGeneration || index.classCount != len(vm.Classes) || index.partial != (root != nil) {
		if root != nil {
			index = &enumSuffixClassIndex{generation: vm.classLookupGeneration, classCount: len(vm.Classes), bySuffix: make(map[string][]string), partial: true}
		} else {
			index = vm.buildEnumSuffixClassIndex()
		}
		vm.enumSuffixClasses = index
	}
	if root != nil {
		if _, ok := index.bySuffix[suffix]; !ok {
			index.bySuffix[suffix] = vm.enumSuffixCandidatesFromRoot(root, suffix)
		}
	}
	return index.bySuffix[suffix]
}

// buildFrozenEnumSuffixKeys retains every alias, including duplicate class
// names. A later registration can replace the first alias for a name.
func (vm *VM) buildFrozenEnumSuffixKeys() map[string][]string {
	out := make(map[string][]string)
	for key, class := range vm.classEntriesNoLazy() {
		if len(class.EnumValues) == 0 {
			continue
		}
		dot := strings.LastIndexByte(class.Name, '.')
		if dot < 0 || dot+1 >= len(class.Name) {
			continue
		}
		suffix := class.Name[dot+1:]
		out[suffix] = append(out[suffix], key)
	}
	for _, keys := range out {
		sort.Strings(keys)
	}
	return out
}

func (vm *VM) enumSuffixFrozenRoot() *frozenClassLookup {
	if vm.classValuesWritten || (vm.classMapWritten && vm.classOverlay == nil) {
		return nil
	}
	var generation *frozenClassLookup
	if vm.classOverlay != nil {
		generation = vm.classOverlay.from
	} else {
		generation = vm.frozenClassLookup
	}
	if generation == nil || generation.copyPlan == nil || !generation.copyPlan.uniformAliases {
		return nil
	}
	if generation.searchEntries == nil {
		return nil
	}
	expectedCount := len(generation.searchEntries)
	if vm.classOverlay != nil {
		expectedCount += len(vm.classOverlay.added)
	}
	if len(vm.Classes) != expectedCount {
		return nil
	}
	root := generation.rootGeneration()
	if root.copyPlan == nil || !root.copyPlan.uniformAliases {
		return nil
	}
	return root
}

func (vm *VM) enumSuffixCandidatesFromRoot(root *frozenClassLookup, suffix string) []string {
	var written map[string]struct{}
	if vm.classOverlay != nil {
		written = vm.classOverlay.written
	} else if vm.frozenClassLookup != root {
		written = vm.frozenClassLookup.written
	}
	keys := append([]string(nil), root.enumSuffixKeys[suffix]...)
	for key := range written {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	seen := make(map[string]bool, len(keys))
	var out []string
	for _, key := range keys {
		class, ok := vm.classEntryRaw(key)
		dot := strings.LastIndexByte(class.Name, '.')
		if !ok || class.lazy != nil || len(class.EnumValues) == 0 || dot < 0 || dot+1 >= len(class.Name) || class.Name[dot+1:] != suffix || seen[class.Name] {
			continue
		}
		seen[class.Name] = true
		out = append(out, key)
	}
	return out
}

func (vm *VM) buildEnumSuffixClassIndex() *enumSuffixClassIndex {
	index := &enumSuffixClassIndex{generation: vm.classLookupGeneration, classCount: len(vm.Classes), bySuffix: make(map[string][]string)}
	keys := make([]string, 0, len(vm.Classes))
	for key, class := range vm.classEntriesNoLazy() {
		if len(class.EnumValues) == 0 {
			continue
		}
		if dot := strings.LastIndexByte(class.Name, '.'); dot < 0 || dot+1 >= len(class.Name) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		class, ok := vm.classEntry(key)
		if !ok || seen[class.Name] {
			continue
		}
		seen[class.Name] = true
		suffix := class.Name[strings.LastIndexByte(class.Name, '.')+1:]
		index.bySuffix[suffix] = append(index.bySuffix[suffix], key)
	}
	return index
}
