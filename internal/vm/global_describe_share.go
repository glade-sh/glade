package vm

import (
	"maps"
	"reflect"
)

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
		share.registerTokenFields(value.Map)
	}
	share.mu.Unlock()
	// Type tokens are writable in the existing local runtime. The token
	// set is immutable shared data; a VM privatizes a token when assignPath
	// first writes it (privatizeSharedDescribeToken). Keys and order stay
	// shared too; typed bindings share this VM's entries until their first
	// map write.
	vm.noteDescribeTokenShare(share)
	value.Ref = newValueRef()
	value.Map = maps.Clone(value.Map)
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

// registerTokenFields records the Fields maps of a shared build's tokens.
// Callers hold share.mu.
func (share *describeShareCache) registerTokenFields(entries map[string]Value) {
	if share.tokenFields == nil {
		share.tokenFields = make(map[uintptr]struct{}, len(entries)/2+1)
	}
	for _, token := range entries {
		if token.Fields != nil {
			share.tokenFields[reflect.ValueOf(token.Fields).Pointer()] = struct{}{}
		}
	}
}

func (vm *VM) noteDescribeTokenShare(share *describeShareCache) {
	for _, known := range vm.describeTokenShares {
		if known == share {
			return
		}
	}
	vm.describeTokenShares = append(vm.describeTokenShares, share)
}

// privatizeSharedDescribeToken gives token private Fields when they belong to
// a shared global describe build, and points this VM's cached describe maps at
// the private copy. The caller writes the token next and propagates the
// updated value to its other holders by identity, as for any field write.
// The Ref is unchanged, so every holder in this VM still resolves to one token.
func (vm *VM) privatizeSharedDescribeToken(token *Value) {
	if token != nil && token.fieldsShared {
		// A shared child relationship value or one of its tokens. Holders in
		// this VM follow by identity, as for a global describe token.
		token.detachSharedFields()
		return
	}
	if token == nil || len(vm.describeTokenShares) == 0 || token.Kind != ValueObject || token.Fields == nil {
		return
	}
	pointer := reflect.ValueOf(token.Fields).Pointer()
	shared := false
	for _, share := range vm.describeTokenShares {
		share.mu.Lock()
		_, shared = share.tokenFields[pointer]
		share.mu.Unlock()
		if shared {
			break
		}
	}
	if !shared {
		return
	}
	sharedFields := token.Fields
	token.Fields = maps.Clone(sharedFields)
	replace := func(entries map[string]Value) {
		for key, held := range entries {
			if held.Ref == token.Ref && sameMapBacking(held.Fields, sharedFields) {
				held.Fields = token.Fields
				entries[key] = held
			}
		}
	}
	// The raw cache and the per-VM extensions cloned from it hold the token
	// by value; update them all, so every route reads one private token.
	if vm.globalDescribeCache != nil {
		replace(vm.globalDescribeCache.Map)
	}
	for _, entry := range vm.globalDescribeVersionCaches {
		if entry != nil && entry != vm.globalDescribeCache {
			replace(entry.Map)
		}
	}
	if vm.namedGlobalDescribe != nil {
		replace(vm.namedGlobalDescribe.extended)
	}
	if vm.changeEventGlobal != nil {
		replace(vm.changeEventGlobal.extended)
	}
}
