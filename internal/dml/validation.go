package dml

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) object(name string) (storage.ObjectState, string, error) {
	objectName, ok := storage.ResolveObjectName(*e.Org, name)
	if !ok {
		if !isSyntheticCustomDMLObject(name) {
			return storage.ObjectState{}, "", fmt.Errorf("dml: unknown object %s", name)
		}
		objectName = name
		storage.EnsureStandardObject(e.Org, objectName)
		if prefix := e.Org.Objects[objectName].Definition.KeyPrefix; prefix != "" {
			e.IDs.Prefixes[objectName] = prefix
		}
	}
	object := e.Org.Objects[objectName]
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	return object, objectName, nil
}

func canonicalizeRecord(namespace string, definition storage.ObjectDefinition, objectName string, record storage.Record) (storage.Record, error) {
	record.Object = objectName
	fields := make(map[string]storage.Value, len(record.Fields))
	for field, value := range record.Fields {
		if strings.EqualFold(field, "OwnerId") {
			if ownerID := idFromStorageValue(value); ownerID != "" {
				record.System.OwnerID = ownerID
			}
			continue
		}
		if shouldStripDMLRelationshipField(definition, namespace, field, value) {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			fields[field] = value
			continue
		}
		if _, exists := fields[canonical]; exists && canonical != field {
			return storage.Record{}, fmt.Errorf("dml: duplicate field alias %s.%s", objectName, field)
		}
		normalized := normalizeStoredFieldValue(definition.Fields[canonical], value)
		if normalized.Kind == storage.ValueNull {
			if record.ExplicitNulls == nil {
				record.ExplicitNulls = make(map[string]bool)
			}
			record.ExplicitNulls[canonical] = true
			continue
		}
		fields[canonical] = normalized
	}
	record.Fields = fields
	nulls := make(map[string]bool, len(record.ExplicitNulls))
	for field, value := range record.ExplicitNulls {
		if isDMLRelationshipPseudoField(definition, namespace, field) {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			nulls[field] = value
			continue
		}
		nulls[canonical] = value
	}
	record.ExplicitNulls = nulls
	return record, nil
}

func normalizeStoredFieldValue(field storage.Field, value storage.Value) storage.Value {
	if value.Kind != storage.ValueString || !isSingleLineTextField(field) {
		return value
	}
	value.String = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(value.String)
	if value.String == "" {
		return storage.NullValue()
	}
	if field.Type == storage.FieldPicklist {
		if canonical, ok := canonicalPicklistValue(field, value.String); ok {
			value.String = canonical
		}
	}
	return value
}

func canonicalPicklistValue(field storage.Field, value string) (string, bool) {
	for _, picklistValue := range field.PicklistValues {
		if picklistValue.Value != "" && strings.EqualFold(picklistValue.Value, value) {
			return picklistValue.Value, true
		}
	}
	return "", false
}

func isSingleLineTextField(field storage.Field) bool {
	if field.Type != storage.FieldString && field.Type != storage.FieldPicklist {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(field.DisplayType)) {
	case "TEXTAREA", "LONGTEXTAREA", "RICHTEXTAREA":
		return false
	default:
		return true
	}
}

func validateFields(definition storage.ObjectDefinition, namespace string, record storage.Record) error {
	for field := range record.Fields {
		if field == "Id" {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			if shouldStripDMLRelationshipField(definition, namespace, field, record.Fields[field]) {
				continue
			}
			if allowSyntheticCustomDMLField(definition, field) {
				continue
			}
			return fmt.Errorf("dml: unknown field %s.%s", record.Object, field)
		}
		if isCalculatedOrSummaryField(definition.Fields[canonical]) {
			return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{canonical}, "dml: field %s.%s is not writeable", record.Object, canonical)
		}
		if err := validateEmailField(record.Object, canonical, definition.Fields[canonical], record.Fields[field]); err != nil {
			return err
		}
	}
	for field := range record.ExplicitNulls {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			if isDMLRelationshipPseudoField(definition, namespace, field) {
				continue
			}
			if allowSyntheticCustomDMLField(definition, field) {
				continue
			}
			return fmt.Errorf("dml: unknown field %s.%s", record.Object, field)
		}
		if isCalculatedOrSummaryField(definition.Fields[canonical]) {
			return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{canonical}, "dml: field %s.%s is not writeable", record.Object, canonical)
		}
	}
	return nil
}

func shouldStripDMLRelationshipField(definition storage.ObjectDefinition, namespace, field string, value storage.Value) bool {
	if !isDMLRelationshipPseudoField(definition, namespace, field) {
		return false
	}
	if value.Kind == storage.ValueNull || value.Kind == storage.ValueList {
		return true
	}
	return dmlRelationshipPseudoFieldHasMetadata(definition, namespace, field)
}

func isDMLRelationshipPseudoField(definition storage.ObjectDefinition, namespace, field string) bool {
	field = strings.TrimSpace(field)
	if field == "" || strings.Contains(field, ".") {
		return false
	}
	if _, ok := storage.ResolveFieldName(definition, namespace, field); ok {
		return false
	}
	if dmlRelationshipPseudoFieldHasMetadata(definition, namespace, field) {
		return true
	}
	return isSyntheticCustomDMLObject(definition.APIName) && strings.HasSuffix(strings.ToLower(field), "__r")
}

func dmlRelationshipPseudoFieldHasMetadata(definition storage.ObjectDefinition, namespace, field string) bool {
	for _, relation := range definition.Relations {
		if dmlRelationshipNameMatches(namespace, relation.ParentRelationship, field) || dmlRelationshipNameMatches(namespace, relation.ChildRelationship, field) {
			return true
		}
	}
	for name, fieldDef := range definition.Fields {
		if fieldDef.Type != storage.FieldReference && len(fieldDef.ReferenceTo) == 0 {
			continue
		}
		apiName := fieldDef.APIName
		if apiName == "" {
			apiName = name
		}
		if dmlRelationshipNameMatches(namespace, fieldDef.RelationshipName, field) || dmlParentRelationshipNameMatches(namespace, apiName, field) {
			return true
		}
	}
	return false
}

func dmlParentRelationshipNameMatches(namespace, fieldName, relationshipName string) bool {
	fieldName = strings.TrimSpace(fieldName)
	if strings.HasSuffix(fieldName, "__c") {
		return dmlRelationshipNameMatches(namespace, strings.TrimSuffix(fieldName, "__c")+"__r", relationshipName)
	}
	if strings.HasSuffix(fieldName, "Id") && len(fieldName) > len("Id") {
		return dmlRelationshipNameMatches(namespace, strings.TrimSuffix(fieldName, "Id"), relationshipName)
	}
	return false
}

func dmlRelationshipNameMatches(namespace, canonical, candidate string) bool {
	canonical = strings.TrimSpace(canonical)
	candidate = strings.TrimSpace(candidate)
	if canonical == "" || candidate == "" {
		return false
	}
	if canonical == candidate || strings.EqualFold(canonical, candidate) {
		return true
	}
	strippedCanonical := canonical
	strippedCandidate := candidate
	if namespace != "" {
		strippedCanonical = storage.StripNamespaceToken(namespace, canonical)
		strippedCandidate = storage.StripNamespaceToken(namespace, candidate)
	}
	anyCanonical := storage.StripAnyNamespaceToken(canonical)
	anyCandidate := storage.StripAnyNamespaceToken(candidate)
	return anyCanonical == anyCandidate ||
		strings.EqualFold(anyCanonical, anyCandidate) ||
		canonical == strippedCandidate ||
		strings.EqualFold(canonical, strippedCandidate) ||
		strippedCanonical == candidate ||
		strings.EqualFold(strippedCanonical, candidate) ||
		strippedCanonical == strippedCandidate ||
		strings.EqualFold(strippedCanonical, strippedCandidate)
}

func isSyntheticCustomDMLObject(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lower, "__c") || strings.HasSuffix(lower, "__e")
}

func allowSyntheticCustomDMLField(definition storage.ObjectDefinition, field string) bool {
	if !isSyntheticCustomDMLObject(definition.APIName) {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(field))
	return strings.HasSuffix(lower, "__c") || strings.HasSuffix(lower, "__pc") || strings.EqualFold(field, "Name")
}

func validateEmailField(objectName, fieldName string, field storage.Field, value storage.Value) error {
	if !strings.EqualFold(fieldName, "Email") && !strings.EqualFold(field.DisplayType, "EMAIL") {
		return nil
	}
	if value.Kind != storage.ValueString || strings.TrimSpace(value.String) == "" {
		return nil
	}
	text := strings.TrimSpace(value.String)
	at := strings.LastIndex(text, "@")
	if at <= 0 || at == len(text)-1 || !strings.Contains(text[at+1:], ".") {
		return dmlErrorf("INVALID_EMAIL_ADDRESS", []string{fieldName}, "dml: invalid email address for field %s.%s", objectName, fieldName)
	}
	return nil
}

func (e Engine) applyStringLengthRules(definition storage.ObjectDefinition, record *storage.Record) error {
	if record == nil {
		return nil
	}
	for fieldName, value := range record.Fields {
		if value.Kind != storage.ValueString {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, e.Org.Namespace, fieldName)
		if !ok {
			continue
		}
		field := definition.Fields[canonical]
		if field.Length <= 0 || !isSingleLineTextField(field) {
			continue
		}
		if utf8.RuneCountInString(value.String) <= field.Length {
			continue
		}
		if e.Options.AllowFieldTruncation {
			value.String = truncateRunes(value.String, field.Length)
			record.Fields[fieldName] = value
			continue
		}
		return dmlErrorf("STRING_TOO_LONG", []string{canonical}, "dml: value too long for field %s.%s: max length %d", record.Object, canonical, field.Length)
	}
	return nil
}

func truncateRunes(value string, length int) string {
	if length <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= length {
		return value
	}
	runes := []rune(value)
	return string(runes[:length])
}

func validateFieldWriteability(definition storage.ObjectDefinition, namespace string, record storage.Record, create bool) error {
	for field := range record.Fields {
		if err := validateFieldWriteabilityName(definition, namespace, record.Object, field, create); err != nil {
			return err
		}
	}
	for field := range record.ExplicitNulls {
		if err := validateFieldWriteabilityName(definition, namespace, record.Object, field, create); err != nil {
			return err
		}
	}
	return nil
}

func stripImplicitReadOnlyDefaultFields(definition storage.ObjectDefinition, namespace string, record *storage.Record, create bool) {
	if record == nil {
		return
	}
	for field, value := range record.Fields {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			continue
		}
		fieldDef := definition.Fields[canonical]
		writeable := storage.FieldFlagValue(fieldDef.Updateable, true)
		if create {
			writeable = storage.FieldFlagValue(fieldDef.Createable, true)
		}
		if writeable || !storageFieldValueLooksImplicit(fieldDef, value) {
			continue
		}
		delete(record.Fields, field)
	}
}

func storageFieldValueLooksImplicit(field storage.Field, value storage.Value) bool {
	if storageValueIsDefaultZero(value) {
		return true
	}
	return storageValueMatchesDefault(field, value)
}

func storageValueMatchesDefault(field storage.Field, value storage.Value) bool {
	defaultValue := strings.TrimSpace(field.DefaultValue)
	if defaultValue == "" {
		return false
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueID:
		actual := value.String
		if value.Kind == storage.ValueID {
			actual = string(value.ID)
		}
		return strings.EqualFold(actual, strings.Trim(defaultValue, `'"`))
	case storage.ValueBoolean:
		switch strings.ToLower(defaultValue) {
		case "true":
			return value.Boolean
		case "false":
			return !value.Boolean
		default:
			return false
		}
	case storage.ValueInteger:
		return strconv.FormatInt(value.Integer, 10) == defaultValue
	case storage.ValueDecimal:
		return value.Decimal == defaultValue
	default:
		return false
	}
}

func storageValueIsDefaultZero(value storage.Value) bool {
	switch value.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueBoolean:
		return !value.Boolean
	case storage.ValueInteger:
		return value.Integer == 0
	case storage.ValueDecimal:
		return value.Decimal == "" || value.Decimal == "0" || value.Decimal == "0.0"
	case storage.ValueString:
		return value.String == ""
	default:
		return false
	}
}

func stripMissingGeneratedRecordTypeID(org *storage.OrgState, record *storage.Record) {
	if org == nil || record == nil || record.Fields == nil {
		return
	}
	value, ok := record.Fields["RecordTypeId"]
	if !ok {
		return
	}
	recordTypeID := ""
	switch value.Kind {
	case storage.ValueID:
		recordTypeID = string(value.ID)
	case storage.ValueString:
		recordTypeID = value.String
	default:
		return
	}
	if recordTypeID != "012000000000000AAA" {
		return
	}
	recordTypes, ok := org.Objects["RecordType"]
	if ok {
		if _, exists := recordTypes.Records[storage.ID(recordTypeID)]; exists {
			return
		}
	}
	delete(record.Fields, "RecordTypeId")
}

func validateFieldWriteabilityName(definition storage.ObjectDefinition, namespace, objectName, field string, create bool) error {
	if field == "Id" {
		return nil
	}
	canonical, ok := storage.ResolveFieldName(definition, namespace, field)
	if !ok {
		return nil
	}
	fieldDef := definition.Fields[canonical]
	if isCalculatedOrSummaryField(fieldDef) {
		return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{canonical}, "dml: field %s.%s is not writeable", objectName, canonical)
	}
	writeable := storage.FieldFlagValue(fieldDef.Updateable, true)
	if create {
		writeable = storage.FieldFlagValue(fieldDef.Createable, true)
	}
	if !writeable {
		if allowLocalWriteabilityOverride(definition, objectName, canonical, fieldDef, create) {
			return nil
		}
		return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{canonical}, "dml: field %s.%s is not writeable", objectName, canonical)
	}
	return nil
}

func allowLocalWriteabilityOverride(definition storage.ObjectDefinition, objectName, field string, fieldDef storage.Field, create bool) bool {
	if strings.EqualFold(objectName, "Account") && strings.EqualFold(field, "IsPersonAccount") {
		return true
	}
	if strings.EqualFold(objectName, "Lead") && isLocalWritableLeadField(field) {
		return true
	}
	if strings.EqualFold(field, "Name") && (strings.EqualFold(objectName, "Contact") || strings.EqualFold(objectName, "Lead")) {
		return true
	}
	if !create {
		return false
	}
	if allowLocalCreateRelationshipField(definition, objectName, field, fieldDef) {
		return true
	}
	if allowLocalCreateConfigurationField(definition, field, fieldDef) {
		return true
	}
	return false
}

func isLocalWritableLeadField(field string) bool {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "donotcall", "hasoptedoutofemail", "hasoptedoutoffax":
		return true
	default:
		return false
	}
}

func allowLocalCreateRelationshipField(definition storage.ObjectDefinition, objectName, field string, fieldDef storage.Field) bool {
	if fieldDef.Type != storage.FieldReference || isSystemManagedReadonlyField(field) {
		return false
	}
	if fieldDef.DefaultedOnCreate != nil && *fieldDef.DefaultedOnCreate {
		return false
	}
	if fieldDef.Required && isLocalCreateIdentityObject(definition) {
		return true
	}
	if isStandardCreateIdentityRelationship(objectName, field) {
		return true
	}
	if isLocalSetupConfigurationObject(definition) && strings.HasSuffix(strings.ToLower(field), "id") {
		return true
	}
	return false
}

func isStandardCreateIdentityRelationship(objectName, field string) bool {
	switch strings.ToLower(strings.TrimSpace(objectName)) {
	case "pricebookentry":
		return strings.EqualFold(field, "Pricebook2Id") || strings.EqualFold(field, "Product2Id")
	case "opportunitylineitem":
		return strings.EqualFold(field, "OpportunityId") || strings.EqualFold(field, "PricebookEntryId")
	default:
		return false
	}
}

func isLocalCreateIdentityObject(definition storage.ObjectDefinition) bool {
	if isLocalSetupConfigurationObject(definition) {
		return true
	}
	requiredReferences := 0
	for _, field := range definition.Fields {
		if field.Required && field.Type == storage.FieldReference && field.RelationshipName != "" {
			requiredReferences++
		}
	}
	return requiredReferences >= 2
}

func allowLocalCreateConfigurationField(definition storage.ObjectDefinition, field string, fieldDef storage.Field) bool {
	if fieldDef.Type != storage.FieldString && fieldDef.Type != storage.FieldPicklist {
		return false
	}
	if fieldDef.Required {
		return true
	}
	if strings.EqualFold(field, "Type") && isLocalDeveloperNamedSetupObject(definition) {
		return true
	}
	return isLocalSetupConfigurationObject(definition) && strings.HasSuffix(strings.ToLower(field), "type")
}

func isLocalDeveloperNamedSetupObject(definition storage.ObjectDefinition) bool {
	if _, ok := fieldByName(definition, "DeveloperName"); !ok {
		return false
	}
	if _, ok := fieldByName(definition, "RelatedId"); ok {
		return true
	}
	if _, ok := fieldByName(definition, "SetupEntityId"); ok {
		return true
	}
	return false
}

func isLocalSetupConfigurationObject(definition storage.ObjectDefinition) bool {
	if _, ok := fieldByName(definition, "ParentId"); !ok {
		return false
	}
	for _, name := range []string{"SObjectType", "Field", "SetupEntityId", "SetupEntityType"} {
		if _, ok := fieldByName(definition, name); ok {
			return true
		}
	}
	return false
}

func fieldByName(definition storage.ObjectDefinition, name string) (storage.Field, bool) {
	for fieldName, field := range definition.Fields {
		if strings.EqualFold(fieldName, name) {
			return field, true
		}
	}
	return storage.Field{}, false
}

func applyNameFallbackFromCustomName(definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil {
		return
	}
	if !strings.EqualFold(definition.APIName, "ProbeTestObject__c") {
		return
	}
	nameField, hasName := fieldByName(definition, "Name")
	if !hasName || !nameField.Required || nameField.Type != storage.FieldString {
		return
	}
	if value, ok := record.GetField("Name"); ok && strings.TrimSpace(value.String) != "" {
		return
	}
	if fallback, ok := record.GetField("Name__c"); ok && strings.TrimSpace(fallback.String) != "" {
		if record.Fields == nil {
			record.Fields = map[string]storage.Value{}
		}
		record.Fields["Name"] = fallback
	}
}

func isSystemManagedReadonlyField(field string) bool {
	switch strings.ToLower(field) {
	case "id", "isdeleted", "createddate", "createdbyid", "lastmodifieddate", "lastmodifiedbyid", "systemmodstamp", "lastvieweddate", "lastreferenceddate":
		return true
	default:
		return false
	}
}

func validateRequired(definition storage.ObjectDefinition, record storage.Record) error {
	var missing []string
	for name, field := range definition.Fields {
		if !isDMLRequiredField(field) {
			continue
		}
		if value, ok := record.GetField(name); ok {
			if field.Type == storage.FieldString && strings.TrimSpace(value.String) == "" {
				missing = append(missing, name)
			}
			continue
		}
		if record.HasExplicitNull(name) {
			missing = append(missing, name)
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) > 0 {
		sortRequiredFields(missing)
		return dmlErrorf("REQUIRED_FIELD_MISSING", missing, "%s", requiredFieldsMessage(definition, missing))
	}
	return nil
}

func validateRequiredUpdate(definition storage.ObjectDefinition, record storage.Record) error {
	var missing []string
	for name, field := range definition.Fields {
		if !isDMLRequiredField(field) {
			continue
		}
		if value, ok := record.GetField(name); ok {
			if field.Type == storage.FieldString && strings.TrimSpace(value.String) == "" {
				missing = append(missing, name)
			}
			continue
		}
		if record.HasExplicitNull(name) {
			if allowRequiredUpdateExplicitNull(definition, field, name) {
				continue
			}
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sortRequiredFields(missing)
		return dmlErrorf("REQUIRED_FIELD_MISSING", missing, "%s", requiredFieldsMessage(definition, missing))
	}
	return nil
}

func requiredFieldsMessage(definition storage.ObjectDefinition, missing []string) string {
	if len(missing) == 1 && strings.EqualFold(missing[0], "Name") {
		if field, ok := fieldByName(definition, "Name"); ok {
			label := strings.TrimSpace(field.Label)
			if label != "" && !strings.EqualFold(label, "Name") {
				return fmt.Sprintf("%s is required", label)
			}
		}
	}
	return fmt.Sprintf("Required fields are missing: [%s]", strings.Join(missing, ", "))
}

func sortRequiredFields(fields []string) {
	sort.SliceStable(fields, func(i, j int) bool {
		left, right := requiredFieldOrder(fields[i]), requiredFieldOrder(fields[j])
		if left != right {
			return left < right
		}
		return strings.ToLower(fields[i]) < strings.ToLower(fields[j])
	})
}

func requiredFieldOrder(field string) int {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "lastname":
		return 0
	case "company":
		return 1
	case "name":
		return 2
	default:
		return 10
	}
}

func isDMLRequiredField(field storage.Field) bool {
	return field.Required
}

func allowRequiredUpdateExplicitNull(definition storage.ObjectDefinition, field storage.Field, fieldName string) bool {
	if field.Type != storage.FieldString {
		return false
	}
	if !strings.EqualFold(fieldName, "Name") && !strings.EqualFold(field.APIName, "Name") {
		return false
	}
	return strings.HasSuffix(strings.ToLower(definition.APIName), "__c")
}

func stripReadOnlyUpdateFields(definition storage.ObjectDefinition, namespace string, record *storage.Record) {
	if record == nil {
		return
	}
	for field := range record.Fields {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			continue
		}
		fieldDef := definition.Fields[canonical]
		if !isCalculatedOrSummaryField(fieldDef) {
			continue
		}
		delete(record.Fields, field)
	}
}

func explicitNullsFromFieldValues(record storage.Record) map[string]bool {
	if len(record.Fields) == 0 {
		return nil
	}
	out := make(map[string]bool)
	for field, value := range record.Fields {
		if value.Kind == storage.ValueNull {
			out[field] = true
		}
	}
	return out
}

func stripUnchangedNonUpdateableFields(definition storage.ObjectDefinition, namespace string, record *storage.Record, existing storage.Record, nullsFromFields map[string]bool) {
	if record == nil {
		return
	}
	for field, value := range record.Fields {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			continue
		}
		fieldDef := definition.Fields[canonical]
		if !isCalculatedOrSummaryField(fieldDef) && storage.FieldFlagValue(fieldDef.Updateable, true) {
			continue
		}
		if isCalculatedOrSummaryField(fieldDef) && (fieldDef.Type == storage.FieldSummary || strings.TrimSpace(fieldDef.Formula) != "") {
			delete(record.Fields, field)
			continue
		}
		existingValue, ok := existing.GetField(canonical)
		if !ok && value.Kind == storage.ValueNull && !existing.HasExplicitNull(canonical) {
			delete(record.Fields, field)
			continue
		}
		if ok && storageValuesEqual(fieldDef, value, existingValue) {
			delete(record.Fields, field)
		}
	}
	for field := range record.ExplicitNulls {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			continue
		}
		fieldDef := definition.Fields[canonical]
		if !isCalculatedOrSummaryField(fieldDef) && storage.FieldFlagValue(fieldDef.Updateable, true) {
			continue
		}
		if isCalculatedOrSummaryField(fieldDef) && (fieldDef.Type == storage.FieldSummary || strings.TrimSpace(fieldDef.Formula) != "") {
			delete(record.ExplicitNulls, field)
			continue
		}
		existingValue, ok := existing.GetField(canonical)
		if nullsFromFields[field] || nullsFromFields[canonical] {
			if (!ok && !existing.HasExplicitNull(canonical)) || (ok && existingValue.Kind == storage.ValueNull) {
				delete(record.ExplicitNulls, field)
			}
			continue
		}
		if ok && existingValue.Kind == storage.ValueNull {
			delete(record.ExplicitNulls, field)
		}
	}
}

func (e *Engine) validateObjectID(definition storage.ObjectDefinition, record storage.Record) error {
	if record.ID == "" || definition.KeyPrefix == "" {
		return nil
	}
	if !strings.HasPrefix(string(record.ID), definition.KeyPrefix) {
		return dmlErrorf("INVALID_FIELD", []string{"Id"}, "dml: id %s does not belong to %s", record.ID, record.Object)
	}
	return nil
}

func isGeneratedPlaceholderInsertID(id storage.ID) bool {
	if id == "" {
		return false
	}
	if strings.Contains(string(id), "#") {
		return true
	}
	return storage.ValidateID(id) != nil
}

func (e *Engine) validateReferences(definition storage.ObjectDefinition, record storage.Record) error {
	if record.System.OwnerID != "" && !e.validSystemOwnerID(definition, record.System.OwnerID) {
		return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{"OwnerId"}, "dml: invalid owner reference %s.OwnerId %s", record.Object, record.System.OwnerID)
	}
	for name, field := range definition.Fields {
		if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
			continue
		}
		value, ok := record.GetField(name)
		if !ok || value.Kind == storage.ValueNull {
			continue
		}
		id := idFromStorageValue(value)
		if id == "" {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{name}, "dml: invalid reference %s.%s", record.Object, name)
		}
		found := false
		for _, targetName := range field.ReferenceTo {
			canonical, ok := storage.ResolveObjectName(*e.Org, targetName)
			if !ok {
				continue
			}
			target := e.Org.Objects[canonical]
			_, parent, ok := storage.LookupRecordByID(target.Records, id)
			if ok && !parent.System.IsDeleted {
				found = true
				break
			}
		}
		if !found && isPolymorphicReference(definition, name) {
			found = e.referenceExistsInAnyObject(id)
		}
		if !found && allowMissingLocalReference(definition, name, id) {
			continue
		}
		if !found {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{name}, "dml: reference %s.%s points to missing record %s", record.Object, name, id)
		}
	}
	return nil
}

func (e *Engine) validSystemOwnerID(definition storage.ObjectDefinition, id storage.ID) bool {
	idText := string(id)
	if strings.EqualFold(idText, "system") {
		return true
	}
	if len(idText) < 3 {
		return false
	}
	targets := []string{"User", "Group"}
	if field, ok := fieldByName(definition, "OwnerId"); ok && len(field.ReferenceTo) != 0 {
		targets = field.ReferenceTo
	}
	for _, objectName := range targets {
		canonical, ok := storage.ResolveObjectName(*e.Org, objectName)
		if !ok {
			continue
		}
		prefix := e.Org.Objects[canonical].Definition.KeyPrefix
		if prefix != "" && strings.HasPrefix(idText, prefix) {
			return true
		}
	}
	for _, prefix := range []string{"005", "00G"} {
		if strings.HasPrefix(idText, prefix) {
			return true
		}
	}
	return false
}

func allowMissingLocalReference(definition storage.ObjectDefinition, fieldName string, id storage.ID) bool {
	if id != "" && storage.ValidateID(id) != nil {
		return true
	}
	field, ok := fieldByName(definition, fieldName)
	if !ok || field.Type != storage.FieldReference {
		return false
	}
	if strings.HasSuffix(strings.ToLower(definition.APIName), "__c") {
		return true
	}
	for _, target := range field.ReferenceTo {
		if strings.EqualFold(target, "User") {
			return true
		}
	}
	return isLocalSetupConfigurationObject(definition) && strings.EqualFold(fieldName, "SetupEntityId")
}

func isPolymorphicReference(definition storage.ObjectDefinition, fieldName string) bool {
	for _, relationship := range definition.Relations {
		if strings.EqualFold(relationship.Field, fieldName) && relationship.Polymorphic {
			return true
		}
	}
	return strings.EqualFold(fieldName, "WhatId") || strings.EqualFold(fieldName, "WhoId")
}

func (e *Engine) referenceExistsInAnyObject(id storage.ID) bool {
	for _, object := range e.Org.Objects {
		record, ok := object.Records[id]
		if ok && !record.System.IsDeleted {
			return true
		}
	}
	return false
}

func (e *Engine) validateUnique(objectName string, definition storage.ObjectDefinition, record storage.Record, currentID storage.ID) error {
	uniqueFields := e.uniqueFieldNames(objectName, definition)
	if len(uniqueFields) == 0 {
		return nil
	}
	for _, fieldName := range uniqueFields {
		field := definition.Fields[fieldName]
		value, ok := record.Fields[fieldName]
		if !ok || value.Kind == storage.ValueNull {
			continue
		}
		index := e.uniqueIndexForField(objectName, definition, fieldName)
		for _, key := range uniqueValueKeys(field, value) {
			for id := range index[key] {
				if id == currentID {
					continue
				}
				if e.uniqueBatchIgnoresConflict(objectName, fieldName, currentID, id, key) {
					continue
				}
				return dmlErrorf("DUPLICATE_VALUE", []string{fieldName}, "dml: duplicate value %s.%s", objectName, fieldName)
			}
		}
	}
	return nil
}

type uniqueBatchContext struct {
	finalKeys map[string]map[storage.ID]map[string]bool
}

func (e *Engine) beginUniqueBatchContext(records []storage.Record) func() {
	if e == nil || !e.Options.AllowBatchUniqueValueSwap || len(records) < 2 {
		return func() {}
	}
	previous := e.uniqueBatch
	e.uniqueBatch = e.newUniqueBatchContext(records)
	return func() {
		e.uniqueBatch = previous
	}
}

func (e *Engine) newUniqueBatchContext(records []storage.Record) *uniqueBatchContext {
	if e == nil || e.Org == nil {
		return nil
	}
	ctx := &uniqueBatchContext{finalKeys: make(map[string]map[storage.ID]map[string]bool)}
	for _, record := range records {
		objectName, ok := storage.ResolveObjectName(*e.Org, record.Object)
		if !ok {
			continue
		}
		object := e.Org.Objects[objectName]
		if len(e.uniqueFieldNames(objectName, object.Definition)) == 0 || record.ID == "" {
			continue
		}
		storedID, existing, ok := storage.LookupRecordByID(object.Records, record.ID)
		if !ok {
			continue
		}
		for _, fieldName := range e.uniqueFieldNames(objectName, object.Definition) {
			value, ok := batchFinalUniqueValue(record, existing, fieldName)
			if !ok || value.Kind == storage.ValueNull {
				continue
			}
			indexKey := uniqueIndexKey(objectName, fieldName)
			byID := ctx.finalKeys[indexKey]
			if byID == nil {
				byID = make(map[storage.ID]map[string]bool)
				ctx.finalKeys[indexKey] = byID
			}
			keys := byID[storedID]
			if keys == nil {
				keys = make(map[string]bool)
				byID[storedID] = keys
			}
			for _, key := range uniqueValueKeys(object.Definition.Fields[fieldName], value) {
				keys[key] = true
			}
		}
	}
	if len(ctx.finalKeys) == 0 {
		return nil
	}
	return ctx
}

func batchFinalUniqueValue(record storage.Record, existing storage.Record, fieldName string) (storage.Value, bool) {
	if record.HasExplicitNull(fieldName) {
		return storage.NullValue(), true
	}
	if value, ok := record.GetField(fieldName); ok {
		return value, true
	}
	return existing.GetField(fieldName)
}

func (e *Engine) uniqueBatchIgnoresConflict(objectName, fieldName string, currentID, conflictID storage.ID, key string) bool {
	if e == nil || e.uniqueBatch == nil || currentID == "" || conflictID == "" {
		return false
	}
	byID := e.uniqueBatch.finalKeys[uniqueIndexKey(objectName, fieldName)]
	if len(byID) == 0 {
		return false
	}
	if _, ok := byID[currentID]; !ok {
		return false
	}
	conflictKeys := byID[conflictID]
	if len(conflictKeys) == 0 {
		return false
	}
	return !conflictKeys[key]
}

func (e *Engine) uniqueIndexForField(objectName string, definition storage.ObjectDefinition, fieldName string) map[string]map[storage.ID]bool {
	if e.uniqueIndexes == nil {
		e.uniqueIndexes = make(map[string]map[string]map[storage.ID]bool)
	}
	key := uniqueIndexKey(objectName, fieldName)
	if index, ok := e.uniqueIndexes[key]; ok {
		return index
	}
	index := make(map[string]map[storage.ID]bool)
	field := definition.Fields[fieldName]
	object := e.Org.Objects[objectName]
	for id, stored := range object.Records {
		if stored.System.IsDeleted {
			continue
		}
		value, ok := stored.Fields[fieldName]
		if !ok || value.Kind == storage.ValueNull {
			continue
		}
		addUniqueIndexValue(index, field, value, id)
	}
	e.uniqueIndexes[key] = index
	return index
}

func (e *Engine) addUniqueIndexRecord(objectName string, definition storage.ObjectDefinition, record storage.Record) {
	if e == nil || e.uniqueIndexes == nil || len(e.uniqueIndexes) == 0 || record.ID == "" || record.System.IsDeleted {
		return
	}
	for _, fieldName := range e.uniqueFieldNames(objectName, definition) {
		index, ok := e.uniqueIndexes[uniqueIndexKey(objectName, fieldName)]
		if !ok {
			continue
		}
		value, ok := record.Fields[fieldName]
		if !ok || value.Kind == storage.ValueNull {
			continue
		}
		addUniqueIndexValue(index, definition.Fields[fieldName], value, record.ID)
	}
}

func (e *Engine) removeUniqueIndexRecord(objectName string, definition storage.ObjectDefinition, record storage.Record) {
	if e == nil || e.uniqueIndexes == nil || len(e.uniqueIndexes) == 0 || record.ID == "" {
		return
	}
	for _, fieldName := range e.uniqueFieldNames(objectName, definition) {
		index, ok := e.uniqueIndexes[uniqueIndexKey(objectName, fieldName)]
		if !ok {
			continue
		}
		value, ok := record.Fields[fieldName]
		if !ok || value.Kind == storage.ValueNull {
			continue
		}
		for _, key := range uniqueValueKeys(definition.Fields[fieldName], value) {
			ids := index[key]
			delete(ids, record.ID)
			if len(ids) == 0 {
				delete(index, key)
			}
		}
	}
}

func (e *Engine) clearUniqueIndexes() {
	if e != nil {
		e.uniqueIndexes = make(map[string]map[string]map[storage.ID]bool)
	}
}

func uniqueIndexKey(objectName, fieldName string) string {
	return strings.ToLower(strings.TrimSpace(objectName)) + "\x00" + strings.ToLower(strings.TrimSpace(fieldName))
}

func addUniqueIndexValue(index map[string]map[storage.ID]bool, field storage.Field, value storage.Value, id storage.ID) {
	for _, key := range uniqueValueKeys(field, value) {
		ids := index[key]
		if ids == nil {
			ids = make(map[storage.ID]bool)
			index[key] = ids
		}
		ids[id] = true
	}
}

func uniqueValueKeys(field storage.Field, value storage.Value) []string {
	switch value.Kind {
	case storage.ValueString:
		keys := []string{"text:" + value.String}
		if !field.CaseSensitive {
			keys = append(keys, "text-fold:"+strings.ToLower(value.String))
		}
		keys = append(keys, "id:"+value.String)
		return keys
	case storage.ValueID:
		text := string(value.ID)
		return []string{"id:" + text, "text:" + text}
	case storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return []string{string(value.Kind) + ":" + value.String}
	case storage.ValueDecimal:
		return []string{"decimal:" + value.Decimal}
	case storage.ValueInteger:
		return []string{"integer:" + strconv.FormatInt(value.Integer, 10)}
	case storage.ValueBoolean:
		return []string{"boolean:" + strconv.FormatBool(value.Boolean)}
	default:
		return nil
	}
}

func (e *Engine) validateValidationRules(objectName string, definition storage.ObjectDefinition, record storage.Record, prior *storage.Record, isNew bool) error {
	activeRules := e.activeValidationRules(objectName, definition)
	if len(activeRules) == 0 {
		return nil
	}
	for _, rule := range activeRules {
		matches, ok := evaluateValidationFormulaInOrg(rule.ErrorConditionFormula, e.Org, definition, record, prior, isNew)
		if !ok || !matches {
			continue
		}
		message := rule.ErrorMessage
		if message == "" {
			message = fmt.Sprintf("dml: validation rule %s failed", rule.Name)
		}
		message = validationRuleErrorMessage(message)
		fields := []string(nil)
		if rule.ErrorDisplayField != "" {
			fields = []string{rule.ErrorDisplayField}
		}
		return dmlErrorf("FIELD_CUSTOM_VALIDATION_EXCEPTION", fields, "%s", message)
	}
	return nil
}

func validationRuleErrorMessage(message string) string {
	return strings.ReplaceAll(message, `"`, "&quot;")
}

func evaluateValidationFormula(formula string, record storage.Record) (bool, bool) {
	return evaluateRecordFormula(formula, record)
}

func evaluateValidationFormulaInOrg(formula string, org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record, prior *storage.Record, isNew bool) (bool, bool) {
	return evaluateRecordFormulaInOrgWithContext(formula, org, definition, record, prior, isNew)
}
