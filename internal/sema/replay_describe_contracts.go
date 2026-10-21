package sema

import "strings"

// The captured rows: event constructors reject the four
// captured system fields and unknown fields. Ordinary SObjects and source
// classes retain their own initializer rules.
func semaReplayDescribeConstructorField(receiver, field string, model *semaTypeMemberView) string {
	members, _, ok := semaLookupTypeMembers(model, receiver)
	if !ok || !members.sobject || (!members.platformEvent && !strings.EqualFold(members.name, "BatchApexErrorEvent")) {
		return ""
	}
	switch strings.ToLower(field) {
	case "replayid", "eventuuid", "createddate", "createdbyid":
		return "Field is not writeable: " + receiver + "." + field
	}
	if _, exists := semaResolveFieldPath(model, receiver, field); !exists && !members.partialSObject {
		return "unknown field " + field + " on " + receiver
	}
	return ""
}
