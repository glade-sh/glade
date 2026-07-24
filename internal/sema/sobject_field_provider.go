package sema

import (
	"sort"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

// semaSObjectFieldProvider presents one SObject's fields without exposing or
// mutating the maps and slices owned by schema or the standard catalog.
type semaSObjectFieldProvider interface {
	lookup(fieldName string) (schema.Field, bool)
	visit(func(schema.Field))
	visitDeclared(func(schema.Field))
	hasFields() bool
}

type semaNormalizedSObjectFieldProvider interface {
	lookupNormalized(fieldName, normalizedName string) (schema.Field, bool)
}

func semaLookupSObjectFieldNormalized(provider semaSObjectFieldProvider, fieldName, normalizedName string) (schema.Field, bool) {
	if normalized, ok := provider.(semaNormalizedSObjectFieldProvider); ok {
		return normalized.lookupNormalized(fieldName, normalizedName)
	}
	return provider.lookup(fieldName)
}

type semaSObjectFieldMapProvider struct {
	fields  map[string]schema.Field
	aliases map[string]string
	keys    []string
}

func newSemaSObjectFieldProvider(namespace string, object schema.Object) semaSObjectFieldProvider {
	definitionName := object.Name
	isChangeEvent := false
	if baseName, ok := semaChangeEventBaseObjectName(object.Name); ok {
		definitionName = baseName
		isChangeEvent = true
	}
	var compatibilityFields []schema.Field
	if isChangeEvent {
		compatibilityFields = append(compatibilityFields, schema.Field{Name: "ChangeEventHeader", Type: "EventBus.ChangeEventHeader"})
	}
	return &semaLayeredSObjectFieldProvider{
		partialStandard: object.Partial,
		declaredLayers:  1,
		layers: []semaSObjectFieldProvider{
			newSemaSObjectFieldMapProvider(namespace, object.Name, object.Fields),
			newSemaSObjectFieldMapProvider(namespace, object.Name, semaFallbackStandardSObjectFields(object.Name, !storage.IsKnownStandardObject(definitionName))),
			semaStandardSObjectFieldProviderFor(definitionName),
			newSemaSObjectFieldMapProvider(namespace, object.Name, compatibilityFields),
			newSemaSObjectFieldMapProvider(namespace, object.Name, semaCommonSObjectSchemaFields(object)),
		},
	}
}

func newSemaSObjectFieldMapProvider(namespace, objectName string, fields []schema.Field) *semaSObjectFieldMapProvider {
	provider := &semaSObjectFieldMapProvider{
		fields:  make(map[string]schema.Field, len(fields)),
		aliases: make(map[string]string, len(fields)*3),
	}
	provider.mergeLayer(namespace, objectName, fields, semaEnrichSObjectProviderField)
	provider.keys = make([]string, 0, len(provider.fields))
	for key := range provider.fields {
		provider.keys = append(provider.keys, key)
	}
	sort.Strings(provider.keys)
	return provider
}

type semaLayeredSObjectFieldProvider struct {
	layers          []semaSObjectFieldProvider
	partialStandard bool
	declaredLayers  int
	lookups         sync.Map
}

type semaSObjectFieldLookup struct {
	field schema.Field
	ok    bool
}

func (p *semaLayeredSObjectFieldProvider) lookup(fieldName string) (schema.Field, bool) {
	return p.lookupNormalized(fieldName, normalizeName(fieldName))
}

func (p *semaLayeredSObjectFieldProvider) lookupNormalized(fieldName, normalizedName string) (schema.Field, bool) {
	if p == nil {
		return schema.Field{}, false
	}
	if cached, ok := p.lookups.Load(normalizedName); ok {
		result := cached.(semaSObjectFieldLookup)
		return semaCloneSchemaField(result.field), result.ok
	}
	var field schema.Field
	found := false
	for i, layer := range p.layers {
		if layer == nil {
			continue
		}
		if standard, ok := layer.(*semaStandardSObjectFieldProvider); ok &&
			p.canSkipStandardLookup(standard, fieldName, normalizedName, field, found) {
			continue
		}
		incoming, ok := semaLookupSObjectFieldNormalized(layer, fieldName, normalizedName)
		if !ok {
			continue
		}
		if !found {
			field = incoming
			found = true
			continue
		}
		enrich := semaEnrichSObjectProviderField
		if p.partialStandard && i == 2 {
			enrich = semaEnrichPartialStandardSObjectProviderField
		}
		field = enrich(field, incoming)
	}
	if !found {
		p.lookups.LoadOrStore(normalizedName, semaSObjectFieldLookup{})
		return schema.Field{}, false
	}
	actual, _ := p.lookups.LoadOrStore(normalizedName, semaSObjectFieldLookup{field: field, ok: true})
	result := actual.(semaSObjectFieldLookup)
	return semaCloneSchemaField(result.field), result.ok
}

func (p *semaLayeredSObjectFieldProvider) canSkipStandardLookup(standard *semaStandardSObjectFieldProvider, fieldName, normalizedName string, field schema.Field, found bool) bool {
	if p == nil || standard == nil || p.partialStandard {
		return false
	}
	if semaIsCustomAPIName(fieldName) && !semaIsCustomAPIName(standard.objectName) {
		// The canonical standard-object catalog cannot supply an org-owned custom
		// field. A declared project field was already checked in an earlier layer.
		return true
	}
	if !found || (normalizedName != normalizeName("Id") && normalizedName != normalizeName("Name")) || len(p.layers) == 0 {
		return false
	}
	if field.Type == "" || strings.EqualFold(field.Type, "Any") || strings.EqualFold(field.Type, "Object") {
		return false
	}
	// Id and Name may bypass the full standard decode only when the concrete
	// common layer for this object actually contains the requested field.
	_, ok := semaLookupSObjectFieldNormalized(p.layers[len(p.layers)-1], fieldName, normalizedName)
	return ok
}

func (p *semaLayeredSObjectFieldProvider) visit(visit func(schema.Field)) {
	if p == nil || visit == nil {
		return
	}
	keys := make(map[string]string)
	for _, layer := range p.layers {
		if layer == nil {
			continue
		}
		layer.visit(func(field schema.Field) {
			key := normalizeName(field.Name)
			if key != "" {
				keys[key] = field.Name
			}
		})
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	for _, key := range ordered {
		if field, ok := p.lookup(keys[key]); ok {
			visit(field)
		}
	}
}

func (p *semaLayeredSObjectFieldProvider) hasFields() bool {
	if p == nil {
		return false
	}
	for _, layer := range p.layers {
		if layer != nil && layer.hasFields() {
			return true
		}
	}
	return false
}

func (p *semaLayeredSObjectFieldProvider) visitDeclared(visit func(schema.Field)) {
	if p == nil || visit == nil {
		return
	}
	limit := p.declaredLayers
	if limit > len(p.layers) {
		limit = len(p.layers)
	}
	for _, layer := range p.layers[:limit] {
		if layer != nil {
			layer.visitDeclared(visit)
		}
	}
}

type semaStandardSObjectFieldProvider struct {
	objectName string
	once       sync.Once
	fields     *semaSObjectFieldMapProvider
}

var semaStandardSObjectFieldProviders sync.Map

func semaStandardSObjectFieldProviderFor(objectName string) semaSObjectFieldProvider {
	canonical, ok := storage.ResolveKnownStandardObjectName(objectName)
	if !ok {
		return nil
	}
	key := normalizeName(canonical)
	if cached, ok := semaStandardSObjectFieldProviders.Load(key); ok {
		return cached.(*semaStandardSObjectFieldProvider)
	}
	provider := &semaStandardSObjectFieldProvider{objectName: canonical}
	actual, _ := semaStandardSObjectFieldProviders.LoadOrStore(key, provider)
	return actual.(*semaStandardSObjectFieldProvider)
}

func (p *semaStandardSObjectFieldProvider) load() *semaSObjectFieldMapProvider {
	if p == nil {
		return nil
	}
	p.once.Do(func() {
		definition, ok := storage.StandardObjectDefinition(p.objectName)
		if !ok {
			p.fields = newSemaSObjectFieldMapProvider("", p.objectName, nil)
			return
		}
		if strings.EqualFold(p.objectName, "Account") {
			storage.EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})
		}
		p.fields = newSemaSObjectFieldMapProvider("", p.objectName, semaSchemaFieldsFromStandardDefinition(definition))
	})
	return p.fields
}

func (p *semaStandardSObjectFieldProvider) lookup(fieldName string) (schema.Field, bool) {
	return p.lookupNormalized(fieldName, normalizeName(fieldName))
}

func (p *semaStandardSObjectFieldProvider) lookupNormalized(fieldName, normalizedName string) (schema.Field, bool) {
	fields := p.load()
	if fields == nil {
		return schema.Field{}, false
	}
	return fields.lookupNormalized(fieldName, normalizedName)
}

func (p *semaStandardSObjectFieldProvider) visit(visit func(schema.Field)) {
	fields := p.load()
	if fields != nil {
		fields.visit(visit)
	}
}

func (p *semaStandardSObjectFieldProvider) visitDeclared(func(schema.Field)) {}

func (p *semaStandardSObjectFieldProvider) hasFields() bool {
	return p != nil && storage.IsKnownStandardObject(p.objectName)
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
	return p.lookupNormalized(fieldName, normalizeName(fieldName))
}

func (p *semaSObjectFieldMapProvider) lookupNormalized(fieldName, normalizedName string) (schema.Field, bool) {
	key := normalizedName
	if canonical, ok := p.aliases[key]; ok {
		key = canonical
	}
	field, ok := p.fields[key]
	if !ok && semaIsCustomAPIName(fieldName) && semaHasNamespaceToken(fieldName) {
		localKey := normalizeName(semaSchemaLocalAPIName(fieldName))
		if canonical, exists := p.aliases[localKey]; exists {
			localKey = canonical
		}
		field, ok = p.fields[localKey]
	}
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

func (p *semaSObjectFieldMapProvider) hasFields() bool {
	return p != nil && len(p.fields) > 0
}

func (p *semaSObjectFieldMapProvider) visitDeclared(visit func(schema.Field)) {
	p.visit(visit)
}

func semaEnrichSObjectProviderField(existing, incoming schema.Field) schema.Field {
	existing = semaCloneSchemaField(existing)
	incoming = semaCloneSchemaField(incoming)
	if existing.Type == "" || strings.EqualFold(existing.Type, "any") || strings.EqualFold(existing.Type, "object") {
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
