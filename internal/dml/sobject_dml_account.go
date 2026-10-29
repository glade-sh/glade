package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func normalizePersonAccountFields(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if !strings.EqualFold(objectName, "Account") || record == nil {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if !hasPersonAccountSignal(*record) {
		return
	}
	if _, enabled := storage.ResolveFieldName(definition, "", "IsPersonAccount"); !enabled {
		return
	}
	record.Fields["IsPersonAccount"] = storage.BooleanValue(true)
	normalizeFirstLastName(definition, record)
}

func hasPersonAccountSignal(record storage.Record) bool {
	return hasPersonAccountFieldSignal(record)
}

func hasPersonAccountFieldSignal(record storage.Record) bool {
	for field, value := range record.Fields {
		lower := strings.ToLower(field)
		if (strings.HasPrefix(lower, "person") || lower == "firstname" || lower == "lastname") && nonDefaultPersonValue(value) {
			return true
		}
	}
	return false
}

func isPersonAccountFeatureField(field string) bool {
	lower := strings.ToLower(field)
	if strings.HasSuffix(lower, "__c") {
		return false
	}
	if strings.HasPrefix(lower, "person") {
		return true
	}
	switch lower {
	case "firstname", "lastname", "middlename", "suffix", "salutation", "ispersonaccount":
		return true
	default:
		return false
	}
}

func personAccountFeatureFieldError(definition storage.ObjectDefinition, objectName, field string) error {
	if !strings.EqualFold(objectName, "Account") || !isPersonAccountFeatureField(field) {
		return nil
	}
	if _, enabled := storage.ResolveFieldName(definition, "", "IsPersonAccount"); enabled {
		return nil
	}
	return dmlErrorf(
		"INVALID_FIELD_FOR_INSERT_UPDATE",
		[]string{field},
		"dml: unknown field %s.%s. Person Account fields require the PersonAccounts org feature; set org.features: [PersonAccounts] in glade.yml (nested under org), or add \"PersonAccounts\" to the features array in config/project-scratch-def.json.",
		objectName,
		field,
	)
}

func nonDefaultPersonValue(value storage.Value) bool {
	switch value.Kind {
	case storage.ValueNull, "":
		return false
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal:
		text := strings.TrimSpace(value.String)
		return text != "" && !strings.EqualFold(text, "false")
	case storage.ValueID:
		return value.ID != ""
	case storage.ValueInteger:
		return value.Integer != 0
	default:
		return true
	}
}

func isPersonAccountRecord(record storage.Record) bool {
	if value, ok := record.Fields["IsPersonAccount"]; ok && value.Kind == storage.ValueBoolean {
		if !value.Boolean {
			return false
		}
		return hasPersonAccountFieldSignal(record) || idFromStorageValue(record.Fields["PersonContactId"]) != ""
	}
	return false
}

func createsPersonContactOnInsert(objectName string, record storage.Record) bool {
	return objectName == "Account" && isPersonAccountRecord(record)
}

func forcePersonAccountRecordType(objectName string, record storage.Record) bool {
	return strings.EqualFold(objectName, "Account") && (isPersonAccountRecord(record) || hasPersonAccountFieldSignal(record))
}

func validatePersonAccountRequiredFields(objectName string, record storage.Record) error {
	if !strings.EqualFold(objectName, "Account") || !isPersonAccountRecord(record) {
		return nil
	}
	if strings.TrimSpace(stringField(record.Fields, "LastName")) != "" {
		return nil
	}
	return dmlErrorf("REQUIRED_FIELD_MISSING", []string{"LastName"}, "Required fields are missing: [LastName]")
}

func validateAccountNameRequiredFields(objectName string, definition storage.ObjectDefinition, record storage.Record) error {
	if !strings.EqualFold(objectName, "Account") || isPersonAccountRecord(record) {
		return nil
	}
	if _, ok := personAccountRecordType(definition.RecordTypes); !ok {
		return nil
	}
	if strings.TrimSpace(stringField(record.Fields, "Name")) != "" {
		return nil
	}
	return dmlErrorf("REQUIRED_FIELD_MISSING", []string{"Name"}, "Required fields are missing: [Name]")
}

func defaultRecordTypeForRecord(objectName string, recordTypes []storage.RecordTypeInfo, record storage.Record) (storage.RecordTypeInfo, bool) {
	if strings.EqualFold(objectName, "Account") {
		if isPersonAccountRecord(record) || hasPersonAccountFieldSignal(record) {
			if recordType, ok := personAccountRecordType(recordTypes); ok {
				return recordType, true
			}
		} else if recordType, ok := businessAccountRecordType(recordTypes); ok {
			return recordType, true
		}
	}
	return defaultRecordType(recordTypes)
}

// AccountDefaultRecordType selects the running user's profile default for the
// account's kind, falling back to the org's record types when none is declared.
// Both the Apex pre-insert defaults and storage DML use this decision.
func AccountDefaultRecordType(org *storage.OrgState, userID storage.ID, definition storage.ObjectDefinition, record storage.Record) (storage.RecordTypeInfo, bool) {
	if org != nil && len(org.Metadata.AccountRecordTypeDefaults) != 0 {
		if userID == "" {
			userID = storage.DefaultUserID
		}
		_, user, found := storage.LookupRecordByID(org.Objects["User"].Records, userID)
		if found {
			profileID := idFromStorageValue(user.Fields["ProfileId"])
			person := forcePersonAccountRecordType("Account", record)
			for _, mapping := range org.Metadata.AccountRecordTypeDefaults {
				if !storage.IDsEqual(mapping.ProfileID, profileID) || mapping.PersonAccount != person {
					continue
				}
				for _, recordType := range definition.RecordTypes {
					if strings.EqualFold(recordType.DeveloperName, mapping.DeveloperName) && recordType.Active && recordType.Available {
						return recordType, true
					}
				}
			}
		}
	}
	return defaultRecordTypeForRecord("Account", definition.RecordTypes, record)
}

func personAccountRecordType(recordTypes []storage.RecordTypeInfo) (storage.RecordTypeInfo, bool) {
	if recordType, ok := preferredPersonAccountRecordType(recordTypes, false, func(rt storage.RecordTypeInfo) bool {
		return rt.Active && rt.Available && rt.Default
	}); ok {
		return recordType, true
	}
	if recordType, ok := preferredPersonAccountRecordType(recordTypes, false, func(rt storage.RecordTypeInfo) bool {
		return rt.Active && rt.Available
	}); ok {
		return recordType, true
	}
	if recordType, ok := preferredPersonAccountRecordType(recordTypes, true, func(rt storage.RecordTypeInfo) bool {
		return rt.Active && rt.Available && rt.Default
	}); ok {
		return recordType, true
	}
	if recordType, ok := preferredPersonAccountRecordType(recordTypes, false, func(rt storage.RecordTypeInfo) bool {
		return rt.Active
	}); ok {
		return recordType, true
	}
	if recordType, ok := preferredPersonAccountRecordType(recordTypes, false, func(storage.RecordTypeInfo) bool { return true }); ok {
		return recordType, true
	}
	return preferredPersonAccountRecordType(recordTypes, true, func(storage.RecordTypeInfo) bool { return true })
}

func preferredPersonAccountRecordType(recordTypes []storage.RecordTypeInfo, allowGeneric bool, accept func(storage.RecordTypeInfo) bool) (storage.RecordTypeInfo, bool) {
	for _, recordType := range recordTypes {
		if !accept(recordType) || !recordTypeLooksPersonAccount(recordType) {
			continue
		}
		if !allowGeneric && recordTypeLooksGenericPersonAccount(recordType) {
			continue
		}
		return recordType, true
	}
	return storage.RecordTypeInfo{}, false
}

func recordTypeLooksPersonAccount(recordType storage.RecordTypeInfo) bool {
	if recordType.PersonAccount {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(recordType.Name + " " + recordType.DeveloperName))
	return strings.Contains(name, "person") || strings.Contains(name, "individual")
}

// explicitPersonAccountRecordType reports whether id names a record type that
// belongs to person accounts, whatever its developer name.
func explicitPersonAccountRecordType(recordTypes []storage.RecordTypeInfo, id storage.ID) bool {
	for _, recordType := range recordTypes {
		if recordType.ID != "" && storage.IDsEqual(recordType.ID, id) {
			return recordType.PersonAccount || recordTypeLooksPersonAccount(recordType)
		}
	}
	return false
}

func recordTypeLooksGenericPersonAccount(recordType storage.RecordTypeInfo) bool {
	for _, value := range []string{recordType.DeveloperName, recordType.Name} {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "personaccount", "person_account", "person account":
			return true
		}
	}
	return false
}

func businessAccountRecordType(recordTypes []storage.RecordTypeInfo) (storage.RecordTypeInfo, bool) {
	for _, recordType := range recordTypes {
		if recordType.Default && recordType.Active && recordType.Available && !recordType.PersonAccount && !recordTypeLooksPersonAccount(recordType) {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if recordType.Active && recordType.Available && !recordType.PersonAccount && !recordTypeLooksPersonAccount(recordType) {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if recordType.Active && !recordType.PersonAccount && !recordTypeLooksPersonAccount(recordType) {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if !recordType.PersonAccount && !recordTypeLooksPersonAccount(recordType) {
			return recordType, true
		}
	}
	return storage.RecordTypeInfo{}, false
}

func (e *Engine) afterInsertPersonAccount(account storage.Record) error {
	if _, ok := e.Org.Objects["Contact"]; !ok {
		return nil
	}
	contact := storage.Record{
		Object: "Contact",
		Fields: e.personContactFields(account),
	}
	contactID, err := e.insertOne(contact, nil)
	if err != nil {
		return err
	}
	storage.EnsureMutableObjectRecords(e.Org, "Account")
	accountObject := e.Org.Objects["Account"]
	stored := accountObject.Records[account.ID]
	if stored.Fields == nil {
		stored.Fields = make(map[string]storage.Value)
	}
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordUpdate("Account", account.ID, stored)
	}
	stored.Fields["PersonContactId"] = storage.IDValue(contactID)
	accountObject.Records[account.ID] = stored
	e.Org.Objects["Account"] = accountObject
	return nil
}

func (e *Engine) personContactFields(account storage.Record) map[string]storage.Value {
	fields := map[string]storage.Value{
		"AccountId": storage.IDValue(account.ID),
	}
	for _, mapping := range e.personAccountFieldMappings() {
		copyPersonContactField(fields, account, mapping.account, mapping.contact)
	}
	if _, ok := fields["LastName"]; !ok || fields["LastName"].Kind == storage.ValueNull {
		fields["LastName"] = storage.StringValue(stringField(account.Fields, "Name"))
	}
	for field, value := range fields {
		if value.Kind == "" {
			delete(fields, field)
		}
	}
	return fields
}

type personContactFieldMapping struct {
	account string
	contact string
}

func personContactFieldMappings() []personContactFieldMapping {
	standard := storage.PersonAccountStandardFieldMappings()
	mappings := make([]personContactFieldMapping, 0, len(standard))
	for _, mapping := range standard {
		mappings = append(mappings, personContactFieldMapping{mapping.Account, mapping.Contact})
	}
	return mappings
}

func copyPersonContactField(fields map[string]storage.Value, account storage.Record, source, target string) {
	if value, ok := account.GetField(source); ok {
		fields[target] = value.Clone()
	}
}

func (e *Engine) syncPersonContact(account storage.Record) error {
	if strings.EqualFold(account.Object, "Contact") {
		e.syncPersonAccountCustomFields(account)
		return nil
	}
	if !isPersonAccountRecord(account) {
		return nil
	}
	contactID := idFromStorageValue(account.Fields["PersonContactId"])
	if contactID == "" {
		return nil
	}
	contactObject, ok := e.Org.Objects["Contact"]
	if !ok {
		return nil
	}
	contact, ok := contactObject.Records[contactID]
	if !ok || contact.System.IsDeleted {
		return nil
	}
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, "Contact"); cloned {
		contactObject = e.Org.Objects["Contact"]
		contact = contactObject.Records[contactID]
	}
	if contact.Fields == nil {
		contact.Fields = make(map[string]storage.Value)
	}
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordUpdate("Contact", contactID, contact)
	}
	for field, value := range e.personContactFields(account) {
		contact.Fields[field] = value.Clone()
		delete(contact.ExplicitNulls, field)
	}
	clearExplicitPersonContactFields(&contact, account, e.personAccountFieldMappings())
	contactObject.Records[contactID] = contact
	e.Org.Objects["Contact"] = contactObject
	return nil
}

func clearExplicitPersonContactFields(contact *storage.Record, account storage.Record, mappings []personContactFieldMapping) {
	if contact == nil {
		return
	}
	for _, mapping := range mappings {
		if !account.HasExplicitNull(mapping.account) {
			continue
		}
		deleteCaseInsensitiveField(contact.Fields, mapping.contact)
		delete(contact.Fields, mapping.contact)
		if contact.ExplicitNulls == nil {
			contact.ExplicitNulls = make(map[string]bool)
		}
		contact.ExplicitNulls[mapping.contact] = true
	}
}

// Only fields declared on both objects participate in custom-field mirroring.
// The Account schema has already been derived from the Contact metadata.
func (e *Engine) personAccountCustomFieldMappings() []personContactFieldMapping {
	account := e.Org.Objects["Account"].Definition
	contact := e.Org.Objects["Contact"].Definition
	var mappings []personContactFieldMapping
	for name := range account.Fields {
		if !strings.HasSuffix(strings.ToLower(name), "__pc") {
			continue
		}
		source, ok := storage.ResolveFieldName(contact, e.Org.Namespace, name[:len(name)-4]+"__c")
		if ok {
			mappings = append(mappings, personContactFieldMapping{name, source})
		}
	}
	return mappings
}

func (e *Engine) personAccountFieldMappings() []personContactFieldMapping {
	return append(personContactFieldMappings(), e.personAccountCustomFieldMappings()...)
}

func (e *Engine) validatePersonAccountCustomFieldWrites(record storage.Record, existing ...storage.Record) error {
	if !strings.EqualFold(record.Object, "Account") || isPersonAccountRecord(record) {
		return nil
	}
	if len(existing) > 0 && isPersonAccountRecord(existing[0]) {
		return nil
	}
	for _, mapping := range e.personAccountCustomFieldMappings() {
		// Queried or deserialized formula values are ignored during updates.
		// Other person fields still reject explicit Business Account writes.
		if len(existing) > 0 && strings.TrimSpace(e.Org.Objects["Account"].Definition.Fields[mapping.account].Formula) != "" {
			continue
		}
		_, present := record.GetField(mapping.account)
		if present || record.HasExplicitNull(mapping.account) {
			return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{mapping.account}, "Cannot specify Person Account field %s on a Business Account", mapping.account)
		}
	}
	return nil
}

func (e *Engine) syncPersonAccountCustomFields(contact storage.Record) {
	accountID := idFromStorageValue(contact.Fields["AccountId"])
	object := e.Org.Objects["Account"]
	storedID, account, found := storage.LookupRecordByID(object.Records, accountID)
	if !found || account.System.IsDeleted || !isPersonAccountRecord(account) || !storage.IDsEqual(idFromStorageValue(account.Fields["PersonContactId"]), contact.ID) {
		return
	}
	mappings := e.personAccountCustomFieldMappings()
	if len(mappings) == 0 {
		return
	}
	storage.EnsureMutableObjectRecords(e.Org, "Account")
	object = e.Org.Objects["Account"]
	account = object.Records[storedID]
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordUpdate("Account", storedID, account)
	}
	for _, mapping := range mappings {
		if contact.HasExplicitNull(mapping.contact) {
			deleteCaseInsensitiveField(account.Fields, mapping.account)
			if account.ExplicitNulls == nil {
				account.ExplicitNulls = make(map[string]bool)
			}
			account.ExplicitNulls[mapping.account] = true
		} else if value, present := contact.GetField(mapping.contact); present {
			account.Fields[mapping.account] = value.Clone()
			delete(account.ExplicitNulls, mapping.account)
		}
	}
	object.Records[storedID] = account
	e.Org.Objects["Account"] = object
}

// PersonAccountCustomFieldDefaultAllowed keeps Contact defaults on Person
// Accounts. Callers use the declared mirror and Contact source metadata; a
// Business Account must not acquire a person-only field as an implicit input.
func PersonAccountCustomFieldDefaultAllowed(org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record, field storage.Field) bool {
	name := field.APIName
	if org == nil || !strings.EqualFold(definition.APIName, "Account") || !strings.HasSuffix(strings.ToLower(name), "__pc") {
		return true
	}
	if _, declared := storage.ResolveFieldName(org.Objects["Contact"].Definition, org.Namespace, name[:len(name)-4]+"__c"); !declared {
		return true
	}
	return isPersonAccountRecord(record) || hasPersonAccountFieldSignal(record)
}
