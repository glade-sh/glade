package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func flowStepsReadPriorDecision(steps []storage.FlowStep, targets map[string][]storage.FlowStep) bool {
	readsPrior := false
	valid := flowStepsSatisfy(steps, targets, func(step storage.FlowStep, _ bool) bool {
		for _, branch := range step.Branches {
			for _, criterion := range branch.Criteria {
				if strings.HasPrefix(strings.TrimSpace(criterion.Field), "$Record__Prior.") || strings.HasPrefix(strings.TrimSpace(criterion.Value), "{!$Record__Prior.") || strings.EqualFold(strings.TrimSpace(criterion.Operation), "ischanged") {
					readsPrior = true
				}
			}
		}
		return true
	})
	return valid && readsPrior
}

// Before-save execution has no engine prior map. Resolve the stored update
// target only for a decision reading a prior field or Is Changed. Formula and
// validation prior-value handling remain unchanged. After-save uses the seeded map.
func (e *Engine) seedFlowDecisionPrior(flowName string, record storage.Record, definition storage.ObjectDefinition, steps []storage.FlowStep, frame *flowFrame) {
	if e == nil || e.Org == nil || frame == nil || !frame.priorDecisions || record.ID == "" || !flowStepsReadPriorDecision(steps, frame.stepTargets) {
		return
	}
	key := flowFrameKey("$Record__Prior")
	if prior := frame.records[key]; prior.ID != "" {
		return
	}
	for _, rule := range definition.FlowRules {
		if rule.Name != flowName || !strings.EqualFold(strings.TrimSpace(rule.TriggerType), "RecordBeforeSave") {
			continue
		}
		if objectName, ok := storage.ResolveObjectName(*e.Org, record.Object); ok {
			if _, prior, found := storage.LookupRecordByID(e.Org.Objects[objectName].Records, record.ID); found {
				frame.records[key] = prior.Clone()
			}
		}
		return
	}
}

// Prior decisions read a direct prior field under equality, inequality or
// null checks. Missing prior records are null, including Boolean fields on insert.
func flowPriorDecisionValue(item storage.WorkflowCriteriaItem, definition storage.ObjectDefinition, namespace string, frame *flowFrame) (storage.Value, bool) {
	reference := strings.TrimSpace(item.Field)
	if frame == nil || !frame.priorDecisions || !strings.HasPrefix(reference, "$Record__Prior.") {
		return storage.Value{}, false
	}
	field := strings.TrimPrefix(reference, "$Record__Prior.")
	if strings.Contains(field, ".") {
		return storage.Value{}, false
	}
	name, ok := storage.ResolveFieldName(definition, namespace, field)
	if !ok {
		return storage.Value{}, false
	}
	switch strings.ToLower(strings.TrimSpace(item.Operation)) {
	case "equals", "notequal", "notequals":
	case "isnull":
		if name == "Id" && !strings.EqualFold(trimFormulaLiteral(item.Value), "true") {
			return storage.Value{}, false
		}
	default:
		return storage.Value{}, false
	}
	prior := frame.records[flowFrameKey("$Record__Prior")]
	if prior.ID == "" {
		return storage.NullValue(), true
	}
	value, found := sourceRecordFieldValue(prior, name)
	if !found {
		return storage.NullValue(), true
	}
	return value.Clone(), true
}

// flowRecordPriorReferenceCriterion compares a direct $Record field with the same
// field on $Record__Prior under EqualTo or NotEqualTo. Native rows
// (runtimegaps flow_record_prior_reference, API 62.0 and 67.0) show an update that
// leaves the field unchanged compares equal, including a partial sObject that
// omits it; an insert has no prior record, so the prior reads as null.
func (e *Engine) flowRecordPriorReferenceCriterion(item storage.WorkflowCriteriaItem, source storage.Record, definition storage.ObjectDefinition, frame *flowFrame) (bool, bool, bool) {
	value := strings.TrimSpace(item.Value)
	const prefix = "{!$Record__Prior."
	if frame == nil || !frame.priorDecisions || !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, "}") {
		return false, false, false
	}
	operation := strings.ToLower(strings.TrimSpace(item.Operation))
	if operation != "equals" && operation != "notequal" && operation != "notequals" {
		return false, false, false
	}
	priorField := strings.TrimSuffix(strings.TrimPrefix(value, prefix), "}")
	field := strings.TrimSpace(item.Field)
	if priorField == "" || strings.Contains(priorField, ".") || field == "" || strings.Contains(field, ".") || strings.HasPrefix(field, "$") {
		return false, false, false
	}
	leftName, ok := storage.ResolveFieldName(definition, e.Org.Namespace, field)
	if !ok {
		return false, false, false
	}
	rightName, ok := storage.ResolveFieldName(definition, e.Org.Namespace, priorField)
	if !ok {
		return false, false, false
	}
	left, found := sourceRecordFieldValue(source, leftName)
	if !found {
		left = storage.NullValue()
	}
	right := storage.NullValue()
	if prior := frame.records[flowFrameKey("$Record__Prior")]; prior.ID != "" {
		if priorValue, found := sourceRecordFieldValue(prior, rightName); found {
			right = priorValue
		}
	}
	equal := storage.ValuesEqual(left, right)
	if operation == "equals" {
		return equal, true, true
	}
	return !equal, true, true
}
