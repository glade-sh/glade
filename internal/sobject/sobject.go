package sobject

import (
	"fmt"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/storage"
)

type Value struct {
	Object        string                   `json:"object"`
	ID            storage.ID               `json:"id,omitempty"`
	Fields        map[string]storage.Value `json:"fields,omitempty"`
	ExplicitNulls map[string]bool          `json:"explicitNulls,omitempty"`
}

func New(object string) Value {
	return Value{
		Object:        object,
		Fields:        make(map[string]storage.Value),
		ExplicitNulls: make(map[string]bool),
	}
}

func FromRecord(record storage.Record) Value {
	return Value{
		Object:        record.Object,
		ID:            record.ID,
		Fields:        cloneValues(record.Fields),
		ExplicitNulls: cloneBools(record.ExplicitNulls),
	}
}

func (v Value) ToRecord() storage.Record {
	return storage.Record{
		ID:            v.ID,
		Object:        v.Object,
		Fields:        cloneValues(v.Fields),
		ExplicitNulls: cloneBools(v.ExplicitNulls),
	}
}

func (v Value) Clone() Value {
	return FromRecord(v.ToRecord())
}

func (v *Value) Put(field string, value storage.Value) {
	if v.Fields == nil {
		v.Fields = make(map[string]storage.Value)
	}
	if v.ExplicitNulls == nil {
		v.ExplicitNulls = make(map[string]bool)
	}
	field = canonicalValueFieldName(v.Fields, v.ExplicitNulls, field)
	if value.Kind == storage.ValueNull {
		delete(v.Fields, field)
		v.ExplicitNulls[field] = true
		return
	}
	v.Fields[field] = value.Clone()
	delete(v.ExplicitNulls, field)
}

func (v Value) Get(field string) (storage.Value, bool) {
	if actual, ok := lookupBoolFold(v.ExplicitNulls, field); ok && v.ExplicitNulls[actual] {
		return storage.NullValue(), true
	}
	actual, ok := lookupValueFold(v.Fields, field)
	if !ok {
		return storage.Value{}, false
	}
	value := v.Fields[actual]
	return value.Clone(), ok
}

func canonicalValueFieldName(fields map[string]storage.Value, nulls map[string]bool, field string) string {
	if actual, ok := lookupValueFold(fields, field); ok {
		return actual
	}
	if actual, ok := lookupBoolFold(nulls, field); ok {
		return actual
	}
	return field
}

func lookupValueFold(values map[string]storage.Value, field string) (string, bool) {
	if values == nil {
		return "", false
	}
	if _, ok := values[field]; ok {
		return field, true
	}
	for candidate := range values {
		if strings.EqualFold(candidate, field) {
			return candidate, true
		}
	}
	return "", false
}

func lookupBoolFold(values map[string]bool, field string) (string, bool) {
	if values == nil {
		return "", false
	}
	if _, ok := values[field]; ok {
		return field, true
	}
	for candidate := range values {
		if strings.EqualFold(candidate, field) {
			return candidate, true
		}
	}
	return "", false
}

func (v Value) FieldNames() []string {
	names := make([]string, 0, len(v.Fields)+len(v.ExplicitNulls))
	seen := make(map[string]bool, len(v.Fields)+len(v.ExplicitNulls))
	for name := range v.Fields {
		names = append(names, name)
		seen[name] = true
	}
	for name := range v.ExplicitNulls {
		if !seen[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

type DescribeRegistry struct {
	Objects map[string]DescribeSObjectResult `json:"objects"`
}

type DescribeSObjectResult struct {
	Name            string                         `json:"name"`
	Label           string                         `json:"label,omitempty"`
	PluralLabel     string                         `json:"pluralLabel,omitempty"`
	KeyPrefix       string                         `json:"keyPrefix,omitempty"`
	Metadata        map[string]string              `json:"metadata,omitempty"`
	Fields          map[string]DescribeFieldResult `json:"fields,omitempty"`
	Relationships   []storage.Relationship         `json:"relationships,omitempty"`
	RecordTypes     []DescribeRecordTypeInfo       `json:"recordTypes,omitempty"`
	ValidationRules []storage.ValidationRule       `json:"validationRules,omitempty"`
}

type DescribeFieldResult struct {
	Name                  string                      `json:"name"`
	Type                  storage.FieldType           `json:"type"`
	DisplayType           string                      `json:"displayType,omitempty"`
	Label                 string                      `json:"label,omitempty"`
	Length                int                         `json:"length,omitempty"`
	Precision             int                         `json:"precision,omitempty"`
	Scale                 int                         `json:"scale,omitempty"`
	Formula               string                      `json:"formula,omitempty"`
	AutoNumber            bool                        `json:"autoNumber,omitempty"`
	DisplayFormat         string                      `json:"displayFormat,omitempty"`
	SummarizedField       string                      `json:"summarizedField,omitempty"`
	SummaryForeignKey     string                      `json:"summaryForeignKey,omitempty"`
	SummaryOperation      string                      `json:"summaryOperation,omitempty"`
	SummaryFilterItems    []storage.SummaryFilterItem `json:"summaryFilterItems,omitempty"`
	ReferenceTo           []string                    `json:"referenceTo,omitempty"`
	RelationshipName      string                      `json:"relationshipName,omitempty"`
	ChildRelationshipName string                      `json:"childRelationshipName,omitempty"`
	DeleteConstraint      string                      `json:"deleteConstraint,omitempty"`
	DefaultValue          string                      `json:"defaultValue,omitempty"`
	Required              bool                        `json:"required,omitempty"`
	ExternalID            bool                        `json:"externalId,omitempty"`
	Unique                bool                        `json:"unique,omitempty"`
	Encrypted             bool                        `json:"encrypted,omitempty"`
	PicklistValues        []storage.PicklistValue     `json:"picklistValues,omitempty"`
}

type DescribeRecordTypeInfo struct {
	ID            storage.ID `json:"id,omitempty"`
	DeveloperName string     `json:"developerName"`
	Name          string     `json:"name,omitempty"`
	Active        bool       `json:"active,omitempty"`
	Available     bool       `json:"available,omitempty"`
	Default       bool       `json:"default,omitempty"`
	Description   string     `json:"description,omitempty"`
}

func BuildDescribeRegistry(s schema.Schema) DescribeRegistry {
	objects := make([]schema.Object, len(s.Objects))
	copy(objects, s.Objects)
	sort.Slice(objects, func(i, j int) bool { return objects[i].Name < objects[j].Name })
	prefixes := storage.AssignDeterministicPrefixes(objectNames(objects), nil)

	registry := DescribeRegistry{Objects: make(map[string]DescribeSObjectResult, len(objects))}
	recordTypeIDs := storage.NewIDGenerator(map[string]string{"RecordType": storage.StandardKeyPrefixes()["RecordType"]})
	for _, object := range objects {
		describe := DescribeSObjectResult{
			Name:        object.Name,
			Label:       object.Label,
			PluralLabel: object.PluralLabel,
			KeyPrefix:   prefixes[object.Name],
			Fields:      make(map[string]DescribeFieldResult, len(object.Fields)),
		}
		if strings.HasSuffix(object.Name, "__mdt") {
			ensureDescribeField(describe.Fields, "DeveloperName", "Text", "Developer Name")
			ensureDescribeField(describe.Fields, "MasterLabel", "Text", "Master Label")
			ensureDescribeField(describe.Fields, "NamespacePrefix", "Text", "Namespace Prefix")
			ensureDescribeField(describe.Fields, "QualifiedApiName", "Text", "Qualified API Name")
			describe.Metadata = map[string]string{"kind": "customMetadata"}
		}
		if object.CustomSettingsType != "" {
			ensureDescribeField(describe.Fields, "Name", "Text", "Name")
			ensureDescribeField(describe.Fields, "SetupOwnerId", "Text", "Setup Owner ID")
			describe.Metadata = map[string]string{"kind": "customSetting", "customSettingsType": object.CustomSettingsType}
		}
		if object.NameField.Type != "" {
			describe.Fields["Name"] = DescribeFieldResult{
				Name:          "Name",
				Type:          storage.FieldString,
				DisplayType:   displayFieldType(object.NameField.Type),
				Label:         labelOrName(object.NameField.Label, "Name"),
				Required:      true,
				AutoNumber:    strings.EqualFold(object.NameField.Type, "AutoNumber"),
				DisplayFormat: object.NameField.DisplayFormat,
			}
		}
		for _, field := range object.Fields {
			fieldType := storageFieldType(field.Type)
			if field.Formula != "" {
				fieldType = storage.FieldCalculated
			}
			if strings.EqualFold(field.Type, "Summary") {
				fieldType = storage.FieldSummary
			}
			describe.Fields[field.Name] = DescribeFieldResult{
				Name:                  field.Name,
				Type:                  fieldType,
				DisplayType:           displayFieldType(field.Type),
				Label:                 labelOrName(field.Label, field.Name),
				Length:                field.Length,
				Precision:             field.Precision,
				Scale:                 field.Scale,
				Formula:               field.Formula,
				ReferenceTo:           referenceTargets(field.ReferenceTo),
				SummarizedField:       field.SummarizedField,
				SummaryForeignKey:     field.SummaryForeignKey,
				SummaryOperation:      field.SummaryOperation,
				SummaryFilterItems:    storageSummaryFilters(field.SummaryFilterItems),
				RelationshipName:      field.RelationshipName,
				ChildRelationshipName: field.ChildRelationshipName,
				DeleteConstraint:      field.DeleteConstraint,
				DefaultValue:          field.DefaultValue,
				Required:              field.Required,
				ExternalID:            field.ExternalID,
				Unique:                field.Unique,
				Encrypted:             field.Encrypted,
				PicklistValues:        storagePicklistValues(field.PicklistValues),
			}
			references := referenceTargets(field.ReferenceTo)
			if len(references) != 0 {
				parentRelationship := storage.ParentRelationshipName(storage.Field{
					APIName:          field.Name,
					RelationshipName: field.RelationshipName,
				})
				childRelationship := field.ChildRelationshipName
				if childRelationship == "" && !strings.EqualFold(field.RelationshipName, parentRelationship) {
					childRelationship = field.RelationshipName
				}
				describe.Relationships = append(describe.Relationships, storage.Relationship{
					Field:              field.Name,
					ParentObjects:      references,
					ParentRelationship: parentRelationship,
					ChildRelationship:  childRelationship,
					Polymorphic:        len(references) > 1,
					CascadeDelete:      strings.EqualFold(field.DeleteConstraint, "Cascade"),
					RestrictedDelete:   strings.EqualFold(field.DeleteConstraint, "Restrict"),
				})
			}
		}
		for _, recordType := range object.RecordTypes {
			id, err := recordTypeIDs.Next("RecordType")
			if err != nil {
				id = ""
			}
			describe.RecordTypes = append(describe.RecordTypes, DescribeRecordTypeInfo{
				ID:            id,
				DeveloperName: recordType.DeveloperName,
				Name:          recordType.Label,
				Active:        recordType.Active,
				Available:     recordType.Active,
				Default:       recordType.Default,
				Description:   recordType.Description,
			})
		}
		if len(describe.RecordTypes) > 0 {
			describe.Fields["RecordTypeId"] = DescribeFieldResult{
				Name:             "RecordTypeId",
				Type:             storage.FieldReference,
				DisplayType:      string(storage.FieldReference),
				Label:            "Record Type ID",
				ReferenceTo:      []string{"RecordType"},
				RelationshipName: "RecordType",
			}
		}
		for _, rule := range object.ValidationRules {
			describe.ValidationRules = append(describe.ValidationRules, storage.ValidationRule{
				Name:                  rule.Name,
				Active:                rule.Active,
				ErrorConditionFormula: rule.ErrorConditionFormula,
				ErrorMessage:          rule.ErrorMessage,
				ErrorDisplayField:     rule.ErrorDisplayField,
			})
		}
		definition := ToObjectDefinition(describe)
		storage.EnsureStandardObjectFields(&definition)
		registry.Objects[object.Name] = FromObjectDefinition(definition)
	}
	return registry
}

func (r DescribeRegistry) GlobalDescribe() map[string]DescribeSObjectResult {
	out := make(map[string]DescribeSObjectResult, len(r.Objects))
	for name, describe := range r.Objects {
		out[name] = describe.Clone()
	}
	return out
}

func (r DescribeRegistry) Describe(object string) (DescribeSObjectResult, error) {
	describe, ok := r.Objects[object]
	if !ok {
		return DescribeSObjectResult{}, fmt.Errorf("sobject: unknown object %s", object)
	}
	return describe.Clone(), nil
}

func (d DescribeSObjectResult) Clone() DescribeSObjectResult {
	out := d
	if d.Fields != nil {
		out.Fields = make(map[string]DescribeFieldResult, len(d.Fields))
		for name, field := range d.Fields {
			field.ReferenceTo = append([]string(nil), field.ReferenceTo...)
			field.PicklistValues = append([]storage.PicklistValue(nil), field.PicklistValues...)
			out.Fields[name] = field
		}
	}
	out.Relationships = append([]storage.Relationship(nil), d.Relationships...)
	for i := range out.Relationships {
		out.Relationships[i].ParentObjects = append([]string(nil), d.Relationships[i].ParentObjects...)
	}
	out.RecordTypes = append([]DescribeRecordTypeInfo(nil), d.RecordTypes...)
	out.ValidationRules = append([]storage.ValidationRule(nil), d.ValidationRules...)
	if d.Metadata != nil {
		out.Metadata = make(map[string]string, len(d.Metadata))
		for key, value := range d.Metadata {
			out.Metadata[key] = value
		}
	}
	return out
}

func ToObjectDefinition(describe DescribeSObjectResult) storage.ObjectDefinition {
	definition := storage.ObjectDefinition{
		APIName:         describe.Name,
		Label:           describe.Label,
		PluralLabel:     describe.PluralLabel,
		KeyPrefix:       describe.KeyPrefix,
		Fields:          make(map[string]storage.Field, len(describe.Fields)),
		Relations:       append([]storage.Relationship(nil), describe.Relationships...),
		RecordTypes:     make([]storage.RecordTypeInfo, 0, len(describe.RecordTypes)),
		ValidationRules: append([]storage.ValidationRule(nil), describe.ValidationRules...),
	}
	if describe.Metadata != nil {
		definition.Metadata = make(map[string]string, len(describe.Metadata))
		for key, value := range describe.Metadata {
			definition.Metadata[key] = value
		}
	}
	for name, field := range describe.Fields {
		definition.Fields[name] = storage.Field{
			APIName:            field.Name,
			Label:              labelOrName(field.Label, field.Name),
			Type:               field.Type,
			DisplayType:        field.DisplayType,
			Length:             field.Length,
			Precision:          field.Precision,
			Scale:              field.Scale,
			Formula:            field.Formula,
			DefaultValue:       field.DefaultValue,
			AutoNumber:         field.AutoNumber,
			DisplayFormat:      field.DisplayFormat,
			SummarizedField:    field.SummarizedField,
			SummaryForeignKey:  field.SummaryForeignKey,
			SummaryOperation:   field.SummaryOperation,
			SummaryFilterItems: append([]storage.SummaryFilterItem(nil), field.SummaryFilterItems...),
			Required:           field.Required,
			ExternalID:         field.ExternalID,
			Unique:             field.Unique,
			Encrypted:          field.Encrypted,
			ReferenceTo:        append([]string(nil), field.ReferenceTo...),
			RelationshipName:   field.RelationshipName,
			PicklistValues:     append([]storage.PicklistValue(nil), field.PicklistValues...),
		}
	}
	for _, recordType := range describe.RecordTypes {
		definition.RecordTypes = append(definition.RecordTypes, storage.RecordTypeInfo{
			ID:            recordType.ID,
			DeveloperName: recordType.DeveloperName,
			Name:          recordType.Name,
			Active:        recordType.Active,
			Available:     recordType.Available,
			Default:       recordType.Default,
			Description:   recordType.Description,
		})
	}
	storage.EnsureRecordTypeIDField(&definition)
	return definition
}

func FromObjectDefinition(definition storage.ObjectDefinition) DescribeSObjectResult {
	describe := DescribeSObjectResult{
		Name:            definition.APIName,
		Label:           definition.Label,
		PluralLabel:     definition.PluralLabel,
		KeyPrefix:       definition.KeyPrefix,
		Fields:          make(map[string]DescribeFieldResult, len(definition.Fields)),
		Relationships:   append([]storage.Relationship(nil), definition.Relations...),
		RecordTypes:     make([]DescribeRecordTypeInfo, 0, len(definition.RecordTypes)),
		ValidationRules: append([]storage.ValidationRule(nil), definition.ValidationRules...),
	}
	if definition.Metadata != nil {
		describe.Metadata = make(map[string]string, len(definition.Metadata))
		for key, value := range definition.Metadata {
			describe.Metadata[key] = value
		}
	}
	for name, field := range definition.Fields {
		describe.Fields[name] = DescribeFieldResult{
			Name:               field.APIName,
			Type:               field.Type,
			DisplayType:        field.DisplayType,
			Label:              labelOrName(field.Label, field.APIName),
			Length:             field.Length,
			Precision:          field.Precision,
			Scale:              field.Scale,
			Formula:            field.Formula,
			AutoNumber:         field.AutoNumber,
			DisplayFormat:      field.DisplayFormat,
			SummarizedField:    field.SummarizedField,
			SummaryForeignKey:  field.SummaryForeignKey,
			SummaryOperation:   field.SummaryOperation,
			SummaryFilterItems: append([]storage.SummaryFilterItem(nil), field.SummaryFilterItems...),
			ReferenceTo:        append([]string(nil), field.ReferenceTo...),
			RelationshipName:   field.RelationshipName,
			DefaultValue:       field.DefaultValue,
			Required:           field.Required,
			ExternalID:         field.ExternalID,
			Unique:             field.Unique,
			Encrypted:          field.Encrypted,
			PicklistValues:     append([]storage.PicklistValue(nil), field.PicklistValues...),
		}
	}
	for _, recordType := range definition.RecordTypes {
		describe.RecordTypes = append(describe.RecordTypes, DescribeRecordTypeInfo{
			ID:            recordType.ID,
			DeveloperName: recordType.DeveloperName,
			Name:          recordType.Name,
			Active:        recordType.Active,
			Available:     recordType.Available,
			Default:       recordType.Default,
			Description:   recordType.Description,
		})
	}
	if len(describe.RecordTypes) > 0 {
		describe.Fields["RecordTypeId"] = DescribeFieldResult{
			Name:             "RecordTypeId",
			Type:             storage.FieldReference,
			DisplayType:      string(storage.FieldReference),
			Label:            "Record Type ID",
			ReferenceTo:      []string{"RecordType"},
			RelationshipName: "RecordType",
		}
	}
	return describe
}

func ensureDescribeField(fields map[string]DescribeFieldResult, name, typ, label string) {
	if _, ok := fields[name]; ok {
		return
	}
	fields[name] = DescribeFieldResult{Name: name, Type: storageFieldType(typ), DisplayType: displayFieldType(typ), Label: label}
}

func displayFieldType(raw string) string {
	switch raw {
	case "Number":
		return "DOUBLE"
	case "Currency":
		return "CURRENCY"
	case "Percent":
		return "PERCENT"
	default:
		return string(storageFieldType(raw))
	}
}

func storagePicklistValues(values []schema.PicklistValue) []storage.PicklistValue {
	out := make([]storage.PicklistValue, 0, len(values))
	for _, value := range values {
		out = append(out, storage.PicklistValue{
			Value:   value.FullName,
			Label:   value.Label,
			Default: value.Default,
			Active:  value.Active,
		})
	}
	return out
}

func storageSummaryFilters(values []schema.SummaryFilter) []storage.SummaryFilterItem {
	out := make([]storage.SummaryFilterItem, 0, len(values))
	for _, value := range values {
		out = append(out, storage.SummaryFilterItem{
			Field:     value.Field,
			Operation: value.Operation,
			Value:     value.Value,
		})
	}
	return out
}

func cloneValues(in map[string]storage.Value) map[string]storage.Value {
	if in == nil {
		return nil
	}
	out := make(map[string]storage.Value, len(in))
	for name, value := range in {
		out[name] = value.Clone()
	}
	return out
}

func cloneBools(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for name, value := range in {
		out[name] = value
	}
	return out
}

func objectNames(objects []schema.Object) []string {
	names := make([]string, 0, len(objects))
	for _, object := range objects {
		names = append(names, object.Name)
	}
	return names
}

func referenceTargets(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func labelOrName(label, name string) string {
	if label != "" {
		return label
	}
	return name
}

func storageFieldType(raw string) storage.FieldType {
	switch raw {
	case "Text", "TextArea", "LongTextArea", "Email", "Phone", "Url", "EncryptedText":
		return storage.FieldString
	case "Picklist", "MultiselectPicklist":
		return storage.FieldPicklist
	case "Checkbox":
		return storage.FieldBoolean
	case "Number", "Currency", "Percent":
		return storage.FieldDecimal
	case "Date":
		return storage.FieldDate
	case "DateTime":
		return storage.FieldDateTime
	case "Lookup", "MasterDetail", "MetadataRelationship":
		return storage.FieldReference
	case "Id":
		return storage.FieldID
	case "Base64":
		return storage.FieldBlob
	case "Formula":
		return storage.FieldCalculated
	case "Summary":
		return storage.FieldSummary
	default:
		return storage.FieldAny
	}
}
