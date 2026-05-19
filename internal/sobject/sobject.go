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
	SharingModel    string                         `json:"sharingModel,omitempty"`
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
	CompoundFieldName     string                      `json:"compoundFieldName,omitempty"`
	AutoNumber            bool                        `json:"autoNumber,omitempty"`
	DisplayFormat         string                      `json:"displayFormat,omitempty"`
	SummarizedField       string                      `json:"summarizedField,omitempty"`
	SummaryForeignKey     string                      `json:"summaryForeignKey,omitempty"`
	SummaryOperation      string                      `json:"summaryOperation,omitempty"`
	SummaryFilterItems    []storage.SummaryFilterItem `json:"summaryFilterItems,omitempty"`
	Nillable              *bool                       `json:"nillable,omitempty"`
	DefaultedOnCreate     *bool                       `json:"defaultedOnCreate,omitempty"`
	Accessible            *bool                       `json:"accessible,omitempty"`
	Createable            *bool                       `json:"createable,omitempty"`
	Updateable            *bool                       `json:"updateable,omitempty"`
	Filterable            *bool                       `json:"filterable,omitempty"`
	Groupable             *bool                       `json:"groupable,omitempty"`
	Sortable              *bool                       `json:"sortable,omitempty"`
	Aggregatable          *bool                       `json:"aggregatable,omitempty"`
	Permissionable        *bool                       `json:"permissionable,omitempty"`
	DeprecatedAndHidden   *bool                       `json:"deprecatedAndHidden,omitempty"`
	ReferenceTo           []string                    `json:"referenceTo,omitempty"`
	RelationshipName      string                      `json:"relationshipName,omitempty"`
	ChildRelationshipName string                      `json:"childRelationshipName,omitempty"`
	DeleteConstraint      string                      `json:"deleteConstraint,omitempty"`
	DefaultValue          string                      `json:"defaultValue,omitempty"`
	Required              bool                        `json:"required,omitempty"`
	ExternalID            bool                        `json:"externalId,omitempty"`
	Unique                bool                        `json:"unique,omitempty"`
	Encrypted             bool                        `json:"encrypted,omitempty"`
	CaseSensitive         bool                        `json:"caseSensitive,omitempty"`
	PicklistValues        []storage.PicklistValue     `json:"picklistValues,omitempty"`
}

type DescribeRecordTypeInfo struct {
	ID               storage.ID        `json:"id,omitempty"`
	DeveloperName    string            `json:"developerName"`
	Name             string            `json:"name,omitempty"`
	Active           bool              `json:"active,omitempty"`
	Available        bool              `json:"available,omitempty"`
	Default          bool              `json:"default,omitempty"`
	Description      string            `json:"description,omitempty"`
	PicklistDefaults map[string]string `json:"picklistDefaults,omitempty"`
}

func BuildDescribeRegistry(s schema.Schema) DescribeRegistry {
	objects := mergeSchemaObjects(s.Objects)
	sort.Slice(objects, func(i, j int) bool { return objects[i].Name < objects[j].Name })
	prefixes := storage.AssignDeterministicPrefixes(objectNames(objects), nil)

	registry := DescribeRegistry{Objects: make(map[string]DescribeSObjectResult, len(objects))}
	recordTypeIDs := storage.NewIDGenerator(map[string]string{"RecordType": storage.StandardKeyPrefixes()["RecordType"]})
	for _, object := range objects {
		describe := DescribeSObjectResult{
			Name:         object.Name,
			Label:        object.Label,
			PluralLabel:  object.PluralLabel,
			KeyPrefix:    prefixes[object.Name],
			SharingModel: object.SharingModel,
			Fields:       make(map[string]DescribeFieldResult, len(object.Fields)),
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
			childRelationshipName := field.ChildRelationshipName
			references := referenceTargets(field.ReferenceTo)
			if len(references) != 0 && childRelationshipName == "" {
				parentRelationship := storage.ParentRelationshipName(storage.Field{
					APIName:          field.Name,
					RelationshipName: field.RelationshipName,
				})
				if !strings.EqualFold(field.RelationshipName, parentRelationship) {
					childRelationshipName = apexChildRelationshipName(field.RelationshipName)
				}
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
				ChildRelationshipName: childRelationshipName,
				DeleteConstraint:      field.DeleteConstraint,
				DefaultValue:          field.DefaultValue,
				Required:              field.Required,
				ExternalID:            field.ExternalID,
				Unique:                field.Unique,
				Encrypted:             field.Encrypted,
				PicklistValues:        storagePicklistValues(field.PicklistValues),
			}
			if len(references) != 0 {
				parentRelationship := storage.ParentRelationshipName(storage.Field{
					APIName:          field.Name,
					RelationshipName: field.RelationshipName,
				})
				childRelationship := childRelationshipName
				if childRelationship == "" && !strings.EqualFold(field.RelationshipName, parentRelationship) {
					childRelationship = apexChildRelationshipName(field.RelationshipName)
				}
				describe.Relationships = append(describe.Relationships, storage.Relationship{
					Field:              field.Name,
					ParentObjects:      references,
					ParentRelationship: parentRelationship,
					ChildRelationship:  childRelationship,
					Polymorphic:        len(references) > 1,
					CascadeDelete:      strings.EqualFold(field.DeleteConstraint, "Cascade") || strings.EqualFold(field.Type, "MasterDetail"),
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
				ID:               id,
				DeveloperName:    recordType.DeveloperName,
				Name:             recordType.Label,
				Active:           recordType.Active,
				Available:        recordType.Active,
				Default:          recordType.Default,
				Description:      recordType.Description,
				PicklistDefaults: cloneStringMap(recordType.PicklistDefaults),
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

func mergeSchemaObjects(objects []schema.Object) []schema.Object {
	if len(objects) < 2 {
		out := make([]schema.Object, len(objects))
		copy(out, objects)
		return out
	}
	byName := make(map[string]int, len(objects))
	out := make([]schema.Object, 0, len(objects))
	for _, object := range objects {
		key := strings.ToLower(strings.TrimSpace(object.Name))
		if key == "" {
			out = append(out, object)
			continue
		}
		if idx, ok := byName[key]; ok {
			out[idx] = mergeSchemaObject(out[idx], object)
			continue
		}
		byName[key] = len(out)
		out = append(out, object)
	}
	return out
}

func mergeSchemaObject(base, overlay schema.Object) schema.Object {
	if strings.TrimSpace(base.Name) == "" {
		base.Name = overlay.Name
	}
	if overlay.Label != "" {
		base.Label = overlay.Label
	}
	if overlay.PluralLabel != "" {
		base.PluralLabel = overlay.PluralLabel
	}
	if overlay.SharingModel != "" {
		base.SharingModel = overlay.SharingModel
	}
	if overlay.CustomSettingsType != "" {
		base.CustomSettingsType = overlay.CustomSettingsType
	}
	if overlay.NameField.Type != "" || overlay.NameField.Label != "" || overlay.NameField.DisplayFormat != "" {
		base.NameField = overlay.NameField
	}
	base.Fields = mergeSchemaFields(base.Fields, overlay.Fields)
	base.RecordTypes = mergeSchemaRecordTypes(base.RecordTypes, overlay.RecordTypes)
	base.ValidationRules = mergeSchemaValidationRules(base.ValidationRules, overlay.ValidationRules)
	return base
}

func mergeSchemaFields(base, overlay []schema.Field) []schema.Field {
	byName := make(map[string]int, len(base)+len(overlay))
	out := append([]schema.Field(nil), base...)
	for i, field := range out {
		byName[strings.ToLower(strings.TrimSpace(field.Name))] = i
	}
	for _, field := range overlay {
		key := strings.ToLower(strings.TrimSpace(field.Name))
		if idx, ok := byName[key]; key != "" && ok {
			out[idx] = field
			continue
		}
		if key != "" {
			byName[key] = len(out)
		}
		out = append(out, field)
	}
	return out
}

func mergeSchemaRecordTypes(base, overlay []schema.RecordType) []schema.RecordType {
	byName := make(map[string]int, len(base)+len(overlay))
	out := append([]schema.RecordType(nil), base...)
	for i, recordType := range out {
		byName[strings.ToLower(strings.TrimSpace(recordType.DeveloperName))] = i
	}
	for _, recordType := range overlay {
		key := strings.ToLower(strings.TrimSpace(recordType.DeveloperName))
		if idx, ok := byName[key]; key != "" && ok {
			out[idx] = recordType
			continue
		}
		if key != "" {
			byName[key] = len(out)
		}
		out = append(out, recordType)
	}
	return out
}

func mergeSchemaValidationRules(base, overlay []schema.ValidationRule) []schema.ValidationRule {
	byName := make(map[string]int, len(base)+len(overlay))
	out := append([]schema.ValidationRule(nil), base...)
	for i, rule := range out {
		byName[strings.ToLower(strings.TrimSpace(rule.Name))] = i
	}
	for _, rule := range overlay {
		key := strings.ToLower(strings.TrimSpace(rule.Name))
		if idx, ok := byName[key]; key != "" && ok {
			out[idx] = rule
			continue
		}
		if key != "" {
			byName[key] = len(out)
		}
		out = append(out, rule)
	}
	return out
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
		SharingModel:    describe.SharingModel,
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
			APIName:               field.Name,
			Label:                 labelOrName(field.Label, field.Name),
			Type:                  field.Type,
			DisplayType:           field.DisplayType,
			Length:                field.Length,
			Precision:             field.Precision,
			Scale:                 field.Scale,
			Formula:               field.Formula,
			CompoundFieldName:     field.CompoundFieldName,
			DefaultValue:          field.DefaultValue,
			AutoNumber:            field.AutoNumber,
			DisplayFormat:         field.DisplayFormat,
			SummarizedField:       field.SummarizedField,
			SummaryForeignKey:     field.SummaryForeignKey,
			SummaryOperation:      field.SummaryOperation,
			SummaryFilterItems:    append([]storage.SummaryFilterItem(nil), field.SummaryFilterItems...),
			Required:              field.Required,
			Nillable:              field.Nillable,
			DefaultedOnCreate:     field.DefaultedOnCreate,
			Accessible:            field.Accessible,
			Createable:            field.Createable,
			Updateable:            field.Updateable,
			Filterable:            field.Filterable,
			Groupable:             field.Groupable,
			Sortable:              field.Sortable,
			Aggregatable:          field.Aggregatable,
			Permissionable:        field.Permissionable,
			DeprecatedAndHidden:   field.DeprecatedAndHidden,
			ExternalID:            field.ExternalID,
			Unique:                field.Unique,
			Encrypted:             field.Encrypted,
			CaseSensitive:         field.CaseSensitive,
			ReferenceTo:           append([]string(nil), field.ReferenceTo...),
			RelationshipName:      field.RelationshipName,
			ChildRelationshipName: field.ChildRelationshipName,
			PicklistValues:        append([]storage.PicklistValue(nil), field.PicklistValues...),
		}
	}
	for _, recordType := range describe.RecordTypes {
		definition.RecordTypes = append(definition.RecordTypes, storage.RecordTypeInfo{
			ID:               recordType.ID,
			DeveloperName:    recordType.DeveloperName,
			Name:             recordType.Name,
			Active:           recordType.Active,
			Available:        recordType.Available,
			Default:          recordType.Default,
			Description:      recordType.Description,
			PicklistDefaults: cloneStringMap(recordType.PicklistDefaults),
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
		SharingModel:    definition.SharingModel,
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
			Name:                  field.APIName,
			Type:                  field.Type,
			DisplayType:           field.DisplayType,
			Label:                 labelOrName(field.Label, field.APIName),
			Length:                field.Length,
			Precision:             field.Precision,
			Scale:                 field.Scale,
			Formula:               field.Formula,
			CompoundFieldName:     field.CompoundFieldName,
			AutoNumber:            field.AutoNumber,
			DisplayFormat:         field.DisplayFormat,
			SummarizedField:       field.SummarizedField,
			SummaryForeignKey:     field.SummaryForeignKey,
			SummaryOperation:      field.SummaryOperation,
			SummaryFilterItems:    append([]storage.SummaryFilterItem(nil), field.SummaryFilterItems...),
			Nillable:              field.Nillable,
			DefaultedOnCreate:     field.DefaultedOnCreate,
			Accessible:            field.Accessible,
			Createable:            field.Createable,
			Updateable:            field.Updateable,
			Filterable:            field.Filterable,
			Groupable:             field.Groupable,
			Sortable:              field.Sortable,
			Aggregatable:          field.Aggregatable,
			Permissionable:        field.Permissionable,
			DeprecatedAndHidden:   field.DeprecatedAndHidden,
			ReferenceTo:           append([]string(nil), field.ReferenceTo...),
			RelationshipName:      field.RelationshipName,
			ChildRelationshipName: field.ChildRelationshipName,
			DefaultValue:          field.DefaultValue,
			Required:              field.Required,
			ExternalID:            field.ExternalID,
			Unique:                field.Unique,
			Encrypted:             field.Encrypted,
			CaseSensitive:         field.CaseSensitive,
			PicklistValues:        append([]storage.PicklistValue(nil), field.PicklistValues...),
		}
	}
	for _, recordType := range definition.RecordTypes {
		describe.RecordTypes = append(describe.RecordTypes, DescribeRecordTypeInfo{
			ID:               recordType.ID,
			DeveloperName:    recordType.DeveloperName,
			Name:             recordType.Name,
			Active:           recordType.Active,
			Available:        recordType.Available,
			Default:          recordType.Default,
			Description:      recordType.Description,
			PicklistDefaults: cloneStringMap(recordType.PicklistDefaults),
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
	case "TextArea", "LongTextArea":
		return "TEXTAREA"
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

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
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

func apexChildRelationshipName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasSuffix(name, "__r") {
		return name
	}
	return name + "__r"
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
	case "Location":
		return storage.FieldLocation
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
