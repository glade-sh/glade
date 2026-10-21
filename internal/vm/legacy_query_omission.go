package vm

import "github.com/glade-sh/glade/internal/apexversion"

// Assignment history belongs to the query view, including same-value writes.
// DML alone does not set it, and updating a modified view does not clear it.
func markSObjectModifiedSinceQuery(value *Value) {
	if value == nil || value.Kind != ValueObject || dmlAccessibleSObject(*value) {
		return
	}
	if selected := value.Fields[sobjectQueriedFieldsField]; selected.Kind == ValueMap {
		value.Fields[sobjectModifiedSinceQueryField] = Bool(true)
	}
}

func (vm *VM) legacyModifiedQueryCanReadNull(row Value, field string) bool {
	if !row.Fields[sobjectModifiedSinceQueryField].Bool || row.Fields[sobjectQueriedFieldsField].Kind != ValueMap {
		return false
	}
	policy := vm.currentSourceExecutionPolicy()
	if !apexversion.Before(policy.APIVersion, 44) ||
		policy.Trigger || dmlAccessibleSObject(row) || isTriggerSObject(row) || isParentProjectionSObject(row) ||
		row.Fields[sobjectSubqueryProjectionField].Bool {
		return false
	}
	if vm.queriedSObjectFieldsIncludes(row, field) || isExplicitSObjectField(row, field) ||
		vm.loadedChildRelationshipForField(row, field) || vm.loadedParentRelationshipForField(row, field) {
		return false
	}
	// Child collections retain their existing projection rules.
	if _, child := vm.jsonSObjectChildRelationshipType(row.Type, field); child {
		return false
	}
	if _, _, declared := vm.sObjectFieldDefinition(row.Type, field); !declared {
		_, parent := vm.parentRelationshipForName(row, field)
		return parent
	}
	return true
}
