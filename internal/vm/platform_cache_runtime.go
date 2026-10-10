package vm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Partition names and capacity come from the local org's setup records. Cache
// contents remain VM-local; no hosted cache service is contacted.
func (vm *VM) cachePartitionRecord(name string) (storage.Record, bool) {
	if vm.Org != nil {
		for _, record := range vm.Org.Objects["PlatformCachePartition"].Records {
			if strings.EqualFold(cacheRecordPartitionName(record), name) {
				return record, true
			}
		}
	}
	return storage.Record{}, false
}

func cacheRecordPartitionName(record storage.Record) string {
	namespace := record.Fields["NamespacePrefix"].String
	if namespace == "" {
		namespace = "local"
	}
	return namespace + "." + record.Fields["DeveloperName"].String
}

func (vm *VM) cacheDefaultName() string {
	selected := ""
	selectedLocal := false
	if vm.Org != nil {
		for _, record := range vm.Org.Objects["PlatformCachePartition"].Records {
			if !record.Fields["IsDefaultPartition"].Boolean {
				continue
			}
			name := cacheRecordPartitionName(record)
			local := record.Fields["NamespacePrefix"].String == ""
			// The configured local default owns bare keys.
			// Mixed managed/local defaults cannot be captured in the scratch
			// org: prefer local, with a stable name tie-break for imported data.
			if selected == "" || (local && !selectedLocal) || (local == selectedLocal && name < selected) {
				selected, selectedLocal = name, local
			}
		}
	}
	if selected != "" {
		return selected
	}
	return "local.default"
}

func cacheScope(callee string) string {
	if strings.Contains(strings.ToLower(callee), "cache.session") {
		return "Cache.Session"
	}
	return "Cache.Org"
}

func cachePartitionException(callee, message string) error {
	exceptionType := "cache.Org.OrgCacheException"
	if cacheScope(callee) == "Cache.Session" {
		exceptionType = "cache.Session.SessionCacheException"
	}
	return newExceptionError(exceptionType, message)
}

func cacheAlphanumeric(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func (vm *VM) cachePartition(callee string, arg Value) (Value, error) {
	if arg.Kind == ValueNull || (arg.Kind == ValueString && arg.Text == "") {
		return Null, newExceptionError("cache.InvalidParamException", "Partition name cannot be null or empty and must be alphanumeric")
	}
	if arg.Kind != ValueString {
		return Null, fmt.Errorf("%s expects String partition name", callee)
	}
	parts := strings.Split(arg.Text, ".")
	namespace, name := "local", parts[0]
	if len(parts) > 1 {
		namespace, name = parts[0], parts[1]
	}
	if name == "" {
		return Null, cachePartitionException(callee, "Invalid partition: partition is null")
	}
	if !cacheAlphanumeric(name) {
		return Null, cachePartitionException(callee, "Invalid partition: partition name must be alphanumeric.")
	}
	qualified := namespace + "." + name
	record, found := vm.cachePartitionRecord(qualified)
	if found {
		qualified = cacheRecordPartitionName(record)
	} else {
		// Preserve the implicit default used by an unconfigured local VM.
		configured := vm.Org != nil && len(vm.Org.Objects["PlatformCachePartition"].Records) != 0
		if configured || !strings.EqualFold(qualified, "local.default") {
			prefix := "Invalid partition - "
			if cacheScope(callee) == "Cache.Session" {
				prefix = "Invalid partition: "
			}
			return Null, cachePartitionException(callee, prefix+"partition '"+qualified+"' does not exist.")
		}
		qualified = "local.default"
	}
	partition := Object(cachePartitionTypeFromCallee(callee))
	partition.Fields["name"] = String(qualified)
	partition.Fields["scope"] = String(cacheScope(callee))
	return partition, nil
}

const cacheInvalidKey = "Invalid Key, Key cannot be null or empty and must be alphanumeric"
const cacheUnqualifiedKey = "Invalid Key, Key is not properly qualified with partition and namespace. Valid key format is <namespace>.<partition>.<key>"

// receiverPartition is empty for a static call, whose key can select a
// partition. Partition members accept a single unqualified key.
func (vm *VM) cacheOperationKey(callee string, arg Value, receiverPartition string) (string, string, error) {
	if arg.Kind == ValueNull || (arg.Kind == ValueString && arg.Text == "") {
		return "", "", newExceptionError("cache.InvalidParamException", cacheInvalidKey)
	}
	if arg.Kind != ValueString {
		return "", "", fmt.Errorf("%s key expects String", callee)
	}
	key, partition := arg.Text, receiverPartition
	parts := strings.Split(key, ".")
	if receiverPartition == "" {
		partition = vm.cacheDefaultName()
		if len(parts) > 1 {
			if len(parts) != 3 {
				return "", "", newExceptionError("cache.InvalidParamException", cacheUnqualifiedKey)
			}
			partition, key = parts[0]+"."+parts[1], parts[2]
			resolved, err := vm.cachePartition(callee, String(partition))
			if err != nil {
				return "", "", err
			}
			partition = resolved.Fields["name"].Text
		}
	} else if len(parts) > 1 {
		message := cacheUnqualifiedKey
		if len(parts) == 3 {
			message = cacheInvalidKey
		}
		return "", "", newExceptionError("cache.InvalidParamException", message)
	}
	if key == "" {
		return "", "", newExceptionError("cache.InvalidParamException", cacheInvalidKey)
	}
	if receiverPartition != "" {
		for _, r := range key {
			if r < 128 && !cacheAlphanumeric(string(r)) {
				return "", "", newExceptionError("cache.InvalidParamException", cacheInvalidKey)
			}
		}
	}
	if !cacheAlphanumeric(key) {
		return "", "", newExceptionError("cache.InvalidParamException", fmt.Sprintf("Failed %s operation for key '%s': Invalid key: key must be alphanumeric.", cacheScope(callee), arg.Text))
	}
	if len(key) > 50 {
		return "", "", newExceptionError("cache.InvalidParamException", fmt.Sprintf("Failed %s operation for key '%s': Invalid key: max key length is 50", cacheScope(callee), arg.Text))
	}
	return cachePartitionKey(cachePartitionTypeFromCallee(callee), partition), key, nil
}

func (vm *VM) cacheHasCapacity(partition string) bool {
	_, name, _ := strings.Cut(partition, ":")
	record, ok := vm.cachePartitionRecord(name)
	if !ok || vm.Org == nil {
		return true
	}
	cacheType := "Organization"
	if strings.HasPrefix(partition, "cache.sessionpartition:") {
		cacheType = "Session"
	}
	for _, capacity := range vm.Org.Objects["PlatformCachePartitionType"].Records {
		ref := capacity.Fields["PlatformCachePartitionId"]
		if (string(ref.ID) == string(record.ID) || ref.String == string(record.ID)) &&
			strings.EqualFold(capacity.Fields["CacheType"].String, cacheType) {
			return capacity.Fields["AllocatedCapacity"].Integer > 0
		}
	}
	return true
}

func (vm *VM) cachePutValue(callee, partition, key string, args []Value) error {
	if args[1].Kind == ValueNull {
		return newExceptionError("cache.InvalidParamException", "Value cannot be null")
	}
	ttl, err := cachePutTTL(callee, args)
	if err != nil {
		return err
	}
	if !vm.cacheHasCapacity(partition) {
		return nil
	}
	vm.cachePut(partition, key, args[1], ttl)
	if cacheScope(callee) == "Cache.Org" && len(args) == 5 && args[4].Kind == ValueBool {
		entry := vm.platformCache[partition][key]
		entry.Immutable = args[4].Bool
		vm.platformCache[partition][key] = entry
	}
	return nil
}

func (vm *VM) cacheRemoveValue(callee, partition, key, rawKey string) (Value, error) {
	if _, exists := vm.cacheGet(partition, key); exists && vm.platformCache[partition][key].Immutable {
		return Null, newExceptionError("cache.Org.OrgCacheException", fmt.Sprintf("Failed %s.remove() for key '%s': Key '%s' is immutable", cacheScope(callee), rawKey, rawKey))
	}
	_, removed := vm.cacheRemove(partition, key)
	return Bool(removed), nil
}

func (vm *VM) cacheBuilderExecutionError(callee, rawKey string, err error) error {
	var thrown *apexThrowError
	if !errors.As(err, &thrown) {
		return err
	}
	return newExceptionError("cache.CacheBuilderExecutionException", fmt.Sprintf("Failed %s.get() for key '%s': Exception while executing cache builder's load method: %s, %s", cacheScope(callee), rawKey, vm.exceptionQualifiedTypeName(thrown.value.Type), thrown.value.Fields["message"].Text))
}
