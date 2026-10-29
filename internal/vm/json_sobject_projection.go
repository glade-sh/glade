package vm

import (
	"maps"
	"strings"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// JSON retains the populated query projection, independently of the readable
// field set and Apex's record display order. Text stores canonical field names;
// Fields holds implicit subquery join values without exposing them to getters.
const jsonSObjectQueryProjectionField = "__glade_json_query_projection"

func jsonProjectedSObjectValue(value Value) Value {
	projection := value.Fields[jsonSObjectQueryProjectionField]
	if projection.Kind != ValueString || len(projection.Fields) == 0 {
		return value
	}
	value.Fields = maps.Clone(value.Fields)
	for field, item := range projection.Fields {
		if _, present := value.Fields[field]; !present {
			value.Fields[field] = item
		}
	}
	return value
}

func jsonSerializedSObjectFieldNames(value Value) []string {
	fields := jsonSObjectFieldNames(value)
	projection := value.Fields[jsonSObjectQueryProjectionField]
	if projection.Kind != ValueString {
		return fields
	}
	available := make(map[string]string, len(fields))
	for _, field := range fields {
		if value.Fields[field].Kind != ValueNull || isExplicitSObjectField(value, field) {
			available[strings.ToLower(field)] = field
		}
	}
	ordered := make([]string, 0, len(available))
	appendField := func(name string) {
		key := strings.ToLower(name)
		if field, ok := available[key]; ok {
			ordered = append(ordered, field)
			delete(available, key)
		}
	}
	for _, field := range strings.Split(projection.Text, "\x00") {
		appendField(field)
	}
	// A queried null has no initial JSON slot. Its first explicit assignment,
	// including assignment of null, appends it (queryThenWrite native rows).
	for _, field := range explicitSObjectFieldNamesInInsertionOrder(value) {
		appendField(field)
	}
	for _, field := range fields {
		appendField(field)
	}
	return ordered
}

func (vm *VM) markJSONQueryProjection(value *Value, paths []string, children []soql.ChildQuery, typedParents map[string]bool) {
	var ordered []string
	seen := make(map[string]bool)
	appendField := func(name string) {
		actual, item, present := objectFieldValue(*value, name)
		if present && item.Kind != ValueNull && !seen[actual] {
			ordered = append(ordered, actual)
			seen[actual] = true
		}
	}
	var parents []string
	parentPaths := make(map[string][]string)
	for _, path := range paths {
		parts := splitSOQLRelationshipFieldPath(path)
		if len(parts) > 1 && vm.soqlFieldQualifierMatchesObject(value.Type, parts[0]) {
			parts = parts[1:]
		}
		if len(parts) == 0 {
			continue
		}
		if len(parts) == 1 {
			appendField(parts[0])
			continue
		}
		actual, parent, present := objectFieldValue(*value, parts[0])
		if !present || parent.Kind != ValueObject {
			continue
		}
		if lookup, ok := vm.parentRelationshipField(value.Type, actual); ok {
			appendField(lookup)
		}
		if _, present := parentPaths[actual]; !present {
			parents = append(parents, actual)
		}
		parentPaths[actual] = append(parentPaths[actual], strings.Join(parts[1:], "."))
	}
	appendField("Id")
	for _, name := range parents {
		parent := value.Fields[name]
		vm.markJSONQueryProjection(&parent, append([]string{"Id"}, parentPaths[name]...), nil, nil)
		if relation, ok := vm.parentRelationshipMetadata(value.Type, name); ok && relation.Polymorphic && !typedParents[strings.ToLower(name)] {
			projection := parent.Fields[jsonSObjectQueryProjectionField]
			projection.Runtime = "Name"
			parent.Fields[jsonSObjectQueryProjectionField] = projection
		}
		value.Fields[name] = parent
		appendField(name)
	}
	for _, child := range children {
		appendField(child.Relationship)
	}
	value.Fields[jsonSObjectQueryProjectionField] = String(strings.Join(ordered, "\x00"))
}

func (vm *VM) markJSONSubqueryProjection(value *Value, parentObject, relationship string, parentID storage.ID) {
	_, definition, known := vm.sObjectDefinitionForRead(value.Type)
	if !known || parentID == "" {
		return
	}
	for _, relation := range definition.Relations {
		name := relation.ChildRelationship
		if name == "" {
			name = derivedVMChildRelationshipName(definition)
		}
		if relation.Field == "" || !relationshipTargetsObject(relation, parentObject) || !vmRelationshipNameMatches(vm.Org.Namespace, name, relationship) {
			continue
		}
		projection := value.Fields[jsonSObjectQueryProjectionField]
		ordered := []string{relation.Field, "Id"}
		for _, field := range strings.Split(projection.Text, "\x00") {
			if field != "" && !strings.EqualFold(field, relation.Field) && !strings.EqualFold(field, "Id") {
				ordered = append(ordered, field)
			}
		}
		projection.Text = strings.Join(ordered, "\x00")
		projection.Fields = map[string]Value{relation.Field: platformScalar("Id", string(parentID))}
		value.Fields[jsonSObjectQueryProjectionField] = projection
		return
	}
}
