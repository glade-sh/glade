package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) enqueueFlowPaths(objectName string, id storage.ID, rules []storage.FlowRule) {
	if e.FlowPathEnqueuer == nil {
		return
	}
	for _, rule := range rules {
		for _, path := range rule.Paths {
			if strings.EqualFold(path.Kind, "async") {
				e.FlowPathEnqueuer(objectName, rule, path, id)
			}
		}
	}
}

// RunFlowPath executes a deferred graph against the current saved record.
func (e *Engine) RunFlowPath(objectName string, rule storage.FlowRule, path storage.FlowPath, id storage.ID) error {
	if e == nil || e.Org == nil || !strings.EqualFold(path.Kind, "async") {
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
