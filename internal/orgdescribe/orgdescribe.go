package orgdescribe

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

// Catalog is a normalized collection of REST describe results captured from a
// Salesforce org. It is intentionally file/data shaped; live org access belongs
// in contributor tooling, not the local runtime path.
type Catalog struct {
	Objects []SObject `json:"objects"`
}

type SObject struct {
	Name               string              `json:"name"`
	Label              string              `json:"label,omitempty"`
	LabelPlural        string              `json:"labelPlural,omitempty"`
	KeyPrefix          string              `json:"keyPrefix,omitempty"`
	Custom             bool                `json:"custom,omitempty"`
	Queryable          bool                `json:"queryable,omitempty"`
	Createable         bool                `json:"createable,omitempty"`
	Updateable         bool                `json:"updateable,omitempty"`
	Deletable          bool                `json:"deletable,omitempty"`
	Fields             []Field             `json:"fields,omitempty"`
	ChildRelationships []ChildRelationship `json:"childRelationships,omitempty"`
	RecordTypeInfos    []RecordTypeInfo    `json:"recordTypeInfos,omitempty"`
	NamespacePrefix    string              `json:"namespacePrefix,omitempty"`
}

type Field struct {
	Name              string          `json:"name"`
	Label             string          `json:"label,omitempty"`
	Type              string          `json:"type,omitempty"`
	Length            int             `json:"length,omitempty"`
	Precision         int             `json:"precision,omitempty"`
	Scale             int             `json:"scale,omitempty"`
	Nillable          bool            `json:"nillable"`
	Createable        bool            `json:"createable,omitempty"`
	Updateable        bool            `json:"updateable,omitempty"`
	Custom            bool            `json:"custom,omitempty"`
	ExternalID        bool            `json:"externalId,omitempty"`
	Unique            bool            `json:"unique,omitempty"`
	CaseSensitive     bool            `json:"caseSensitive,omitempty"`
	ReferenceTo       []string        `json:"referenceTo,omitempty"`
	RelationshipName  string          `json:"relationshipName,omitempty"`
	InlineHelpText    string          `json:"inlineHelpText,omitempty"`
	Description       string          `json:"description,omitempty"`
	PicklistValues    []PicklistValue `json:"picklistValues,omitempty"`
	DefaultValue      any             `json:"defaultValue,omitempty"`
	Calculated        bool            `json:"calculated,omitempty"`
	Formula           string          `json:"formula,omitempty"`
	RelationshipOrder *int            `json:"relationshipOrder,omitempty"`
}

type PicklistValue struct {
	Value   string `json:"value"`
	Label   string `json:"label,omitempty"`
	Active  bool   `json:"active,omitempty"`
	Default bool   `json:"defaultValue,omitempty"`
}

type ChildRelationship struct {
	ChildSObject        string `json:"childSObject"`
	Field               string `json:"field"`
	RelationshipName    string `json:"relationshipName,omitempty"`
	CascadeDelete       bool   `json:"cascadeDelete,omitempty"`
	DeprecatedAndHidden bool   `json:"deprecatedAndHidden,omitempty"`
	RestrictedDelete    bool   `json:"restrictedDelete,omitempty"`
}

type RecordTypeInfo struct {
	Name                     string `json:"name,omitempty"`
	DeveloperName            string `json:"developerName,omitempty"`
	RecordTypeID             string `json:"recordTypeId,omitempty"`
	Available                bool   `json:"available,omitempty"`
	DefaultRecordTypeMapping bool   `json:"defaultRecordTypeMapping,omitempty"`
	Master                   bool   `json:"master,omitempty"`
	Active                   bool   `json:"active,omitempty"`
}

func (c Catalog) ToSchema() schema.Schema {
	objects := sortedObjects(c.Objects)
	out := schema.Schema{Objects: make([]schema.Object, 0, len(objects))}
	childNames := childRelationshipNames(objects)
	for _, object := range objects {
		out.Objects = append(out.Objects, object.ToSchemaObject(childNames[object.Name]))
	}
	return out
}

func (o SObject) ToSchemaObject(childNames map[string]string) schema.Object {
	object := schema.Object{
		Name:        o.Name,
		Label:       o.Label,
		PluralLabel: o.LabelPlural,
		Fields:      make([]schema.Field, 0, len(o.Fields)),
		RecordTypes: make([]schema.RecordType, 0, len(o.RecordTypeInfos)),
	}
	for _, field := range sortedFields(o.Fields) {
		object.Fields = append(object.Fields, field.ToSchemaField(childNames[field.Name]))
	}
	for _, recordType := range sortedRecordTypes(o.RecordTypeInfos) {
		object.RecordTypes = append(object.RecordTypes, recordType.ToSchemaRecordType())
	}
	return object
}

func (f Field) ToSchemaField(childRelationshipName string) schema.Field {
	return schema.Field{
		Name:                  f.Name,
		Label:                 labelOrName(f.Label, f.Name),
		Type:                  metadataFieldType(f),
		ReferenceTo:           cleanStrings(f.ReferenceTo),
		RelationshipName:      strings.TrimSpace(f.RelationshipName),
		ChildRelationshipName: childRelationshipName,
		DefaultValue:          defaultString(f.DefaultValue),
		Required:              requiredForCreate(f),
		ExternalID:            f.ExternalID,
		Unique:                f.Unique,
		Formula:               strings.TrimSpace(f.Formula),
		PicklistValues:        schemaPicklistValues(f.PicklistValues),
	}
}

func (r RecordTypeInfo) ToSchemaRecordType() schema.RecordType {
	developerName := strings.TrimSpace(r.DeveloperName)
	if developerName == "" && r.Master {
		developerName = "Master"
	}
	if developerName == "" {
		developerName = strings.TrimSpace(r.Name)
	}
	active := r.Active || r.Available || r.Master
	return schema.RecordType{
		DeveloperName: developerName,
		Label:         strings.TrimSpace(r.Name),
		Active:        active,
		Default:       r.DefaultRecordTypeMapping,
	}
}

func (c Catalog) ToObjectDefinitions() map[string]storage.ObjectDefinition {
	objects := sortedObjects(c.Objects)
	out := make(map[string]storage.ObjectDefinition, len(objects))
	for _, object := range objects {
		out[object.Name] = object.ToObjectDefinition()
	}
	applyChildRelationships(out, objects)
	return out
}

func (o SObject) ToObjectDefinition() storage.ObjectDefinition {
	describe := o.ToDescribeSObjectResult()
	return sobject.ToObjectDefinition(describe)
}

func (o SObject) ToDescribeSObjectResult() sobject.DescribeSObjectResult {
	fields := make(map[string]sobject.DescribeFieldResult, len(o.Fields))
	relationships := make([]storage.Relationship, 0, len(o.Fields))
	for _, field := range sortedFields(o.Fields) {
		describeField := field.ToDescribeFieldResult()
		fields[field.Name] = describeField
		if len(describeField.ReferenceTo) == 0 {
			continue
		}
		relationships = append(relationships, storage.Relationship{
			Field:              field.Name,
			ParentObjects:      append([]string(nil), describeField.ReferenceTo...),
			ParentRelationship: storage.ParentRelationshipName(storage.Field{APIName: field.Name, RelationshipName: describeField.RelationshipName}),
			Polymorphic:        len(describeField.ReferenceTo) > 1,
		})
	}
	recordTypes := make([]sobject.DescribeRecordTypeInfo, 0, len(o.RecordTypeInfos))
	for _, recordType := range sortedRecordTypes(o.RecordTypeInfos) {
		recordTypes = append(recordTypes, recordType.ToDescribeRecordTypeInfo())
	}
	return sobject.DescribeSObjectResult{
		Name:          o.Name,
		Label:         o.Label,
		PluralLabel:   o.LabelPlural,
		KeyPrefix:     o.KeyPrefix,
		Fields:        fields,
		Relationships: relationships,
		RecordTypes:   recordTypes,
	}
}

func (f Field) ToDescribeFieldResult() sobject.DescribeFieldResult {
	return sobject.DescribeFieldResult{
		Name:             f.Name,
		Type:             storageFieldType(f),
		DisplayType:      displayFieldType(f.Type),
		Label:            labelOrName(f.Label, f.Name),
		ReferenceTo:      cleanStrings(f.ReferenceTo),
		RelationshipName: strings.TrimSpace(f.RelationshipName),
		DefaultValue:     defaultString(f.DefaultValue),
		Required:         requiredForCreate(f),
		ExternalID:       f.ExternalID,
		Unique:           f.Unique,
		PicklistValues:   storagePicklistValues(f.PicklistValues),
	}
}

func (r RecordTypeInfo) ToDescribeRecordTypeInfo() sobject.DescribeRecordTypeInfo {
	developerName := strings.TrimSpace(r.DeveloperName)
	if developerName == "" && r.Master {
		developerName = "Master"
	}
	if developerName == "" {
		developerName = strings.TrimSpace(r.Name)
	}
	active := r.Active || r.Available || r.Master
	return sobject.DescribeRecordTypeInfo{
		ID:            storage.ID(strings.TrimSpace(r.RecordTypeID)),
		DeveloperName: developerName,
		Name:          strings.TrimSpace(r.Name),
		Active:        active,
		Available:     r.Available || r.Master,
		Default:       r.DefaultRecordTypeMapping,
	}
}

func childRelationshipNames(objects []SObject) map[string]map[string]string {
	out := make(map[string]map[string]string)
	for _, parent := range objects {
		for _, rel := range parent.ChildRelationships {
			if rel.ChildSObject == "" || rel.Field == "" || rel.RelationshipName == "" || rel.DeprecatedAndHidden {
				continue
			}
			if out[rel.ChildSObject] == nil {
				out[rel.ChildSObject] = make(map[string]string)
			}
			out[rel.ChildSObject][rel.Field] = rel.RelationshipName
		}
	}
	return out
}

func applyChildRelationships(definitions map[string]storage.ObjectDefinition, objects []SObject) {
	for _, parent := range objects {
		for _, child := range parent.ChildRelationships {
			if child.ChildSObject == "" || child.Field == "" || child.RelationshipName == "" || child.DeprecatedAndHidden {
				continue
			}
			definition, ok := definitions[child.ChildSObject]
			if !ok {
				continue
			}
			for i := range definition.Relations {
				if !strings.EqualFold(definition.Relations[i].Field, child.Field) {
					continue
				}
				definition.Relations[i].ChildRelationship = child.RelationshipName
				definition.Relations[i].CascadeDelete = child.CascadeDelete
				definition.Relations[i].RestrictedDelete = child.RestrictedDelete
			}
			definitions[child.ChildSObject] = definition
		}
	}
}

func metadataFieldType(f Field) string {
	switch strings.ToLower(strings.TrimSpace(f.Type)) {
	case "string", "textarea", "url", "phone", "email", "encryptedstring", "autonumber":
		return "Text"
	case "picklist", "combobox":
		return "Picklist"
	case "multipicklist":
		return "MultiselectPicklist"
	case "boolean":
		return "Checkbox"
	case "int", "integer", "double", "decimal":
		return "Number"
	case "currency":
		return "Currency"
	case "percent":
		return "Percent"
	case "date":
		return "Date"
	case "datetime":
		return "DateTime"
	case "reference", "masterrecord":
		return "Lookup"
	case "id":
		return "Id"
	case "base64":
		return "Base64"
	default:
		if f.Calculated || f.Formula != "" {
			return "Formula"
		}
		return strings.TrimSpace(f.Type)
	}
}

func storageFieldType(f Field) storage.FieldType {
	if f.Calculated || f.Formula != "" {
		return storage.FieldCalculated
	}
	switch strings.ToLower(strings.TrimSpace(f.Type)) {
	case "id":
		return storage.FieldID
	case "string", "textarea", "url", "phone", "email", "encryptedstring", "autonumber", "combobox":
		return storage.FieldString
	case "picklist", "multipicklist":
		return storage.FieldPicklist
	case "boolean":
		return storage.FieldBoolean
	case "int", "integer":
		return storage.FieldInteger
	case "double", "decimal", "currency", "percent":
		return storage.FieldDecimal
	case "date":
		return storage.FieldDate
	case "datetime", "time":
		return storage.FieldDateTime
	case "reference", "masterrecord":
		return storage.FieldReference
	case "base64":
		return storage.FieldBlob
	case "address":
		return storage.FieldAddress
	case "location":
		return storage.FieldLocation
	default:
		return storage.FieldAny
	}
}

func displayFieldType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "id":
		return "ID"
	case "string", "textarea", "encryptedstring", "autonumber":
		return "STRING"
	case "url":
		return "URL"
	case "phone":
		return "PHONE"
	case "email":
		return "EMAIL"
	case "picklist", "combobox":
		return "PICKLIST"
	case "multipicklist":
		return "MULTIPICKLIST"
	case "boolean":
		return "BOOLEAN"
	case "int", "integer":
		return "INTEGER"
	case "double", "decimal":
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
	case "reference", "masterrecord":
		return "REFERENCE"
	case "base64":
		return "BASE64"
	case "address":
		return "ADDRESS"
	case "location":
		return "LOCATION"
	default:
		return strings.ToUpper(strings.TrimSpace(raw))
	}
}

func schemaPicklistValues(values []PicklistValue) []schema.PicklistValue {
	out := make([]schema.PicklistValue, 0, len(values))
	for _, value := range values {
		out = append(out, schema.PicklistValue{
			FullName: value.Value,
			Label:    labelOrName(value.Label, value.Value),
			Default:  value.Default,
			Active:   value.Active,
		})
	}
	return out
}

func storagePicklistValues(values []PicklistValue) []storage.PicklistValue {
	out := make([]storage.PicklistValue, 0, len(values))
	for _, value := range values {
		out = append(out, storage.PicklistValue{
			Value:   value.Value,
			Label:   labelOrName(value.Label, value.Value),
			Default: value.Default,
			Active:  value.Active,
		})
	}
	return out
}

func requiredForCreate(f Field) bool {
	return !f.Nillable && f.Createable
}

func defaultString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func labelOrName(label, name string) string {
	label = strings.TrimSpace(label)
	if label != "" {
		return label
	}
	return strings.TrimSpace(name)
}

func sortedObjects(objects []SObject) []SObject {
	out := append([]SObject(nil), objects...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedFields(fields []Field) []Field {
	out := append([]Field(nil), fields...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedRecordTypes(recordTypes []RecordTypeInfo) []RecordTypeInfo {
	out := append([]RecordTypeInfo(nil), recordTypes...)
	sort.Slice(out, func(i, j int) bool {
		left := out[i].DeveloperName
		if left == "" {
			left = out[i].Name
		}
		right := out[j].DeveloperName
		if right == "" {
			right = out[j].Name
		}
		return left < right
	})
	return out
}
