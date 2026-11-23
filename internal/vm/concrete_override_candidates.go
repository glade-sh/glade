package vm

import "strings"

// concreteOverrideCandidateKey names everything the candidate filter of
// uniqueConcreteOverride reads: the class lookup generation and size, the
// org metadata lifetime, and the frame state that class and object name
// resolution consult. The candidate list holds only the type filter; every
// per-call check (test scope, overload resolution, receiver fields) still
// runs on each call. The order of candidates does not matter: the result
// depends only on how many candidates match and which one matches alone.
type concreteOverrideCandidateKey struct {
	baseType   string
	generation uint64
	classes    int
	lifetime   metadataCacheLifetime
	class      string
	namespace  string
	execution  string
	caller     string
	trigger    bool
}

// verifyConcreteOverrideCandidates, set by the package tests, recomputes
// every cached candidate list and panics on a difference.
var verifyConcreteOverrideCandidates bool

// concreteOverrideCandidates lists the classes that are, or extend, baseType
// without being it.
func (vm *VM) concreteOverrideCandidates(baseType string) []string {
	cacheable := vm.classOverlay == nil && !vm.classMapWritten && vm.classLookupGeneration != 0
	var key concreteOverrideCandidateKey
	if cacheable {
		key = concreteOverrideCandidateKey{
			baseType:   baseType,
			generation: vm.classLookupGeneration,
			classes:    len(vm.Classes),
			lifetime:   metadataLifetimeForOrg(vm.Org),
			class:      vm.currentClass,
			namespace:  vm.currentNamespace,
			execution:  vm.currentExecutionNamespace(),
			caller:     vm.currentCallerNamespace(),
			trigger:    vm.currentTrigger,
		}
		if names, ok := vm.concreteOverrideCandidateMemo[key]; ok {
			if verifyConcreteOverrideCandidates {
				if direct := vm.concreteOverrideCandidatesDirect(baseType); !sameNameSet(direct, names) {
					panic("concrete override candidates changed under an unchanged key: " + baseType)
				}
			}
			return names
		}
	}
	names := vm.concreteOverrideCandidatesDirect(baseType)
	if cacheable {
		if vm.concreteOverrideCandidateMemo == nil || len(vm.concreteOverrideCandidateMemo) >= 4096 {
			vm.concreteOverrideCandidateMemo = make(map[concreteOverrideCandidateKey][]string)
		}
		vm.concreteOverrideCandidateMemo[key] = names
	}
	return names
}

func (vm *VM) concreteOverrideCandidatesDirect(baseType string) []string {
	var names []string
	for className, class := range vm.classEntriesRaw() {
		if strings.EqualFold(className, baseType) || (!vm.typeMatches(className, baseType, make(map[string]bool)) && !classExtendsType(class, baseType)) {
			continue
		}
		names = append(names, className)
	}
	return names
}

func sameNameSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]int, len(left))
	for _, name := range left {
		seen[name]++
	}
	for _, name := range right {
		if seen[name] == 0 {
			return false
		}
		seen[name]--
	}
	return true
}
