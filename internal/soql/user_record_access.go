package soql

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// UserRecordAccessProjectionError validates the restricted projection surface
// exposed by the record-access pseudo-parent, independently of ordinary lookups.
func UserRecordAccessProjectionError(parentObject, remainder string) error {
	message := ""
	if !storage.SupportsUserRecordAccessRelationship(parentObject) {
		message = MissingRelationshipMessage("UserRecordAccess", "field path")
	} else if _, ok := storage.UserRecordAccessRelationshipField(remainder); ok {
		return nil
	} else if strings.EqualFold(remainder, "Id") || strings.EqualFold(remainder, "RecordId") || strings.EqualFold(remainder, "UserId") {
		message = "Can select only a Has*Access field, and MaxAccessLevel"
	} else {
		message = MissingColumnMessage(remainder, "UserRecordAccess")
	}
	return queryError(message, message)
}

func userRecordAccessSegment(field string) int {
	parts := strings.Split(field, ".")
	for i, part := range parts[:len(parts)-1] {
		if strings.EqualFold(part, "UserRecordAccess") {
			return i
		}
	}
	return -1
}

// UserRecordAccessQueryError rejects clauses unavailable on the pseudo-parent.
// Direct queries against the UserRecordAccess object retain ordinary semantics.
func UserRecordAccessQueryError(query Query) error {
	if strings.EqualFold(query.Object, "UserRecordAccess") {
		return nil
	}
	failure := func(clause string) error {
		message := "You cannot " + clause + " UserRecordAccess when used in a relationship"
		return queryError(message, message)
	}
	var conditionUses func(*Condition) bool
	conditionUses = func(condition *Condition) bool {
		if condition == nil {
			return false
		}
		if userRecordAccessSegment(condition.Field) >= 0 {
			return true
		}
		for i := range condition.And {
			if conditionUses(&condition.And[i]) {
				return true
			}
		}
		for i := range condition.Or {
			if conditionUses(&condition.Or[i]) {
				return true
			}
		}
		return false
	}
	if conditionUses(query.Where) {
		return failure("filter on")
	}
	for _, field := range query.GroupBy {
		if userRecordAccessSegment(field) >= 0 {
			return failure("group by")
		}
	}
	for _, order := range query.Order {
		if userRecordAccessSegment(order.Field) >= 0 {
			return failure("order by")
		}
	}
	return nil
}

func validateUserRecordAccessProjection(org storage.OrgState, definition storage.ObjectDefinition, field string) (bool, error) {
	if userRecordAccessSegment(field) < 0 {
		return false, nil
	}
	if base, ok := stripQualifiedCurrentObjectField(org, definition, field); ok {
		field = base
	}
	segment, remainder, ok := strings.Cut(field, ".")
	if !ok {
		return false, nil
	}
	if strings.EqualFold(segment, "UserRecordAccess") {
		return true, UserRecordAccessProjectionError(definition.APIName, remainder)
	}
	for _, relation := range matchingParentRelations(org.Namespace, definition, segment) {
		for _, parent := range relation.ParentObjects {
			parentOrg, canonical, found := orgWithLazyStandardObject(org, parent)
			if !found {
				continue
			}
			if handled, err := validateUserRecordAccessProjection(parentOrg, parentOrg.Objects[canonical].Definition, remainder); handled {
				return true, err
			}
		}
	}
	return false, nil
}

func userRecordAccessProjectionValue(org storage.OrgState, record storage.Record, field string, context ExecutionContext) (string, storage.Value, bool) {
	index := userRecordAccessSegment(field)
	if index < 0 {
		return "", storage.Value{}, false
	}
	parts := strings.Split(field, ".")
	current := record
	for i, relationship := range parts[:index] {
		_, parent, ok := parentRelationshipRecord(org, current, relationship)
		if !ok {
			return strings.Join(parts[:i+1], "."), storage.NullValue(), true
		}
		current = parent
	}
	leaf, ok := storage.UserRecordAccessRelationshipField(strings.Join(parts[index+1:], "."))
	if !ok {
		return "", storage.Value{}, false
	}
	parts[index] = "UserRecordAccess"
	parts[index+1] = leaf.APIName
	key := strings.Join(parts, ".")
	if context.UserRecordAccess == nil {
		return key, storage.NullValue(), true
	}
	accessFn := context.UserRecordAccess
	if index > 0 && context.NestedUserRecordAccess != nil {
		accessFn = context.NestedUserRecordAccess
	}
	access := accessFn(current.Object, current)
	value, ok := access.GetField(leaf.APIName)
	if !ok {
		value = storage.NullValue()
	}
	return key, value.Clone(), true
}
