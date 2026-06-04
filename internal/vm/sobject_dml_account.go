package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

var accountDMLPolicy = objectDMLPolicy{
	beforeDMLDerivedFields: applyAccountBeforeDMLDerivedFields,
	defaultRecordTypeID:    accountDefaultRecordTypeID,
}

func applyAccountBeforeDMLDerivedFields(_ *VM, record *storage.Record) {
	if record == nil || !accountRecordHasPersonAccountSignal(*record) {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["IsPersonAccount"] = storage.BooleanValue(true)
}

func accountDefaultRecordTypeID(definition storage.ObjectDefinition, record storage.Record) storage.ID {
	if accountRecordHasPersonAccountSignal(record) {
		return personAccountRecordTypeID(definition)
	}
	return businessAccountRecordTypeID(definition)
}

func personAccountRecordTypeID(definition storage.ObjectDefinition) storage.ID {
	if id := preferredPersonAccountRecordTypeID(definition.RecordTypes, false, func(rt storage.RecordTypeInfo) bool {
		return rt.Active && rt.Available && rt.Default
	}); id != "" {
		return id
	}
	if id := preferredPersonAccountRecordTypeID(definition.RecordTypes, false, func(rt storage.RecordTypeInfo) bool {
		return rt.Active && rt.Available
	}); id != "" {
		return id
	}
	if id := preferredPersonAccountRecordTypeID(definition.RecordTypes, true, func(rt storage.RecordTypeInfo) bool {
		return rt.Active && rt.Available && rt.Default
	}); id != "" {
		return id
	}
	if id := preferredPersonAccountRecordTypeID(definition.RecordTypes, false, func(rt storage.RecordTypeInfo) bool {
		return rt.Active
	}); id != "" {
		return id
	}
	if id := preferredPersonAccountRecordTypeID(definition.RecordTypes, false, func(storage.RecordTypeInfo) bool { return true }); id != "" {
		return id
	}
	return preferredPersonAccountRecordTypeID(definition.RecordTypes, true, func(storage.RecordTypeInfo) bool { return true })
}

func preferredPersonAccountRecordTypeID(recordTypes []storage.RecordTypeInfo, allowGeneric bool, accept func(storage.RecordTypeInfo) bool) storage.ID {
	for _, recordType := range recordTypes {
		if recordType.ID == "" || !accept(recordType) || !recordTypeLooksPersonAccount(recordType) {
			continue
		}
		if !allowGeneric && recordTypeLooksGenericPersonAccount(recordType) {
			continue
		}
		return recordType.ID
	}
	return ""
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

func businessAccountRecordTypeID(definition storage.ObjectDefinition) storage.ID {
	for _, recordType := range definition.RecordTypes {
		if recordType.ID != "" && recordType.Default && recordType.Active && recordType.Available && !recordTypeLooksPersonAccount(recordType) {
			return recordType.ID
		}
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.ID != "" && recordType.Active && recordType.Available && !recordTypeLooksPersonAccount(recordType) {
			return recordType.ID
		}
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.ID != "" && recordType.Active && !recordTypeLooksPersonAccount(recordType) {
			return recordType.ID
		}
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.ID != "" && !recordTypeLooksPersonAccount(recordType) {
			return recordType.ID
		}
	}
	return ""
}

func accountRecordHasPersonAccountSignal(record storage.Record) bool {
	for field, value := range record.Fields {
		if !strings.HasPrefix(field, "Person") && field != "FirstName" && field != "LastName" {
			continue
		}
		if storageValueHasNonZeroContent(value) {
			return true
		}
	}
	return false
}

func storageValueHasNonZeroContent(value storage.Value) bool {
	switch value.Kind {
	case storage.ValueNull, "":
		return false
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal:
		return value.String != ""
	case storage.ValueID:
		return value.ID != ""
	case storage.ValueInteger:
		return value.Integer != 0
	default:
		return true
	}
}
