package dml

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func normalizeNameFields(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil {
		return
	}
	if strings.EqualFold(objectName, "Contact") || strings.EqualFold(objectName, "Lead") {
		normalizeFirstLastName(definition, record)
		return
	}
	normalizePersonAccountFields(objectName, definition, record)
}

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

func normalizeFirstLastName(definition storage.ObjectDefinition, record *storage.Record) {
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if _, ok := definition.Fields["Name"]; !ok {
		return
	}
	if _, ok := record.Fields["Name"]; ok && !hasNameComponentField(record.Fields) {
		return
	}
	firstName := stringField(record.Fields, "FirstName")
	lastName := stringField(record.Fields, "LastName")
	switch {
	case firstName != "" && lastName != "":
		record.Fields["Name"] = storage.StringValue(firstName + " " + lastName)
	case lastName != "":
		record.Fields["Name"] = storage.StringValue(lastName)
	}
}

func hasNameComponentField(fields map[string]storage.Value) bool {
	_, hasFirst := fields["FirstName"]
	_, hasLast := fields["LastName"]
	return hasFirst || hasLast
}

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

func stringField(fields map[string]storage.Value, name string) string {
	value, ok := fields[name]
	if !ok || value.Kind != storage.ValueString {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func applyCustomSettingInsertDefaults(org *storage.OrgState, definition storage.ObjectDefinition, record *storage.Record) {
	if org == nil || record == nil || !storage.IsCustomSettingDefinition(definition) {
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
	if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
		if _, fieldOK := definition.Fields["SetupOwnerId"]; fieldOK {
			if value, ok := record.Fields["SetupOwnerId"]; !ok || value.Kind == storage.ValueNull || (value.Kind == storage.ValueString && strings.TrimSpace(value.String) == "") {
				record.Fields["SetupOwnerId"] = storage.StringValue(orgID)
			}
		}
	}
}

func applySetupInsertDefaults(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil {
		return
	}
	defaultRequiredBoolean := func(fieldName string) bool {
		switch {
		case (strings.EqualFold(objectName, "PermissionSet") || strings.EqualFold(objectName, "Profile")) && strings.HasPrefix(fieldName, "Permissions"):
			return true
		case strings.EqualFold(objectName, "User"):
			return true
		default:
			return false
		}
	}
	if !strings.EqualFold(objectName, "PermissionSet") && !strings.EqualFold(objectName, "Profile") && !strings.EqualFold(objectName, "User") {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	for name, field := range definition.Fields {
		if field.Type != storage.FieldBoolean || !field.Required || !defaultRequiredBoolean(name) {
			continue
		}
		if _, ok := record.GetField(name); ok {
			continue
		}
		if record.HasExplicitNull(name) {
			continue
		}
		record.Fields[name] = storage.BooleanValue(false)
	}
	if strings.EqualFold(objectName, "User") {
		if _, ok := definition.Fields["CommunityNickname"]; !ok {
			return
		}
		defaultUserCommunityNickname(record)
	}
}

func (e *Engine) applyFileInsertDefaults(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil || !strings.EqualFold(objectName, "Document") {
		return
	}
	field, ok := definition.Fields["FolderId"]
	if !ok || !fieldReferencesObject(field, "User") {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if _, ok := record.Fields["FolderId"]; ok {
		return
	}
	if record.ExplicitNulls != nil && record.ExplicitNulls["FolderId"] {
		return
	}
	record.Fields["FolderId"] = storage.IDValue(e.systemUserID())
}

func fieldReferencesObject(field storage.Field, objectName string) bool {
	for _, target := range field.ReferenceTo {
		if strings.EqualFold(target, objectName) {
			return true
		}
	}
	return false
}

func defaultUserCommunityNickname(record *storage.Record) {
	if record == nil || record.Fields == nil {
		return
	}
	if _, ok := record.Fields["CommunityNickname"]; ok {
		return
	}
	if record.ExplicitNulls != nil && record.ExplicitNulls["CommunityNickname"] {
		return
	}
	for _, field := range []string{"Alias", "Username", "LastName"} {
		value, ok := record.Fields[field]
		if !ok || value.Kind != storage.ValueString || strings.TrimSpace(value.String) == "" {
			continue
		}
		record.Fields["CommunityNickname"] = storage.StringValue(strings.TrimSpace(value.String))
		return
	}
}

func (e *Engine) applyUserContactAccountDefault(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if e == nil || e.Org == nil || record == nil || !strings.EqualFold(objectName, "User") {
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
	contact, ok := contacts.Records[contactID]
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

func (e *Engine) afterInsertUser(record storage.Record) {
	if e == nil || e.Org == nil || record.ID == "" {
		return
	}
	storage.EnsureStandardObject(e.Org, "UserLogin")
	state := e.Org.Objects["UserLogin"]
	for _, existing := range state.Records {
		if idFromStorageValue(existing.Fields["UserId"]) == record.ID {
			return
		}
	}
	id, err := e.IDs.Next("UserLogin")
	if err != nil {
		return
	}
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, "UserLogin"); cloned {
		state = e.Org.Objects["UserLogin"]
	}
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	state.Records[id] = storage.Record{
		ID:     id,
		Object: "UserLogin",
		Fields: map[string]storage.Value{
			"UserId":   storage.IDValue(record.ID),
			"IsFrozen": storage.BooleanValue(false),
		},
	}
	e.Org.Objects["UserLogin"] = state
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
	personSignal := strings.EqualFold(objectName, "Account") && (isPersonAccountRecord(*record) || hasPersonAccountFieldSignal(*record))
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

func (e *Engine) afterInsertContentVersion(version storage.Record) error {
	contentDocumentID := idFromStorageValue(version.Fields["ContentDocumentId"])
	contentDocumentWasCreated := contentDocumentID == ""
	if contentDocumentID == "" {
		document := storage.Record{
			Object: "ContentDocument",
			Fields: map[string]storage.Value{
				"Title":                    version.Fields["Title"].Clone(),
				"LatestPublishedVersionId": storage.IDValue(version.ID),
			},
		}
		if size, ok := e.contentDocumentSize(version); ok {
			document.Fields["ContentSize"] = storage.IntegerValue(size)
			document.Fields["ContentSizeLong"] = storage.IntegerValue(size)
		}
		if path, ok := version.Fields["PathOnClient"]; ok {
			extension := fileExtension(path.String)
			document.Fields["FileExtension"] = storage.StringValue(extension)
			if fileType := contentDocumentFileType(extension); fileType != "" {
				document.Fields["FileType"] = storage.StringValue(fileType)
			}
		}
		id, err := e.insertPlatformRecord(document)
		if err != nil {
			return err
		}
		contentDocumentID = id
		storage.EnsureMutableObjectRecords(e.Org, "ContentVersion")
		contentVersionObject := e.Org.Objects["ContentVersion"]
		stored := contentVersionObject.Records[version.ID]
		if stored.Fields == nil {
			stored.Fields = make(map[string]storage.Value)
		}
		stored.Fields["ContentDocumentId"] = storage.IDValue(contentDocumentID)
		contentVersionObject.Records[version.ID] = stored
		e.Org.Objects["ContentVersion"] = contentVersionObject
	} else {
		storage.EnsureMutableObjectRecords(e.Org, "ContentDocument")
		contentDocumentObject := e.Org.Objects["ContentDocument"]
		document, exists := contentDocumentObject.Records[contentDocumentID]
		if !exists {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{"ContentDocumentId"}, "dml: ContentDocument %s does not exist", contentDocumentID)
		}
		if document.Fields == nil {
			document.Fields = make(map[string]storage.Value)
		}
		document.Fields["LatestPublishedVersionId"] = storage.IDValue(version.ID)
		if title, ok := version.Fields["Title"]; ok {
			document.Fields["Title"] = title.Clone()
		}
		if size, ok := e.contentDocumentSize(version); ok {
			document.Fields["ContentSize"] = storage.IntegerValue(size)
			document.Fields["ContentSizeLong"] = storage.IntegerValue(size)
		}
		if path, ok := version.Fields["PathOnClient"]; ok {
			extension := fileExtension(path.String)
			document.Fields["FileExtension"] = storage.StringValue(extension)
			if fileType := contentDocumentFileType(extension); fileType != "" {
				document.Fields["FileType"] = storage.StringValue(fileType)
			}
		}
		contentDocumentObject.Records[contentDocumentID] = document
		e.Org.Objects["ContentDocument"] = contentDocumentObject
	}
	e.markLatestContentVersion(contentDocumentID, version.ID)
	locationID := idFromStorageValue(version.Fields["FirstPublishLocationId"])
	if locationID == "" && contentDocumentWasCreated {
		if version.System.OwnerID != "" {
			locationID = version.System.OwnerID
		} else if version.System.CreatedByID != "" {
			locationID = version.System.CreatedByID
		} else {
			locationID = e.systemUserID()
		}
	}
	if locationID != "" {
		link := storage.Record{
			Object: "ContentDocumentLink",
			Fields: map[string]storage.Value{
				"ContentDocumentId": storage.IDValue(contentDocumentID),
				"LinkedEntityId":    storage.IDValue(locationID),
				"ShareType":         storage.StringValue("V"),
				"Visibility":        storage.StringValue("AllUsers"),
			},
		}
		if _, err := e.insertOne(link); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) afterInsertContentDistribution(id storage.ID) {
	storage.EnsureMutableObjectRecords(e.Org, "ContentDistribution")
	object := e.Org.Objects["ContentDistribution"]
	record, ok := object.Records[id]
	if !ok {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	base := "https://glade.local/content/" + string(id)
	if _, ok := record.Fields["ContentDownloadUrl"]; !ok {
		record.Fields["ContentDownloadUrl"] = storage.StringValue(base + "/download")
	}
	if _, ok := record.Fields["DistributionPublicUrl"]; !ok {
		record.Fields["DistributionPublicUrl"] = storage.StringValue(base)
	}
	object.Records[id] = record
	e.Org.Objects["ContentDistribution"] = object
}

func (e *Engine) afterInsertEmailMessage(message storage.Record) error {
	toIDs, ok := message.GetField("ToIds")
	if !ok || toIDs.Kind != storage.ValueList {
		return nil
	}
	for _, toID := range toIDs.List {
		relationID := valueAsIDString(toID)
		if relationID == "" {
			continue
		}
		if relationID == "system" {
			relationID = string(e.systemUserID())
		}
		if err := storage.ValidateID(storage.ID(relationID)); err != nil {
			continue
		}
		storage.EnsureStandardObject(e.Org, "EmailMessageRelation")
		relation := storage.Record{
			Object: "EmailMessageRelation",
			Fields: map[string]storage.Value{
				"EmailMessageId": storage.IDValue(message.ID),
				"RelationId":     storage.IDValue(storage.ID(relationID)),
				"RelationType":   storage.StringValue("ToAddress"),
			},
		}
		if toAddress, ok := message.GetField("ToAddress"); ok && toAddress.String != "" {
			relation.Fields["RelationAddress"] = storage.StringValue(toAddress.String)
		}
		if _, err := e.insertPlatformRecord(relation); err != nil {
			return err
		}
	}
	return nil
}

func valueAsIDString(value storage.Value) string {
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString:
		return strings.TrimSpace(value.String)
	default:
		return ""
	}
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
	object.Records[record.ID] = record.Clone()
	e.Org.Objects[objectName] = object
	return record.ID, nil
}

func (e *Engine) contentDocumentSize(version storage.Record) (int64, bool) {
	documentObject, ok := e.Org.Objects["ContentDocument"]
	if !ok {
		return 0, false
	}
	if _, ok := documentObject.Definition.Fields["ContentSize"]; !ok {
		return 0, false
	}
	data, ok := version.Fields["VersionData"]
	if !ok {
		return 0, false
	}
	switch data.Kind {
	case storage.ValueBlob, storage.ValueString:
		return int64(len(data.String)), true
	default:
		return 0, false
	}
}

func (e *Engine) markLatestContentVersion(contentDocumentID storage.ID, latestVersionID storage.ID) {
	storage.EnsureMutableObjectRecords(e.Org, "ContentVersion")
	contentVersionObject := e.Org.Objects["ContentVersion"]
	changed := false
	for id, stored := range contentVersionObject.Records {
		if idFromStorageValue(stored.Fields["ContentDocumentId"]) != contentDocumentID {
			continue
		}
		if stored.Fields == nil {
			stored.Fields = make(map[string]storage.Value)
		}
		stored.Fields["IsLatest"] = storage.BooleanValue(id == latestVersionID)
		contentVersionObject.Records[id] = stored
		changed = true
	}
	if changed {
		e.Org.Objects["ContentVersion"] = contentVersionObject
	}
}

func (e *Engine) afterInsertPersonAccount(account storage.Record) error {
	if _, ok := e.Org.Objects["Contact"]; !ok {
		return nil
	}
	contact := storage.Record{
		Object: "Contact",
		Fields: personContactFields(account),
	}
	contactID, err := e.insertOne(contact)
	if err != nil {
		return err
	}
	storage.EnsureMutableObjectRecords(e.Org, "Account")
	accountObject := e.Org.Objects["Account"]
	stored := accountObject.Records[account.ID]
	if stored.Fields == nil {
		stored.Fields = make(map[string]storage.Value)
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

func fileExtension(path string) string {
	lastSlash := strings.LastIndexAny(path, `/\`)
	lastDot := strings.LastIndex(path, ".")
	if lastDot <= lastSlash || lastDot == len(path)-1 {
		return ""
	}
	return path[lastDot+1:]
}

func contentDocumentFileType(extension string) string {
	switch strings.ToLower(strings.TrimPrefix(extension, ".")) {
	case "docx":
		return "WORD_X"
	case "xlsx":
		return "EXCEL_X"
	case "pptx":
		return "POWER_POINT_X"
	case "pdf":
		return "PDF"
	case "jpg", "jpeg", "gif", "png":
		return strings.ToUpper(extension)
	case "m4a":
		return "M4A"
	default:
		return strings.ToUpper(extension)
	}
}
