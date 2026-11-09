package sema

import (
	"strings"
)

// semaChangeEventHeaderPathField rejects a property read through a change
// event's ChangeEventHeader field (ev.ChangeEventHeader.changeType). The
// header is not a foreign key, so it cannot start a path expression. Native
// inner_batchable_change_event_header org.tsv, API 62.0 and 67.0, anonymous,
// isTest and trigger routes: changeType, entityName and recordIds through the
// path are rejected with "A non foreign key field cannot be referenced in a
// path expression: ChangeEventHeader"; method calls through the path
// (getChangeType(), getEntityName(), getRecordIds(), getCommitUser(),
// getChangedFields()), the header itself and property reads on a header local
// compile.
func semaChangeEventHeaderPathField(name string, scope irSemaScope, model *semaTypeMemberView) (string, bool) {
	parts := strings.Split(strings.TrimSpace(name), ".")
	if len(parts) != 3 || !strings.EqualFold(parts[1], "ChangeEventHeader") || parts[2] == "" {
		return "", false
	}
	recordType, scoped := scope.lookup(parts[0])
	if !scoped {
		return "", false
	}
	members, _, known := semaLookupTypeMembers(model, recordType)
	if !known || !members.sobject {
		return "", false
	}
	field, ok := semaResolveField(model, recordType, parts[1], make(map[string]bool))
	if !ok || !strings.EqualFold(field.member.Type, "EventBus.ChangeEventHeader") {
		return "", false
	}
	return "A non foreign key field cannot be referenced in a path expression: ChangeEventHeader", true
}
