package dml

import "github.com/glade-sh/glade/internal/storage"

// insertRecordFlowRecord saves a record that a record-triggered flow's Create
// Records element creates. The duplicate rules run as they do for an Apex
// insert and honor the saving statement's allowSave header. The native rows
// pin Block and Alert outcomes only, so the duplicate reports that an allowed
// match would queue are not recorded.
func (e *Engine) insertRecordFlowRecord(record storage.Record) (storage.ID, error) {
	previous := e.Options.EnforceDuplicateRules
	e.Options.EnforceDuplicateRules = true
	reports := len(e.duplicates.reports)
	defer func() {
		e.Options.EnforceDuplicateRules = previous
		if len(e.duplicates.reports) > reports {
			e.duplicates.reports = e.duplicates.reports[:reports]
		}
	}()
	statement := ""
	return e.insertOne(record, &statement)
}

// recordFlowDuplicateUpdate applies the duplicate rules to the triggering
// record that a record-triggered flow changed. The failure is an Update
// Records element fault, which the flow reports as its process failure.
func (e *Engine) recordFlowDuplicateUpdate(record, prior storage.Record) error {
	if e.recordFlowDuplicateRules == 0 {
		return nil
	}
	previous := e.Options.EnforceDuplicateRules
	e.Options.EnforceDuplicateRules = true
	defer func() { e.Options.EnforceDuplicateRules = previous }()
	err := e.enforceDuplicateRulesOnUpdate(record, prior)
	if err == nil {
		return nil
	}
	failure := dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update failed: %v", err)
	return e.flowUpdateError(failure, err, []storage.ID{record.ID})
}
