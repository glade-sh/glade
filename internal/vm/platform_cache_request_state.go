package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// The local session mock identifies one offline session. Only synchronous,
// non-test String entries with the observed TTL cross request boundaries.
// Other values, scopes and lifetimes remain VM-local.
func (vm *VM) cacheCommittedStringSupported(partition string) bool {
	if !vm.cacheCapturedAPI() || vm.Org.Namespace != "" || vm.testContext != nil ||
		vm.currentAsyncKind != "" || vm.localAsyncDrain {
		return false
	}
	return vm.cacheLocalStringPartitionSupported(partition)
}

func (vm *VM) cacheLocalStringPartitionSupported(partition string) bool {
	if !vm.cacheCapturedAPI() || vm.Org.Namespace != "" {
		return false
	}
	kind, name, _ := strings.Cut(partition, ":")
	cacheType := "Organization"
	switch kind {
	case "cache.orgpartition":
	case "cache.sessionpartition":
		cacheType = "Session"
	default:
		return false
	}
	record, ok := vm.cachePartitionRecord(name)
	if !ok || record.Fields["NamespacePrefix"].String != "" {
		return false
	}
	for _, capacity := range vm.Org.Objects["PlatformCachePartitionType"].Records {
		ref := capacity.Fields["PlatformCachePartitionId"]
		if (ref.ID == record.ID || ref.String == string(record.ID)) &&
			strings.EqualFold(capacity.Fields["CacheType"].String, cacheType) {
			return capacity.Fields["AllocatedCapacity"].Integer == 1
		}
	}
	return false
}

func (vm *VM) cacheRestoreCommittedString(partition, key string) {
	if !vm.cacheCommittedStringSupported(partition) {
		return
	}
	stored, exists := vm.Org.PlatformCache.StringEntry(partition, key)
	if !exists {
		return
	}
	if vm.platformCache == nil {
		vm.platformCache = map[string]map[string]cacheEntry{}
	}
	if vm.platformCache[partition] == nil {
		vm.platformCache[partition] = map[string]cacheEntry{}
	}
	vm.platformCache[partition][key] = cacheEntry{Value: String(stored.Text), ExpireAt: stored.ExpireAt}
}

func (vm *VM) cacheStageCommittedString(partition, key string, entry cacheEntry, ttlSeconds int64) {
	if !vm.cacheCommittedStringSupported(partition) {
		return
	}
	if entry.Value.Kind == ValueString && ttlSeconds == 300 {
		vm.Org.PlatformCache = vm.Org.PlatformCache.WithStringEntry(partition, key, storage.PlatformCacheString{Text: entry.Value.Text, ExpireAt: entry.ExpireAt})
	} else {
		vm.Org.PlatformCache = vm.Org.PlatformCache.WithoutStringEntry(partition, key)
	}
}

func (vm *VM) cacheForgetCommittedString(partition, key string) {
	if vm.cacheCommittedStringSupported(partition) {
		vm.Org.PlatformCache = vm.Org.PlatformCache.WithoutStringEntry(partition, key)
	}
}
