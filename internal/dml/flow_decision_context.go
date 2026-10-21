package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func flowRuleHasSaveDecisions(rule storage.FlowRule) bool {
	trigger := strings.TrimSpace(rule.TriggerType)
	return strings.EqualFold(trigger, "RecordBeforeSave") || strings.EqualFold(trigger, "RecordAfterSave")
}

func flowSaveDecisionMode(flowName string, definition storage.ObjectDefinition) (save, prior, before bool) {
	for _, rule := range definition.FlowRules {
		if rule.Name != flowName || !flowRuleHasSaveDecisions(rule) {
			continue
		}
		mode := strings.TrimSpace(rule.RecordTriggerType)
		return true, strings.EqualFold(mode, "Update") || strings.EqualFold(mode, "CreateAndUpdate"),
			strings.EqualFold(strings.TrimSpace(rule.TriggerType), "RecordBeforeSave") && strings.EqualFold(mode, "CreateAndUpdate")
	}
	return false, false, false
}
