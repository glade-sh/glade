package storage

import (
	"compress/gzip"
	"embed"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
)

//go:embed standard_describe_catalog.json.gz
var standardDescribeCatalogFS embed.FS

type standardDescribeCatalogBundle struct {
	Describes map[string]standardDescribeObject `json:"describes"`
}

type standardDescribeObject struct {
	Name               string                              `json:"name"`
	Label              string                              `json:"label"`
	LabelPlural        string                              `json:"labelPlural"`
	KeyPrefix          *string                             `json:"keyPrefix"`
	Mergeable          *bool                               `json:"mergeable"`
	Triggerable        *bool                               `json:"triggerable"`
	Fields             []standardDescribeField             `json:"fields"`
	ChildRelationships []standardDescribeChildRelationship `json:"childRelationships"`
	RecordTypeInfos    []standardDescribeRecordTypeInfo    `json:"recordTypeInfos"`
}

var standardObjectTriggerableCache sync.Map

// StandardObjectDescribeKeyPrefix distinguishes an absent describe entry from
// a captured null key prefix. Objects without a stable prefix return null from
// Apex describe, even when local storage assigned an internal identifier prefix.
func StandardObjectDescribeKeyPrefix(objectName string) (prefix *string, known bool) {
	canonical, ok := standardDescribeCatalogCanonicalName(objectName)
	if !ok {
		return nil, false
	}
	describe, found, err := lookupStandardDescribeCatalogV2(canonical)
	return describe.KeyPrefix, found && err == nil
}

func (describe standardDescribeObject) keyPrefixValue() string {
	if describe.KeyPrefix == nil {
		return ""
	}
	return *describe.KeyPrefix
}

// VisitStandardChildRelationships reads the parent-side describe catalog without
// materializing unrelated objects. UI metadata must include unloaded children.
func VisitStandardChildRelationships(objectName string, visit func(string, Relationship)) bool {
	canonical, ok := standardDescribeCatalogCanonicalName(objectName)
	if !ok {
		return false
	}
	describe, found, err := lookupStandardDescribeCatalogV2(canonical)
	if err != nil || !found {
		return false
	}
	for _, child := range describe.ChildRelationships {
		visit(child.ChildSObject, Relationship{
			Field: child.Field, ChildRelationship: child.RelationshipName,
			ParentObjects: []string{canonical}, CascadeDelete: child.CascadeDelete,
			RestrictedDelete: child.RestrictedDelete,
		})
	}
	return true
}

// StandardObjectTriggerable reports the describe-provided `triggerable` flag for
// a standard object. The second result is false when the embedded describe
// catalog does not cover objectName, so callers can distinguish "known but not
// triggerable" from "no evidence".
func StandardObjectTriggerable(objectName string) (triggerable, known bool) {
	canonical, ok := standardDescribeCatalogCanonicalName(objectName)
	if !ok {
		return false, false
	}
	if cached, ok := standardObjectTriggerableCache.Load(canonical); ok {
		flag, _ := cached.(*bool)
		if flag == nil {
			return false, false
		}
		return *flag, true
	}
	describe, found, err := lookupStandardDescribeCatalogV2(canonical)
	if err != nil || !found || describe.Triggerable == nil {
		standardObjectTriggerableCache.Store(canonical, (*bool)(nil))
		return false, false
	}
	value := *describe.Triggerable
	standardObjectTriggerableCache.Store(canonical, &value)
	return value, true
}

// StandardFieldNameFlag returns an explicit nameField observation from the
// owned standard describe catalog, scoped to the declared org feature shape.
// Missing facets retain the caller's fallback.
func StandardFieldNameFlag(definition ObjectDefinition, fieldName string) (nameField, known bool) {
	field, found := standardDescribeFieldForName(definition.APIName, fieldName)
	if !found || field.NameField == nil {
		return false, false
	}
	// The shared catalog includes Person Account name components. A standard
	// object metadata delta can retain those tokens without enabling the org
	// feature; business Account observations have only Name as the name field.
	if strings.EqualFold(definition.APIName, "Account") && !featureSignatureHas(definition.Metadata[standardFieldsOverlayMarker], "PersonAccounts") {
		return strings.EqualFold(field.Name, "Name"), true
	}
	return *field.NameField, true
}

// StandardFieldRichTextLength exposes only an explicit rich-text textarea facet
// from the owned catalog. Other field kinds and absent facets retain fallbacks.
func StandardFieldRichTextLength(objectName, fieldName string) (length int, known bool) {
	field, found := standardDescribeFieldForName(objectName, fieldName)
	if !found || !strings.EqualFold(field.Type, "textarea") || !strings.EqualFold(field.ExtraTypeInfo, "richtextarea") {
		return 0, false
	}
	return field.Length, true
}

func standardDescribeFieldForName(objectName, fieldName string) (standardDescribeField, bool) {
	canonical, ok := standardDescribeCatalogCanonicalName(objectName)
	if !ok {
		return standardDescribeField{}, false
	}
	describe, found, err := lookupStandardDescribeCatalogV2(canonical)
	if err != nil || !found {
		return standardDescribeField{}, false
	}
	for _, field := range describe.Fields {
		if strings.EqualFold(field.Name, fieldName) {
			return field, true
		}
	}
	return standardDescribeField{}, false
}

type standardDescribeField struct {
	Name                string                          `json:"name"`
	NameField           *bool                           `json:"nameField"`
	Label               string                          `json:"label"`
	Type                string                          `json:"type"`
	Length              int                             `json:"length"`
	Precision           int                             `json:"precision"`
	Scale               int                             `json:"scale"`
	Calculated          bool                            `json:"calculated"`
	DefaultValue        any                             `json:"defaultValue"`
	DefaultValueFormula *string                         `json:"defaultValueFormula"`
	CompoundFieldName   string                          `json:"compoundFieldName"`
	Nillable            *bool                           `json:"nillable"`
	DefaultedOnCreate   *bool                           `json:"defaultedOnCreate"`
	Createable          *bool                           `json:"createable"`
	Updateable          *bool                           `json:"updateable"`
	Filterable          *bool                           `json:"filterable"`
	Groupable           *bool                           `json:"groupable"`
	Sortable            *bool                           `json:"sortable"`
	Aggregatable        *bool                           `json:"aggregatable"`
	Permissionable      *bool                           `json:"permissionable"`
	DeprecatedAndHidden *bool                           `json:"deprecatedAndHidden"`
	ExternalID          bool                            `json:"externalId"`
	Unique              bool                            `json:"unique"`
	Encrypted           bool                            `json:"encrypted"`
	CaseSensitive       bool                            `json:"caseSensitive"`
	IDLookup            bool                            `json:"idLookup"`
	ReferenceTo         []string                        `json:"referenceTo"`
	RelationshipName    string                          `json:"relationshipName"`
	Polymorphic         bool                            `json:"polymorphicForeignKey"`
	PicklistValues      []standardDescribePicklistValue `json:"picklistValues"`
	ExtraTypeInfo       string                          `json:"extraTypeInfo"`
}

type standardDescribePicklistValue struct {
	Value        string `json:"value"`
	Label        string `json:"label"`
	Active       bool   `json:"active"`
	DefaultValue bool   `json:"defaultValue"`
}

type standardDescribeChildRelationship struct {
	ChildSObject        string `json:"childSObject"`
	Field               string `json:"field"`
	RelationshipName    string `json:"relationshipName"`
	CascadeDelete       bool   `json:"cascadeDelete"`
	RestrictedDelete    bool   `json:"restrictedDelete"`
	DeprecatedAndHidden bool   `json:"deprecatedAndHidden"`
}

type standardDescribeRecordTypeInfo struct {
	RecordTypeID             string `json:"recordTypeId"`
	DeveloperName            string `json:"developerName"`
	Name                     string `json:"name"`
	Active                   bool   `json:"active"`
	Available                bool   `json:"available"`
	DefaultRecordTypeMapping bool   `json:"defaultRecordTypeMapping"`
}

type standardDescribeChildRelationshipInfo struct {
	relationshipName string
	cascadeDelete    bool
	restrictedDelete bool
	conflict         bool
}

var standardObjectCatalogLookupCache struct {
	generatedOnce    sync.Once
	generatedByLC    map[string]standardObjectCatalogEntry
	describeNameOnce sync.Once
	describeNameByLC map[string]string
	describeOnce     sync.Once
	describeByLC     map[string]standardObjectCatalogEntry
}

func standardObjectCatalogEntryForName(objectName string) (standardObjectCatalogEntry, bool) {
	objectName = strings.TrimSpace(objectName)
	if objectName == "" {
		return standardObjectCatalogEntry{}, false
	}
	if entry, ok := standardObjectCatalogData[objectName]; ok {
		return entry, true
	}
	standardObjectCatalogLookupCache.generatedOnce.Do(func() {
		byLC := make(map[string]standardObjectCatalogEntry, len(standardObjectCatalogData))
		for name, entry := range standardObjectCatalogData {
			byLC[standardObjectLookupKey(name)] = entry
		}
		standardObjectCatalogLookupCache.generatedByLC = byLC
	})
	if entry, ok := standardObjectCatalogLookupCache.generatedByLC[standardObjectLookupKey(objectName)]; ok {
		return entry, true
	}
	canonical, ok := standardDescribeCatalogCanonicalName(objectName)
	if !ok {
		return standardObjectCatalogEntry{}, false
	}
	if standardSObjectStubHasFieldsExact(canonical) {
		return standardObjectCatalogEntry{}, false
	}
	entry, ok, err := standardDescribeCatalogV2EntryForName(canonical)
	if err != nil {
		return standardObjectCatalogEntry{}, false
	}
	return entry, ok
}

func standardDescribeCatalogCanonicalName(objectName string) (string, bool) {
	objectName = strings.TrimSpace(objectName)
	if objectName == "" {
		return "", false
	}
	standardObjectCatalogLookupCache.describeNameOnce.Do(func() {
		byLC := make(map[string]string, len(standardDescribeCatalogObjectNames))
		for _, name := range standardDescribeCatalogObjectNames {
			if name = strings.TrimSpace(name); name != "" {
				byLC[standardObjectLookupKey(name)] = name
			}
		}
		standardObjectCatalogLookupCache.describeNameByLC = byLC
	})
	canonical, ok := standardObjectCatalogLookupCache.describeNameByLC[standardObjectLookupKey(objectName)]
	if ok {
		return canonical, true
	}
	// The V2 pack is the authoritative describe catalog used by runtime
	// metadata. Keep names-only resolution aligned with it so newly added
	// standard objects do not fail sema reference checks before a field lookup
	// has a chance to decode the V2 member.
	entry, ok := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, objectName)
	if !ok {
		return "", false
	}
	return entry.Name, true
}

func loadEmbeddedStandardDescribeCatalog() map[string]standardObjectCatalogEntry {
	file, err := standardDescribeCatalogFS.Open("standard_describe_catalog.json.gz")
	if err != nil {
		return nil
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil
	}
	var bundle standardDescribeCatalogBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil
	}
	childRelationships := describeChildRelationshipMap(bundle.Describes)
	out := make(map[string]standardObjectCatalogEntry, len(bundle.Describes))
	for objectName, describe := range bundle.Describes {
		if strings.TrimSpace(describe.Name) == "" {
			describe.Name = objectName
		}
		entry := standardObjectCatalogEntry{
			Definition: ObjectDefinition{
				APIName:     describe.Name,
				Label:       firstStorageString(describe.Label, describe.Name),
				PluralLabel: firstStorageString(describe.LabelPlural, describe.Label+"s", describe.Name+"s"),
				KeyPrefix:   describe.keyPrefixValue(),
				Fields:      describeFieldMap(describe.Fields),
				Relations:   describeRelationships(describe.Name, describe.Fields, childRelationships),
				RecordTypes: describeRecordTypes(describe.RecordTypeInfos),
			},
		}
		out[describe.Name] = entry
	}
	return out
}

func describeChildRelationshipMap(describes map[string]standardDescribeObject) map[string]standardDescribeChildRelationshipInfo {
	out := map[string]standardDescribeChildRelationshipInfo{}
	for _, objectDescribe := range describes {
		for _, relationship := range objectDescribe.ChildRelationships {
			if relationship.ChildSObject == "" || relationship.Field == "" || relationship.RelationshipName == "" || relationship.DeprecatedAndHidden {
				continue
			}
			key := describeChildRelationshipKey(relationship.ChildSObject, relationship.Field)
			existing := out[key]
			if existing.relationshipName == "" {
				existing.relationshipName = relationship.RelationshipName
			} else if existing.relationshipName != relationship.RelationshipName {
				existing.conflict = true
			}
			existing.cascadeDelete = existing.cascadeDelete || relationship.CascadeDelete
			existing.restrictedDelete = existing.restrictedDelete || relationship.RestrictedDelete
			out[key] = existing
		}
	}
	return out
}

func describeFieldMap(fields []standardDescribeField) map[string]Field {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]Field, len(fields))
	for _, field := range fields {
		if field.Name == "" {
			continue
		}
		out[field.Name] = describeField(field)
	}
	return out
}

func describeField(field standardDescribeField) Field {
	out := Field{
		APIName:             field.Name,
		Label:               firstStorageString(field.Label, field.Name),
		Type:                describeFieldType(field),
		DisplayType:         describeDisplayType(field.Type),
		Length:              field.Length,
		Precision:           field.Precision,
		Scale:               field.Scale,
		CompoundFieldName:   field.CompoundFieldName,
		Nillable:            cloneBoolFlag(field.Nillable),
		DefaultedOnCreate:   cloneBoolFlag(field.DefaultedOnCreate),
		Createable:          cloneBoolFlag(field.Createable),
		Updateable:          cloneBoolFlag(field.Updateable),
		Filterable:          cloneBoolFlag(field.Filterable),
		Groupable:           cloneBoolFlag(field.Groupable),
		Sortable:            cloneBoolFlag(field.Sortable),
		Aggregatable:        cloneBoolFlag(field.Aggregatable),
		Permissionable:      cloneBoolFlag(field.Permissionable),
		DeprecatedAndHidden: cloneBoolFlag(field.DeprecatedAndHidden),
		ExternalID:          field.ExternalID,
		Unique:              field.Unique,
		Encrypted:           field.Encrypted,
		CaseSensitive:       field.CaseSensitive,
		IDLookup:            field.IDLookup,
		ReferenceTo:         append([]string(nil), field.ReferenceTo...),
		RelationshipName:    field.RelationshipName,
		PicklistValues:      describePicklistValues(field.PicklistValues),
	}
	if field.DefaultValue != nil {
		out.DefaultValue = strings.TrimSpace(strings.Trim(strings.TrimSpace(toStorageString(field.DefaultValue)), `"`))
	}
	if field.Nillable != nil && field.Createable != nil && field.DefaultedOnCreate != nil &&
		!*field.Nillable && *field.Createable && !*field.DefaultedOnCreate && field.DefaultValue == nil && field.DefaultValueFormula == nil {
		out.Required = true
	}
	return out
}

func describeFieldType(field standardDescribeField) FieldType {
	if field.Calculated {
		return FieldCalculated
	}
	switch field.Type {
	case "id":
		return FieldID
	case "string", "textarea", "email", "phone", "url", "combobox", "encryptedstring":
		return FieldString
	case "picklist", "multipicklist":
		return FieldPicklist
	case "boolean":
		return FieldBoolean
	case "int":
		return FieldInteger
	case "double", "currency", "percent":
		return FieldDecimal
	case "date":
		return FieldDate
	case "datetime":
		return FieldDateTime
	case "time":
		return FieldTime
	case "reference":
		return FieldReference
	case "base64":
		return FieldBlob
	case "address":
		return FieldAddress
	case "location":
		return FieldLocation
	default:
		return FieldAny
	}
}

func describeDisplayType(fieldType string) string {
	switch fieldType {
	case "id":
		return "ID"
	case "string":
		return "STRING"
	case "textarea":
		return "TEXTAREA"
	case "email":
		return "EMAIL"
	case "phone":
		return "PHONE"
	case "url":
		return "URL"
	case "picklist":
		return "PICKLIST"
	case "multipicklist":
		return "MULTIPICKLIST"
	case "boolean":
		return "BOOLEAN"
	case "int":
		return "INTEGER"
	case "double":
		return "DOUBLE"
	case "currency":
		return "CURRENCY"
	case "percent":
		return "PERCENT"
	case "date":
		return "DATE"
	case "datetime":
		return "DATETIME"
	case "time":
		return "TIME"
	case "reference":
		return "REFERENCE"
	case "base64":
		return "BLOB"
	case "address":
		return "ADDRESS"
	case "location":
		return "LOCATION"
	default:
		return ""
	}
}

func describePicklistValues(values []standardDescribePicklistValue) []PicklistValue {
	if len(values) == 0 {
		return nil
	}
	out := make([]PicklistValue, 0, len(values))
	for _, value := range values {
		if value.Value == "" {
			continue
		}
		out = append(out, PicklistValue{
			Value:   value.Value,
			Label:   value.Label,
			Active:  value.Active,
			Default: value.DefaultValue,
		})
	}
	return out
}

func describeRelationships(objectName string, fields []standardDescribeField, childRelationships map[string]standardDescribeChildRelationshipInfo) []Relationship {
	var out []Relationship
	for _, field := range fields {
		if len(field.ReferenceTo) == 0 {
			continue
		}
		childRelationship := childRelationships[describeChildRelationshipKey(objectName, field.Name)]
		relationship := Relationship{
			Field:              field.Name,
			ParentObjects:      append([]string(nil), field.ReferenceTo...),
			ParentRelationship: field.RelationshipName,
			Polymorphic:        len(field.ReferenceTo) > 1 || field.Polymorphic,
		}
		if !childRelationship.conflict {
			relationship.ChildRelationship = childRelationship.relationshipName
			relationship.CascadeDelete = childRelationship.cascadeDelete
			relationship.RestrictedDelete = childRelationship.restrictedDelete
		}
		if relationship.ParentRelationship == "" && strings.HasSuffix(field.Name, "Id") {
			relationship.ParentRelationship = strings.TrimSuffix(field.Name, "Id")
		}
		out = append(out, relationship)
	}
	return out
}

func describeRecordTypes(recordTypes []standardDescribeRecordTypeInfo) []RecordTypeInfo {
	if len(recordTypes) == 0 {
		return nil
	}
	out := make([]RecordTypeInfo, 0, len(recordTypes))
	for _, recordType := range recordTypes {
		if recordType.DeveloperName == "" {
			continue
		}
		out = append(out, RecordTypeInfo{
			ID:            ID(recordType.RecordTypeID),
			DeveloperName: recordType.DeveloperName,
			Name:          firstStorageString(recordType.Name, recordType.DeveloperName),
			Active:        recordType.Active,
			Available:     recordType.Available,
			Default:       recordType.DefaultRecordTypeMapping,
		})
	}
	return out
}

func describeChildRelationshipKey(childObject, fieldName string) string {
	return strings.ToLower(strings.TrimSpace(childObject)) + "." + strings.ToLower(strings.TrimSpace(fieldName))
}

func firstStorageString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func toStorageString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(data)
	}
}
