package dml

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) rollbackInsertedRecord(objectName string, definition storage.ObjectDefinition, record storage.Record, sequences map[string]uint64) {
	if e == nil || e.Org == nil || record.ID == "" {
		return
	}
	object, ok := e.Org.Objects[objectName]
	if ok && object.Records != nil {
		delete(object.Records, record.ID)
		e.Org.Objects[objectName] = object
	}
	e.removeUniqueIndexRecord(objectName, definition, record)
	if sequences != nil {
		e.IDs.Sequences = copySequences(sequences)
		e.Org.IDSequences = copySequences(sequences)
	}
}

func applyCustomSettingInsertDefaults(org *storage.OrgState, definition storage.ObjectDefinition, record *storage.Record) {
	if org == nil || record == nil || !storage.IsCustomSettingDefinition(definition) {
		return
	}
	if !strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	orgID := strings.TrimSpace(org.OrgID)
	if orgID == "" {
		orgID = "00D000000000001"
	}
	if value, ok := record.Fields["Name"]; !ok || value.Kind == storage.ValueNull || (value.Kind == storage.ValueString && strings.TrimSpace(value.String) == "") {
		record.Fields["Name"] = storage.StringValue(orgID)
	}
	if _, fieldOK := definition.Fields["SetupOwnerId"]; fieldOK {
		if value, ok := record.Fields["SetupOwnerId"]; !ok || value.Kind == storage.ValueNull || (value.Kind == storage.ValueString && strings.TrimSpace(value.String) == "") {
			record.Fields["SetupOwnerId"] = storage.StringValue(orgID)
		}
	}
}

func applyFieldDefaults(org *storage.OrgState, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	for name, field := range definition.Fields {
		if existing, ok := formulaRecordField(*record, name); ok {
			if shouldRefreshRecordTypeDerivedDefault(definition, *record, field, existing) {
				if value, ok := defaultValueForRecordField(org, definition, *record, field); ok {
					record.Fields[name] = value
				}
			}
			continue
		}
		if strings.EqualFold(name, "RecordTypeId") {
			continue
		}
		if record.ExplicitNulls != nil && record.ExplicitNulls[name] {
			continue
		}
		if value, ok := defaultValueForRecordField(org, definition, *record, field); ok {
			record.Fields[name] = value
		}
	}
}

func shouldRefreshRecordTypeDerivedDefault(definition storage.ObjectDefinition, record storage.Record, field storage.Field, existing storage.Value) bool {
	rawDefault := strings.TrimSpace(field.DefaultValue)
	if !strings.EqualFold(rawDefault, "$RecordType.Name") && !strings.EqualFold(rawDefault, "$RecordType.DeveloperName") {
		return false
	}
	currentDefault := formulaRecordTypeValue(definition, record, rawDefault)
	if currentDefault == "" {
		return false
	}
	existingText := strings.TrimSpace(workflowValueString(existing))
	if strings.EqualFold(existingText, rawDefault) {
		return true
	}
	if existingText == "" || strings.EqualFold(existingText, currentDefault) {
		return false
	}
	for _, recordType := range definition.RecordTypes {
		candidate := recordType.Name
		if strings.EqualFold(rawDefault, "$RecordType.DeveloperName") {
			candidate = recordType.DeveloperName
		} else if candidate == "" {
			candidate = recordType.DeveloperName
		}
		if candidate != "" && strings.EqualFold(existingText, candidate) {
			return true
		}
	}
	return false
}

func applyDefaultRecordTypeID(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil || len(definition.RecordTypes) == 0 {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	personSignal := forcePersonAccountRecordType(objectName, *record)
	if value, ok := record.GetField("RecordTypeId"); ok {
		if personSignal {
			if recordType, found := personAccountRecordType(definition.RecordTypes); found && recordType.ID != "" && idFromStorageValue(value) != recordType.ID {
				record.Fields["RecordTypeId"] = storage.IDValue(recordType.ID)
			}
		}
		return
	}
	if record.ExplicitNulls != nil && record.ExplicitNulls["RecordTypeId"] {
		return
	}
	recordType, ok := defaultRecordTypeForRecord(objectName, definition.RecordTypes, *record)
	if !ok || recordType.ID == "" {
		return
	}
	record.Fields["RecordTypeId"] = storage.IDValue(recordType.ID)
}

func defaultRecordType(recordTypes []storage.RecordTypeInfo) (storage.RecordTypeInfo, bool) {
	for _, recordType := range recordTypes {
		if recordType.Default && recordType.Active && recordType.Available {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if recordType.Default && recordType.Active {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if recordType.Default {
			return recordType, true
		}
	}
	var fallback storage.RecordTypeInfo
	for _, recordType := range recordTypes {
		if recordType.Active && recordType.Available {
			if fallback.ID != "" {
				return storage.RecordTypeInfo{}, false
			}
			fallback = recordType
		}
	}
	if fallback.ID != "" {
		return fallback, true
	}
	for _, recordType := range recordTypes {
		if recordType.Active {
			if fallback.ID != "" {
				return storage.RecordTypeInfo{}, false
			}
			fallback = recordType
		}
	}
	if fallback.ID != "" {
		return fallback, true
	}
	return storage.RecordTypeInfo{}, false
}

func defaultValueForRecordField(org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record, field storage.Field) (storage.Value, bool) {
	rawDefault := strings.TrimSpace(field.DefaultValue)
	if strings.EqualFold(rawDefault, "$RecordType.Name") || strings.EqualFold(rawDefault, "$RecordType.DeveloperName") {
		if value, _, ok := workflowLiteralValue(field, formulaRecordTypeValue(definition, record, rawDefault)); ok {
			return value, true
		}
	}
	if org != nil && (strings.Contains(rawDefault, "$RecordType") || formulaDefaultShouldEvaluate(field, rawDefault)) {
		if value, _, ok := EvaluateRecordFormulaValueInOrg(rawDefault, field, org, definition, record); ok {
			return value, true
		}
	}
	if value, ok := storage.DefaultValueForRecordField(definition, record, field); ok {
		return value, true
	}
	if org == nil || rawDefault == "" {
		return storage.Value{}, false
	}
	value, _, ok := EvaluateRecordFormulaValueInOrg(rawDefault, field, org, definition, record)
	return value, ok
}

func applyAutoNumberName(definition storage.ObjectDefinition, sequence uint64, record *storage.Record) {
	if record == nil {
		return
	}
	nameField, ok := definition.Fields["Name"]
	if !ok || !nameField.AutoNumber {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if value, ok := record.Fields["Name"]; ok && value.Kind == storage.ValueString && strings.TrimSpace(value.String) != "" {
		return
	}
	record.Fields["Name"] = storage.StringValue(formatAutoNumber(nameField.DisplayFormat, sequence))
}

func formatAutoNumber(format string, sequence uint64) string {
	format = strings.TrimSpace(format)
	if format == "" {
		return fmt.Sprintf("%d", sequence)
	}
	start := strings.Index(format, "{")
	end := strings.Index(format, "}")
	if start < 0 || end <= start {
		return format
	}
	token := format[start+1 : end]
	width := 0
	for _, r := range token {
		if r != '0' {
			width = 0
			break
		}
		width++
	}
	number := fmt.Sprintf("%d", sequence)
	if width > 0 {
		number = fmt.Sprintf("%0*d", width, sequence)
	}
	return format[:start] + number + format[end+1:]
}

func (e *Engine) insertPlatformRecord(record storage.Record) (storage.ID, error) {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return "", err
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return "", err
	}
	applyFieldDefaults(e.Org, object.Definition, &record)
	if err := validateFields(object.Definition, e.Org.Namespace, record); err != nil {
		return "", err
	}
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return "", err
	}
	if record.ID == "" {
		e.recordJournalSequence(objectName)
		id, err := e.IDs.Next(objectName)
		if err != nil {
			return "", err
		}
		record.ID = id
	}
	if err := storage.ValidateID(record.ID); err != nil {
		return "", err
	}
	if _, exists := object.Records[record.ID]; exists {
		return "", dmlErrorf("DUPLICATE_VALUE", []string{"Id"}, "dml: duplicate id %s", record.ID)
	}
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, objectName); cloned {
		object = e.Org.Objects[objectName]
	}
	stamp := e.systemTimestamp()
	userID := e.systemUserID()
	if record.System.CreatedDate == "" {
		record.System.CreatedDate = stamp
	}
	if record.System.LastModifiedDate == "" {
		record.System.LastModifiedDate = stamp
	}
	if record.System.SystemModstamp == "" {
		record.System.SystemModstamp = stamp
	}
	if record.System.CreatedByID == "" {
		record.System.CreatedByID = userID
	}
	if record.System.LastModifiedByID == "" {
		record.System.LastModifiedByID = userID
	}
	if record.System.OwnerID == "" {
		record.System.OwnerID = userID
	}
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordInsert(objectName, record.ID)
	}
	object.Records[record.ID] = record.Clone()
	e.Org.Objects[objectName] = object
	return record.ID, nil
}

func deleteCaseInsensitiveField(fields map[string]storage.Value, field string) {
	if fields == nil || field == "" {
		return
	}
	for existing := range fields {
		if existing != field && strings.EqualFold(existing, field) {
			delete(fields, existing)
		}
	}
}

func deleteCaseInsensitiveFieldAlias(definition storage.ObjectDefinition, namespace string, fields map[string]storage.Value, field string) {
	if fields == nil || field == "" {
		return
	}
	for existing := range fields {
		if existing == field {
			continue
		}
		if dmlFieldAliasMatches(definition, namespace, existing, field) {
			delete(fields, existing)
		}
	}
}

func deleteCaseInsensitiveNullAlias(definition storage.ObjectDefinition, namespace string, fields map[string]bool, field string) {
	if fields == nil || field == "" {
		return
	}
	for existing := range fields {
		if existing == field {
			continue
		}
		if dmlFieldAliasMatches(definition, namespace, existing, field) {
			delete(fields, existing)
		}
	}
}

func dmlFieldAliasMatches(definition storage.ObjectDefinition, namespace, existing, field string) bool {
	if strings.EqualFold(existing, field) {
		return true
	}
	canonicalField, ok := storage.ResolveFieldName(definition, namespace, field)
	if !ok {
		return false
	}
	canonicalExisting, ok := storage.ResolveFieldName(definition, namespace, existing)
	return ok && strings.EqualFold(canonicalExisting, canonicalField)
}
