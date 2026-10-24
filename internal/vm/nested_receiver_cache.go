package vm

import "reflect"

const nestedReceiverCacheMaxEntries = 32
const nestedReceiverCacheMaxRefs = 1 << 16

// Cache a complete receiver graph after a negative mutation-window walk.
// Collection writes invalidate containing graphs; object-field and list-slot
// writes clear all graphs because they can introduce edges through shared slots.
// Divergent backings retain the conservative walk and never enter this cache.
type nestedReceiverGraph struct {
	root           aliasRuntimeBacking
	refs           map[uint64]struct{}
	maps           map[uintptr]map[string]Value
	latestMutation uint64
}

func (vm *VM) clearNestedReceiverCache() {
	clear(vm.nestedReceiverCache)
	clear(vm.nestedReceiverMapRoots)
	vm.nestedReceiverCacheRefs = 0
}

func (vm *VM) deleteNestedReceiverGraph(root uint64) {
	graph, ok := vm.nestedReceiverCache[root]
	if !ok {
		return
	}
	delete(vm.nestedReceiverCache, root)
	vm.nestedReceiverCacheRefs -= len(graph.refs)
	for backing := range graph.maps {
		roots := vm.nestedReceiverMapRoots[backing]
		delete(roots, root)
		if len(roots) == 0 {
			delete(vm.nestedReceiverMapRoots, backing)
		}
	}
}

func (vm *VM) invalidateNestedReceiverCache(ref uint64, updated ...Value) {
	if ref == 0 {
		vm.clearNestedReceiverCache()
		return
	}
	for root, graph := range vm.nestedReceiverCache {
		if _, ok := graph.refs[ref]; ok {
			vm.deleteNestedReceiverGraph(root)
		}
	}
	// Different identities may share map storage. Index its physical backing
	// too, so a write through either identity invalidates every containing graph.
	for _, value := range updated {
		if value.Kind != ValueMap {
			continue
		}
		for _, backing := range []map[string]Value{value.Map, value.MapKeys} {
			if backing == nil {
				continue
			}
			pointer := reflect.ValueOf(backing).Pointer()
			for root := range vm.nestedReceiverMapRoots[pointer] {
				vm.deleteNestedReceiverGraph(root)
			}
		}
	}
}

func (vm *VM) rememberNestedReceiverGraph(root Value, seen map[uint64]aliasRuntimeBacking) {
	if _, untracked := seen[0]; untracked {
		return
	}
	if root.Ref == 0 || len(seen) > nestedReceiverCacheMaxRefs {
		return
	}
	if vm.nestedReceiverCache == nil {
		vm.nestedReceiverCache = make(map[uint64]nestedReceiverGraph)
		vm.nestedReceiverMapRoots = make(map[uintptr]map[uint64]struct{})
	}
	vm.deleteNestedReceiverGraph(root.Ref)
	if len(vm.nestedReceiverCache) >= nestedReceiverCacheMaxEntries || vm.nestedReceiverCacheRefs+len(seen) > nestedReceiverCacheMaxRefs {
		vm.clearNestedReceiverCache()
	}
	graph := nestedReceiverGraph{root: aliasBackingOf(root), refs: make(map[uint64]struct{}, len(seen)), maps: make(map[uintptr]map[string]Value)}
	for ref, backing := range seen {
		graph.refs[ref] = struct{}{}
		if ref != root.Ref && sameBackingAliasRefreshKind(backing.kind) {
			graph.latestMutation = max(graph.latestMutation, vm.collectionMutationRefSeq[ref])
		}
		if backing.kind != ValueMap {
			continue
		}
		for _, mapping := range []map[string]Value{backing.fieldsOrMap, backing.mapKeys} {
			if mapping == nil {
				continue
			}
			pointer := reflect.ValueOf(mapping).Pointer()
			graph.maps[pointer] = mapping
			roots := vm.nestedReceiverMapRoots[pointer]
			if roots == nil {
				roots = make(map[uint64]struct{})
				vm.nestedReceiverMapRoots[pointer] = roots
			}
			roots[root.Ref] = struct{}{}
		}
	}
	vm.nestedReceiverCache[root.Ref] = graph
	vm.nestedReceiverCacheRefs += len(graph.refs)
}
