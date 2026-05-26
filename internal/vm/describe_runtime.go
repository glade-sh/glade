package vm

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) describeSObjectValue(name string, definition storage.ObjectDefinition) Value {
	cacheKey := strings.ToLower(strings.TrimSpace(name))
	if cacheKey != "" {
		if cached, ok := vm.describeCache[cacheKey]; ok {
			return cached
		}
	}
	definition = vm.describePreparedDefinition(name, definition)
	desc := Object("Schema.DescribeSObjectResult")
	desc.Fields["name"] = String(vm.describeObjectName(name))
	desc.Fields["label"] = String(definition.Label)
	desc.Fields["labelPlural"] = String(definition.PluralLabel)
	desc.Fields["keyPrefix"] = String(definition.KeyPrefix)
	desc.Fields["customSetting"] = Bool(storage.IsCustomSettingDefinition(definition))
	desc.Fields["custom"] = Bool(isCustomObjectLikeName(definition.APIName))
	desc.Fields["feedEnabled"] = Bool(false)
	desc.Fields["mergeable"] = Bool(false)
	desc.Fields["mruEnabled"] = Bool(true)
	desc.Fields["undeletable"] = Bool(true)
	desc.Fields["deprecatedAndHidden"] = Bool(false)
	fieldsMap := Map()
	fieldsMap.Type = "Schema.SObjectFieldMap"
	fieldsMap.Runtime = "sobjectfieldmap:" + name
	fieldNames := make([]string, 0, len(definition.Fields))
	for fieldName := range definition.Fields {
		fieldNames = append(fieldNames, fieldName)
	}
	sort.Strings(fieldNames)
	for _, fieldName := range fieldNames {
		field := definition.Fields[fieldName]
		apiName := field.APIName
		if strings.TrimSpace(apiName) == "" {
			apiName = fieldName
		}
		if strings.TrimSpace(apiName) == "" {
			continue
		}
		field.APIName = apiName
		field = describeFieldWithSystemOverlay(field)
		token := vm.sObjectFieldTokenFromField(name, field)
		vm.addSObjectFieldMapFieldEntry(&fieldsMap, name, definition, field, token)
	}
	defaultFieldNames := []string{"Id", "CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById", "SystemModstamp"}
	if isCustomObjectLikeName(definition.APIName) {
		defaultFieldNames = append(defaultFieldNames, "Name", "OwnerId")
	}
	for _, fieldName := range defaultFieldNames {
		if _, ok := fieldsMap.Map[mapKey(String(fieldName))]; ok {
			continue
		}
		token := vm.sObjectFieldToken(name, fieldName)
		vm.addSObjectFieldMapEntry(&fieldsMap, name, fieldName, token)
	}
	fields := Object("Schema.SObjectFieldMap")
	fields.Fields["map"] = fieldsMap
	desc.Fields["fields"] = fields
	desc.Fields["fieldSets"] = vm.fieldSetMapValue(name, definition)
	childRelationships := vm.describeChildRelationships(name)
	desc.Fields["childRelationships"] = List(childRelationships...)
	recordTypes := make([]Value, 0, len(definition.RecordTypes))
	byName := Map()
	byName.Type = "Map<String,Schema.RecordTypeInfo>"
	byDeveloperName := Map()
	byDeveloperName.Type = "Map<String,Schema.RecordTypeInfo>"
	byID := Map()
	byID.Type = "Map<Id,Schema.RecordTypeInfo>"
	for _, recordType := range definition.RecordTypes {
		value := recordTypeInfoValue(recordType)
		recordTypes = append(recordTypes, value)
		if name := recordTypeName(recordType); name != "" {
			key := String(name)
			encodedKey := mapKey(key)
			byName.Map[encodedKey] = value
			byName.MapKeys[encodedKey] = key
		}
		if recordType.DeveloperName != "" {
			key := String(recordType.DeveloperName)
			encodedKey := mapKey(key)
			byDeveloperName.Map[encodedKey] = value
			byDeveloperName.MapKeys[encodedKey] = key
		}
		if recordType.ID != "" {
			key := platformScalar("Id", recordType.ID.String())
			encodedKey := mapKey(key)
			byID.Map[encodedKey] = value
			byID.MapKeys[encodedKey] = key
		}
	}
	desc.Fields["recordTypeInfos"] = List(recordTypes...)
	desc.Fields["recordTypeInfosByName"] = byName
	desc.Fields["recordTypeInfosByDeveloperName"] = byDeveloperName
	desc.Fields["recordTypeInfosById"] = byID
	if cacheKey != "" {
		vm.describeCache[cacheKey] = desc
	}
	return desc
}

func (vm *VM) describePreparedDefinition(name string, definition storage.ObjectDefinition) storage.ObjectDefinition {
	cacheKey := strings.ToLower(strings.TrimSpace(name))
	if vm != nil && cacheKey != "" {
		if vm.describeDefCache == nil {
			vm.describeDefCache = make(map[string]storage.ObjectDefinition)
		}
		if cached, ok := vm.describeDefCache[cacheKey]; ok {
			return cached
		}
	}
	definition = cloneDescribeObjectDefinition(definition)
	if definition.KeyPrefix == "" {
		prefixName := definition.APIName
		if canonical, ok := storage.ResolveKnownStandardObjectName(prefixName); ok {
			prefixName = canonical
		} else if canonical, ok := storage.ResolveKnownStandardObjectName(name); ok {
			prefixName = canonical
		}
		if prefix := storage.StandardKeyPrefixes()[prefixName]; prefix != "" {
			definition.KeyPrefix = prefix
		} else {
			definition.KeyPrefix = storage.AssignDeterministicPrefixes([]string{prefixName}, nil)[prefixName]
		}
	}
	storage.EnsureStandardObjectFields(&definition)
	if strings.EqualFold(definition.APIName, "Account") && len(definition.RecordTypes) == 0 {
		storage.EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})
	}
	if strings.EqualFold(definition.APIName, "Account") && vm != nil && vm.Org != nil {
		if personAccountName, ok := vm.resolveObjectName("PersonAccount"); ok {
			personAccount := vm.Org.Objects[personAccountName]
			definition.RecordTypes = appendMissingRecordTypes(definition.RecordTypes, personAccount.Definition.RecordTypes)
		}
	}
	if vm != nil && cacheKey != "" {
		vm.describeDefCache[cacheKey] = definition
	}
	return definition
}

func cloneDescribeObjectDefinition(definition storage.ObjectDefinition) storage.ObjectDefinition {
	out := definition
	if definition.Fields != nil {
		out.Fields = make(map[string]storage.Field, len(definition.Fields))
		for name, field := range definition.Fields {
			copied := field
			copied.ReferenceTo = append([]string(nil), field.ReferenceTo...)
			copied.PicklistValues = append([]storage.PicklistValue(nil), field.PicklistValues...)
			out.Fields[name] = copied
		}
	}
	if definition.Relations != nil {
		out.Relations = append([]storage.Relationship(nil), definition.Relations...)
		for i := range out.Relations {
			out.Relations[i].ParentObjects = append([]string(nil), definition.Relations[i].ParentObjects...)
		}
	}
	if definition.RecordTypes != nil {
		out.RecordTypes = append([]storage.RecordTypeInfo(nil), definition.RecordTypes...)
		for i := range out.RecordTypes {
			if definition.RecordTypes[i].PicklistDefaults == nil {
				continue
			}
			out.RecordTypes[i].PicklistDefaults = make(map[string]string, len(definition.RecordTypes[i].PicklistDefaults))
			for fieldName, value := range definition.RecordTypes[i].PicklistDefaults {
				out.RecordTypes[i].PicklistDefaults[fieldName] = value
			}
		}
	}
	if definition.Metadata != nil {
		out.Metadata = make(map[string]string, len(definition.Metadata))
		for key, value := range definition.Metadata {
			out.Metadata[key] = value
		}
	}
	return out
}

func (vm *VM) addSObjectFieldMapEntry(fieldsMap *Value, objectName, fieldName string, token Value) {
	for _, alias := range vm.sObjectFieldMapAliases(objectName, fieldName) {
		key := mapKey(String(alias))
		if _, exists := fieldsMap.Map[key]; !exists {
			fieldsMap.MapOrder = append(fieldsMap.MapOrder, key)
		}
		fieldsMap.Map[key] = token
		fieldsMap.MapKeys[key] = String(alias)
	}
}

func (vm *VM) addSObjectFieldMapFieldEntry(fieldsMap *Value, objectName string, definition storage.ObjectDefinition, field storage.Field, token Value) {
	vm.addSObjectFieldMapEntry(fieldsMap, objectName, field.APIName, token)
	if field.Type != storage.FieldReference {
		return
	}
	relationshipName := vm.parentRelationshipNameForReferenceField(definition, field)
	if relationshipName == "" || !isCustomFieldOrRelationshipType(relationshipName) {
		return
	}
	vm.addSObjectFieldMapEntry(fieldsMap, objectName, relationshipName, token)
}

func (vm *VM) sObjectFieldMapAliases(objectName, fieldName string) []string {
	seen := map[string]bool{}
	var aliases []string
	add := func(alias string) {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			return
		}
		if seen[alias] {
			return
		}
		seen[alias] = true
		aliases = append(aliases, alias)
		if lowered := strings.ToLower(alias); lowered != alias && !seen[lowered] {
			seen[lowered] = true
			aliases = append(aliases, lowered)
		}
	}
	add(fieldName)
	if strippedField := stripAnyNamespaceToken(fieldName); strippedField != fieldName {
		add(strippedField)
	}
	if objectName != "" {
		add(objectName + "." + fieldName)
		add(localSchemaName(objectName) + "." + fieldName)
		if strippedField := stripAnyNamespaceToken(fieldName); strippedField != fieldName {
			add(objectName + "." + strippedField)
			add(localSchemaName(objectName) + "." + strippedField)
			add(stripAnyNamespaceToken(objectName) + "." + strippedField)
		}
	}
	if vm.Org != nil && vm.Org.Namespace != "" {
		add(storage.NamespaceTokenName(vm.Org.Namespace, fieldName))
		add(storage.StripNamespaceToken(vm.Org.Namespace, fieldName))
		if objectName != "" {
			add(storage.NamespaceTokenName(vm.Org.Namespace, objectName) + "." + fieldName)
			add(storage.StripNamespaceToken(vm.Org.Namespace, objectName) + "." + fieldName)
		}
	}
	if objectName != "" {
		add(stripAnyNamespaceToken(objectName) + "." + fieldName)
	}
	return aliases
}

func (vm *VM) describeChildRelationships(name string) []Value {
	if vm == nil || vm.Org == nil {
		return nil
	}
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return nil
	}
	if vm.childRelCache == nil {
		vm.childRelCache = make(map[string][]Value)
	}
	if cached, ok := vm.childRelCache[target]; ok {
		return append([]Value(nil), cached...)
	}
	childRelationships := make([]Value, 0)
	childObjects := make([]string, 0, len(vm.Org.Objects))
	for childName := range vm.Org.Objects {
		childObjects = append(childObjects, childName)
	}
	sort.Strings(childObjects)
	for _, childName := range childObjects {
		childDefinition := vm.describePreparedDefinition(childName, vm.Org.Objects[childName].Definition)
		relationships := append([]storage.Relationship(nil), childDefinition.Relations...)
		relationships = append(relationships, syntheticSystemParentRelationsForDefinition(childDefinition)...)
		sort.Slice(relationships, func(i, j int) bool {
			if relationships[i].ChildRelationship == relationships[j].ChildRelationship {
				return relationships[i].Field < relationships[j].Field
			}
			return relationships[i].ChildRelationship < relationships[j].ChildRelationship
		})
		for _, relationship := range relationships {
			if relationshipTargetsObject(relationship, name) {
				if relationship.ChildRelationship == "" && canDeriveChildRelationshipName(name, childName, relationship) {
					relationship.ChildRelationship = derivedVMChildRelationshipName(childDefinition)
				}
				childRelationships = append(childRelationships, vm.childRelationshipValue(childName, relationship))
			}
		}
	}
	vm.childRelCache[target] = append([]Value(nil), childRelationships...)
	return childRelationships
}

func (vm *VM) schemaDescribeMapAliases(name string) []string {
	seen := map[string]bool{}
	var aliases []string
	add := func(alias string) {
		alias = strings.TrimSpace(alias)
		if alias == "" || seen[alias] {
			return
		}
		seen[alias] = true
		aliases = append(aliases, alias)
		if lowered := strings.ToLower(alias); lowered != alias && !seen[lowered] {
			seen[lowered] = true
			aliases = append(aliases, lowered)
		}
	}
	add(name)
	if vm.Org != nil && vm.Org.Namespace != "" {
		add(storage.NamespaceTokenName(vm.Org.Namespace, name))
		add(storage.StripNamespaceToken(vm.Org.Namespace, name))
	}
	add(stripAnyNamespaceToken(name))
	return aliases
}

func (vm *VM) describeObjectDefinition(objectName string) (string, storage.ObjectDefinition, bool) {
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(objectName); ok {
			return canonical, vm.Org.Objects[canonical].Definition, true
		}
	}
	if definition, ok := storage.StandardObjectDefinition(objectName); ok {
		return definition.APIName, definition, true
	}
	switch {
	case strings.EqualFold(objectName, "SObject"):
		return "SObject", storage.ObjectDefinition{APIName: "SObject", Label: "SObject", PluralLabel: "SObjects"}, true
	case strings.EqualFold(objectName, "AggregateResult"):
		return "AggregateResult", storage.ObjectDefinition{APIName: "AggregateResult", Label: "Aggregate Result", PluralLabel: "Aggregate Results"}, true
	case isCustomObjectLikeName(objectName):
		return objectName, storage.ObjectDefinition{
			APIName:     objectName,
			Label:       objectName,
			PluralLabel: objectName,
			Fields:      map[string]storage.Field{},
		}, true
	default:
		return "", storage.ObjectDefinition{}, false
	}
}

func appendMissingRecordTypes(recordTypes []storage.RecordTypeInfo, extra []storage.RecordTypeInfo) []storage.RecordTypeInfo {
	for _, candidate := range extra {
		found := false
		for _, existing := range recordTypes {
			if candidate.ID != "" && existing.ID == candidate.ID {
				found = true
				break
			}
			if candidate.DeveloperName != "" && strings.EqualFold(existing.DeveloperName, candidate.DeveloperName) {
				found = true
				break
			}
			if recordTypeName(candidate) != "" && strings.EqualFold(recordTypeName(existing), recordTypeName(candidate)) {
				found = true
				break
			}
		}
		if !found {
			recordTypes = append(recordTypes, candidate)
		}
	}
	return recordTypes
}

func recordTypeName(recordType storage.RecordTypeInfo) string {
	if recordType.Name != "" {
		return recordType.Name
	}
	return recordType.DeveloperName
}

func defaultRecordTypeID(definition storage.ObjectDefinition) storage.ID {
	for _, recordType := range definition.RecordTypes {
		if recordType.Default && (recordType.Available || recordType.Active) && recordType.ID != "" {
			return recordType.ID
		}
	}
	var fallback storage.ID
	for _, recordType := range definition.RecordTypes {
		if recordType.Available || recordType.Active {
			if recordType.ID != "" {
				if fallback != "" {
					return ""
				}
				fallback = recordType.ID
			}
		}
	}
	if fallback != "" {
		return fallback
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.ID == "" {
			continue
		}
		if fallback != "" {
			return ""
		}
		fallback = recordType.ID
	}
	return ""
}

func (vm *VM) defaultValueForNewSObjectField(definition storage.ObjectDefinition, record Value, field storage.Field) (storage.Value, bool) {
	if vm == nil || vm.Org == nil {
		return storage.DefaultValueForField(field)
	}
	stored, err := vm.recordFromValue(&record)
	if err != nil {
		return storage.DefaultValueForField(field)
	}
	rawDefault := strings.TrimSpace(field.DefaultValue)
	if rawDefault != "" && (strings.Contains(rawDefault, "$RecordType") || strings.ContainsAny(rawDefault, "()+-*/&<>=")) {
		if value, _, ok := dml.EvaluateRecordFormulaValueInOrg(rawDefault, field, vm.Org, definition, stored); ok {
			return value, true
		}
	}
	if value, ok := storage.DefaultValueForRecordField(definition, stored, field); ok {
		return value, true
	}
	return storage.DefaultValueForField(field)
}

func isNameFieldDescribe(field storage.Field) bool {
	return strings.EqualFold(field.APIName, "Name")
}

func isCustomSchemaName(name string) bool {
	name = strings.ToLower(name)
	return strings.HasSuffix(name, "__c") || strings.HasSuffix(name, "__pc") || strings.HasSuffix(name, "__pr")
}

func compoundFieldNameValue(fieldName string) Value {
	fieldName = strings.TrimSpace(fieldName)
	for _, suffix := range []string{"GeocodeAccuracy", "PostalCode", "CountryCode", "StateCode", "Longitude", "Latitude", "Street", "Country", "State", "City"} {
		if !strings.HasSuffix(fieldName, suffix) || len(fieldName) == len(suffix) {
			continue
		}
		return String(strings.TrimSuffix(fieldName, suffix) + "Address")
	}
	return Null
}

func describeCompoundFieldNameValue(field storage.Field) Value {
	if field.CompoundFieldName != "" {
		return String(field.CompoundFieldName)
	}
	return compoundFieldNameValue(field.APIName)
}

func localSchemaName(name string) string {
	parts := strings.Split(name, "__")
	if len(parts) > 2 {
		return strings.Join(parts[1:], "__")
	}
	return name
}

func methodDescribeBoolField(method string) string {
	if strings.HasPrefix(method, "is") && len(method) > 2 {
		name := method[2:]
		return strings.ToLower(name[:1]) + name[1:]
	}
	return method
}

func describeFieldLength(field storage.Field) int {
	if field.Length > 0 {
		return field.Length
	}
	switch strings.ToUpper(field.DisplayType) {
	case "TEXTAREA", "LONGTEXTAREA", "RICHTEXTAREA":
		return 32768
	}
	switch field.Type {
	case storage.FieldString, storage.FieldPicklist, storage.FieldReference, storage.FieldID:
		return 255
	default:
		return 0
	}
}

func describeFieldPrecision(field storage.Field) int {
	if field.Precision > 0 {
		return field.Precision
	}
	switch field.Type {
	case storage.FieldDecimal:
		return 18
	default:
		return 0
	}
}

func describeFieldScale(field storage.Field) int {
	if field.Scale > 0 {
		return field.Scale
	}
	if field.Type != storage.FieldDecimal {
		return 0
	}
	switch strings.ToUpper(field.DisplayType) {
	case "CURRENCY", "PERCENT":
		return 2
	default:
		return 0
	}
}

func describeFieldIsHTMLFormatted(field storage.Field) bool {
	return strings.EqualFold(field.DisplayType, "RICHTEXTAREA") || strings.EqualFold(string(field.Type), "RICHTEXTAREA")
}

func describeFieldCalculated(field storage.Field) bool {
	return field.Type == storage.FieldCalculated || strings.TrimSpace(field.Formula) != ""
}

func describeFieldSortable(field storage.Field) bool {
	displayType := field.DisplayType
	if displayType == "" {
		displayType = string(field.Type)
	}
	switch strings.ToUpper(displayType) {
	case "MULTIPICKLIST", "TEXTAREA", "ENCRYPTEDSTRING", "BASE64", "BLOB", "ADDRESS", "LOCATION":
		return false
	default:
		return true
	}
}

func isCustomObjectLikeName(name string) bool {
	name = strings.ToLower(name)
	return strings.HasSuffix(name, "__c") || strings.HasSuffix(name, "__e") || strings.HasSuffix(name, "__mdt")
}

func isCustomFieldOrRelationshipType(name string) bool {
	name = strings.ToLower(name)
	return strings.HasSuffix(name, "__c") || strings.HasSuffix(name, "__r")
}

func relationshipTargetsObject(relationship storage.Relationship, objectName string) bool {
	for _, parent := range relationship.ParentObjects {
		if strings.EqualFold(parent, objectName) ||
			strings.EqualFold(stripAnyNamespaceToken(parent), stripAnyNamespaceToken(objectName)) ||
			strings.EqualFold(stripStandardObjectNamespaceToken(parent), stripStandardObjectNamespaceToken(objectName)) {
			return true
		}
	}
	return false
}

func stripStandardObjectNamespaceToken(name string) string {
	first := strings.Index(name, "__")
	if first <= 0 || first+2 >= len(name) {
		return name
	}
	rest := name[first+2:]
	if strings.Contains(rest, "__") {
		return stripAnyNamespaceToken(name)
	}
	if isCustomAPISuffix(rest) {
		return name
	}
	return rest
}

func isCustomAPISuffix(name string) bool {
	switch strings.ToLower(name) {
	case "c", "r", "mdt", "e", "b", "x", "kav", "ka":
		return true
	default:
		return false
	}
}

func (vm *VM) childRelationshipValue(childObject string, relationship storage.Relationship) Value {
	value := Object("Schema.ChildRelationship")
	relationshipName := relationship.ChildRelationship
	if vm != nil && vm.Org != nil {
		relationshipName = describeChildRelationshipName(vm.Org.Namespace, childObject, relationshipName)
	}
	value.Fields["relationshipName"] = String(relationshipName)
	value.Fields["field"] = sObjectFieldToken(childObject, relationship.Field)
	value.Fields["childSObject"] = sObjectTypeToken(childObject)
	value.Fields["cascadeDelete"] = Bool(relationship.CascadeDelete)
	value.Fields["restrictedDelete"] = Bool(relationship.RestrictedDelete)
	return value
}

func describeChildRelationshipName(namespace, childObject, relationshipName string) string {
	name := strings.TrimSpace(relationshipName)
	if name == "" {
		return name
	}
	if isCustomObjectLikeName(storage.StripNamespaceToken(namespace, childObject)) && !strings.HasSuffix(strings.ToLower(name), "__r") {
		name += "__r"
	}
	return storage.NamespaceTokenName(namespace, name)
}

func syntheticSystemParentRelationsForDefinition(definition storage.ObjectDefinition) []storage.Relationship {
	fields := []string{"CreatedById", "LastModifiedById", "OwnerId"}
	relations := make([]storage.Relationship, 0, len(fields))
	for _, fieldName := range fields {
		if hasRelationForField(definition.Relations, fieldName) {
			continue
		}
		field, ok := syntheticSObjectSystemField(fieldName)
		if !ok || len(field.ReferenceTo) == 0 {
			continue
		}
		relations = append(relations, storage.Relationship{
			Field:              field.APIName,
			ParentObjects:      append([]string(nil), field.ReferenceTo...),
			ParentRelationship: field.RelationshipName,
		})
	}
	return relations
}

func canDeriveChildRelationshipName(parentObject, childObject string, relationship storage.Relationship) bool {
	if sameSObjectName(parentObject, childObject) {
		return false
	}
	return !strings.EqualFold(relationship.Field, "CreatedById") &&
		!strings.EqualFold(relationship.Field, "LastModifiedById") &&
		!strings.EqualFold(relationship.Field, "OwnerId")
}

func sameSObjectName(a, b string) bool {
	return strings.EqualFold(a, b) ||
		strings.EqualFold(stripAnyNamespaceToken(a), stripAnyNamespaceToken(b)) ||
		strings.EqualFold(stripStandardObjectNamespaceToken(a), stripStandardObjectNamespaceToken(b))
}

func hasRelationForField(relations []storage.Relationship, fieldName string) bool {
	for _, relation := range relations {
		if strings.EqualFold(relation.Field, fieldName) {
			return true
		}
	}
	return false
}

func (vm *VM) fieldSetMapValue(objectName string, definition storage.ObjectDefinition) Value {
	fieldSets := Object("Schema.FieldSetMap")
	m := Map()
	if vm.Org == nil {
		fieldSets.Fields["map"] = m
		return fieldSets
	}
	for _, fieldSet := range vm.Org.Metadata.FieldSets {
		if !metadataObjectNameMatches(vm.Org.Namespace, objectName, fieldSet.ObjectName) {
			continue
		}
		value := vm.fieldSetValue(objectName, definition, fieldSet)
		for _, alias := range fieldSetMapAliases(vm.Org.Namespace, fieldSet.Name) {
			m.Map[mapKey(String(alias))] = value
		}
	}
	fieldSets.Fields["map"] = m
	return fieldSets
}

func fieldSetMapAliases(namespace, name string) []string {
	seen := map[string]bool{}
	var aliases []string
	add := func(alias string) {
		alias = strings.TrimSpace(alias)
		if alias == "" || seen[alias] {
			return
		}
		seen[alias] = true
		aliases = append(aliases, alias)
		if lower := strings.ToLower(alias); lower != alias && !seen[lower] {
			seen[lower] = true
			aliases = append(aliases, lower)
		}
	}
	add(name)
	if namespace != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(namespace+"__")) {
		add(namespace + "__" + name)
	}
	if namespace != "" {
		add(storage.StripNamespaceToken(namespace, name))
	}
	add(stripAnyNamespaceToken(name))
	return aliases
}

func (vm *VM) fieldSetValue(objectName string, definition storage.ObjectDefinition, fieldSet storage.FieldSetMetadata) Value {
	value := Object("Schema.FieldSet")
	defaultNamespace := ""
	if vm.Org != nil {
		defaultNamespace = strings.TrimSpace(vm.Org.Namespace)
	}
	namespace, localName := fieldSetNamespaceAndLocalName(defaultNamespace, fieldSet.Name)
	value.Fields["name"] = String(localName)
	if namespace == "" {
		value.Fields["namespace"] = Null
	} else {
		value.Fields["namespace"] = String(namespace)
	}
	value.Fields["sObjectType"] = sObjectTypeToken(objectName)
	label := fieldSet.Label
	if label == "" {
		label = localName
	}
	value.Fields["label"] = String(label)
	members := make([]Value, 0, len(fieldSet.Fields))
	for _, member := range fieldSet.Fields {
		members = append(members, vm.fieldSetMemberValue(objectName, definition, member))
	}
	value.Fields["fields"] = List(members...)
	return value
}

func fieldSetNamespaceAndLocalName(defaultNamespace, name string) (string, string) {
	name = strings.TrimSpace(name)
	namespace := strings.TrimSpace(defaultNamespace)
	if namespace != "" {
		prefix := namespace + "__"
		if len(name) > len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) {
			return namespace, name[len(prefix):]
		}
		return namespace, name
	}
	if idx := strings.Index(name, "__"); idx > 0 && idx+2 < len(name) {
		return name[:idx], name[idx+2:]
	}
	return namespace, name
}

func (vm *VM) fieldSetMemberValue(objectName string, definition storage.ObjectDefinition, member storage.FieldSetMemberMetadata) Value {
	value := Object("Schema.FieldSetMember")
	value.Fields["fieldPath"] = String(member.Field)
	value.Fields["required"] = Bool(member.Required)
	value.Fields["dbRequired"] = Bool(member.Required)
	label := member.Field
	displayType := String("STRING")
	if fieldName, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, member.Field); ok {
		field := definition.Fields[fieldName]
		if field.Label != "" {
			label = field.Label
		} else if field.APIName != "" {
			label = field.APIName
		}
		display := field.DisplayType
		if display == "" {
			display = string(field.Type)
		}
		displayType = schemaDisplayTypeValue(display)
		value.Fields["dbRequired"] = Bool(field.Required)
		value.Fields["sObjectField"] = vm.sObjectFieldTokenFromField(objectName, field)
	} else {
		value.Fields["sObjectField"] = Value{Kind: ValueNull, Type: "Schema.SObjectField"}
	}
	value.Fields["label"] = String(label)
	value.Fields["type"] = displayType
	return value
}

func metadataObjectNameMatches(namespace, canonical, candidate string) bool {
	if candidate == "" {
		return false
	}
	if strings.EqualFold(canonical, candidate) {
		return true
	}
	if namespace == "" {
		return false
	}
	stripped := storage.StripNamespaceToken(namespace, candidate)
	return strings.EqualFold(canonical, stripped)
}

func recordTypeInfoValue(recordType storage.RecordTypeInfo) Value {
	value := Object("Schema.RecordTypeInfo")
	value.Fields["recordTypeId"] = platformScalar("Id", recordType.ID.String())
	value.Fields["developerName"] = String(recordType.DeveloperName)
	value.Fields["name"] = String(recordTypeName(recordType))
	value.Fields["active"] = Bool(recordType.Active)
	value.Fields["available"] = Bool(recordType.Available || recordType.Active)
	value.Fields["default"] = Bool(recordType.Default)
	return value
}

func (vm *VM) describeFieldValue(objectName, fieldName string) (Value, error) {
	objectName, definition, ok := vm.describeObjectDefinition(objectName)
	if !ok {
		return Null, fmt.Errorf("Schema field describe unknown object %s", objectName)
	}
	requestedFieldName := fieldName
	if cached, ok := vm.lookupFieldDescribeCache(objectName, requestedFieldName); ok {
		return cached, nil
	}
	definition = vm.describePreparedDefinition(objectName, definition)
	namespace := ""
	if vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	fieldName, ok = storage.ResolveFieldName(definition, namespace, fieldName)
	if !ok {
		fieldName = requestedFieldName
		if systemField, systemOK := syntheticSObjectSystemField(fieldName); systemOK {
			fieldName = systemField.APIName
			field := systemField
			return vm.describeSyntheticFieldValue(objectName, fieldName, field)
		}
		if vm.canSynthesizeSchemaField(objectName) {
			if field := vm.syntheticSchemaFieldForObject(objectName, fieldName); field.APIName != "" {
				return vm.describeSyntheticFieldValue(objectName, field.APIName, field)
			}
		}
		if strings.TrimSpace(fieldName) == "" {
			return emptySObjectFieldDescribe(objectName), nil
		}
		return Null, fmt.Errorf("Schema field describe unknown field %s.%s", objectName, fieldName)
	}
	if strings.TrimSpace(fieldName) == "" {
		fieldName = requestedFieldName
	}
	cacheKey := strings.ToLower(objectName) + "." + strings.ToLower(fieldName)
	if cached, ok := vm.fieldDescribeCache[cacheKey]; ok {
		vm.storeFieldDescribeCacheAliases(objectName, requestedFieldName, fieldName, cached)
		return cached, nil
	}
	field := definition.Fields[fieldName]
	if strings.TrimSpace(field.APIName) == "" {
		if strings.EqualFold(fieldName, "Id") {
			field = storage.Field{APIName: "Id", Label: "Record ID", Type: storage.FieldID}
		} else if strings.TrimSpace(requestedFieldName) != "" {
			field.APIName = requestedFieldName
		} else {
			field.APIName = fieldName
		}
	}
	field = describeFieldWithSystemOverlay(field)
	desc := Object("Schema.DescribeFieldResult")
	describeName := vm.describeFieldName(field.APIName)
	desc.Fields["name"] = String(describeName)
	desc.Fields["sObjectName"] = String(objectName)
	label := field.Label
	if label == "" {
		label = field.APIName
	}
	desc.Fields["label"] = String(label)
	desc.Fields["compoundFieldName"] = describeCompoundFieldNameValue(field)
	desc.Fields["localName"] = String(localSchemaName(describeName))
	displayType := field.DisplayType
	if displayType == "" {
		displayType = string(field.Type)
	}
	desc.Fields["type"] = schemaDisplayTypeValue(displayType)
	desc.Fields["soapType"] = schemaSOAPTypeValue(soapTypeForStorageField(field))
	desc.Fields["nillable"] = Bool(storage.FieldFlagValue(field.Nillable, !field.Required))
	desc.Fields["externalId"] = Bool(field.ExternalID)
	desc.Fields["unique"] = Bool(field.Unique)
	desc.Fields["encrypted"] = Bool(field.Encrypted)
	calculated := describeFieldCalculated(field)
	desc.Fields["calculated"] = Bool(calculated)
	desc.Fields["autoNumber"] = Bool(field.AutoNumber)
	desc.Fields["nameField"] = Bool(isNameFieldDescribe(field))
	desc.Fields["custom"] = Bool(isCustomSchemaName(field.APIName))
	desc.Fields["length"] = Int(int64(describeFieldLength(field)))
	desc.Fields["precision"] = Int(int64(describeFieldPrecision(field)))
	desc.Fields["scale"] = Int(int64(describeFieldScale(field)))
	desc.Fields["htmlFormatted"] = Bool(describeFieldIsHTMLFormatted(field))
	if defaultValue, ok := storage.DefaultValueForField(field); ok {
		desc.Fields["defaultValue"] = vmValueFromStorage(defaultValue)
		desc.Fields["defaultedOnCreate"] = Bool(storage.FieldFlagValue(field.DefaultedOnCreate, true))
	} else {
		desc.Fields["defaultValue"] = Null
		desc.Fields["defaultedOnCreate"] = Bool(storage.FieldFlagValue(field.DefaultedOnCreate, false))
	}
	if strings.TrimSpace(field.DefaultValue) == "" {
		desc.Fields["defaultValueFormula"] = Null
	} else {
		desc.Fields["defaultValueFormula"] = String(field.DefaultValue)
	}
	relationshipName := field.RelationshipName
	if field.Type == storage.FieldReference {
		relationshipName = vm.parentRelationshipNameForReferenceField(definition, field)
	}
	if relationshipName == "" {
		desc.Fields["relationshipName"] = Null
	} else {
		desc.Fields["relationshipName"] = String(relationshipName)
	}
	referenceTo := describeFieldReferenceTargets(definition, field)
	references := make([]Value, 0, len(referenceTo))
	for _, target := range referenceTo {
		references = append(references, sObjectTypeToken(target))
	}
	desc.Fields["referenceTo"] = List(references...)
	desc.Fields["accessible"] = Bool(storage.FieldFlagValue(field.Accessible, true))
	desc.Fields["createable"] = Bool(storage.FieldFlagValue(field.Createable, !calculated))
	desc.Fields["updateable"] = Bool(storage.FieldFlagValue(field.Updateable, !calculated))
	desc.Fields["filterable"] = Bool(storage.FieldFlagValue(field.Filterable, true))
	desc.Fields["groupable"] = Bool(storage.FieldFlagValue(field.Groupable, true))
	desc.Fields["sortable"] = Bool(storage.FieldFlagValue(field.Sortable, describeFieldSortable(field)))
	desc.Fields["aggregatable"] = Bool(storage.FieldFlagValue(field.Aggregatable, true))
	desc.Fields["permissionable"] = Bool(storage.FieldFlagValue(field.Permissionable, true))
	desc.Fields["deprecatedAndHidden"] = Bool(storage.FieldFlagValue(field.DeprecatedAndHidden, false))
	picklistValues := make([]Value, 0, len(field.PicklistValues))
	for _, value := range field.PicklistValues {
		entry := Object("Schema.PicklistEntry")
		entry.Fields["value"] = String(value.Value)
		label := value.Label
		if label == "" {
			label = value.Value
		}
		entry.Fields["label"] = String(label)
		entry.Fields["default"] = Bool(value.Default)
		entry.Fields["active"] = Bool(value.Active)
		picklistValues = append(picklistValues, entry)
	}
	desc.Fields["picklistValues"] = List(picklistValues...)
	vm.fieldDescribeCache[cacheKey] = desc
	vm.storeFieldDescribeCacheAliases(objectName, requestedFieldName, fieldName, desc)
	return desc, nil
}

func (vm *VM) lookupFieldDescribeCache(objectName, fieldName string) (Value, bool) {
	for _, key := range vm.fieldDescribeCacheKeys(objectName, fieldName) {
		if cached, ok := vm.fieldDescribeCache[key]; ok {
			return cached, true
		}
	}
	return Null, false
}

func (vm *VM) storeFieldDescribeCacheAliases(objectName, requestedFieldName, resolvedFieldName string, value Value) {
	keys := vm.fieldDescribeCacheKeys(objectName, requestedFieldName)
	keys = append(keys, vm.fieldDescribeCacheKeys(objectName, resolvedFieldName)...)
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		vm.fieldDescribeCache[key] = value
	}
}

func (vm *VM) fieldDescribeCacheKeys(objectName, fieldName string) []string {
	fieldName = strings.TrimSpace(fieldName)
	if fieldName == "" {
		return nil
	}
	aliases := []string{fieldName}
	if dot := strings.LastIndexByte(fieldName, '.'); dot >= 0 && dot+1 < len(fieldName) {
		aliases = append(aliases, fieldName[dot+1:])
	}
	if vm != nil && vm.Org != nil && vm.Org.Namespace != "" {
		aliases = append(aliases,
			storage.NamespaceTokenName(vm.Org.Namespace, fieldName),
			storage.StripNamespaceToken(vm.Org.Namespace, fieldName),
		)
	}
	keys := make([]string, 0, len(aliases))
	seen := map[string]struct{}{}
	base := strings.ToLower(strings.TrimSpace(objectName)) + "."
	for _, alias := range aliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" {
			continue
		}
		key := base + alias
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func describeFieldReferenceTargets(definition storage.ObjectDefinition, field storage.Field) []string {
	referenceTo := append([]string(nil), field.ReferenceTo...)
	if field.Type == storage.FieldReference &&
		strings.HasSuffix(strings.ToLower(definition.APIName), "__c") &&
		describeProviderIDAllowsUserTarget(definition, field) {
		referenceTo = appendUniqueStringFold(referenceTo, "User")
	}
	return referenceTo
}

func describeProviderIDAllowsUserTarget(definition storage.ObjectDefinition, field storage.Field) bool {
	apiName := strings.ToLower(strings.TrimSpace(field.APIName))
	if strings.Contains(apiName, "provider_id") {
		return true
	}
	if !strings.EqualFold(field.APIName, "Provider__c") {
		return false
	}
	for name, candidate := range definition.Fields {
		candidateName := candidate.APIName
		if strings.TrimSpace(candidateName) == "" {
			candidateName = name
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(candidateName)), "provider_id") {
			return true
		}
	}
	return false
}

func appendUniqueStringFold(values []string, value string) []string {
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}

func (vm *VM) describeSyntheticFieldValue(objectName, fieldName string, field storage.Field) (Value, error) {
	cacheKey := strings.ToLower(objectName) + "." + strings.ToLower(fieldName)
	if cached, ok := vm.fieldDescribeCache[cacheKey]; ok {
		return cached, nil
	}
	if field.APIName == "" {
		field.APIName = fieldName
	}
	desc := Object("Schema.DescribeFieldResult")
	desc.Fields["name"] = String(field.APIName)
	desc.Fields["sObjectName"] = String(objectName)
	label := field.Label
	if label == "" {
		label = field.APIName
	}
	desc.Fields["label"] = String(label)
	desc.Fields["compoundFieldName"] = describeCompoundFieldNameValue(field)
	displayType := field.DisplayType
	if displayType == "" {
		displayType = string(field.Type)
	}
	desc.Fields["type"] = schemaDisplayTypeValue(displayType)
	desc.Fields["soapType"] = schemaSOAPTypeValue(soapTypeForStorageField(field))
	desc.Fields["nillable"] = Bool(storage.FieldFlagValue(field.Nillable, !field.Required))
	desc.Fields["externalId"] = Bool(false)
	desc.Fields["unique"] = Bool(false)
	desc.Fields["encrypted"] = Bool(false)
	desc.Fields["calculated"] = Bool(false)
	desc.Fields["autoNumber"] = Bool(false)
	desc.Fields["nameField"] = Bool(false)
	desc.Fields["custom"] = Bool(false)
	desc.Fields["length"] = Int(int64(describeFieldLength(field)))
	desc.Fields["precision"] = Int(int64(describeFieldPrecision(field)))
	desc.Fields["scale"] = Int(int64(describeFieldScale(field)))
	desc.Fields["htmlFormatted"] = Bool(false)
	if field.RelationshipName == "" {
		desc.Fields["relationshipName"] = Null
	} else {
		desc.Fields["relationshipName"] = String(field.RelationshipName)
	}
	references := make([]Value, 0, len(field.ReferenceTo))
	for _, target := range field.ReferenceTo {
		references = append(references, sObjectTypeToken(target))
	}
	desc.Fields["referenceTo"] = List(references...)
	desc.Fields["picklistValues"] = List()
	desc.Fields["sObjectType"] = sObjectTypeToken(objectName)
	desc.Fields["sortable"] = Bool(storage.FieldFlagValue(field.Sortable, describeFieldSortable(field)))
	desc.Fields["accessible"] = Bool(storage.FieldFlagValue(field.Accessible, true))
	desc.Fields["createable"] = Bool(storage.FieldFlagValue(field.Createable, false))
	desc.Fields["updateable"] = Bool(storage.FieldFlagValue(field.Updateable, false))
	desc.Fields["filterable"] = Bool(storage.FieldFlagValue(field.Filterable, true))
	desc.Fields["groupable"] = Bool(storage.FieldFlagValue(field.Groupable, true))
	desc.Fields["aggregatable"] = Bool(storage.FieldFlagValue(field.Aggregatable, true))
	desc.Fields["permissionable"] = Bool(storage.FieldFlagValue(field.Permissionable, true))
	desc.Fields["deprecatedAndHidden"] = Bool(storage.FieldFlagValue(field.DeprecatedAndHidden, false))
	token := vm.sObjectFieldTokenFromField(objectName, field)
	desc.Fields["sObjectField"] = token
	vm.fieldDescribeCache[cacheKey] = desc
	return desc, nil
}

func syntheticSObjectSystemField(fieldName string) (storage.Field, bool) {
	switch {
	case strings.EqualFold(fieldName, "CreatedDate"):
		return storage.Field{APIName: "CreatedDate", Label: "Created Date", Type: storage.FieldDateTime, DisplayType: "DATETIME"}, true
	case strings.EqualFold(fieldName, "CreatedById"):
		return storage.Field{APIName: "CreatedById", Label: "Created By ID", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy"}, true
	case strings.EqualFold(fieldName, "LastModifiedDate"):
		return storage.Field{APIName: "LastModifiedDate", Label: "Last Modified Date", Type: storage.FieldDateTime, DisplayType: "DATETIME"}, true
	case strings.EqualFold(fieldName, "LastModifiedById"):
		return storage.Field{APIName: "LastModifiedById", Label: "Last Modified By ID", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "LastModifiedBy"}, true
	case strings.EqualFold(fieldName, "SystemModstamp"):
		return storage.Field{APIName: "SystemModstamp", Label: "System Modstamp", Type: storage.FieldDateTime, DisplayType: "DATETIME"}, true
	case strings.EqualFold(fieldName, "OwnerId"):
		return storage.Field{APIName: "OwnerId", Label: "Owner ID", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "Owner"}, true
	case strings.EqualFold(fieldName, "IsDeleted"):
		return storage.Field{APIName: "IsDeleted", Label: "Deleted", Type: storage.FieldBoolean, DisplayType: "BOOLEAN"}, true
	default:
		return storage.Field{}, false
	}
}

func (vm *VM) describeObjectName(name string) string {
	if vm == nil || vm.Org == nil {
		return name
	}
	return storage.NamespaceTokenName(vm.Org.Namespace, name)
}

func (vm *VM) describeFieldName(name string) string {
	if vm == nil || vm.Org == nil || vm.Org.Namespace == "" {
		return name
	}
	if !isCustomSchemaName(name) && !strings.HasSuffix(strings.ToLower(name), "__mdt") {
		return name
	}
	if hasNamespaceTokenInSchemaName(name) {
		return name
	}
	return vm.Org.Namespace + "__" + name
}

func hasNamespaceTokenInSchemaName(name string) bool {
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	return first > 0 && first < last
}

func describeFieldWithSystemOverlay(field storage.Field) storage.Field {
	systemField, ok := syntheticSObjectSystemField(field.APIName)
	if !ok {
		return field
	}
	if field.Label != "" {
		systemField.Label = field.Label
	}
	if field.Length != 0 {
		systemField.Length = field.Length
	}
	if field.Nillable != nil {
		systemField.Nillable = field.Nillable
	}
	if field.DefaultedOnCreate != nil {
		systemField.DefaultedOnCreate = field.DefaultedOnCreate
	}
	if field.Createable != nil {
		systemField.Createable = field.Createable
	}
	if field.Updateable != nil {
		systemField.Updateable = field.Updateable
	}
	if field.Filterable != nil {
		systemField.Filterable = field.Filterable
	}
	if field.Groupable != nil {
		systemField.Groupable = field.Groupable
	}
	if field.Sortable != nil {
		systemField.Sortable = field.Sortable
	}
	if len(field.ReferenceTo) != 0 {
		systemField.ReferenceTo = append([]string(nil), field.ReferenceTo...)
	}
	if field.RelationshipName != "" {
		systemField.RelationshipName = field.RelationshipName
	}
	return systemField
}

func syntheticSchemaField(fieldName string) storage.Field {
	fieldName = strings.TrimSpace(fieldName)
	if dot := strings.LastIndex(fieldName, "."); dot >= 0 && dot+1 < len(fieldName) {
		fieldName = fieldName[dot+1:]
	}
	if fieldName == "" || (!apexIdentifierStartsUpper(fieldName) && !isCustomFieldOrRelationshipType(fieldName)) {
		return storage.Field{}
	}
	field := storage.Field{APIName: fieldName, Label: fieldName, Type: storage.FieldString, DisplayType: "STRING"}
	switch {
	case strings.EqualFold(fieldName, "Id") || strings.HasSuffix(fieldName, "Id"):
		field.Type = storage.FieldReference
		field.DisplayType = "REFERENCE"
	case strings.HasSuffix(fieldName, "Date"):
		field.Type = storage.FieldDate
		field.DisplayType = "DATE"
	case strings.HasSuffix(fieldName, "DateTime") || strings.HasSuffix(fieldName, "Timestamp"):
		field.Type = storage.FieldDateTime
		field.DisplayType = "DATETIME"
	case strings.HasPrefix(fieldName, "Is") || strings.HasPrefix(fieldName, "Has"):
		field.Type = storage.FieldBoolean
		field.DisplayType = "BOOLEAN"
	}
	return field
}

func (vm *VM) syntheticSchemaFieldForObject(objectName, fieldName string) storage.Field {
	field := syntheticSchemaField(fieldName)
	if field.APIName == "" || vm == nil || vm.Org == nil {
		return field
	}
	if !strings.HasSuffix(strings.ToLower(field.APIName), "__c") {
		return field
	}
	target := vm.inferredCustomFieldReferenceTarget(objectName, field.APIName)
	if target == "" {
		return field
	}
	field.Type = storage.FieldReference
	field.DisplayType = "REFERENCE"
	field.ReferenceTo = []string{target}
	field.RelationshipName = lookupFieldRelationshipName(field.APIName)
	return field
}

func emptySObjectFieldDescribe(objectName string) Value {
	desc := Object("Schema.DescribeFieldResult")
	desc.Fields["name"] = String("")
	desc.Fields["sObjectName"] = String(objectName)
	desc.Fields["label"] = String("")
	desc.Fields["localName"] = String("")
	desc.Fields["compoundFieldName"] = Null
	desc.Fields["type"] = String("")
	desc.Fields["soapType"] = schemaSOAPTypeValue("xsd:string")
	desc.Fields["nillable"] = Bool(true)
	desc.Fields["externalId"] = Bool(false)
	desc.Fields["unique"] = Bool(false)
	desc.Fields["encrypted"] = Bool(false)
	desc.Fields["calculated"] = Bool(false)
	desc.Fields["autoNumber"] = Bool(false)
	desc.Fields["nameField"] = Bool(false)
	desc.Fields["custom"] = Bool(false)
	desc.Fields["length"] = Int(0)
	desc.Fields["precision"] = Int(0)
	desc.Fields["scale"] = Int(0)
	desc.Fields["htmlFormatted"] = Bool(false)
	desc.Fields["defaultValue"] = Null
	desc.Fields["defaultValueFormula"] = Null
	desc.Fields["defaultedOnCreate"] = Bool(false)
	desc.Fields["accessible"] = Bool(true)
	desc.Fields["createable"] = Bool(false)
	desc.Fields["updateable"] = Bool(false)
	desc.Fields["filterable"] = Bool(true)
	desc.Fields["groupable"] = Bool(true)
	desc.Fields["sortable"] = Bool(true)
	desc.Fields["aggregatable"] = Bool(true)
	desc.Fields["permissionable"] = Bool(true)
	desc.Fields["deprecatedAndHidden"] = Bool(false)
	desc.Fields["relationshipName"] = Null
	desc.Fields["referenceTo"] = List()
	desc.Fields["picklistValues"] = List()
	return desc
}

type approxVisitKey struct {
	kind ValueKind
	ptr  uintptr
}

func approxValueSize(value Value) int {
	return approxValueSizeSeen(value, make(map[approxVisitKey]bool))
}

func approxValueSizeSeen(value Value, seen map[approxVisitKey]bool) int {
	switch value.Kind {
	case ValueNull:
		return 4
	case ValueInt, ValueDecimal, ValueBool:
		return 8
	case ValueString:
		return len(value.Text)
	case ValueList:
		if len(value.List) > 0 {
			key := approxVisitKey{kind: value.Kind, ptr: reflect.ValueOf(value.List).Pointer()}
			if seen[key] {
				return 0
			}
			seen[key] = true
		}
		total := 24
		for _, item := range value.List {
			total += approxValueSizeSeen(item, seen)
		}
		return total
	case ValueSet:
		if len(value.Set) > 0 {
			key := approxVisitKey{kind: value.Kind, ptr: reflect.ValueOf(value.Set).Pointer()}
			if seen[key] {
				return 0
			}
			seen[key] = true
		}
		total := 24
		for _, item := range value.Set {
			total += approxValueSizeSeen(item, seen)
		}
		return total
	case ValueMap:
		if value.Map != nil {
			key := approxVisitKey{kind: value.Kind, ptr: reflect.ValueOf(value.Map).Pointer()}
			if seen[key] {
				return 0
			}
			seen[key] = true
		}
		total := 24
		for key, item := range value.Map {
			total += len(key) + approxValueSizeSeen(item, seen)
		}
		return total
	case ValueObject:
		if value.Fields != nil {
			key := approxVisitKey{kind: value.Kind, ptr: reflect.ValueOf(value.Fields).Pointer()}
			if seen[key] {
				return 0
			}
			seen[key] = true
		}
		total := 32 + len(value.Type)
		for key, item := range value.Fields {
			total += len(key) + approxValueSizeSeen(item, seen)
		}
		return total
	default:
		return 0
	}
}
