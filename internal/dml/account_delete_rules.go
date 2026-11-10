package dml

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rows: account_delete_children (fresh Developer scratch org, API 62.0
// and 67.0, anonymous and isTest).

// contractStatusCodes maps the captured default Contract statuses to their
// status category. Other statuses leave StatusCode null.
var contractStatusCodes = map[string]string{
	"draft":               "Draft",
	"in approval process": "InApproval",
	"activated":           "Activated",
}

// applyContractStatusCode derives the read-only Contract.StatusCode from
// Status on insert and update.
func (e *Engine) applyContractStatusCode(record *storage.Record) {
	if e == nil || record == nil || !strings.EqualFold(record.Object, "Contract") {
		return
	}
	status := record.Fields["Status"]
	code, ok := "", false
	if status.Kind == storage.ValueString {
		code, ok = contractStatusCodes[strings.ToLower(strings.TrimSpace(status.String))]
	}
	if !ok {
		delete(record.Fields, "StatusCode")
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["StatusCode"] = storage.StringValue(code)
}

// applyCaseContactAccountDefault gives a Case inserted with a Contact and no
// Account the Contact account, as established by captured observations.
func (e *Engine) applyCaseContactAccountDefault(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if !strings.EqualFold(objectName, "Case") {
		return
	}
	e.applyContactAccountDefault(definition, record)
}

func (e *Engine) applyContactAccountDefault(definition storage.ObjectDefinition, record *storage.Record) {
	if e == nil || e.Org == nil || record == nil {
		return
	}
	if _, ok := definition.Fields["AccountId"]; !ok {
		return
	}
	if _, ok := record.GetField("AccountId"); ok || record.HasExplicitNull("AccountId") {
		return
	}
	contactID := idFromStorageValue(record.Fields["ContactId"])
	if contactID == "" {
		return
	}
	contacts, ok := e.Org.Objects["Contact"]
	if !ok {
		return
	}
	_, contact, ok := storage.LookupRecordByID(contacts.Records, contactID)
	if !ok || contact.System.IsDeleted {
		return
	}
	accountID := idFromStorageValue(contact.Fields["AccountId"])
	if accountID == "" {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["AccountId"] = storage.IDValue(accountID)
}

// accountDeleteCascades reports whether a child on a restrict-and-cascade
// relation to Account goes with the Account: a Contract outside the Activated
// status category (Draft and In Approval Process rows), or a Draft Order.
func accountDeleteCascades(childObject, field string, child storage.Record) bool {
	if !strings.EqualFold(field, "AccountId") {
		return false
	}
	switch {
	case strings.EqualFold(childObject, "Contract"):
		return !contractActivated(child)
	case strings.EqualFold(childObject, "Order"):
		status := child.Fields["Status"]
		return status.Kind == storage.ValueString && strings.EqualFold(strings.TrimSpace(status.String), "Draft")
	}
	return false
}

func contractActivated(contract storage.Record) bool {
	if code := contract.Fields["StatusCode"]; code.Kind == storage.ValueString && code.String != "" {
		return strings.EqualFold(code.String, "Activated")
	}
	status := contract.Fields["Status"]
	return status.Kind == storage.ValueString && strings.EqualFold(strings.TrimSpace(status.String), "Activated")
}

func opportunityClosedWon(opportunity storage.Record) bool {
	if won := opportunity.Fields["IsWon"]; won.Kind == storage.ValueBoolean {
		return won.Boolean
	}
	stage := opportunity.Fields["StageName"]
	return stage.Kind == storage.ValueString && strings.EqualFold(strings.TrimSpace(stage.String), "Closed Won")
}

// validateAccountDeleteChildren applies the native Account blockers in a
// fixed order: activated Contracts, Cases, then Closed Won Opportunities.
// Each fails with DELETE_FAILED, no fields, and the native message listing
// the blocking records. It reports handled for every Account delete, so the
// generic restricted-relation check covers only other parents' relations.
func (e *Engine) validateAccountDeleteChildren(id storage.ID) error {
	account, ok := e.liveRecord("Account", id)
	if !ok {
		return nil
	}
	name := ""
	if value := account.Fields["Name"]; value.Kind == storage.ValueString {
		name = value.String
	}
	checks := []struct {
		object, label, item string
		blocks              func(storage.Record) bool
	}{
		{"Contract", "it is associated with the following active contracts.", "ContractNumber", contractActivated},
		{"Case", "it is associated with the following cases.", "CaseNumber", func(storage.Record) bool { return true }},
		{"Opportunity", "some opportunities in that account were closed won. The opportunities that could not be deleted are shown below.", "Name", opportunityClosedWon},
	}
	for _, check := range checks {
		var items []string
		for _, child := range e.liveChildren(check.object, "AccountId", id) {
			if !check.blocks(child) {
				continue
			}
			item := ""
			if value := child.Fields[check.item]; value.Kind == storage.ValueString {
				item = value.String
			}
			items = append(items, item)
		}
		if len(items) == 0 {
			continue
		}
		sort.Strings(items)
		return dmlErrorf("DELETE_FAILED", nil, "Your attempt to delete %s could not be completed because %s: %s\n", name, check.label, strings.Join(items, "\n"))
	}
	return nil
}

func (e *Engine) liveRecord(objectName string, id storage.ID) (storage.Record, bool) {
	object, ok := e.Org.Objects[objectName]
	if !ok {
		return storage.Record{}, false
	}
	_, record, ok := storage.LookupRecordByID(object.Records, id)
	return record, ok && !record.System.IsDeleted
}

// liveChildren lists the live records of childObject whose field references id.
func (e *Engine) liveChildren(childObject, field string, id storage.ID) []storage.Record {
	object, ok := e.Org.Objects[childObject]
	if !ok {
		return nil
	}
	var out []storage.Record
	for _, child := range object.Records {
		if child.System.IsDeleted {
			continue
		}
		if value, ok := child.Fields[field]; ok && storage.IDsEqual(idFromStorageValue(value), id) {
			out = append(out, child)
		}
	}
	return out
}
