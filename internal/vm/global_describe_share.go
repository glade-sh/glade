package vm

import "maps"

// Shared maps belong to an immutable runtime schema generation. Each VM has
// its own map identity and token overlay; writes never reach the shared build.
func (vm *VM) sharedGlobalDescribeBuild() (Value, bool) {
	vm.ensureMetadataCacheGeneration()
	if vm.describeShare == nil || vm.Org == nil || vm.Org.RuntimeSchemaStamp == "" || vm.Org.RuntimeSchemaStamp != vm.describeShare.stamp {
		return Value{}, false
	}
	share := vm.describeShare
	key := vm.currentCallerNamespace() + "\x00" + vm.globalDescribeAvailabilityKey()
	share.mu.Lock()
	if share.globalMaps == nil {
		share.globalMaps = make(map[string]Value)
	}
	value, ok := share.globalMaps[key]
	if !ok {
		value = vm.buildSchemaGlobalDescribe()
		share.globalMaps[key] = value
	}
	share.mu.Unlock()
	// Type tokens are writable in the existing local runtime. Privatize their
	// fields once per VM, retaining aliases between different dictionary keys.
	// Keys and order remain immutable and shared; typed bindings share this
	// VM's entries until their first map write.
	value.Ref = newValueRef()
	shared := value.Map
	value.Map = make(map[string]Value, len(shared))
	tokens := make(map[uint64]Value, len(shared)/2+1)
	for key, token := range shared {
		private, ok := tokens[token.Ref]
		if !ok {
			private = token
			private.Ref = newValueRef()
			private.Fields = maps.Clone(token.Fields)
			tokens[token.Ref] = private
		}
		value.Map[key] = private
	}
	return value, true
}

type globalDescribeKeyMetadata struct {
	keys  map[string]Value
	order []string
}

// A projection's keys are immutable schema data. Reuse them only after checking
// the exact key set: prior writes through the raw app/module map can change a
// VM's projection even while the org schema itself is unchanged.
func (vm *VM) fillGlobalDescribeProjectionKeys(value *Value, caller string) {
	vm.ensureMetadataCacheGeneration()
	// A shared backing map is detached before any write, so the same backing
	// with the same caller still holds the key set verified last time.
	memo := &vm.globalDescribeKeysVerified
	if value.mapBackingShared && memo.keys != nil && memo.caller == caller && sameMapBacking(memo.backing, value.Map) {
		value.MapKeys, value.MapOrder = memo.keys, memo.order
		return
	}
	defer func() {
		if value.mapBackingShared && value.MapKeys != nil && len(value.MapKeys) == len(value.Map) {
			*memo = globalDescribeKeysVerifiedMemo{backing: value.Map, caller: caller, keys: value.MapKeys, order: value.MapOrder}
		}
	}()
	// A mutable worker cannot borrow preparation from its old frozen
	// generation. It may retain private immutable keys, validated against the
	// complete current key set every time. Values/tokens are never cached here.
	cache := &vm.globalDescribeProjectionKeys
	share := vm.describeShare
	if share != nil && vm.Org != nil && vm.Org.RuntimeSchemaStamp != "" && vm.Org.RuntimeSchemaStamp == share.stamp {
		share.mu.Lock()
		defer share.mu.Unlock()
		cache = &share.globalProjectionKeys
	}
	if cached, ok := (*cache)[caller]; ok {
		matches := len(cached.keys) == len(value.Map)
		if matches {
			for key := range value.Map {
				if _, ok := cached.keys[key]; !ok {
					matches = false
					break
				}
			}
		}
		if matches {
			value.MapKeys, value.MapOrder = cached.keys, cached.order
			value.mapBackingShared = true
			return
		}
		fillDescribeMapStoredKeys(value)
		return
	}
	fillDescribeMapStoredKeys(value)
	if *cache == nil {
		*cache = make(map[string]globalDescribeKeyMetadata)
	}
	(*cache)[caller] = globalDescribeKeyMetadata{keys: value.MapKeys, order: value.MapOrder}
}

// globalDescribeKeysVerifiedMemo remembers the stored keys of the last shared
// backing map, so repeated getGlobalDescribe calls skip the O(n) key check.
type globalDescribeKeysVerifiedMemo struct {
	backing map[string]Value
	caller  string
	keys    map[string]Value
	order   []string
}
