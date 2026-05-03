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
	if value.Kind == storage.ValueNull {
		delete(v.Fields, field)
		v.ExplicitNulls[field] = true
		return
	}
	v.Fields[field] = value.Clone()
	delete(v.ExplicitNulls, field)
}

func (v Value) Get(field string) (storage.Value, bool) {
	if v.ExplicitNulls[field] {
		return storage.NullValue(), true
	}
	value, ok := v.Fields[field]
	return value.Clone(), ok
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
	Name          string                         `json:"name"`
	Label         string                         `json:"label,omitempty"`
	PluralLabel   string                         `json:"pluralLabel,omitempty"`
	KeyPrefix     string                         `json:"keyPrefix,omitempty"`
	Fields        map[string]DescribeFieldResult `json:"fields,omitempty"`
	Relationships []storage.Relationship         `json:"relationships,omitempty"`
	RecordTypes   []DescribeRecordTypeInfo       `json:"recordTypes,omitempty"`
}

type DescribeFieldResult struct {
	Name                  string                  `json:"name"`
	Type                  storage.FieldType       `json:"type"`
	Label                 string                  `json:"label,omitempty"`
	ReferenceTo           []string                `json:"referenceTo,omitempty"`
	RelationshipName      string                  `json:"relationshipName,omitempty"`
	ChildRelationshipName string                  `json:"childRelationshipName,omitempty"`
	DeleteConstraint      string                  `json:"deleteConstraint,omitempty"`
	Required              bool                    `json:"required,omitempty"`
	ExternalID            bool                    `json:"externalId,omitempty"`
	Unique                bool                    `json:"unique,omitempty"`
	PicklistValues        []storage.PicklistValue `json:"picklistValues,omitempty"`
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
		for _, field := range object.Fields {
			describe.Fields[field.Name] = DescribeFieldResult{
				Name:                  field.Name,
				Type:                  storageFieldType(field.Type),
				Label:                 field.Label,
				ReferenceTo:           referenceTargets(field.ReferenceTo),
				RelationshipName:      field.RelationshipName,
				ChildRelationshipName: field.ChildRelationshipName,
				DeleteConstraint:      field.DeleteConstraint,
				Required:              field.Required,
				ExternalID:            field.ExternalID,
				Unique:                field.Unique,
				PicklistValues:        storagePicklistValues(field.PicklistValues),
			}
			if field.ReferenceTo != "" {
				describe.Relationships = append(describe.Relationships, storage.Relationship{
					Field:              field.Name,
					ParentObjects:      referenceTargets(field.ReferenceTo),
					ParentRelationship: field.RelationshipName,
					ChildRelationship:  field.ChildRelationshipName,
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
		registry.Objects[object.Name] = describe
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
	return out
}

func ToObjectDefinition(describe DescribeSObjectResult) storage.ObjectDefinition {
	definition := storage.ObjectDefinition{
		APIName:     describe.Name,
		Label:       describe.Label,
		PluralLabel: describe.PluralLabel,
		KeyPrefix:   describe.KeyPrefix,
		Fields:      make(map[string]storage.Field, len(describe.Fields)),
		Relations:   append([]storage.Relationship(nil), describe.Relationships...),
		RecordTypes: make([]storage.RecordTypeInfo, 0, len(describe.RecordTypes)),
	}
	for name, field := range describe.Fields {
		definition.Fields[name] = storage.Field{
			APIName:          field.Name,
			Type:             field.Type,
			Required:         field.Required,
			ExternalID:       field.ExternalID,
			Unique:           field.Unique,
			ReferenceTo:      append([]string(nil), field.ReferenceTo...),
			RelationshipName: field.RelationshipName,
			PicklistValues:   append([]storage.PicklistValue(nil), field.PicklistValues...),
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
	return definition
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

func referenceTargets(raw string) []string {
	if raw == "" {
		return nil
	}
	return []string{raw}
}

func storageFieldType(raw string) storage.FieldType {
	switch raw {
	case "Text", "TextArea", "LongTextArea", "Email", "Phone", "Url":
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
	case "Lookup", "MasterDetail":
		return storage.FieldReference
	case "Id":
		return storage.FieldID
	default:
		return storage.FieldAny
	}
}
