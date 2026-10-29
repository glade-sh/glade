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
}

// enumSuffixClassCandidates returns the class entry keys whose current class
// name ends with "."+suffix and that have enum values, in sorted key order.
func (vm *VM) enumSuffixClassCandidates(suffix string) []string {
	index := vm.enumSuffixClasses
	if index == nil || index.generation != vm.classLookupGeneration || index.classCount != len(vm.Classes) {
		index = vm.buildEnumSuffixClassIndex()
		vm.enumSuffixClasses = index
	}
	return index.bySuffix[suffix]
}

func (vm *VM) buildEnumSuffixClassIndex() *enumSuffixClassIndex {
	index := &enumSuffixClassIndex{generation: vm.classLookupGeneration, classCount: len(vm.Classes), bySuffix: make(map[string][]string)}
	keys := make([]string, 0, len(vm.Classes))
	for key, class := range vm.ClassEntries() {
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
