package sema

import "strings"

// FieldKey 099-103 capture statement upsert on a statically typed List<User>.
// Database.upsert and scalar, Set or generic SObject operands are separate
// shapes and do not acquire this compilation rule.
func fieldKeyTypedUserUpsert(operation, operandType string, model *semaTypeMemberView) string {
	if !strings.EqualFold(operation, "upsert") {
		return ""
	}
	base, arguments := semaGenericBaseAndArgs(operandType)
	if !strings.EqualFold(base, "List") || len(arguments) != 1 {
		return ""
	}
	element := strings.TrimSpace(arguments[0])
	members, _, resolved := semaLookupTypeMembers(model, element)
	if !resolved || !members.sobject {
		return ""
	}
	if qualified, ok := semaSchemaQualifiedTypeName(element); ok {
		element = qualified
	}
	if !strings.EqualFold(element, "User") || !semaDMLRecordType(element, model) {
		return ""
	}
	return "DML operation Upsert not allowed on List<User>"
}
