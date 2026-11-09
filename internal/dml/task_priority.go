package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// PENDING-ROWS: derive only the stored standard Task flag from an authoritative
// TaskPriority row. Native caller and trigger-phase timing remains unproven.
// Priority labels and custom field names do not determine this flag.
func (e *Engine) applyTaskPriorityFlag(record *storage.Record) {
	if e == nil || e.Org == nil || record == nil || !strings.EqualFold(record.Object, "Task") {
		return
	}
	priority := record.Fields["Priority"]
	if priority.Kind != storage.ValueString {
		return
	}
	for _, configured := range e.Org.Objects["TaskPriority"].Records {
		name, flag := configured.Fields["ApiName"], configured.Fields["IsHighPriority"]
		if name.Kind == storage.ValueString && name.String == priority.String && flag.Kind == storage.ValueBoolean {
			record.Fields["IsHighPriority"] = storage.BooleanValue(flag.Boolean)
			return
		}
	}
}

// applyActivityAccountFromWhat derives a Task or Event AccountId. Native
// (activity_account_derivation and record_save_residuals at API 62.0 and
// 67.0, anonymous and isTest): a WhatId on an Account names that account; on
// an Opportunity, a Contract or a custom object with a master-detail field to
// Account it is that record's account, null when it has none, whatever the
// WhoId. Any other WhatId (Case, Campaign, a custom lookup child) or none
// falls back to a Contact WhoId's account, else null. Updates recompute it.
func (e *Engine) applyActivityAccountFromWhat(record *storage.Record) {
	if e == nil || e.Org == nil || record == nil || (!strings.EqualFold(record.Object, "Task") && !strings.EqualFold(record.Object, "Event")) {
		return
	}
	if _, has := e.Org.Objects[record.Object].Definition.Fields["AccountId"]; !has {
		return
	}
	account := e.activityAccount(*record)
	if account == "" {
		delete(record.Fields, "AccountId")
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["AccountId"] = storage.IDValue(account)
	delete(record.ExplicitNulls, "AccountId")
}

func (e *Engine) activityAccount(record storage.Record) storage.ID {
	if what := idFromStorageValue(record.Fields["WhatId"]); what != "" {
		if _, _, found := storage.LookupRecordByID(e.Org.Objects["Account"].Records, what); found {
			return what
		}
		for _, parent := range []string{"Opportunity", "Contract"} {
			if _, stored, found := storage.LookupRecordByID(e.Org.Objects[parent].Records, what); found {
				return idFromStorageValue(stored.Fields["AccountId"])
			}
		}
		if account, bearing := e.masterDetailAccount(what); bearing {
			return account
		}
	}
	if who := idFromStorageValue(record.Fields["WhoId"]); who != "" {
		if _, stored, found := storage.LookupRecordByID(e.Org.Objects["Contact"].Records, who); found {
			return idFromStorageValue(stored.Fields["AccountId"])
		}
	}
	return ""
}

// masterDetailAccount reports the master Account of a custom record with one
// master-detail field to Account.
func (e *Engine) masterDetailAccount(id storage.ID) (storage.ID, bool) {
	if len(id) < 3 {
		return "", false
	}
	for name, object := range e.Org.Objects {
		if !strings.HasSuffix(strings.ToLower(name), "__c") || object.Definition.KeyPrefix == "" || !strings.HasPrefix(string(id), object.Definition.KeyPrefix) {
			continue
		}
		field := activityMasterAccountField(object.Definition)
		if field == "" {
			return "", false
		}
		_, stored, found := storage.LookupRecordByID(object.Records, id)
		if !found {
			return "", false
		}
		return idFromStorageValue(stored.Fields[field]), true
	}
	return "", false
}

func activityMasterAccountField(definition storage.ObjectDefinition) string {
	found := ""
	for name, field := range definition.Fields {
		if !field.MasterDetail || len(field.ReferenceTo) != 1 || !strings.EqualFold(field.ReferenceTo[0], "Account") {
			continue
		}
		if found != "" {
			return ""
		}
		found = name
	}
	return found
}

// activityAccountSourceField names the field whose change moves the account
// of activities that reference a record of objectName.
func activityAccountSourceField(objectName string, definition storage.ObjectDefinition) string {
	switch {
	case strings.EqualFold(objectName, "Opportunity"), strings.EqualFold(objectName, "Contract"), strings.EqualFold(objectName, "Contact"):
		return "AccountId"
	case strings.HasSuffix(strings.ToLower(objectName), "__c"):
		return activityMasterAccountField(definition)
	}
	return ""
}

// activityRefreshScans counts stored-activity scans for tests.
var activityRefreshScans int

// activityRefreshBatch collects the parents whose account changed during one
// DML batch so the stored Tasks and Events are scanned once per batch instead
// of once per parent.
type activityRefreshBatch struct {
	parents map[storage.ID]struct{}
}

// beginActivityRefreshBatch defers activity AccountId refreshes until the
// returned function runs or until automation needs the stored activities.
// Nested batches (trigger DML) keep their own set.
func (e *Engine) beginActivityRefreshBatch() func() {
	if e == nil {
		return func() {}
	}
	previous := e.activityRefresh
	e.activityRefresh = &activityRefreshBatch{}
	return func() {
		e.flushActivityRefresh()
		e.activityRefresh = previous
	}
}

// flushActivityRefresh applies the deferred activity refreshes of the active
// batch in one scan.
func (e *Engine) flushActivityRefresh() {
	if e == nil || e.activityRefresh == nil || len(e.activityRefresh.parents) == 0 {
		return
	}
	parents := e.activityRefresh.parents
	e.activityRefresh.parents = nil
	e.refreshActivityAccountsFor(parents)
}

// refreshActivityAccounts recomputes AccountId on stored Tasks and Events
// that name id as WhatId or WhoId. Native (activity_account_derivation T14,
// U05, U07): an Opportunity, Contract or Contact account change moves them.
// Inside a batch the scan is deferred to flushActivityRefresh.
func (e *Engine) refreshActivityAccounts(objectName string, definition storage.ObjectDefinition, id storage.ID, before, after storage.Record) {
	field := activityAccountSourceField(objectName, definition)
	if field == "" || storage.IDsEqual(idFromStorageValue(before.Fields[field]), idFromStorageValue(after.Fields[field])) {
		return
	}
	key := activityParentKey(id)
	if e.activityRefresh != nil {
		if e.activityRefresh.parents == nil {
			e.activityRefresh.parents = make(map[storage.ID]struct{})
		}
		e.activityRefresh.parents[key] = struct{}{}
		return
	}
	e.refreshActivityAccountsFor(map[storage.ID]struct{}{key: {}})
}

// activityParentKey keys an ID the way storage.IDsEqual compares it: the
// 15-character prefix when present, else the whole text.
func activityParentKey(id storage.ID) storage.ID {
	if len(id) >= 15 {
		return id[:15]
	}
	return id
}

// refreshActivityAccountsFor scans the stored Tasks and Events once and
// recomputes AccountId on those whose WhatId or WhoId names one of parents
// (keyed by activityParentKey).
func (e *Engine) refreshActivityAccountsFor(parents map[storage.ID]struct{}) {
	if len(parents) == 0 || e.Org == nil {
		return
	}
	activityRefreshScans++
	names := func(record storage.Record) bool {
		if what := idFromStorageValue(record.Fields["WhatId"]); what != "" {
			if _, ok := parents[activityParentKey(what)]; ok {
				return true
			}
		}
		if who := idFromStorageValue(record.Fields["WhoId"]); who != "" {
			if _, ok := parents[activityParentKey(who)]; ok {
				return true
			}
		}
		return false
	}
	for _, activity := range []string{"Task", "Event"} {
		object, ok := e.Org.Objects[activity]
		if !ok {
			continue
		}
		if _, has := object.Definition.Fields["AccountId"]; !has {
			continue
		}
		var changed []storage.ID
		for recordID, record := range object.Records {
			if names(record) {
				changed = append(changed, recordID)
			}
		}
		if len(changed) == 0 {
			continue
		}
		mutable, _ := storage.EnsureMutableObjectRecords(e.Org, activity)
		for _, recordID := range changed {
			record := mutable.Records[recordID]
			if storage.IDsEqual(idFromStorageValue(record.Fields["AccountId"]), e.activityAccount(record)) {
				continue
			}
			// The stored record is replaced, not mutated, so the journal
			// clones it itself.
			if e.IsolationJournal != nil {
				e.IsolationJournal.RecordUpdate(activity, recordID, record)
			}
			next := record.Clone()
			e.applyActivityAccountFromWhat(&next)
			mutable.Records[recordID] = next
		}
		e.Org.Objects[activity] = *mutable
	}
}
