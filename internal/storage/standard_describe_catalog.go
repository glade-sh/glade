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
	KeyPrefix          string                              `json:"keyPrefix"`
	Triggerable        *bool                               `json:"triggerable"`
	Fields             []standardDescribeField             `json:"fields"`
	ChildRelationships []standardDescribeChildRelationship `json:"childRelationships"`
	RecordTypeInfos    []standardDescribeRecordTypeInfo    `json:"recordTypeInfos"`
}

var standardObjectTriggerableCache sync.Map

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

type standardDescribeField struct {
	Name                string                          `json:"name"`
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
	ReferenceTo         []string                        `json:"referenceTo"`
	RelationshipName    string                          `json:"relationshipName"`
	Polymorphic         bool                            `json:"polymorphicForeignKey"`
	PicklistValues      []standardDescribePicklistValue `json:"picklistValues"`
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
	if _, ok := standardSObjectStubFieldData[canonical]; ok {
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
	return canonical, ok
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
				KeyPrefix:   describe.KeyPrefix,
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
