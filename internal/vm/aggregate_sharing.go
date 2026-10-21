package vm

import (
	"maps"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// Sharing restricts the contributing records before grouping, HAVING and LIMIT.
// AggregateResult has no owner on which the later row filter could operate.
// Keep the existing COUNT recount and live query-handle paths separate.
func (vm *VM) orgWithVisibleAggregateInputs(org *storage.OrgState, query soql.Query) *storage.OrgState {
	if org == nil || len(query.Aggregates) == 0 && len(query.GroupBy) == 0 || soqlSharingCountQuery(query) {
		return org
	}
	userID := vm.soqlSharingUserID(query)
	if userID == "" {
		return org
	}
	objectName, ok := storage.ResolveObjectName(*org, query.Object)
	if !ok {
		return org
	}
	state := org.Objects[objectName]
	visible := make(map[storage.ID]storage.Record, len(state.Records))
	for id, record := range state.Records {
		if vm.userModeRecordVisible(objectName, record, userID) {
			visible[id] = record
		}
	}
	if len(visible) == len(state.Records) {
		return org
	}
	filtered := *org
	filtered.Objects = maps.Clone(org.Objects)
	state.Records = visible
	filtered.Objects[objectName] = state
	return &filtered
}
