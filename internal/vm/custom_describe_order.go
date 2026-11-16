package vm

import (
	"sort"
	"strings"
)

// A custom object's field map lists its system fields in the platform order and
// then its custom fields (default_order_rest o3_*_fields_custom, APIs 62.0 and
// 67.0; the 12-key keySet digest matches this order and no other permutation of
// the three custom fields). Other shapes keep the sorted order.
var capturedCustomDescribeSystemOrder = []string{
	"Id", "OwnerId", "IsDeleted", "Name", "CreatedDate", "CreatedById",
	"LastModifiedDate", "LastModifiedById", "SystemModstamp",
}

func (vm *VM) sortCapturedCustomDescribeFieldNames(objectName string, names []string) bool {
	if vm == nil || vm.Org == nil || vm.Org.Namespace != "" || vm.currentMethod.Namespace != "" || !strings.HasSuffix(objectName, "__c") || strings.Count(objectName, "__") != 1 {
		return false
	}
	ordinals := make(map[string]int, len(capturedCustomDescribeSystemOrder))
	for index, name := range capturedCustomDescribeSystemOrder {
		ordinals[name] = index
	}
	system := 0
	for _, name := range names {
		if _, ok := ordinals[name]; ok {
			system++
		} else if !strings.HasSuffix(name, "__c") || strings.Count(name, "__") != 1 {
			// Any other standard, namespaced or generated field is not captured.
			return false
		}
	}
	if system != len(capturedCustomDescribeSystemOrder) {
		return false
	}
	sort.SliceStable(names, func(i, j int) bool {
		left, leftSystem := ordinals[names[i]]
		right, rightSystem := ordinals[names[j]]
		if leftSystem != rightSystem {
			return leftSystem
		}
		if leftSystem {
			return left < right
		}
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return true
}
