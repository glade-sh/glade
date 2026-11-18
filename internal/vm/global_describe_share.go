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
		share.registerSharedMap(value.Map)
	}
	share.mu.Unlock()
	// Type tokens are writable in the existing local runtime. The token
	// set is immutable shared data; a VM privatizes a token when assignPath
	// first writes it (privatizeSharedDescribeToken). The map itself is shared
	// too: every VM of the generation holds one backing, flagged shared, and
	// the first write through any route clones it for that VM alone
	// (prepareSharedMapMutation, privatizeSharedDescribeToken). Keys and order
	// are shared as well; each VM keeps its own identity.
	vm.noteDescribeTokenShare(share)
	value.Ref = newValueRef()
	value.mapBackingShared = true
	value.mapEntriesShared = true
	return value, true
}

// sharedGlobalDescribeExtension returns the generation's extension of a shared
// global describe map, building it once with build. base must be a map this
// generation's share holds; any other map (a private copy after a write, a
// projection) returns false and the caller builds privately. kind and detail
// name the extension (named references with their object list, change events
// with the API major). The extension's tokens join the shared token set.
func (vm *VM) sharedGlobalDescribeExtension(base map[string]Value, kind, detail string, build func() map[string]Value) (map[string]Value, bool) {
	vm.ensureMetadataCacheGeneration()
	share := vm.describeShare
	if share == nil || vm.Org == nil || vm.Org.RuntimeSchemaStamp == "" || vm.Org.RuntimeSchemaStamp != share.stamp || len(base) == 0 {
		return nil, false
	}
	key := globalDescribeExtensionKey{base: reflect.ValueOf(base).Pointer(), kind: kind, detail: detail}
	share.mu.Lock()
	defer share.mu.Unlock()
	if _, shared := share.sharedMaps[key.base]; !shared {
		return nil, false
	}
	extended, ok := share.extensions[key]
	if !ok {
		// A nil result means the layer adds nothing; that is remembered too.
		extended = build()
		if share.extensions == nil {
			share.extensions = make(map[globalDescribeExtensionKey]map[string]Value)
		}
		share.extensions[key] = extended
		if extended != nil {
			share.registerTokenFields(extended)
			share.registerSharedMap(extended)
		}
	}
	vm.noteDescribeTokenShare(share)
	return extended, true
}

// globalDescribeExtensionKey identifies an extension by the base map it
// extends. The share keeps the base alive, so the pointer stays unique.
type globalDescribeExtensionKey struct {
	base         uintptr
	kind, detail string
}

// registerSharedMap records a map backing that VMs share. Callers hold share.mu.
func (share *describeShareCache) registerSharedMap(entries map[string]Value) {
	if share.sharedMaps == nil {
		share.sharedMaps = make(map[uintptr]struct{})
	}
	share.sharedMaps[reflect.ValueOf(entries).Pointer()] = struct{}{}
}

// sharesMap reports whether entries is a backing map this VM shares with other
// VMs of its generation.
func (vm *VM) sharesMap(entries map[string]Value) bool {
	if len(entries) == 0 {
		return false
	}
	pointer := reflect.ValueOf(entries).Pointer()
	for _, share := range vm.describeTokenShares {
		share.mu.Lock()
		_, shared := share.sharedMaps[pointer]
		share.mu.Unlock()
		if shared {
			return true
		}
	}
	return false
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
	// A map holds the token by value. Repoint the entries at the private copy;
	// a map that other VMs share is cloned first, so their entries stay put.
	replace := func(entries map[string]Value) (map[string]Value, bool) {
		copied := false
		for key, held := range entries {
			if held.Ref == token.Ref && sameMapBacking(held.Fields, sharedFields) {
				if !copied && vm.sharesMap(entries) {
					entries = maps.Clone(entries)
					copied = true
				}
				held.Fields = token.Fields
				entries[key] = held
			}
		}
		return entries, copied
	}
	// The raw cache and the per-VM extensions hold the token by value; update
	// them all, so every route reads one private token.
	replaceEntry := func(entry *Value) {
		var copied bool
		entry.Map, copied = replace(entry.Map)
		if copied {
			entry.mapEntriesShared = false
		}
	}
	if vm.globalDescribeCache != nil {
		replaceEntry(vm.globalDescribeCache)
	}
	for _, entry := range vm.globalDescribeVersionCaches {
		if entry != nil && entry != vm.globalDescribeCache {
			replaceEntry(entry)
		}
	}
	if memo := vm.namedGlobalDescribe; memo != nil {
		var copied bool
		memo.extended, copied = replace(memo.extended)
		memo.shared = memo.shared && !copied
	}
	if memo := vm.changeEventGlobal; memo != nil {
		var copied bool
		memo.extended, copied = replace(memo.extended)
		memo.shared = memo.shared && !copied
	}
}
