package vm

import (
	"fmt"
	"strings"
)

// A real org with no configured partitions has no default.
// Keep the historical no-org VM fallback, and do not decide configured orgs
// with missing/multiple defaults from these bare-org observations.
func (vm *VM) cacheRequireCapturedDefault(callee string) error {
	if vm.Org != nil && vm.currentAsyncKind == "" && !vm.localAsyncDrain &&
		(vm.testContext == nil || !vm.testContext.Draining) {
		if partitions, present := vm.Org.Objects["PlatformCachePartition"]; present && len(partitions.Records) == 0 {
			return cachePartitionException(callee, "A default partition was not found.  To cache without partition reference, designate a partition as default.")
		}
	}
	return nil
}

// Configured local partitions report 0.0 at both zero and one MB.
// Do not reinterpret the getter as the allocated capacity.
func (vm *VM) cacheCapturedCapacity(scope, name string) (Value, bool) {
	if vm.currentAsyncKind != "" || vm.localAsyncDrain || (vm.testContext != nil && vm.testContext.Draining) {
		return Null, false
	}
	if record, ok := vm.cachePartitionRecord(name); ok && record.Fields["NamespacePrefix"].String == "" {
		cacheType := "Organization"
		if scope == "Cache.Session" {
			cacheType = "Session"
		}
		for _, capacity := range vm.Org.Objects["PlatformCachePartitionType"].Records {
			ref := capacity.Fields["PlatformCachePartitionId"]
			allocated := capacity.Fields["AllocatedCapacity"].Integer
			if (ref.ID == record.ID || ref.String == string(record.ID)) && strings.EqualFold(capacity.Fields["CacheType"].String, cacheType) &&
				allocated >= 0 && allocated <= 1 {
				value, _ := decimalFromText("0.0")
				return value, true
			}
		}
	}
	return Null, false
}

// Local test cache can store values while reporting no Session availability.
func (vm *VM) cacheCapturedSessionAvailable(scope, name string) bool {
	if strings.EqualFold(scope, "Cache.Session") && vm.cacheCapturedAPI() &&
		(vm.currentAsyncKind == "Queueable" || vm.currentAsyncKind == "Future" ||
			(vm.currentAsyncKind == "" && vm.testContext == nil && methodHasModifier(vm.currentMethod.Modifiers, "future"))) {
		if record, ok := vm.cachePartitionRecord(name); ok && record.Fields["NamespacePrefix"].String == "" {
			return false
		}
	}
	if strings.EqualFold(scope, "Cache.Session") && vm.testContext != nil &&
		!vm.testContext.Draining && vm.currentAsyncKind == "" {
		if record, ok := vm.cachePartitionRecord(name); ok && record.Fields["NamespacePrefix"].String == "" {
			return false
		}
	}
	return true
}

// A single fully-qualified Org read in SeeAllData=true is rejected
// even after an anonymous write. Partition members, builders and collection
// reads are not covered by this row and retain their existing behavior.
func (vm *VM) cacheCapturedDataSiloRead(callee string, args []Value, receiverPartition string) error {
	if receiverPartition != "" || len(args) != 1 || args[0].Kind != ValueString ||
		vm.testContext == nil || !vm.testContext.SeeAllData || vm.testContext.Draining ||
		vm.currentAsyncKind != "" || cacheScope(callee) != "Cache.Org" {
		return nil
	}
	parts := strings.Split(args[0].Text, ".")
	if len(parts) != 3 {
		return nil
	}
	if record, ok := vm.cachePartitionRecord(parts[0] + "." + parts[1]); ok && record.Fields["NamespacePrefix"].String == "" {
		return cachePartitionException(callee, fmt.Sprintf("Failed Cache.Org.get() for key '%s': Org cache is only supported in Data Silo tests", args[0].Text))
	}
	return nil
}

// Cache.Org.getPartition works beside an empty top-level Cache in an
// unnamespaced org. Do not adopt the uncaptured namespaced/lexical rule,
// or override user Cache.Org types, members, or runtime bindings.
func (vm *VM) cacheCapturedEmptyRootPrecedence(original, canonical string) bool {
	if canonical != "Cache.Org.getPartition" || vm.currentExecutionNamespace() != "" ||
		vm.calleeStartsWithRuntimeReceiver(original, nil) {
		return false
	}
	root, _, _ := strings.Cut(original, ".")
	if !strings.EqualFold(root, "Cache") {
		return false
	}
	class, ok := vm.lookupClass(root)
	if !ok || class.PlatformType || !strings.EqualFold(class.Name, "Cache") || class.Namespace != "" ||
		class.SuperClass != "" || len(class.Interfaces) != 0 || len(class.Fields) != 0 ||
		len(class.StaticFields) != 0 || len(class.Methods) != 0 ||
		len(class.StaticInitializers) != 0 || len(class.InstanceInitializers) != 0 {
		return false
	}
	if org, ok := vm.lookupClass("Cache.Org"); ok && !org.PlatformType {
		return false
	}
	return true
}
