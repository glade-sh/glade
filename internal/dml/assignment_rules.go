package dml

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// AssignmentRuleOwner resolves the header against the final stored fields,
// after Apex after-triggers have run. Unknown and other-object IDs are ignored.
func (e *Engine) AssignmentRuleOwner(input storage.Record) (storage.ID, error) {
	selection := e.Options.AssignmentRule
	if selection == nil {
		selection = input.AssignmentRule
	}
	if selection == nil || (!selection.UseDefault && selection.ID == "") {
		return "", nil
	}
	if !strings.EqualFold(input.Object, "Lead") && !strings.EqualFold(input.Object, "Case") {
		return "", nil
	}
	state := e.Org.Objects[input.Object]
	_, record, ok := storage.LookupRecordByID(state.Records, input.ID)
	if !ok {
		return "", nil
	}
	for _, rule := range state.Definition.AssignmentRules {
		if selection.ID != "" {
			if !storage.IDsEqual(rule.ID, selection.ID) {
				continue
			}
		} else if !rule.Active {
			continue
		}
		for _, entry := range rule.Entries {
			matches, supported := e.assignmentEntryMatches(entry, record, state.Definition)
			if !supported {
				return "", fmt.Errorf("dml: unsupported assignment rule criteria in %s", rule.Name)
			}
			if !matches {
				continue
			}
			if !strings.EqualFold(entry.AssignedToType, "Queue") {
				return "", fmt.Errorf("dml: assignment rule target %s is unsupported", entry.AssignedToType)
			}
			var owner storage.ID
			for id, group := range e.Org.Objects["Group"].Records {
				if !group.System.IsDeleted && strings.EqualFold(group.Fields["Type"].String, "Queue") && strings.EqualFold(group.Fields["DeveloperName"].String, entry.AssignedTo) && (owner == "" || id < owner) {
					owner = id
				}
			}
			if owner == "" {
				return "", fmt.Errorf("dml: assignment rule queue %s is unavailable", entry.AssignedTo)
			}
			return owner, nil
		}
		return "", nil
	}
	return "", nil
}

func (e *Engine) assignmentEntryMatches(entry storage.AssignmentRuleEntry, record storage.Record, definition storage.ObjectDefinition) (bool, bool) {
	if entry.Formula != "" {
		return evaluateWorkflowRule(storage.WorkflowRule{Formula: entry.Formula}, record, definition, e.Org)
	}
	if len(entry.Criteria) == 0 {
		return true, true
	}
	matches := make([]bool, len(entry.Criteria))
	for i, item := range entry.Criteria {
		matched, ok := evaluateWorkflowCriteria(item, record, definition, e.Org.Namespace)
		if !ok {
			return false, false
		}
		matches[i] = matched
	}
	return evaluateLookupFilterLogic(entry.BooleanFilter, matches)
}

// ApplyAssignmentRules is the engine route before workflow and after-save Flow.
// The VM uses AssignmentRuleOwner to include the native owner-update triggers.
func (e *Engine) ApplyAssignmentRules(input storage.Record) error {
	owner, err := e.AssignmentRuleOwner(input)
	if err != nil || owner == "" {
		return err
	}
	state := e.Org.Objects[input.Object]
	storedID, record, ok := storage.LookupRecordByID(state.Records, input.ID)
	if !ok || storage.IDsEqual(owner, record.System.OwnerID) {
		return nil
	}
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, input.Object); cloned {
		state = e.Org.Objects[input.Object]
	}
	before := record.Clone()
	record = record.Clone()
	record.AssignmentRule = nil
	record.System.OwnerID = owner
	if _, exists := record.Fields["OwnerId"]; exists {
		record.Fields["OwnerId"] = storage.IDValue(owner)
	}
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordUpdate(input.Object, storedID, before)
	}
	state.Records[storedID] = record
	e.Org.Objects[input.Object] = state
	return e.reconcilePrivateOwnerShares(input.Object, storedID)
}
