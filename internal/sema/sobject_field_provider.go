package sema

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

// semaSObjectFieldProvider presents one SObject's fields without exposing or
// mutating the maps and slices owned by schema or the standard catalog.
type semaSObjectFieldProvider interface {
	lookup(fieldName string) (schema.Field, bool)
	visit(func(schema.Field))
}

type semaSObjectFieldMapProvider struct {
	fields       map[string]schema.Field
	aliases      map[string]string
	keys         []string
	label        string
	pluralLabel  string
	sharingModel string
}

func newSemaSObjectFieldProvider(namespace string, object schema.Object) semaSObjectFieldProvider {
	provider := &semaSObjectFieldMapProvider{
		fields:  make(map[string]schema.Field, len(object.Fields)+16),
		aliases: make(map[string]string, (len(object.Fields)+16)*3),
	}
	provider.mergeLayer(namespace, object.Name, object.Fields, semaEnrichSObjectProviderField)
	definitionName := object.Name
	isChangeEvent := false
	if baseName, ok := semaChangeEventBaseObjectName(object.Name); ok {
		definitionName = baseName
		isChangeEvent = true
	}
	definition, standardOK := storage.StandardObjectDefinition(definitionName)
	provider.mergeLayer(namespace, object.Name, semaFallbackStandardSObjectFields(object.Name, !standardOK), semaEnrichSObjectProviderField)
	if standardOK {
		provider.label = definition.Label
		provider.pluralLabel = definition.PluralLabel
		provider.sharingModel = definition.SharingModel
		if strings.EqualFold(definitionName, "Account") {
			storage.EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})
		}
		enrich := semaEnrichSObjectProviderField
		if object.Partial {
			enrich = semaEnrichPartialStandardSObjectProviderField
		}
		provider.mergeLayer(namespace, object.Name, semaSchemaFieldsFromStandardDefinition(definition), enrich)
	}
	var compatibilityFields []schema.Field
	if isChangeEvent {
		compatibilityFields = append(compatibilityFields, schema.Field{Name: "ChangeEventHeader", Type: "EventBus.ChangeEventHeader"})
	}
	provider.mergeLayer(namespace, object.Name, compatibilityFields, semaEnrichSObjectProviderField)
	provider.mergeLayer(namespace, object.Name, semaCommonSObjectSchemaFields(object), semaEnrichSObjectProviderField)
	provider.keys = make([]string, 0, len(provider.fields))
	for key := range provider.fields {
		provider.keys = append(provider.keys, key)
	}
	sort.Strings(provider.keys)
	return provider
}

func semaSchemaFieldsFromStandardDefinition(definition storage.ObjectDefinition) []schema.Field {
	object := schemaObjectFromStorageDefinition(definition)
	byKey := make(map[string]storage.Field, len(definition.Fields))
	for name, field := range definition.Fields {
		key := normalizeName(name)
		if field.APIName != "" {
			key = normalizeName(field.APIName)
		}
		byKey[key] = field
	}
	for i := range object.Fields {
		field, ok := byKey[normalizeName(object.Fields[i].Name)]
		if !ok || field.Type != storage.FieldAny || strings.TrimSpace(field.DisplayType) == "" {
			continue
		}
		object.Fields[i].Type = semaSchemaTypeForStorageDisplayType(field.DisplayType)
	}
	return object.Fields
}

func semaSchemaTypeForStorageDisplayType(displayType string) string {
	switch normalizeName(displayType) {
	case "boolean":
		return "Boolean"
	case "byte", "currency", "double", "percent":
		return "Number"
	case "int", "integer":
		return "Integer"
	case "date":
		return "Date"
	case "datetime":
		return "Datetime"
	case "reference":
		return "Lookup"
	case "address":
		return "Address"
	case "base64":
		return "Blob"
	case "email", "encryptedstring", "phone", "picklist", "string", "textarea", "url":
		return "Text"
	default:
		return displayType
	}
}

func (p *semaSObjectFieldMapProvider) mergeLayer(namespace, objectName string, fields []schema.Field, enrich func(schema.Field, schema.Field) schema.Field) {
	for _, source := range fields {
		field := semaCloneSchemaField(source)
		canonical := normalizeName(field.Name)
		if canonical == "" {
			continue
		}
		if existingCanonical, ok := p.aliases[canonical]; ok {
			canonical = existingCanonical
		}
		if existing, exists := p.fields[canonical]; exists {
			field = enrich(existing, field)
		}
		p.fields[canonical] = field
		p.addAlias(canonical, field.Name)
		if local, ok := semaProjectLocalAPIName(namespace, field.Name); ok {
			p.addAlias(canonical, local)
		}
		if namespaced, ok := semaProjectNamespacedAPIName(namespace, field.Name); ok {
			p.addAlias(canonical, namespaced)
		}
		if namespaced, ok := semaOwnerNamespacedAPIName(objectName, field.Name); ok {
			p.addAlias(canonical, namespaced)
		}
		if local := semaSchemaLocalAPIName(field.Name); !strings.EqualFold(local, field.Name) {
			p.addAlias(canonical, local)
		}
	}
}

func (p *semaSObjectFieldMapProvider) addAlias(canonical, alias string) {
	key := normalizeName(alias)
	if key == "" {
		return
	}
	if _, exists := p.aliases[key]; !exists {
		p.aliases[key] = canonical
	}
}

func (p *semaSObjectFieldMapProvider) lookup(fieldName string) (schema.Field, bool) {
	key := normalizeName(fieldName)
	if canonical, ok := p.aliases[key]; ok {
		key = canonical
	}
	field, ok := p.fields[key]
	if !ok {
		return schema.Field{}, false
	}
	return semaCloneSchemaField(field), true
}

func (p *semaSObjectFieldMapProvider) visit(visit func(schema.Field)) {
	if visit == nil {
		return
	}
	for _, key := range p.keys {
		visit(semaCloneSchemaField(p.fields[key]))
	}
}

func semaEnrichSObjectProviderField(existing, incoming schema.Field) schema.Field {
	existing = semaCloneSchemaField(existing)
	incoming = semaCloneSchemaField(incoming)
	existingType := normalizeName(existing.Type)
	if existing.Type == "" || existingType == "any" || existingType == "object" {
		existing.Type = incoming.Type
	}
	if strings.EqualFold(incoming.Name, "PersonDoNotCall") &&
		(strings.EqualFold(incoming.Type, "Boolean") || strings.EqualFold(incoming.Type, "Checkbox")) {
		existing.Type = incoming.Type
	}
	if strings.EqualFold(incoming.Name, "SetupOwnerId") &&
		len(incoming.ReferenceTo) == 1 && strings.EqualFold(incoming.ReferenceTo[0], "Name") &&
		semaReferenceTargetsMatch(existing.ReferenceTo, "Organization", "Profile", "User") {
		// The schema enrichment pass synthesizes this polymorphic target list.
		// Query traversal uses the Salesforce Name pseudo-object for its fields.
		existing.ReferenceTo = append([]string(nil), incoming.ReferenceTo...)
		existing.RelationshipName = incoming.RelationshipName
	}
	return mergeQueryField(existing, incoming)
}

// Partial standard objects include source-inferred fields. Their guessed core
// types and relationship targets are less authoritative than the standard
// catalog. Non-standard extension fields still remain project-owned.
func semaEnrichPartialStandardSObjectProviderField(existing, incoming schema.Field) schema.Field {
	existing = semaEnrichSObjectProviderField(existing, incoming)
	if incoming.Type != "" {
		existing.Type = incoming.Type
	}
	if len(incoming.ReferenceTo) != 0 {
		existing.ReferenceTo = append([]string(nil), incoming.ReferenceTo...)
	}
	if incoming.RelationshipName != "" {
		existing.RelationshipName = incoming.RelationshipName
	}
	if incoming.ChildRelationshipName != "" {
		existing.ChildRelationshipName = incoming.ChildRelationshipName
	}
	return existing
}

func semaReferenceTargetsMatch(actual []string, expected ...string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range expected {
		if !strings.EqualFold(actual[i], expected[i]) {
			return false
		}
	}
	return true
}

func semaCloneSchemaField(field schema.Field) schema.Field {
	field.ReferenceTo = append([]string(nil), field.ReferenceTo...)
	field.SummaryFilterItems = append([]schema.SummaryFilter(nil), field.SummaryFilterItems...)
	field.FilteredLookupInfo.ControllingFields = append([]string(nil), field.FilteredLookupInfo.ControllingFields...)
	field.PicklistValueSettings = append([]schema.PicklistSetting(nil), field.PicklistValueSettings...)
	for i := range field.PicklistValueSettings {
		field.PicklistValueSettings[i].ControllingFieldValues = append([]string(nil), field.PicklistValueSettings[i].ControllingFieldValues...)
	}
	field.PicklistValues = append([]schema.PicklistValue(nil), field.PicklistValues...)
	return field
}

func semaCommonSObjectSchemaFields(object schema.Object) []schema.Field {
	common := []schema.Field{
		{Name: "Id", Type: "Id"},
		{Name: "Name", Type: "Text"},
		{Name: "IsDeleted", Type: "Checkbox"},
		{Name: "CreatedDate", Type: "Datetime"},
		{Name: "CreatedById", Type: "Lookup", ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy"},
		{Name: "LastActivityDate", Type: "Date"},
		{Name: "LastModifiedDate", Type: "Datetime"},
		{Name: "LastModifiedById", Type: "Lookup", ReferenceTo: []string{"User"}, RelationshipName: "LastModifiedBy"},
		{Name: "SystemModstamp", Type: "Datetime"},
	}
	if storage.IsOwnerBackedObject(object.Name) || strings.HasSuffix(strings.ToLower(object.Name), "__c") || semaSchemaHasField(object.Fields, "OwnerId") {
		common = append(common, schema.Field{Name: "OwnerId", Type: "Lookup", ReferenceTo: []string{"Name"}, RelationshipName: "Owner"})
	}
	if strings.HasSuffix(strings.ToLower(object.Name), "__mdt") {
		common = append(common,
			schema.Field{Name: "DeveloperName", Type: "Text"},
			schema.Field{Name: "NamespacePrefix", Type: "Text"},
			schema.Field{Name: "QualifiedAPIName", Type: "Text"},
		)
	}
	if strings.EqualFold(object.CustomSettingsType, "Hierarchy") || semaSchemaHasField(object.Fields, "SetupOwnerId") {
		common = append(common, schema.Field{Name: "SetupOwnerId", Type: "Lookup", ReferenceTo: []string{"Name"}, RelationshipName: "SetupOwner"})
	}
	if semaObjectSupportsRecordTypeRelationship(object) {
		common = append(common, schema.Field{Name: "RecordTypeId", Type: "Lookup", ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"})
	}
	return common
}

func semaSchemaHasField(fields []schema.Field, name string) bool {
	key := normalizeName(name)
	for _, field := range fields {
		if normalizeName(field.Name) == key {
			return true
		}
	}
	return false
}
