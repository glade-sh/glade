package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func flowCoreBeforeSaveChangedEntrySupported(rule storage.FlowRule) bool {
	if rule.CoreEntryFilterLogic {
		return true
	}
	if len(rule.Criteria) == 0 {
		return false
	}
	for _, item := range rule.Criteria {
		if !strings.EqualFold(strings.TrimSpace(item.Operation), "EqualTo") {
			return true
		}
	}
	return false
}

func (e *Engine) flowCoreChangedEntryMatches(rule storage.FlowRule, record storage.Record, definition storage.ObjectDefinition) (bool, bool) {
	rule.DoesRequireRecordChangedToMeetCriteria = false
	matches, ok := e.evaluateFlowRule(rule, record, definition)
	if !matches || !ok {
		return matches, ok
	}
	prior := e.flowPriorRecord(record.ID)
	if prior == nil && strings.EqualFold(rule.TriggerType, "RecordBeforeSave") && e != nil && e.Org != nil && record.ID != "" {
		if objectName, found := storage.ResolveObjectName(*e.Org, record.Object); found {
			if _, stored, exists := storage.LookupRecordByID(e.Org.Objects[objectName].Records, record.ID); exists {
				prior = &stored
			}
		}
	}
	if prior == nil {
		return true, true
	}
	priorMatches, supported := e.evaluateFlowRule(rule, *prior, definition)
	if !supported {
		return false, false
	}
	return !priorMatches, true
}
