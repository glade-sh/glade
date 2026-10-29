package vm

import "strings"

// Isolated native tests retain Task.OwnerId but do not expose the default
// current user's ordinary Owner.Name parent. This is a query-value boundary,
// not a JSON-only omission (sobject_json2_route_context/org.tsv). Concrete
// Account.Owner and unmeasured TYPEOF, runAs and SeeAllData routes are unchanged.
func (vm *VM) applyTestTaskOwnerProjection(value *Value, paths []string, typedParents map[string]bool) {
	if vm.testContext == nil || vm.testContext.SeeAllData || vm.testContext.RunAsDepth != 0 || value == nil || !strings.EqualFold(value.Type, "Task") || typedParents["owner"] {
		return
	}
	selected := false
	for _, path := range paths {
		if strings.EqualFold(path, "Owner.Name") || strings.EqualFold(path, "Task.Owner.Name") {
			selected = true
			break
		}
	}
	if !selected {
		return
	}
	_, ownerID, present := objectFieldValue(*value, "OwnerId")
	id, valid := idValueText(ownerID)
	if !present || !valid || !apexIDTextEqual(id, vm.currentUserID()) {
		return
	}
	name, owner, present := objectFieldValue(*value, "Owner")
	if !present || owner.Kind != ValueObject || !strings.EqualFold(owner.Type, "User") {
		return
	}
	null := Null
	null.Type = "Name"
	null.Runtime = relationshipNullRuntime
	value.Fields[name] = null
}
