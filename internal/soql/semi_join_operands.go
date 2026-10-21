package soql

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// SemiJoinOperandField removes a current-object qualifier, not a relationship
// traversal. Root aliases are already resolved by the parser. Captured standard
// and custom reference operands use this same one-level form.
func SemiJoinOperandField(objectName, field string) string {
	prefix, leaf, dotted := strings.Cut(field, ".")
	if dotted && !strings.Contains(leaf, ".") && strings.EqualFold(prefix, objectName) {
		return leaf
	}
	return field
}

// SemiJoinOperandRejection shares the captured outer-operand contract with
// inline semantics. Unknown columns and deeper paths keep ordinary validation.
// Relationship and reference facts must come from schema, never field names.
func SemiJoinOperandRejection(objectName, operand string, knownField, reference, relationship bool) *QueryError {
	field := SemiJoinOperandField(objectName, operand)
	if prefix, leaf, dotted := strings.Cut(field, "."); dotted {
		if prefix == "" || leaf == "" || strings.Contains(leaf, ".") {
			return nil
		}
		message := MissingRelationshipMessage(prefix, "field path")
		if relationship {
			message = "The left operand '" + operand + "' cannot have more than one level of relationships"
		}
		return &QueryError{Message: message, Err: fmt.Errorf("soql: %s", message)}
	}
	if knownField && !reference && !strings.EqualFold(field, "Id") {
		message := "The left operand field in the where expression for outer query should be an id field, cannot use: '" + operand + "'"
		return &QueryError{Message: message, Err: fmt.Errorf("soql: %s", message)}
	}
	return nil
}

func semiJoinRuntimeOperandRejection(org storage.OrgState, objectName string, definition storage.ObjectDefinition, operand string) *QueryError {
	field := SemiJoinOperandField(objectName, operand)
	_, metadata, known := storage.ResolveFieldDefinition(definition, org.Namespace, field)
	if !known {
		metadata, known = fieldDefinitionsForReferenceField(definition, field)
	}
	prefix, _, dotted := strings.Cut(field, ".")
	relationship := dotted && len(matchingParentRelations(org.Namespace, definition, prefix)) != 0
	return SemiJoinOperandRejection(objectName, operand, known, metadata.Type == storage.FieldReference && len(metadata.ReferenceTo) > 0, relationship)
}

// SemiJoinMissingRelationshipLeafRejection identifies the captured positive
// join whose one-hop parent path has an unknown leaf. The VM decides source-API
// precedence over ordinary column validation. Existing fields, unknown parents,
// anti-joins and deeper/polymorphic paths keep ordinary validation.
func SemiJoinMissingRelationshipLeafRejection(org storage.OrgState, query Query) *QueryError {
	objectName, ok := storage.ResolveObjectName(org, query.Object)
	if !ok {
		return nil
	}
	definition := org.Objects[objectName].Definition
	var walk func(*Condition) *QueryError
	walk = func(condition *Condition) *QueryError {
		if condition == nil {
			return nil
		}
		if condition.Subquery != nil && strings.EqualFold(condition.Op, "IN") && len(condition.Subquery.Fields) == 1 && strings.EqualFold(condition.Subquery.Fields[0], "Id") {
			field := SemiJoinOperandField(objectName, condition.Field)
			prefix, leaf, dotted := strings.Cut(field, ".")
			if dotted && !strings.Contains(leaf, ".") {
				relations := matchingParentRelations(org.Namespace, definition, prefix)
				if len(relations) == 1 && !relations[0].Polymorphic && len(relations[0].ParentObjects) == 1 {
					parentName, knownParent := storage.ResolveObjectName(org, relations[0].ParentObjects[0])
					if knownParent && !fieldKnown(org, org.Objects[parentName].Definition, leaf) {
						return semiJoinRuntimeOperandRejection(org, objectName, definition, condition.Field)
					}
				}
			}
		}
		for i := range condition.And {
			if rejection := walk(&condition.And[i]); rejection != nil {
				return rejection
			}
		}
		for i := range condition.Or {
			if rejection := walk(&condition.Or[i]); rejection != nil {
				return rejection
			}
		}
		return nil
	}
	return walk(query.Where)
}
