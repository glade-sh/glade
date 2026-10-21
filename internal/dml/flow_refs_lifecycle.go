package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Capture-backed Create/Update admission applies only to before/after save.
// Missing metadata and other trigger modes preserve the existing behavior.
func (e *Engine) flowRefsLifecycleMatches(rule storage.FlowRule, record storage.Record) bool {
	if !strings.EqualFold(rule.TriggerType, "RecordBeforeSave") && !strings.EqualFold(rule.TriggerType, "RecordAfterSave") {
		return true
	}
	mode := strings.ToLower(strings.TrimSpace(rule.RecordTriggerType))
	if mode != "create" && mode != "update" {
		return true
	}
	isUpdate := e.flowPriorRecord(record.ID) != nil
	if !isUpdate && strings.EqualFold(rule.TriggerType, "RecordBeforeSave") && e.Org != nil && record.ID != "" {
		// The VM's before-save engine has no prior map. The update target is
		// still stored at this point; an inserted record is not stored yet.
		if objectName, ok := storage.ResolveObjectName(*e.Org, record.Object); ok {
			_, _, isUpdate = storage.LookupRecordByID(e.Org.Objects[objectName].Records, record.ID)
		}
	}
	switch mode {
	case "create":
		return !isUpdate
	case "update":
		return isUpdate
	default:
		return true
	}
}
