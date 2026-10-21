package vm

import "strings"

func (vm *VM) cacheCapturedAPI() bool {
	if vm.Org == nil {
		return false
	}
	version := strings.TrimSpace(vm.currentMethod.APIVersion)
	if version == "" {
		version = vm.Org.APIVersion
	}
	switch version {
	case "62.0", "67.0", "68.0":
		return true
	}
	return false
}

// Queueable Session reads and writes have no session outside a test. The
// configured test cache still accepts those operations in its data silo.
func (vm *VM) cacheCapturedAsyncSessionAccess(callee, partition string, args []Value) error {
	if !vm.cacheCapturedAPI() || vm.currentAsyncKind != "Queueable" ||
		vm.testContext != nil || cacheScope(callee) != "Cache.Session" {
		return nil
	}
	put := strings.EqualFold(callee, "Cache.Session.put") && len(args) == 2
	get := strings.EqualFold(callee, "Cache.Session.get") && len(args) == 1
	if (!put && !get) || args[0].Kind != ValueString || len(strings.Split(args[0].Text, ".")) != 3 {
		return nil
	}
	_, name, _ := strings.Cut(partition, ":")
	if record, ok := vm.cachePartitionRecord(name); ok && record.Fields["NamespacePrefix"].String == "" {
		for _, capacity := range vm.Org.Objects["PlatformCachePartitionType"].Records {
			ref := capacity.Fields["PlatformCachePartitionId"]
			if (ref.ID == record.ID || ref.String == string(record.ID)) && strings.EqualFold(capacity.Fields["CacheType"].String, "Session") && capacity.Fields["AllocatedCapacity"].Integer == 1 {
				return newExceptionError("cache.Session.SessionCacheNoSessionException", "Session not found - Session Cache not supported")
			}
		}
	}
	return nil
}
