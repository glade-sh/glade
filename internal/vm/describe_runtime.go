package vm

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// childRelationshipCache memoizes parent-object -> child relationship describe
// values. The result depends only on the org schema (object definitions and
// namespace), which is immutable across test methods that share a schema, so
// the cache is shared across runtime clones produced by CloneRuntime. The
// RWMutex makes concurrent reuse by parallel test workers safe. Schema-mutating
// tests get a fresh private instance via clearMetadataCaches, preserving
// isolation.
type childRelationshipCache struct {
	mu      sync.RWMutex
	entries map[string][]Value
}

type childRelationshipLookupKey struct {
	ParentObject string
	Relationship string
}

type childRelationshipLookup struct {
	ChildType             string
	CanonicalRelationship string
}

type childRelationshipLookupCache struct {
	mu      sync.RWMutex
	entries map[childRelationshipLookupKey]childRelationshipLookup
}

type loadedChildRelationshipLookupCache struct {
	mu      sync.RWMutex
	entries map[string]loadedChildRelationshipLookup
}

type lazyChildRelationshipLookupCache struct {
	mu      sync.RWMutex
	entries map[string]lazyChildRelationshipLookup
}

func newChildRelationshipCache() *childRelationshipCache {
	return &childRelationshipCache{entries: make(map[string][]Value)}
}

func newChildRelationshipLookupCache() *childRelationshipLookupCache {
	return &childRelationshipLookupCache{entries: make(map[childRelationshipLookupKey]childRelationshipLookup)}
}

func newLoadedChildRelationshipLookupCache() *loadedChildRelationshipLookupCache {
	return &loadedChildRelationshipLookupCache{entries: make(map[string]loadedChildRelationshipLookup)}
}

func newLazyChildRelationshipLookupCache() *lazyChildRelationshipLookupCache {
	return &lazyChildRelationshipLookupCache{entries: make(map[string]lazyChildRelationshipLookup)}
}

func sObjectDescribeOptionsValue(name string) Value {
	return Value{Kind: ValueObject, Type: "Schema.SObjectDescribeOptions", Text: name}
}

// overlaySObjectDescribe returns a request-local describe root. Schema leaf
// values are immutable. Mutable Apex collections remain shared here and are
// copied by privateDescribeCollection at their public getter boundary.
func overlaySObjectDescribe(template Value, nameOverride, optionOverride string) Value {
	out := template
	out.Ref = newValueRef()
	if template.Fields != nil {
		out.Fields = make(map[string]Value, len(template.Fields))
		for name, value := range template.Fields {
			out.Fields[name] = value
		}
	}
	if nameOverride != "" {
		out.Fields["name"] = String(nameOverride)
	}
	if optionOverride != "" {
		out.Fields["sObjectDescribeOption"] = sObjectDescribeOptionsValue(optionOverride)
	}
	return out
}

// privateDescribeCollection copies collection storage without cloning
// immutable schema elements. Schema object members that expose another
// collection call this helper again, so every Apex-mutable branch becomes
// private before it can be changed.
func privateDescribeCollection(value Value) Value {
	switch value.Kind {
	case ValueList:
		out := value
		out.Ref = newValueRef()
		out.List = append([]Value(nil), value.List...)
		return out
	case ValueSet:
		out := value
		out.Ref = newValueRef()
		out.Set = append([]Value(nil), value.Set...)
		return out
	case ValueMap:
		out := value
		out.Ref = newValueRef()
		out.Map = make(map[string]Value, len(value.Map))
		for key, item := range value.Map {
			out.Map[key] = item
		}
		out.MapKeys = make(map[string]Value, len(value.MapKeys))
		for key, item := range value.MapKeys {
			out.MapKeys[key] = item
		}
		out.MapOrder = append([]string(nil), value.MapOrder...)
		return out
	default:
		return value
	}
}

func (c *childRelationshipCache) load(key string) ([]Value, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	return cloneChildRelationshipValues(value), true
}

func (c *childRelationshipCache) store(key string, value []Value) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries[key] = cloneChildRelationshipValues(value)
	c.mu.Unlock()
}

func (c *childRelationshipLookupCache) load(key childRelationshipLookupKey) (childRelationshipLookup, bool) {
	if c == nil {
		return childRelationshipLookup{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.entries[key]
	return value, ok
}

func (c *childRelationshipLookupCache) store(key childRelationshipLookupKey, value childRelationshipLookup) childRelationshipLookup {
	if c == nil {
		return value
	}
	c.mu.Lock()
	c.entries[key] = value
	c.mu.Unlock()
	return value
}

func (c *loadedChildRelationshipLookupCache) load(key string) (loadedChildRelationshipLookup, bool) {
	if c == nil {
		return loadedChildRelationshipLookup{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.entries[key]
	return value, ok
}

func (c *loadedChildRelationshipLookupCache) store(key string, value loadedChildRelationshipLookup) loadedChildRelationshipLookup {
	if c == nil {
		return value
	}
	c.mu.Lock()
	c.entries[key] = value
	c.mu.Unlock()
	return value
}

func (c *lazyChildRelationshipLookupCache) load(key string) (lazyChildRelationshipLookup, bool) {
	if c == nil {
		return lazyChildRelationshipLookup{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.entries[key]
	return value, ok
}

func (c *lazyChildRelationshipLookupCache) store(key string, value lazyChildRelationshipLookup) lazyChildRelationshipLookup {
	if c == nil {
		return value
	}
	c.mu.Lock()
	c.entries[key] = value
	c.mu.Unlock()
	return value
}

func (c *lazyChildRelationshipLookupCache) size() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func cloneChildRelationshipValues(values []Value) []Value {
	if len(values) == 0 {
		return nil
	}
	out := make([]Value, len(values))
	for i, value := range values {
		out[i] = cloneChildRelationshipValue(value)
	}
	return out
}

func cloneChildRelationshipValue(value Value) Value {
	out := value
	if value.Fields == nil {
		return out
	}
	out.Fields = make(map[string]Value, len(value.Fields))
	for name, fieldValue := range value.Fields {
		if fieldValue.Kind == ValueObject && fieldValue.Fields != nil {
			copied := fieldValue
			copied.Fields = make(map[string]Value, len(fieldValue.Fields))
			for nestedName, nestedValue := range fieldValue.Fields {
				copied.Fields[nestedName] = nestedValue
			}
			out.Fields[name] = copied
			continue
		}
		out.Fields[name] = fieldValue
	}
	return out
}

func (vm *VM) describeSObjectValue(name string, definition storage.ObjectDefinition) Value {
	cacheKey := vm.describeSObjectCacheKey(name)
	if cacheKey != "" {
		if cached, ok := vm.describeCache[cacheKey]; ok {
			return cached
		}
	}
	definition = vm.describePreparedDefinition(name, definition)
	desc := Object("Schema.DescribeSObjectResult")
	desc.Fields["name"] = String(vm.describeObjectName(name))
	desc.Fields["localName"] = String(vm.localSchemaName(vm.describeObjectName(name)))
	desc.Fields["label"] = String(definition.Label)
	desc.Fields["labelPlural"] = String(definition.PluralLabel)
	desc.Fields["keyPrefix"] = String(definition.KeyPrefix)
	desc.Fields["sObjectType"] = sObjectTypeToken(name)
	desc.Fields["accessible"] = Bool(vm.currentUserObjectPermission(name, "isAccessible"))
	desc.Fields["createable"] = Bool(vm.currentUserObjectPermission(name, "isCreateable"))
	desc.Fields["updateable"] = Bool(vm.currentUserObjectPermission(name, "isUpdateable"))
	desc.Fields["deletable"] = Bool(vm.currentUserObjectPermission(name, "isDeletable"))
	desc.Fields["queryable"] = Bool(vm.currentUserObjectPermission(name, "isQueryable"))
	desc.Fields["searchable"] = Bool(vm.currentUserObjectPermission(name, "isSearchable"))
	desc.Fields["customSetting"] = Bool(storage.IsCustomSettingDefinition(definition))
	desc.Fields["custom"] = Bool(isCustomObjectLikeName(definition.APIName))
	desc.Fields["feedEnabled"] = Bool(false)
	desc.Fields["mergeable"] = describeMetadataBoolValue(definition.Metadata, "mergeable", false)
	desc.Fields["mruEnabled"] = Bool(true)
	desc.Fields["undeletable"] = Bool(true)
	desc.Fields["deprecatedAndHidden"] = Bool(false)
	desc.Fields["associateEntityType"] = describeMetadataStringValue(definition.Metadata, "associateEntityType")
	desc.Fields["associateParentEntity"] = describeMetadataStringValue(definition.Metadata, "associateParentEntity")
	desc.Fields["dataTranslationEnabled"] = Null
	desc.Fields["defaultImplementation"] = Null
	desc.Fields["hasSubtypes"] = Bool(false)
	desc.Fields["implementedBy"] = Null
	desc.Fields["implementsInterfaces"] = Null
	desc.Fields["isInterface"] = Bool(false)
	desc.Fields["isSubtype"] = Bool(false)
	desc.Fields["sObjectDescribeOption"] = sObjectDescribeOptionsValue("DEFERRED")
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

func (vm *VM) describeChildRelationshipsForDescribe(receiver *Value) (Value, error) {
	if receiver == nil || receiver.Fields == nil {
		return Null, fmt.Errorf("Schema.DescribeSObjectResult token missing object")
	}
	if existing, ok := receiver.Fields["childRelationships"]; ok && existing.Kind == ValueList {
		return existing, nil
	}
	name, ok := receiver.Fields["name"]
	if !ok || name.Kind != ValueString || strings.TrimSpace(name.Text) == "" {
		return Null, fmt.Errorf("Schema.DescribeSObjectResult token missing object")
	}
	childRelationships := List(vm.describeChildRelationships(name.Text)...)
	receiver.Fields["childRelationships"] = childRelationships
	return childRelationships, nil
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
		if prefix := storage.StandardKeyPrefix(prefixName); prefix != "" {
			definition.KeyPrefix = prefix
		} else {
			definition.KeyPrefix = storage.AssignDeterministicPrefixes([]string{prefixName}, nil)[prefixName]
		}
	}
	storage.EnsureStandardObjectFields(&definition)
	storage.RemoveCustomSettingUnsupportedFields(&definition)
	if isCustomSchemaName(definition.APIName) {
		ensureMasterRecordType(&definition)
	}
	vm.applyAccountDescribeDefaults(&definition)
	if vm != nil && cacheKey != "" {
		vm.describeDefCache[cacheKey] = definition
	}
	return definition
}

func ensureMasterRecordType(definition *storage.ObjectDefinition) {
	if definition == nil {
		return
	}
	for _, recordType := range definition.RecordTypes {
		if strings.EqualFold(recordType.DeveloperName, "Master") {
			return
		}
	}
	definition.RecordTypes = append(definition.RecordTypes, storage.RecordTypeInfo{
		DeveloperName: "Master",
		Name:          "Master",
		Active:        true,
		Available:     true,
	})
}

func cloneDescribeObjectDefinition(definition storage.ObjectDefinition) storage.ObjectDefinition {
	out := definition
	if definition.Fields != nil {
		out.Fields = make(map[string]storage.Field, len(definition.Fields))
		for name, field := range definition.Fields {
			copied := field
			copied.SummaryFilterItems = append([]storage.SummaryFilterItem(nil), field.SummaryFilterItems...)
			copied.FilteredLookupInfo = cloneFilteredLookupInfo(field.FilteredLookupInfo)
			copied.ReferenceTo = append([]string(nil), field.ReferenceTo...)
			copied.PicklistValueSettings = clonePicklistSettings(field.PicklistValueSettings)
			copied.PicklistValues = append([]storage.PicklistValue(nil), field.PicklistValues...)
			out.Fields[name] = copied
		}
	}
	if definition.Relations != nil {
		out.Relations = append([]storage.Relationship(nil), definition.Relations...)
		for i := range out.Relations {
			out.Relations[i].ParentObjects = append([]string(nil), definition.Relations[i].ParentObjects...)
			out.Relations[i].JunctionIDListNames = append([]string(nil), definition.Relations[i].JunctionIDListNames...)
			out.Relations[i].JunctionReferenceTo = append([]string(nil), definition.Relations[i].JunctionReferenceTo...)
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

func cloneFilteredLookupInfo(value storage.FilteredLookupInfo) storage.FilteredLookupInfo {
	value.ControllingFields = append([]string(nil), value.ControllingFields...)
	return value
}

func clonePicklistSettings(values []storage.PicklistSetting) []storage.PicklistSetting {
	out := append([]storage.PicklistSetting(nil), values...)
	for i := range out {
		out[i].ControllingFieldValues = append([]string(nil), values[i].ControllingFieldValues...)
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
		vm.childRelCache = newChildRelationshipCache()
	}
	if cached, ok := vm.childRelCache.load(target); ok {
		return cached
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
	vm.childRelCache.store(target, childRelationships)
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
		if namespace := strings.TrimSpace(vm.currentCallerNamespace()); namespace != "" {
			if namespaced := storage.NamespaceTokenName(namespace, objectName); namespaced != objectName {
				return namespaced, storage.ObjectDefinition{
					APIName:     namespaced,
					Label:       namespaced,
					PluralLabel: namespaced,
					Fields:      map[string]storage.Field{},
				}, true
			}
		}
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
			if rawRecordTypeDefaultStorageValue(value, field) {
				return storage.Value{}, false
			}
			return value, true
		}
	}
	if value, ok := storage.DefaultValueForRecordField(definition, stored, field); ok {
		if rawRecordTypeDefaultStorageValue(value, field) {
			return storage.Value{}, false
		}
		return value, true
	}
	value, ok := storage.DefaultValueForField(field)
	if ok && rawRecordTypeDefaultStorageValue(value, field) {
		return storage.Value{}, false
	}
	return value, ok
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

func (vm *VM) localSchemaName(name string) string {
	namespace := ""
	if vm != nil {
		namespace = strings.TrimSpace(vm.currentExecutionNamespace())
		if namespace == "" {
			namespace = strings.TrimSpace(vm.OrgNamespace())
		}
	}
	return storage.StripNamespaceToken(namespace, name)
}

func methodDescribeBoolField(method string) string {
	if strings.HasPrefix(method, "is") && len(method) > 2 {
		name := method[2:]
		return strings.ToLower(name[:1]) + name[1:]
	}
	if strings.HasPrefix(method, "get") && len(method) > 3 {
		name := method[3:]
		return strings.ToLower(name[:1]) + name[1:]
	}
	return method
}

func methodDescribeStringField(method string) string {
	return methodDescribeBoolField(method)
}

func describeMetadataStringValue(metadata map[string]string, key string) Value {
	if value := strings.TrimSpace(metadata[key]); value != "" {
		return String(value)
	}
	return Null
}

func describeMetadataBoolValue(metadata map[string]string, key string, fallback bool) Value {
	value := strings.TrimSpace(metadata[key])
	if value == "" {
		return Bool(fallback)
	}
	if parsed, err := strconv.ParseBool(value); err == nil {
		return Bool(parsed)
	}
	return Bool(fallback)
}

func describeFieldLength(field storage.Field) int {
	if field.AutoNumber {
		return 30
	}
	if field.Length > 0 {
		return field.Length
	}
	if strings.TrimSpace(field.Formula) != "" && strings.EqualFold(field.DisplayType, "STRING") {
		return 1300
	}
	switch strings.ToUpper(strings.TrimSpace(field.DisplayType)) {
	case "EMAIL":
		return 80
	case "URL":
		return 255
	case "TEXTAREA", "LONGTEXTAREA", "RICHTEXTAREA":
		return 255
	}
	switch field.Type {
	case storage.FieldString, storage.FieldPicklist:
		return 255
	case storage.FieldMultiPicklist:
		return 4099
	case storage.FieldReference, storage.FieldID:
		return 18
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

func describeFieldDigits(field storage.Field) int {
	switch field.Type {
	case storage.FieldInteger:
		return describeFieldPrecision(field)
	default:
		return 0
	}
}

func describeFieldByteLength(field storage.Field) int {
	length := describeFieldLength(field)
	if length == 0 {
		return 0
	}
	switch field.Type {
	case storage.FieldReference, storage.FieldID, storage.FieldMultiPicklist:
		return length
	default:
		return length * 3
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

func describeFieldNillable(field storage.Field) bool {
	return !field.Required && !field.AutoNumber && field.Type != storage.FieldBoolean
}

func describeFieldDefaultedOnCreate(field storage.Field) bool {
	return field.AutoNumber || field.DefaultValue != ""
}

func describeFieldIsLongText(field storage.Field) bool {
	displayType := strings.ToUpper(strings.TrimSpace(field.DisplayType))
	return (displayType == "TEXTAREA" || displayType == "LONGTEXTAREA" || displayType == "RICHTEXTAREA") && describeFieldLength(field) > 255
}

func describeFieldFilterable(field storage.Field) bool {
	return !describeFieldIsLongText(field)
}

func describeFieldGroupable(field storage.Field) bool {
	if field.AutoNumber || describeFieldCalculated(field) || describeFieldIsLongText(field) {
		return false
	}
	switch field.Type {
	case storage.FieldDecimal, storage.FieldDateTime, storage.FieldMultiPicklist:
		return false
	default:
		return true
	}
}

func describeFieldAggregatable(field storage.Field) bool {
	if field.Type == storage.FieldBoolean || field.Type == storage.FieldMultiPicklist || describeFieldIsLongText(field) {
		return false
	}
	return true
}

func describeFieldSortable(field storage.Field) bool {
	displayType := field.DisplayType
	if displayType == "" {
		displayType = string(field.Type)
	}
	switch strings.ToUpper(displayType) {
	case "MULTIPICKLIST", "ENCRYPTEDSTRING", "BASE64", "BLOB", "ADDRESS", "LOCATION":
		return false
	case "TEXTAREA", "LONGTEXTAREA", "RICHTEXTAREA":
		return describeFieldLength(field) <= 255
	default:
		return true
	}
}

func filteredLookupInfoValue(definition storage.ObjectDefinition, info storage.FilteredLookupInfo) Value {
	controllingFields := make([]string, 0, len(info.ControllingFields))
	for _, field := range info.ControllingFields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, ok := storage.ResolveFieldName(definition, "", field); ok {
			controllingFields = append(controllingFields, field)
		}
	}
	if len(controllingFields) == 0 && !info.OptionalFilter {
		return Null
	}
	value := Object("Schema.FilteredLookupInfo")
	fields := typedList("List<String>")
	fields.List = make([]Value, 0, len(controllingFields))
	for _, field := range controllingFields {
		fields.List = append(fields.List, String(field))
	}
	value.Fields["controllingFields"] = fields
	value.Fields["dependent"] = Bool(info.Dependent && len(controllingFields) > 0)
	value.Fields["optionalFilter"] = Bool(info.OptionalFilter)
	return value
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
	objectName = strings.TrimSpace(objectName)
	if objectName == "" {
		return false
	}
	objectHasNamespaceToken := strings.Contains(objectName, "__")
	var objectAnyNamespace string
	var objectStandardNamespace string
	for _, parent := range relationship.ParentObjects {
		parent = strings.TrimSpace(parent)
		if parent == "" {
			continue
		}
		if strings.EqualFold(parent, objectName) {
			return true
		}
		if !objectHasNamespaceToken && !strings.Contains(parent, "__") {
			continue
		}
		if objectAnyNamespace == "" {
			objectAnyNamespace = stripAnyNamespaceToken(objectName)
		}
		if strings.EqualFold(stripAnyNamespaceToken(parent), objectAnyNamespace) {
			return true
		}
		if objectStandardNamespace == "" {
			objectStandardNamespace = stripStandardObjectNamespaceToken(objectName)
		}
		if strings.EqualFold(stripStandardObjectNamespaceToken(parent), objectStandardNamespace) {
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
	value.Fields["deprecatedAndHidden"] = Bool(relationship.DeprecatedAndHidden)
	junctionNames := make([]Value, 0, len(relationship.JunctionIDListNames))
	for _, name := range relationship.JunctionIDListNames {
		if strings.TrimSpace(name) != "" {
			junctionNames = append(junctionNames, String(name))
		}
	}
	junctionNameList := typedList("List<String>")
	junctionNameList.List = junctionNames
	value.Fields["junctionIdListNames"] = junctionNameList
	junctionTargets := make([]Value, 0, len(relationship.JunctionReferenceTo))
	for _, target := range relationship.JunctionReferenceTo {
		if strings.TrimSpace(target) != "" {
			junctionTargets = append(junctionTargets, sObjectTypeToken(target))
		}
	}
	junctionTargetList := typedList("List<Schema.SObjectType>")
	junctionTargetList.List = junctionTargets
	value.Fields["junctionReferenceTo"] = junctionTargetList
	return value
}

func describeChildRelationshipName(namespace, childObject, relationshipName string) string {
	name := strings.TrimSpace(relationshipName)
	if name == "" {
		return name
	}
	if isCustomObjectLikeName(storage.StripNamespaceToken(namespace, childObject)) && !hasSuffixFold(name, "__r") {
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
	m.Type = "Map<String,Schema.FieldSet>"
	m.Runtime = fieldSetMapRuntime
	if vm.Org == nil {
		fieldSets.Fields["map"] = m
		return fieldSets
	}
	for _, fieldSet := range vm.Org.Metadata.FieldSets {
		namespace := fieldSetNamespace(vm.Org.Namespace, fieldSet)
		if !metadataObjectNameMatches(namespace, objectName, fieldSet.ObjectName) {
			continue
		}
		value := vm.fieldSetValue(objectName, definition, fieldSet)
		for _, alias := range fieldSetMapAliases(namespace, fieldSet.Name) {
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
	if namespace != "" && !hasPrefixFold(name, namespace+"__") {
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
	if vm != nil && vm.Org != nil {
		defaultNamespace = vm.Org.Namespace
	}
	namespace, localName := fieldSetNamespaceAndLocalName(fieldSetNamespace(defaultNamespace, fieldSet), fieldSet.Name)
	value.Fields["name"] = String(localName)
	if namespace == "" {
		value.Fields["namespace"] = Null
	} else {
		value.Fields["namespace"] = String(namespace)
	}
	if description := strings.TrimSpace(fieldSet.Description); description == "" {
		value.Fields["description"] = Null
	} else {
		value.Fields["description"] = String(description)
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

func fieldSetNamespace(defaultNamespace string, fieldSet storage.FieldSetMetadata) string {
	if namespace := strings.TrimSpace(fieldSet.Namespace); namespace != "" {
		return namespace
	}
	return strings.TrimSpace(defaultNamespace)
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
		if fieldSetMemberDBRequired(objectName, field.APIName) {
			value.Fields["dbRequired"] = Bool(true)
		}
		value.Fields["sObjectField"] = vm.sObjectFieldTokenFromField(objectName, field)
	} else if relatedObjectName, field, ok := vm.resolveRelationshipFieldPath(definition, member.Field); ok {
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
		if fieldSetMemberDBRequired(relatedObjectName, field.APIName) {
			value.Fields["dbRequired"] = Bool(true)
		}
		value.Fields["sObjectField"] = vm.sObjectFieldTokenFromField(relatedObjectName, field)
	} else {
		value.Fields["sObjectField"] = Value{Kind: ValueNull, Type: "Schema.SObjectField"}
	}
	value.Fields["label"] = String(label)
	value.Fields["type"] = displayType
	return value
}

func (vm *VM) resolveRelationshipFieldPath(definition storage.ObjectDefinition, fieldPath string) (string, storage.Field, bool) {
	parts := strings.Split(strings.TrimSpace(fieldPath), ".")
	if len(parts) < 2 {
		return "", storage.Field{}, false
	}
	namespace := ""
	if vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	currentDefinition := definition
	currentObjectName := definition.APIName
	for i, part := range parts[:len(parts)-1] {
		fieldName, ok := resolveRelationshipFieldName(currentDefinition, namespace, part)
		if !ok {
			return "", storage.Field{}, false
		}
		field := currentDefinition.Fields[fieldName]
		if len(field.ReferenceTo) == 0 {
			return "", storage.Field{}, false
		}
		currentObjectName = field.ReferenceTo[0]
		resolvedObjectName, resolvedDefinition, ok := vm.describeObjectDefinition(currentObjectName)
		if !ok {
			return "", storage.Field{}, false
		}
		currentObjectName = resolvedObjectName
		currentDefinition = resolvedDefinition
		if i == len(parts)-2 {
			break
		}
	}
	leafName, ok := storage.ResolveFieldName(currentDefinition, namespace, parts[len(parts)-1])
	if !ok {
		return "", storage.Field{}, false
	}
	return currentObjectName, currentDefinition.Fields[leafName], true
}

func resolveRelationshipFieldName(definition storage.ObjectDefinition, namespace, relationshipName string) (string, bool) {
	for fieldName, field := range definition.Fields {
		if field.RelationshipName == "" {
			continue
		}
		if metadataObjectNameMatches(namespace, field.RelationshipName, relationshipName) {
			return fieldName, true
		}
	}
	return "", false
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
	strippedCanonical := storage.StripNamespaceToken(namespace, canonical)
	strippedCandidate := storage.StripNamespaceToken(namespace, candidate)
	return strings.EqualFold(strippedCanonical, strippedCandidate)
}

func recordTypeInfoValue(recordType storage.RecordTypeInfo) Value {
	value := Object("Schema.RecordTypeInfo")
	value.Fields["recordTypeId"] = platformScalar("Id", recordType.ID.String())
	value.Fields["developerName"] = String(recordType.DeveloperName)
	value.Fields["name"] = String(recordTypeName(recordType))
	value.Fields["active"] = Bool(recordType.Active)
	value.Fields["available"] = Bool(recordType.Available || recordType.Active)
	value.Fields["default"] = Bool(recordType.Default)
	value.Fields["defaultRecordTypeMapping"] = Bool(recordType.Default)
	value.Fields["master"] = Bool(strings.EqualFold(recordType.DeveloperName, "Master"))
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
	cacheKey := vm.fieldDescribeCacheKey(objectName, fieldName)
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
	if strings.TrimSpace(field.InlineHelpText) == "" {
		desc.Fields["inlineHelpText"] = Null
	} else {
		desc.Fields["inlineHelpText"] = String(field.InlineHelpText)
	}
	desc.Fields["compoundFieldName"] = describeCompoundFieldNameValue(field)
	desc.Fields["localName"] = String(vm.localSchemaName(describeName))
	displayType := field.DisplayType
	if displayType == "" {
		displayType = string(field.Type)
	}
	desc.Fields["type"] = schemaDisplayTypeValue(displayType)
	desc.Fields["soapType"] = schemaSOAPTypeValue(soapTypeForStorageField(field))
	desc.Fields["nillable"] = Bool(storage.FieldFlagValue(field.Nillable, describeFieldNillable(field)))
	desc.Fields["externalId"] = Bool(field.ExternalID)
	desc.Fields["unique"] = Bool(field.Unique)
	desc.Fields["encrypted"] = Bool(field.Encrypted)
	calculated := describeFieldCalculated(field)
	desc.Fields["calculated"] = Bool(calculated)
	if strings.TrimSpace(field.Formula) == "" {
		desc.Fields["calculatedFormula"] = Null
	} else {
		desc.Fields["calculatedFormula"] = String(field.Formula)
	}
	desc.Fields["autoNumber"] = Bool(field.AutoNumber)
	desc.Fields["caseSensitive"] = Bool(field.CaseSensitive)
	desc.Fields["nameField"] = Bool(isNameFieldDescribe(field))
	desc.Fields["custom"] = Bool(isCustomSchemaName(field.APIName))
	desc.Fields["length"] = Int(int64(describeFieldLength(field)))
	desc.Fields["precision"] = Int(int64(describeFieldPrecision(field)))
	desc.Fields["scale"] = Int(int64(describeFieldScale(field)))
	desc.Fields["digits"] = Int(int64(describeFieldDigits(field)))
	desc.Fields["byteLength"] = Int(int64(describeFieldByteLength(field)))
	desc.Fields["htmlFormatted"] = Bool(describeFieldIsHTMLFormatted(field))
	desc.Fields["dataTranslationEnabled"] = Bool(false)
	desc.Fields["filteredLookupInfo"] = filteredLookupInfoValue(definition, field.FilteredLookupInfo)
	if defaultValue, ok := storage.DefaultValueForField(field); ok {
		desc.Fields["defaultValue"] = vmValueFromStorage(defaultValue)
		desc.Fields["defaultedOnCreate"] = Bool(storage.FieldFlagValue(field.DefaultedOnCreate, describeFieldDefaultedOnCreate(field)))
	} else {
		desc.Fields["defaultValue"] = Null
		desc.Fields["defaultedOnCreate"] = Bool(storage.FieldFlagValue(field.DefaultedOnCreate, field.AutoNumber))
	}
	if strings.TrimSpace(field.DefaultValue) == "" || field.Type == storage.FieldBoolean {
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
	desc.Fields["referenceTargetField"] = Null
	if field.RelationshipOrder == nil {
		desc.Fields["relationshipOrder"] = Null
	} else {
		desc.Fields["relationshipOrder"] = Int(int64(*field.RelationshipOrder))
	}
	referenceTo := describeFieldReferenceTargets(definition, field)
	references := make([]Value, 0, len(referenceTo))
	for _, target := range referenceTo {
		references = append(references, sObjectTypeToken(target))
	}
	desc.Fields["referenceTo"] = List(references...)
	desc.Fields["accessible"] = Bool(storage.FieldFlagValue(field.Accessible, true) && vm.currentUserFieldPermission(objectName, field.APIName, "isAccessible"))
	desc.Fields["createable"] = Bool(storage.FieldFlagValue(field.Createable, !calculated && !field.AutoNumber) && vm.currentUserFieldPermission(objectName, field.APIName, "isCreateable"))
	desc.Fields["updateable"] = Bool(storage.FieldFlagValue(field.Updateable, !calculated && !field.AutoNumber) && vm.currentUserFieldPermission(objectName, field.APIName, "isUpdateable"))
	desc.Fields["filterable"] = Bool(storage.FieldFlagValue(field.Filterable, describeFieldFilterable(field)))
	desc.Fields["groupable"] = Bool(storage.FieldFlagValue(field.Groupable, describeFieldGroupable(field)))
	desc.Fields["sortable"] = Bool(storage.FieldFlagValue(field.Sortable, describeFieldSortable(field)))
	desc.Fields["aggregatable"] = Bool(storage.FieldFlagValue(field.Aggregatable, describeFieldAggregatable(field)))
	desc.Fields["permissionable"] = Bool(storage.FieldFlagValue(field.Permissionable, true))
	desc.Fields["deprecatedAndHidden"] = Bool(storage.FieldFlagValue(field.DeprecatedAndHidden, false))
	desc.Fields["restrictedPicklist"] = Bool(field.RestrictedPicklist)
	desc.Fields["idLookup"] = Bool(field.IDLookup || field.ExternalID)
	desc.Fields["namePointing"] = Bool(field.NamePointing || len(field.ReferenceTo) > 1)
	controllerField, hasController := describeFieldController(definition, field)
	if hasController {
		desc.Fields["controller"] = vm.sObjectFieldTokenFromField(objectName, controllerField)
		desc.Fields["dependentPicklist"] = Bool(true)
	} else {
		desc.Fields["controller"] = Null
		desc.Fields["dependentPicklist"] = Bool(false)
	}
	desc.Fields["controllerValues"] = describeFieldControllerValues(definition, controllerField, hasController)
	vm.completeDescribeFieldResult(&desc, objectName, field)
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
		entry.Fields["defaultValue"] = Bool(value.Default)
		entry.Fields["active"] = Bool(value.Active)
		picklistValues = append(picklistValues, entry)
	}
	desc.Fields["picklistValues"] = List(picklistValues...)
	vm.fieldDescribeCache[cacheKey] = desc
	vm.storeFieldDescribeCacheAliases(objectName, requestedFieldName, fieldName, desc)
	return desc, nil
}

func describeFieldController(definition storage.ObjectDefinition, field storage.Field) (storage.Field, bool) {
	controllerName := strings.TrimSpace(field.PicklistController)
	if controllerName == "" {
		return storage.Field{}, false
	}
	resolved, ok := storage.ResolveFieldName(definition, "", controllerName)
	if !ok {
		return storage.Field{APIName: controllerName}, true
	}
	controller := definition.Fields[resolved]
	if strings.TrimSpace(controller.APIName) == "" {
		controller.APIName = resolved
	}
	return controller, true
}

func describeFieldControllerValues(definition storage.ObjectDefinition, controller storage.Field, hasController bool) Value {
	out := typedMap("Map<String,Integer>")
	out.Fields = map[string]Value{"__caseInsensitiveStringKeys": Bool(true)}
	if !hasController {
		return out
	}
	values := controller.PicklistValues
	if len(values) == 0 {
		if resolved, ok := storage.ResolveFieldName(definition, "", controller.APIName); ok {
			values = definition.Fields[resolved].PicklistValues
		}
	}
	for i, value := range values {
		name := strings.TrimSpace(value.Value)
		if name == "" {
			continue
		}
		keyValue := String(name)
		key := mapKey(keyValue)
		out.Map[key] = Int(int64(i))
		out.MapKeys[key] = keyValue
	}
	return out
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
		key := vm.describePermissionCacheKey(base + alias)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func (vm *VM) describeSObjectCacheKey(objectName string) string {
	return vm.describePermissionCacheKey(strings.ToLower(strings.TrimSpace(objectName)))
}

func (vm *VM) fieldDescribeCacheKey(objectName, fieldName string) string {
	return vm.describePermissionCacheKey(strings.ToLower(strings.TrimSpace(objectName)) + "." + strings.ToLower(strings.TrimSpace(fieldName)))
}

func (vm *VM) describePermissionCacheKey(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	if vm == nil || vm.Org == nil {
		return base
	}
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	userID := strings.TrimSpace(stringField(user, "Id"))
	profileID := strings.ToLower(strings.TrimSpace(stringField(user, "ProfileId")))
	permissionSetIDs := vm.assignedPermissionSetIDs(userID)
	sort.Strings(permissionSetIDs)
	for i := range permissionSetIDs {
		permissionSetIDs[i] = strings.ToLower(strings.TrimSpace(permissionSetIDs[i]))
	}
	return base + "|profile=" + profileID + "|user=" + strings.ToLower(userID) + "|permissions=" + strings.Join(permissionSetIDs, ",")
}

func describeFieldReferenceTargets(definition storage.ObjectDefinition, field storage.Field) []string {
	referenceTo := append([]string(nil), field.ReferenceTo...)
	if field.Type == storage.FieldReference &&
		hasSuffixFold(definition.APIName, "__c") &&
		describeProviderIDAllowsUserTarget(definition, field) {
		referenceTo = appendUniqueStringFold(referenceTo, "User")
	}
	return referenceTo
}

func describeProviderIDAllowsUserTarget(definition storage.ObjectDefinition, field storage.Field) bool {
	if describeProviderIDStem(field.APIName) != "" {
		return true
	}
	fieldStem := normalizedCustomReferenceStem(field.APIName)
	if fieldStem == "" {
		return false
	}
	for name, candidate := range definition.Fields {
		candidateName := candidate.APIName
		if strings.TrimSpace(candidateName) == "" {
			candidateName = name
		}
		candidateStem := describeProviderIDStem(candidateName)
		if candidateStem != "" && candidateStem == fieldStem+"id" {
			return true
		}
	}
	return false
}

func describeProviderIDStem(name string) string {
	stem := normalizedCustomReferenceStem(name)
	if strings.Contains(stem, "providerid") {
		return stem
	}
	return ""
}

func normalizedCustomReferenceStem(name string) string {
	base := storage.StripAnyNamespaceToken(strings.TrimSpace(name))
	base = strings.TrimSuffix(base, "__c")
	base = strings.TrimSuffix(base, "__C")
	base = strings.ToLower(base)
	base = strings.ReplaceAll(base, "_", "")
	return base
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
	cacheKey := vm.fieldDescribeCacheKey(objectName, fieldName)
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
	desc.Fields["accessible"] = Bool(storage.FieldFlagValue(field.Accessible, true) && vm.currentUserFieldPermission(objectName, field.APIName, "isAccessible"))
	desc.Fields["createable"] = Bool(storage.FieldFlagValue(field.Createable, false) && vm.currentUserFieldPermission(objectName, field.APIName, "isCreateable"))
	desc.Fields["updateable"] = Bool(storage.FieldFlagValue(field.Updateable, false) && vm.currentUserFieldPermission(objectName, field.APIName, "isUpdateable"))
	desc.Fields["filterable"] = Bool(storage.FieldFlagValue(field.Filterable, true))
	desc.Fields["groupable"] = Bool(storage.FieldFlagValue(field.Groupable, true))
	desc.Fields["aggregatable"] = Bool(storage.FieldFlagValue(field.Aggregatable, true))
	desc.Fields["permissionable"] = Bool(storage.FieldFlagValue(field.Permissionable, true))
	desc.Fields["deprecatedAndHidden"] = Bool(storage.FieldFlagValue(field.DeprecatedAndHidden, false))
	token := vm.sObjectFieldTokenFromField(objectName, field)
	desc.Fields["sObjectField"] = token
	vm.completeDescribeFieldResult(&desc, objectName, field)
	vm.fieldDescribeCache[cacheKey] = desc
	return desc, nil
}

func syntheticSObjectSystemField(fieldName string) (storage.Field, bool) {
	switch {
	case strings.EqualFold(fieldName, "Id"):
		return storage.Field{APIName: "Id", Label: "ID", Type: storage.FieldID, DisplayType: "ID"}, true
	case strings.EqualFold(fieldName, "CreatedDate"):
		return storage.Field{APIName: "CreatedDate", Label: "Created Date", Type: storage.FieldDateTime, DisplayType: "DATETIME"}, true
	case strings.EqualFold(fieldName, "CreatedById"):
		return storage.Field{APIName: "CreatedById", Label: "Created By ID", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy"}, true
	case strings.EqualFold(fieldName, "LastModifiedDate"):
		return storage.Field{APIName: "LastModifiedDate", Label: "Last Modified Date", Type: storage.FieldDateTime, DisplayType: "DATETIME"}, true
	case strings.EqualFold(fieldName, "LastModifiedById"):
		return storage.Field{APIName: "LastModifiedById", Label: "Last Modified By ID", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "LastModifiedBy"}, true
	case strings.EqualFold(fieldName, "LastActivityDate"):
		return storage.Field{APIName: "LastActivityDate", Label: "Last Activity", Type: storage.FieldDate, DisplayType: "DATE", Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false), Permissionable: storage.BoolFlag(false)}, true
	case strings.EqualFold(fieldName, "SystemModstamp"):
		return storage.Field{APIName: "SystemModstamp", Label: "System Modstamp", Type: storage.FieldDateTime, DisplayType: "DATETIME"}, true
	case strings.EqualFold(fieldName, "OwnerId"):
		return storage.Field{APIName: "OwnerId", Label: "Owner ID", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "Owner"}, true
	case strings.EqualFold(fieldName, "RecordTypeId"):
		return storage.Field{APIName: "RecordTypeId", Label: "Record Type ID", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"}, true
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
	if namespace := strings.TrimSpace(vm.currentCallerNamespace()); namespace != "" {
		if namespaced := storage.NamespaceTokenName(namespace, name); namespaced != name {
			return namespaced
		}
	}
	return storage.NamespaceTokenName(vm.Org.Namespace, name)
}

func (vm *VM) describeFieldName(name string) string {
	if vm == nil {
		return name
	}
	if !isCustomSchemaName(name) && !hasSuffixFold(name, "__mdt") {
		return name
	}
	if hasNamespaceTokenInSchemaName(name) {
		return name
	}
	if namespace := strings.TrimSpace(vm.currentCallerNamespace()); namespace != "" {
		return namespace + "__" + name
	}
	if vm.Org == nil || vm.Org.Namespace == "" {
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
	return storage.Field{APIName: fieldName, Label: fieldName, Type: storage.FieldAny}
}

func (vm *VM) syntheticSchemaFieldForObject(objectName, fieldName string) storage.Field {
	field := syntheticSchemaField(fieldName)
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
	completeDescribeFieldResultDefaults(&desc)
	return desc
}

func (vm *VM) completeDescribeFieldResult(desc *Value, objectName string, field storage.Field) {
	if desc == nil {
		return
	}
	if desc.Fields == nil {
		desc.Fields = map[string]Value{}
	}
	if _, ok := desc.Fields["sObjectType"]; !ok {
		desc.Fields["sObjectType"] = sObjectTypeToken(objectName)
	}
	if _, ok := desc.Fields["sObjectField"]; !ok {
		desc.Fields["sObjectField"] = vm.sObjectFieldTokenFromField(objectName, field)
	}
	completeDescribeFieldResultDefaults(desc)
}

func completeDescribeFieldResultDefaults(desc *Value) {
	if desc == nil {
		return
	}
	if desc.Fields == nil {
		desc.Fields = map[string]Value{}
	}
	defaults := map[string]Value{
		"aiPredictionField":            Bool(false),
		"cascadeDelete":                Bool(false),
		"controller":                   Null,
		"controllerValues":             typedMap("Map<String,Integer>"),
		"dataTranslationEnabled":       Bool(false),
		"defaultedOnCreate":            Bool(false),
		"defaultValue":                 Null,
		"defaultValueFormula":          Null,
		"dependentPicklist":            Bool(false),
		"displayLocationInDecimal":     Bool(false),
		"filteredLookupInfo":           Null,
		"formulaTreatNullNumberAsZero": Bool(false),
		"highScaleNumber":              Bool(false),
		"inlineHelpText":               String(""),
		"localName":                    Null,
		"mask":                         Null,
		"maskType":                     Null,
		"queryByDistance":              Bool(false),
		"referenceTargetField":         Null,
		"relationshipOrder":            Int(0),
		"restrictedDelete":             Bool(false),
		"searchPrefilterable":          Bool(false),
		"writeRequiresMasterRead":      Bool(false),
	}
	for field, value := range defaults {
		if _, ok := desc.Fields[field]; !ok {
			desc.Fields[field] = value
		}
	}
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
