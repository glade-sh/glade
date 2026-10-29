package dml

import (
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) enqueueFlowPaths(objectName string, id storage.ID, rules []storage.FlowRule) {
	if e.FlowPathEnqueuer == nil {
		return
	}
	for _, rule := range rules {
		for _, path := range rule.Paths {
			// Both deferred kinds use the host's transaction and test-context
			// delivery boundary; a due schedule does not bypass that boundary.
			if strings.EqualFold(path.Kind, "async") || e.flowScheduledPathDue(objectName, id, path) {
				e.FlowPathEnqueuer(objectName, rule, path, id)
			}
		}
	}
}

// Only the zero-offset datetime field schedule is executable here. Other
// schedule metadata remains retained without projecting an unmodeled timer.
func flowRecordFieldSchedule(path storage.FlowPath) bool {
	return strings.EqualFold(path.Kind, "scheduled") &&
		strings.EqualFold(strings.TrimSpace(path.TimeSource), "RecordField") &&
		strings.EqualFold(strings.TrimSpace(path.OffsetUnit), "Minutes") &&
		path.OffsetNumber == 0 && strings.TrimSpace(path.RecordField) != ""
}

func (e *Engine) flowScheduledPathDue(objectName string, id storage.ID, path storage.FlowPath) bool {
	if e.Org == nil || !flowRecordFieldSchedule(path) {
		return false
	}
	record, found := e.Org.Objects[objectName].Records[id]
	if !found || record.System.IsDeleted {
		return false
	}
	value, found := record.GetField(strings.TrimSpace(path.RecordField))
	if !found || value.Kind != storage.ValueDateTime {
		return false
	}
	due, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return false
	}
	now := time.Now()
	if e.Now != nil {
		now = e.Now()
	}
	return !due.After(now)
}

// RunFlowPath executes a deferred graph against the current saved record.
func (e *Engine) RunFlowPath(objectName string, rule storage.FlowRule, path storage.FlowPath, id storage.ID) error {
	if e == nil || e.Org == nil || (!strings.EqualFold(path.Kind, "async") && !flowRecordFieldSchedule(path)) {
		return nil
	}
	objectName, ok := storage.ResolveObjectName(*e.Org, objectName)
	if !ok {
		return nil
	}
	object := e.Org.Objects[objectName]
	_, record, found := storage.LookupRecordByID(object.Records, id)
	if !found || record.System.IsDeleted {
		return nil
	}
	record = record.Clone()
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if record.ExplicitNulls == nil {
		record.ExplicitNulls = make(map[string]bool)
	}
	// Connector references belong to this entry graph, independently of the
	// rule's immediate graph. Keep the shared Flow evaluator and frame setup.
	rule.StepTargets = path.StepTargets
	definition := object.Definition
	definition.FlowRules = []storage.FlowRule{rule}
	changed, err := e.applyFlowEffects(rule.Name, objectName, &record, definition, path.Steps, nil, nil, nil, nil, rule.TextTemplates, rule.Variables)
	if err != nil {
		return err
	}
	if changed {
		if err := e.updateInvocableFlowRecord(record); err != nil {
			return err
		}
	}
	e.Org.IDSequences = maxSequences(e.Org.IDSequences, e.IDs.Sequences)
	return nil
}
