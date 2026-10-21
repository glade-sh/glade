package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func flowCoreDeleteRule(rule storage.FlowRule) bool {
	return strings.EqualFold(strings.TrimSpace(rule.RecordTriggerType), "Delete") && strings.EqualFold(strings.TrimSpace(rule.TriggerType), "RecordBeforeDelete")
}

// Explicit delete-triggered flows can create the captured audit records. The
// before-save and legacy before-delete side-effect checks keep their rules.
func flowCoreTriggeredStepsSafe(rule storage.FlowRule, steps []storage.FlowStep, label string) bool {
	explicitDelete := flowCoreDeleteRule(rule) && strings.EqualFold(label, "before-delete")
	return flowStepsBeforeSaveSafeWithTargets(steps, strings.EqualFold(label, "before-delete"), explicitDelete, rule.StepTargets)
}
