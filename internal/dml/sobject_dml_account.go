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
	record.Fields["IsPersonAccount"] = storage.BooleanValue(true)
	normalizeFirstLastName(definition, record)
}

func hasPersonAccountSignal(record storage.Record) bool {
	return hasPersonAccountFieldSignal(record)
}

func hasPersonAccountFieldSignal(record storage.Record) bool {
	for field, value := range record.Fields {
		if (strings.HasPrefix(field, "Person") || field == "FirstName" || field == "LastName") && nonDefaultPersonValue(value) {
			return true
		}
	}
	return false
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
	name := strings.ToLower(strings.TrimSpace(recordType.Name + " " + recordType.DeveloperName))
	return strings.Contains(name, "person") || strings.Contains(name, "individual")
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
		if recordType.Default && recordType.Active && recordType.Available && !recordTypeLooksPersonAccount(recordType) {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if recordType.Active && recordType.Available && !recordTypeLooksPersonAccount(recordType) {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if recordType.Active && !recordTypeLooksPersonAccount(recordType) {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if !recordTypeLooksPersonAccount(recordType) {
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
		Fields: personContactFields(account),
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

func personContactFields(account storage.Record) map[string]storage.Value {
	fields := map[string]storage.Value{
		"AccountId": storage.IDValue(account.ID),
	}
	for _, mapping := range personContactFieldMappings() {
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
	mappings := []personContactFieldMapping{
		{"FirstName", "FirstName"},
		{"LastName", "LastName"},
		{"PersonEmail", "Email"},
		{"PersonHomePhone", "HomePhone"},
		{"PersonMobilePhone", "MobilePhone"},
		{"PersonTitle", "Title"},
		{"PersonDepartment", "Department"},
		{"PersonBirthdate", "Birthdate"},
		{"PersonDoNotCall", "DoNotCall"},
		{"PersonHasOptedOutOfEmail", "HasOptedOutOfEmail"},
		{"PersonHasOptedOutOfFax", "HasOptedOutOfFax"},
		{"PersonEmailBouncedReason", "EmailBouncedReason"},
		{"PersonEmailBouncedDate", "EmailBouncedDate"},
	}
	for _, suffix := range []string{"Street", "City", "State", "StateCode", "PostalCode", "Country", "CountryCode"} {
		mappings = append(mappings,
			personContactFieldMapping{"PersonMailing" + suffix, "Mailing" + suffix},
			personContactFieldMapping{"PersonOther" + suffix, "Other" + suffix},
		)
	}
	return mappings
}

func copyPersonContactField(fields map[string]storage.Value, account storage.Record, source, target string) {
	if value, ok := account.GetField(source); ok {
		fields[target] = value.Clone()
	}
}

func (e *Engine) syncPersonContact(account storage.Record) error {
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
	for field, value := range personContactFields(account) {
		contact.Fields[field] = value.Clone()
		delete(contact.ExplicitNulls, field)
	}
	clearExplicitPersonContactFields(&contact, account)
	contactObject.Records[contactID] = contact
	e.Org.Objects["Contact"] = contactObject
	return nil
}

func clearExplicitPersonContactFields(contact *storage.Record, account storage.Record) {
	if contact == nil {
		return
	}
	for _, mapping := range personContactFieldMappings() {
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
