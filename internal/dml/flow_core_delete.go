package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func flowCoreDeleteRule(rule storage.FlowRule) bool {
	return strings.EqualFold(strings.TrimSpace(rule.RecordTriggerType), "Delete") && strings.EqualFold(strings.TrimSpace(rule.TriggerType), "RecordBeforeDelete")
}

// Explicit delete-triggered flows can create audit records and update related
// records, including through a subflow. Before-save flows keep their restrictions.
func flowCoreTriggeredStepsSafe(rule storage.FlowRule, steps []storage.FlowStep, label string) bool {
	explicitDelete := flowCoreDeleteRule(rule) && strings.EqualFold(label, "before-delete")
	return flowStepsBeforeSaveSafeWithTargets(steps, strings.EqualFold(label, "before-delete"), explicitDelete, rule.StepTargets)
}
