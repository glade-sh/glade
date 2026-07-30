package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) convertLeadOne(convert Value, result *Result) (Value, error) {
	if vm.Org == nil {
		return Null, fmt.Errorf("DML requires org state")
	}
	if convert.Kind != ValueObject || !strings.EqualFold(convert.Type, "Database.LeadConvert") {
		return Null, fmt.Errorf("Database.convertLead expects Database.LeadConvert")
	}
	if databaseLeadConvertBool(convert, "bypassAccountDedupeCheck") ||
		databaseLeadConvertBool(convert, "bypassContactDedupeCheck") ||
		databaseLeadConvertBool(convert, "overwriteLeadSource") ||
		databaseLeadConvertBool(convert, "sendNotificationEmail") {
		return Null, unsupportedCallError("Database.convertLead dedupe/notification local lead conversion surface")
	}
	if value, ok := databaseLeadConvertField(convert, "relatedPersonAccountId"); ok && value.Kind != ValueNull {
		return Null, unsupportedCallError("Database.convertLead person account local lead conversion surface")
	}
	if value, ok := databaseLeadConvertField(convert, "relatedPersonAccountRecord"); ok && value.Kind != ValueNull {
		return Null, unsupportedCallError("Database.convertLead person account local lead conversion surface")
	}
	leadIDValue, ok := databaseLeadConvertField(convert, "leadId")
	if !ok || !isApexIDLikeValue(leadIDValue) {
		return databaseLeadConvertFailure("", "LeadConvert.leadId is required"), nil
	}
	leadID := storage.ID(scalarText(leadIDValue))
	leadState, ok := vm.Org.Objects["Lead"]
	if !ok {
		return Null, fmt.Errorf("Database.convertLead requires Lead metadata")
	}
	storedLeadID, lead, ok := findRecordByLooseID(leadState, leadID)
	if !ok {
		return databaseLeadConvertFailure(string(leadID), "Lead not found"), nil
	}
	leadID = lead.ID
	accountID, err := vm.convertLeadAccountID(convert, lead, result)
	if err != nil {
		return Null, err
	}
	contactID, err := vm.convertLeadContactID(convert, lead, accountID, result)
	if err != nil {
		return Null, err
	}
	opportunityID, err := vm.convertLeadOpportunityID(convert, lead, accountID, result)
	if err != nil {
		return Null, err
	}
	storage.EnsureMutableObjectRecords(vm.Org, "Lead")
	updatedLeadState := vm.Org.Objects["Lead"]
	updatedLead := updatedLeadState.Records[storedLeadID]
	vm.recordIsolationJournalMutation("Lead", storedLeadID, updatedLead, true)
	if _, ok := leadState.Definition.Fields["IsConverted"]; ok {
		updatedLead.Fields["IsConverted"] = storage.BooleanValue(true)
	}
	if _, ok := leadState.Definition.Fields["ConvertedAccountId"]; ok {
		updatedLead.Fields["ConvertedAccountId"] = storage.IDValue(accountID)
	}
	if _, ok := leadState.Definition.Fields["ConvertedContactId"]; ok {
		updatedLead.Fields["ConvertedContactId"] = storage.IDValue(contactID)
	}
	if opportunityID != "" {
		if _, ok := leadState.Definition.Fields["ConvertedOpportunityId"]; ok {
			updatedLead.Fields["ConvertedOpportunityId"] = storage.IDValue(opportunityID)
		}
	}
	if _, ok := leadState.Definition.Fields["Status"]; ok {
		status := "Converted"
		if value, ok := databaseLeadConvertField(convert, "convertedStatus"); ok && value.Kind == ValueString && strings.TrimSpace(value.Text) != "" {
			status = value.Text
		}
		updatedLead.Fields["Status"] = storage.StringValue(status)
	}
	updatedLeadState.Records[storedLeadID] = updatedLead
	vm.Org.Objects["Lead"] = updatedLeadState
	row := Object("Database.LeadConvertResult")
	row.Fields["success"] = Bool(true)
	row.Fields["leadId"] = platformScalar("Id", string(leadID))
	row.Fields["accountId"] = platformScalar("Id", string(accountID))
	row.Fields["contactId"] = platformScalar("Id", string(contactID))
	if opportunityID == "" {
		row.Fields["opportunityId"] = Null
	} else {
		row.Fields["opportunityId"] = platformScalar("Id", string(opportunityID))
	}
	row.Fields["relatedPersonAccountId"] = Null
	row.Fields["errors"] = List()
	return row, nil
}

func (vm *VM) convertLeadAccountID(convert Value, lead storage.Record, result *Result) (storage.ID, error) {
	if value, ok := databaseLeadConvertField(convert, "accountId"); ok && isApexIDLikeValue(value) {
		return storage.ID(scalarText(value)), nil
	}
	account := Object("Account")
	if value, ok := databaseLeadConvertField(convert, "accountRecord"); ok && value.Kind == ValueObject && !strings.EqualFold(value.Type, "SObject") {
		account = value
		account.Type = "Account"
	}
	if _, _, ok := objectFieldValue(account, "Name"); !ok {
		name := storageRecordStringField(lead, "Company")
		if name == "" {
			name = storageRecordStringField(lead, "LastName")
		}
		vm.setExplicitSObjectFieldValue(&account, "Name", String(name))
	}
	results, err := vm.applyDML("insert", account, true, "", dml.Options{}, result)
	if err != nil {
		return "", err
	}
	if hasDMLFailures(results) {
		return "", databaseDMLException("convertLead", results)
	}
	return results[0].ID, nil
}

func (vm *VM) convertLeadContactID(convert Value, lead storage.Record, accountID storage.ID, result *Result) (storage.ID, error) {
	if value, ok := databaseLeadConvertField(convert, "contactId"); ok && isApexIDLikeValue(value) {
		return storage.ID(scalarText(value)), nil
	}
	contact := Object("Contact")
	if value, ok := databaseLeadConvertField(convert, "contactRecord"); ok && value.Kind == ValueObject && !strings.EqualFold(value.Type, "SObject") {
		contact = value
		contact.Type = "Contact"
	}
	if _, _, ok := objectFieldValue(contact, "FirstName"); !ok {
		if first := storageRecordStringField(lead, "FirstName"); first != "" {
			vm.setExplicitSObjectFieldValue(&contact, "FirstName", String(first))
		}
	}
	if _, _, ok := objectFieldValue(contact, "LastName"); !ok {
		vm.setExplicitSObjectFieldValue(&contact, "LastName", String(storageRecordStringField(lead, "LastName")))
	}
	if _, _, ok := objectFieldValue(contact, "AccountId"); !ok {
		vm.setExplicitSObjectFieldValue(&contact, "AccountId", platformScalar("Id", string(accountID)))
	}
	results, err := vm.applyDML("insert", contact, true, "", dml.Options{}, result)
	if err != nil {
		return "", err
	}
	if hasDMLFailures(results) {
		return "", databaseDMLException("convertLead", results)
	}
	return results[0].ID, nil
}

func (vm *VM) convertLeadOpportunityID(convert Value, lead storage.Record, accountID storage.ID, result *Result) (storage.ID, error) {
	if databaseLeadConvertBool(convert, "doNotCreateOpportunity") {
		return "", nil
	}
	if value, ok := databaseLeadConvertField(convert, "opportunityId"); ok && isApexIDLikeValue(value) {
		return storage.ID(scalarText(value)), nil
	}
	opportunity := Object("Opportunity")
	if value, ok := databaseLeadConvertField(convert, "opportunityRecord"); ok && value.Kind == ValueObject && !strings.EqualFold(value.Type, "SObject") {
		opportunity = value
		opportunity.Type = "Opportunity"
	}
	if _, _, ok := objectFieldValue(opportunity, "Name"); !ok {
		name := storageRecordStringField(lead, "Company")
		if value, ok := databaseLeadConvertField(convert, "opportunityName"); ok && value.Kind == ValueString && strings.TrimSpace(value.Text) != "" {
			name = value.Text
		}
		if name == "" {
			name = storageRecordStringField(lead, "LastName")
		}
		vm.setExplicitSObjectFieldValue(&opportunity, "Name", String(name))
	}
	if _, _, ok := objectFieldValue(opportunity, "AccountId"); !ok {
		vm.setExplicitSObjectFieldValue(&opportunity, "AccountId", platformScalar("Id", string(accountID)))
	}
	if _, _, ok := objectFieldValue(opportunity, "StageName"); !ok {
		vm.setExplicitSObjectFieldValue(&opportunity, "StageName", String("Prospecting"))
	}
	if _, _, ok := objectFieldValue(opportunity, "CloseDate"); !ok {
		vm.setExplicitSObjectFieldValue(&opportunity, "CloseDate", platformScalar("Date", vm.fakeNow.Format("2006-01-02")))
	}
	results, err := vm.applyDML("insert", opportunity, true, "", dml.Options{}, result)
	if err != nil {
		return "", err
	}
	if hasDMLFailures(results) {
		return "", databaseDMLException("convertLead", results)
	}
	return results[0].ID, nil
}

func databaseLeadConvertField(convert Value, name string) (Value, bool) {
	_, value, ok := objectFieldValue(convert, name)
	return value, ok
}

func databaseLeadConvertBool(convert Value, name string) bool {
	value, ok := databaseLeadConvertField(convert, name)
	return ok && value.Kind == ValueBool && value.Bool
}

func databaseLeadConvertFailure(leadID, message string) Value {
	row := Object("Database.LeadConvertResult")
	row.Fields["success"] = Bool(false)
	if leadID == "" {
		row.Fields["leadId"] = Null
	} else {
		row.Fields["leadId"] = platformScalar("Id", leadID)
	}
	row.Fields["accountId"] = Null
	row.Fields["contactId"] = Null
	row.Fields["opportunityId"] = Null
	row.Fields["relatedPersonAccountId"] = Null
	row.Fields["errors"] = List(databaseErrorValue(dml.Error{StatusCode: "INVALID_FIELD", Message: message}))
	return row
}

func databaseLeadConvertSuccess(value Value) bool {
	if value.Kind != ValueObject {
		return false
	}
	success, ok := value.Fields["success"]
	return ok && success.Kind == ValueBool && success.Bool
}

func findRecordByLooseID(object storage.ObjectState, id storage.ID) (storage.ID, storage.Record, bool) {
	if record, ok := object.Records[id]; ok {
		return id, record, true
	}
	wanted := strings.ToLower(string(id))
	for candidateID, record := range object.Records {
		candidate := strings.ToLower(string(candidateID))
		if candidate == wanted || strings.HasPrefix(wanted, candidate) || strings.HasPrefix(candidate, wanted) || apexIDTextEqual(candidate, wanted) {
			return candidateID, record, true
		}
	}
	return "", storage.Record{}, false
}
