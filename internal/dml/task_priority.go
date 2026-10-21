package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// PENDING-ROWS: derive only the stored standard Task flag from an authoritative
// TaskPriority row. Native caller and trigger-phase timing remains unproven.
// Priority labels and custom field names do not determine this flag.
func (e *Engine) applyTaskPriorityFlag(record *storage.Record) {
	if e == nil || e.Org == nil || record == nil || !strings.EqualFold(record.Object, "Task") {
		return
	}
	priority := record.Fields["Priority"]
	if priority.Kind != storage.ValueString {
		return
	}
	for _, configured := range e.Org.Objects["TaskPriority"].Records {
		name, flag := configured.Fields["ApiName"], configured.Fields["IsHighPriority"]
		if name.Kind == storage.ValueString && name.String == priority.String && flag.Kind == storage.ValueBoolean {
			record.Fields["IsHighPriority"] = storage.BooleanValue(flag.Boolean)
			return
		}
	}
}
