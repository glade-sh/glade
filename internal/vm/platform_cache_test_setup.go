package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// EnablePlatformCacheTestSetupSnapshot selects the test runner's setup phase.
// Runtime clones start outside this phase and never publish method writes to
// the immutable setup seed.
func (vm *VM) EnablePlatformCacheTestSetupSnapshot() {
	vm.platformCacheSetupPhase = true
}

func (vm *VM) cacheStageTestSetupString(callee, partition, key string, args []Value) {
	if !vm.platformCacheSetupPhase || vm.testContext == nil || vm.currentAsyncKind != "" ||
		!vm.cacheLocalStringPartitionSupported(partition) {
		return
	}
	if len(args) == 2 && args[0].Kind == ValueString && args[1].Kind == ValueString &&
		len(strings.Split(args[0].Text, ".")) == 3 &&
		(strings.EqualFold(callee, "Cache.Org.put") || strings.EqualFold(callee, "Cache.Session.put")) {
		vm.Org.PlatformCacheTestSetup = vm.Org.PlatformCacheTestSetup.WithStringEntry(partition, key, storage.PlatformCacheString{Text: args[1].Text})
	} else {
		vm.Org.PlatformCacheTestSetup = vm.Org.PlatformCacheTestSetup.WithoutStringEntry(partition, key)
	}
}

func (vm *VM) cacheRestoreTestSetupString(partition, key string) {
	if vm.testContext == nil || vm.Org == nil || vm.Org.PlatformCacheTestSetup == nil ||
		vm.currentAsyncKind != "" || !vm.cacheLocalStringPartitionSupported(partition) {
		return
	}
	identity := partition + "\x00" + key
	if vm.platformCacheSetupTouched[identity] {
		return
	}
	vm.cacheMarkTestSetupStringTouched(partition, key)
	stored, exists := vm.Org.PlatformCacheTestSetup.StringEntry(partition, key)
	if !exists {
		return
	}
	if vm.platformCache == nil {
		vm.platformCache = map[string]map[string]cacheEntry{}
	}
	if vm.platformCache[partition] == nil {
		vm.platformCache[partition] = map[string]cacheEntry{}
	}
	vm.platformCache[partition][key] = cacheEntry{Value: String(stored.Text)}
}

func (vm *VM) cacheMarkTestSetupStringTouched(partition, key string) {
	if vm.testContext == nil || vm.Org == nil || vm.Org.PlatformCacheTestSetup == nil || !vm.cacheLocalStringPartitionSupported(partition) {
		return
	}
	if vm.platformCacheSetupTouched == nil {
		vm.platformCacheSetupTouched = map[string]bool{}
	}
	vm.platformCacheSetupTouched[partition+"\x00"+key] = true
}

func (vm *VM) cacheInvalidateTestSetupString(partition, key string) {
	vm.cacheMarkTestSetupStringTouched(partition, key)
	if vm.platformCacheSetupPhase && vm.testContext != nil && vm.Org != nil && vm.Org.PlatformCacheTestSetup != nil && vm.cacheLocalStringPartitionSupported(partition) {
		vm.Org.PlatformCacheTestSetup = vm.Org.PlatformCacheTestSetup.WithoutStringEntry(partition, key)
	}
}
