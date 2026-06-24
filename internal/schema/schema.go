package schema

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
)

type Schema struct {
	Objects               []Object               `json:"objects"`
	CustomMetadataRecords []CustomMetadataRecord `json:"customMetadataRecords,omitempty"`
}

type Object struct {
	Name               string           `json:"name"`
	Partial            bool             `json:"partial,omitempty"`
	Label              string           `json:"label,omitempty"`
	PluralLabel        string           `json:"pluralLabel,omitempty"`
	SharingModel       string           `json:"sharingModel,omitempty"`
	CustomSettingsType string           `json:"customSettingsType,omitempty"`
	NameField          NameField        `json:"nameField,omitempty"`
	Fields             []Field          `json:"fields,omitempty"`
	RecordTypes        []RecordType     `json:"recordTypes,omitempty"`
	ValidationRules    []ValidationRule `json:"validationRules,omitempty"`
}

type NameField struct {
	Label         string `json:"label,omitempty"`
	Type          string `json:"type,omitempty"`
	DisplayFormat string `json:"displayFormat,omitempty"`
}

type Field struct {
	Name                  string             `json:"name"`
	Label                 string             `json:"label,omitempty"`
	InlineHelpText        string             `json:"inlineHelpText,omitempty"`
	Type                  string             `json:"type,omitempty"`
	Length                int                `json:"length,omitempty"`
	Precision             int                `json:"precision,omitempty"`
	Scale                 int                `json:"scale,omitempty"`
	ReferenceTo           []string           `json:"referenceTo,omitempty"`
	RelationshipName      string             `json:"relationshipName,omitempty"`
	ChildRelationshipName string             `json:"childRelationshipName,omitempty"`
	DeleteConstraint      string             `json:"deleteConstraint,omitempty"`
	DefaultValue          string             `json:"defaultValue,omitempty"`
	Required              bool               `json:"required,omitempty"`
	ExternalID            bool               `json:"externalId,omitempty"`
	IDLookup              bool               `json:"idLookup,omitempty"`
	Unique                bool               `json:"unique,omitempty"`
	Encrypted             bool               `json:"encrypted,omitempty"`
	Formula               string             `json:"formula,omitempty"`
	SummarizedField       string             `json:"summarizedField,omitempty"`
	SummaryForeignKey     string             `json:"summaryForeignKey,omitempty"`
	SummaryOperation      string             `json:"summaryOperation,omitempty"`
	SummaryFilterItems    []SummaryFilter    `json:"summaryFilterItems,omitempty"`
	FilteredLookupInfo    FilteredLookupInfo `json:"filteredLookupInfo,omitempty"`
	PicklistController    string             `json:"picklistController,omitempty"`
	PicklistValueSettings []PicklistSetting  `json:"picklistValueSettings,omitempty"`
	ValueSetName          string             `json:"valueSetName,omitempty"`
	RestrictedPicklist    bool               `json:"restrictedPicklist,omitempty"`
	PicklistValues        []PicklistValue    `json:"picklistValues,omitempty"`
}

type FilteredLookupInfo struct {
	ControllingFields []string `json:"controllingFields,omitempty"`
	Dependent         bool     `json:"dependent,omitempty"`
	OptionalFilter    bool     `json:"optionalFilter,omitempty"`
}

type SummaryFilter struct {
	Field     string `json:"field,omitempty"`
	Operation string `json:"operation,omitempty"`
	Value     string `json:"value,omitempty"`
}

type PicklistValue struct {
	FullName string `json:"fullName"`
	Label    string `json:"label,omitempty"`
	Default  bool   `json:"default,omitempty"`
	Active   bool   `json:"active,omitempty"`
}

type PicklistSetting struct {
	ValueName              string   `json:"valueName,omitempty"`
	ControllingFieldValues []string `json:"controllingFieldValues,omitempty"`
}

type RecordType struct {
	DeveloperName    string            `json:"developerName"`
	Label            string            `json:"label,omitempty"`
	Active           bool              `json:"active,omitempty"`
	Default          bool              `json:"default,omitempty"`
	Description      string            `json:"description,omitempty"`
	PicklistDefaults map[string]string `json:"picklistDefaults,omitempty"`
}

type ValidationRule struct {
	Name                  string `json:"name"`
	Namespace             string `json:"namespace,omitempty"`
	Active                bool   `json:"active,omitempty"`
	ErrorConditionFormula string `json:"errorConditionFormula,omitempty"`
	ErrorMessage          string `json:"errorMessage,omitempty"`
	ErrorDisplayField     string `json:"errorDisplayField,omitempty"`
}

type CustomMetadataRecord struct {
	FullName      string                `json:"fullName"`
	ObjectName    string                `json:"objectName"`
	DeveloperName string                `json:"developerName"`
	Label         string                `json:"label,omitempty"`
	Protected     bool                  `json:"protected,omitempty"`
	Values        []CustomMetadataValue `json:"values,omitempty"`
	File          string                `json:"file,omitempty"`
}

type CustomMetadataValue struct {
	Field string `json:"field"`
	Value string `json:"value,omitempty"`
	Nil   bool   `json:"nil,omitempty"`
}

type customObjectXML struct {
	XMLName            xml.Name            `xml:"CustomObject"`
	Label              string              `xml:"label"`
	PluralLabel        string              `xml:"pluralLabel"`
	SharingModel       string              `xml:"sharingModel"`
	CustomSettingsType string              `xml:"customSettingsType"`
	NameField          nameFieldXML        `xml:"nameField"`
	Fields             []customFieldXML    `xml:"fields"`
	RecordTypes        []recordTypeXML     `xml:"recordTypes"`
	ValidationRules    []validationRuleXML `xml:"validationRules"`
}

type nameFieldXML struct {
	Label         string `xml:"label"`
	Type          string `xml:"type"`
	DisplayFormat string `xml:"displayFormat"`
}

type customFieldXML struct {
	FullName              string             `xml:"fullName"`
	Label                 string             `xml:"label"`
	InlineHelpText        string             `xml:"inlineHelpText"`
	Type                  string             `xml:"type"`
	Length                int                `xml:"length"`
	Precision             int                `xml:"precision"`
	Scale                 int                `xml:"scale"`
	ReferenceTo           []string           `xml:"referenceTo"`
	RelationshipName      string             `xml:"relationshipName"`
	ChildRelationshipName string             `xml:"childRelationshipName"`
	DeleteConstraint      string             `xml:"deleteConstraint"`
	DefaultValue          string             `xml:"defaultValue"`
	Required              bool               `xml:"required"`
	ExternalID            bool               `xml:"externalId"`
	IDLookup              bool               `xml:"idLookup"`
	Unique                bool               `xml:"unique"`
	Formula               string             `xml:"formula"`
	SummarizedField       string             `xml:"summarizedField"`
	SummaryForeignKey     string             `xml:"summaryForeignKey"`
	SummaryOperation      string             `xml:"summaryOperation"`
	SummaryFilterItems    []summaryFilterXML `xml:"summaryFilterItems"`
	LookupFilter          lookupFilterXML    `xml:"lookupFilter"`
	ValueSet              valueSetXML        `xml:"valueSet"`
}

type lookupFilterXML struct {
	Active      *bool                 `xml:"active"`
	FilterItems []lookupFilterItemXML `xml:"filterItems"`
	IsOptional  *bool                 `xml:"isOptional"`
}

type lookupFilterItemXML struct {
	Field string `xml:"field"`
}

type summaryFilterXML struct {
	Field     string `xml:"field"`
	Operation string `xml:"operation"`
	Value     string `xml:"value"`
}

type recordTypeXML struct {
	FullName       string                  `xml:"fullName"`
	Label          string                  `xml:"label"`
	Active         *bool                   `xml:"active"`
	Default        bool                    `xml:"default"`
	Description    string                  `xml:"description"`
	PicklistValues []recordTypePicklistXML `xml:"picklistValues"`
}

type recordTypePicklistXML struct {
	Picklist string                   `xml:"picklist"`
	Values   []recordTypePickValueXML `xml:"values"`
}

type recordTypePickValueXML struct {
	FullName string `xml:"fullName"`
	Default  bool   `xml:"default"`
}

type validationRuleXML struct {
	FullName              string `xml:"fullName"`
	Active                bool   `xml:"active"`
	ErrorConditionFormula string `xml:"errorConditionFormula"`
	ErrorMessage          string `xml:"errorMessage"`
	ErrorDisplayField     string `xml:"errorDisplayField"`
}

type valueSetXML struct {
	ControllingField string                `xml:"controllingField"`
	Definition       valueSetDefinitionXML `xml:"valueSetDefinition"`
	Name             string                `xml:"valueSetName"`
	Restricted       bool                  `xml:"restricted"`
	ValueSettings    []valueSettingXML     `xml:"valueSettings"`
}

type valueSettingXML struct {
	ControllingFieldValues []string `xml:"controllingFieldValue"`
	ValueName              string   `xml:"valueName"`
}

type valueSetDefinitionXML struct {
	Values []picklistValueXML `xml:"value"`
}

type valueSetFileXML struct {
	CustomValues   []picklistValueXML `xml:"customValue"`
	StandardValues []picklistValueXML `xml:"standardValue"`
}

type picklistValueXML struct {
	FullName string `xml:"fullName"`
	Label    string `xml:"label"`
	Default  bool   `xml:"default"`
	IsActive *bool  `xml:"isActive"`
}

type customMetadataXML struct {
	Label     string                   `xml:"label"`
	Protected bool                     `xml:"protected"`
	Values    []customMetadataValueXML `xml:"values"`
}

type customMetadataValueXML struct {
	Field string                `xml:"field"`
	Value customMetadataTextXML `xml:"value"`
}

type customMetadataTextXML struct {
	Text string `xml:",chardata"`
	Nil  bool   `xml:"nil,attr"`
}

func LoadProject(p project.Project) (Schema, error) {
	byName := make(map[string]*Object)
	valueSets, err := loadValueSets(p)
	if err != nil {
		return Schema{}, err
	}

	for _, path := range p.ObjectFiles {
		object, err := loadObject(path, p)
		if err != nil {
			return Schema{}, err
		}
		if existing, ok := byName[object.Name]; ok {
			mergeObjectMetadata(existing, object)
		} else {
			byName[object.Name] = &object
		}
	}

	for _, path := range p.FieldFiles {
		objectName := namespaceProjectObjectName(p.Namespace, remapProjectAPIName(p, objectNameFromFieldPath(path)))
		if objectName == "" {
			continue
		}
		field, err := loadField(path)
		if err != nil {
			return Schema{}, err
		}
		field = remapProjectField(p, field)
		field = namespaceObjectField(p.Namespace, objectName, field)
		applyValueSet(&field, valueSets)
		object := byName[objectName]
		if object == nil {
			object = &Object{Name: objectName, Partial: true}
			byName[objectName] = object
		}
		object.Fields = append(object.Fields, field)
	}

	for _, path := range p.RecordTypeFiles {
		objectName := namespaceProjectObjectName(p.Namespace, remapProjectAPIName(p, objectNameFromRecordTypePath(path)))
		if objectName == "" {
			continue
		}
		recordType, err := loadRecordType(path)
		if err != nil {
			return Schema{}, err
		}
		object := byName[objectName]
		if object == nil {
			object = &Object{Name: objectName, Partial: true}
			byName[objectName] = object
		}
		object.RecordTypes = append(object.RecordTypes, recordType)
	}

	for _, path := range p.ValidationRuleFiles {
		objectName := namespaceProjectObjectName(p.Namespace, remapProjectAPIName(p, objectNameFromValidationRulePath(path)))
		if objectName == "" {
			continue
		}
		rule, err := loadValidationRule(path, validationRuleSourceNamespace(p.Namespace, objectName))
		if err != nil {
			return Schema{}, err
		}
		rule = remapProjectValidationRule(p, rule)
		object := byName[objectName]
		if object == nil {
			object = &Object{Name: objectName, Partial: true}
			byName[objectName] = object
		}
		object.ValidationRules = append(object.ValidationRules, rule)
	}

	addReferencedCustomObjects(byName)

	records := make([]CustomMetadataRecord, 0, len(p.CustomMetadataFiles))
	for _, path := range p.CustomMetadataFiles {
		record, err := loadCustomMetadataRecord(path)
		if err != nil {
			return Schema{}, err
		}
		record = remapProjectCustomMetadataRecord(p, record)
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].ObjectName != records[j].ObjectName {
			return records[i].ObjectName < records[j].ObjectName
		}
		if records[i].DeveloperName != records[j].DeveloperName {
			return records[i].DeveloperName < records[j].DeveloperName
		}
		return records[i].File < records[j].File
	})

	out := Schema{Objects: make([]Object, 0, len(byName)), CustomMetadataRecords: records}
	for _, object := range byName {
		for i := range object.Fields {
			applyValueSet(&object.Fields[i], valueSets)
		}
		sort.Slice(object.Fields, func(i, j int) bool {
			return object.Fields[i].Name < object.Fields[j].Name
		})
		sort.Slice(object.RecordTypes, func(i, j int) bool {
			return object.RecordTypes[i].DeveloperName < object.RecordTypes[j].DeveloperName
		})
		sort.Slice(object.ValidationRules, func(i, j int) bool {
			return object.ValidationRules[i].Name < object.ValidationRules[j].Name
		})
		out.Objects = append(out.Objects, *object)
	}
	sort.Slice(out.Objects, func(i, j int) bool {
		return out.Objects[i].Name < out.Objects[j].Name
	})
	return out, nil
}

func mergeObjectMetadata(dst *Object, src Object) {
	if dst.Label == "" {
		dst.Label = src.Label
	}
	if dst.PluralLabel == "" {
		dst.PluralLabel = src.PluralLabel
	}
	if dst.SharingModel == "" {
		dst.SharingModel = src.SharingModel
	}
	if dst.CustomSettingsType == "" {
		dst.CustomSettingsType = src.CustomSettingsType
	}
	if dst.NameField == (NameField{}) {
		dst.NameField = src.NameField
	}
	for _, field := range src.Fields {
		if !hasFieldNamed(dst.Fields, field.Name) {
			dst.Fields = append(dst.Fields, field)
		}
	}
	for _, recordType := range src.RecordTypes {
		if !hasRecordTypeNamed(dst.RecordTypes, recordType.DeveloperName) {
			dst.RecordTypes = append(dst.RecordTypes, recordType)
		}
	}
	for _, rule := range src.ValidationRules {
		if !hasValidationRuleNamed(dst.ValidationRules, rule.Name) {
			dst.ValidationRules = append(dst.ValidationRules, rule)
		}
	}
}

func hasFieldNamed(fields []Field, name string) bool {
	for _, field := range fields {
		if strings.EqualFold(field.Name, name) {
			return true
		}
	}
	return false
}

func hasRecordTypeNamed(recordTypes []RecordType, name string) bool {
	for _, recordType := range recordTypes {
		if strings.EqualFold(recordType.DeveloperName, name) {
			return true
		}
	}
	return false
}

func hasValidationRuleNamed(rules []ValidationRule, name string) bool {
	for _, rule := range rules {
		if strings.EqualFold(rule.Name, name) {
			return true
		}
	}
	return false
}

func addReferencedCustomObjects(byName map[string]*Object) {
	referenced := make(map[string]bool)
	for _, object := range byName {
		for _, field := range object.Fields {
			for _, referenceTo := range field.ReferenceTo {
				name := strings.TrimSpace(referenceTo)
				if name == "" || !isCustomEntityName(name) {
					continue
				}
				referenced[name] = true
			}
		}
	}
	for name := range referenced {
		if _, ok := byName[name]; ok {
			continue
		}
		byName[name] = &Object{Name: name, Partial: true}
	}
}

func isCustomEntityName(name string) bool {
	for _, suffix := range []string{"__c", "__mdt", "__e", "__b"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func loadValidationRule(path, namespace string) (ValidationRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ValidationRule{}, err
	}
	var raw validationRuleXML
	if err := xml.Unmarshal(escapeBareAmpersands(data), &raw); err != nil {
		return ValidationRule{}, err
	}
	name := raw.FullName
	if name == "" {
		name = trimMetadataSuffix(filepath.Base(path), ".validationRule-meta.xml")
	}
	return ValidationRule{
		Name:                  name,
		Namespace:             namespace,
		Active:                raw.Active,
		ErrorConditionFormula: strings.TrimSpace(raw.ErrorConditionFormula),
		ErrorMessage:          raw.ErrorMessage,
		ErrorDisplayField:     raw.ErrorDisplayField,
	}, nil
}

func validationRuleFromXML(raw validationRuleXML, fallback, namespace string) ValidationRule {
	name := strings.TrimSpace(raw.FullName)
	if name == "" {
		name = fallback
	}
	return ValidationRule{
		Name:                  name,
		Namespace:             namespace,
		Active:                raw.Active,
		ErrorConditionFormula: strings.TrimSpace(raw.ErrorConditionFormula),
		ErrorMessage:          raw.ErrorMessage,
		ErrorDisplayField:     raw.ErrorDisplayField,
	}
}

func loadRecordType(path string) (RecordType, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RecordType{}, err
	}
	var raw recordTypeXML
	if err := xml.Unmarshal(escapeBareAmpersands(data), &raw); err != nil {
		return RecordType{}, err
	}
	developerName := raw.FullName
	if developerName == "" {
		developerName = trimMetadataSuffix(filepath.Base(path), ".recordType-meta.xml")
	}
	label := raw.Label
	if label == "" {
		label = developerName
	}
	active := true
	if raw.Active != nil {
		active = *raw.Active
	}
	return RecordType{
		DeveloperName:    developerName,
		Label:            label,
		Active:           active,
		Default:          raw.Default,
		Description:      raw.Description,
		PicklistDefaults: recordTypePicklistDefaults(raw.PicklistValues),
	}, nil
}

func recordTypeFromXML(raw recordTypeXML, fallback string) RecordType {
	developerName := strings.TrimSpace(raw.FullName)
	if developerName == "" {
		developerName = fallback
	}
	label := raw.Label
	if label == "" {
		label = developerName
	}
	active := true
	if raw.Active != nil {
		active = *raw.Active
	}
	return RecordType{
		DeveloperName:    developerName,
		Label:            label,
		Active:           active,
		Default:          raw.Default,
		Description:      raw.Description,
		PicklistDefaults: recordTypePicklistDefaults(raw.PicklistValues),
	}
}

func recordTypePicklistDefaults(values []recordTypePicklistXML) map[string]string {
	defaults := make(map[string]string)
	for _, picklist := range values {
		field := strings.TrimSpace(picklist.Picklist)
		if field == "" {
			continue
		}
		for _, value := range picklist.Values {
			if !value.Default {
				continue
			}
			name := strings.TrimSpace(value.FullName)
			if name == "" {
				continue
			}
			defaults[field] = name
			break
		}
	}
	if len(defaults) == 0 {
		return nil
	}
	return defaults
}

func loadObject(path string, p project.Project) (Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Object{}, err
	}
	var raw customObjectXML
	if err := xml.Unmarshal(escapeBareAmpersands(data), &raw); err != nil {
		return Object{}, err
	}
	object := Object{
		Name:               namespaceProjectObjectName(p.Namespace, remapProjectAPIName(p, objectNameFromObjectPath(path))),
		Label:              raw.Label,
		PluralLabel:        raw.PluralLabel,
		SharingModel:       raw.SharingModel,
		CustomSettingsType: strings.TrimSpace(raw.CustomSettingsType),
		NameField: NameField{
			Label:         strings.TrimSpace(raw.NameField.Label),
			Type:          strings.TrimSpace(raw.NameField.Type),
			DisplayFormat: strings.TrimSpace(raw.NameField.DisplayFormat),
		},
	}
	for _, rawField := range raw.Fields {
		field := fieldFromXML(rawField, "")
		if field.Name != "" {
			field = remapProjectField(p, field)
			field = namespaceObjectField(p.Namespace, object.Name, field)
			object.Fields = append(object.Fields, field)
		}
	}
	for _, rawRecordType := range raw.RecordTypes {
		recordType := recordTypeFromXML(rawRecordType, "")
		if recordType.DeveloperName != "" {
			object.RecordTypes = append(object.RecordTypes, recordType)
		}
	}
	for _, rawRule := range raw.ValidationRules {
		rule := validationRuleFromXML(rawRule, "", validationRuleSourceNamespace(p.Namespace, object.Name))
		rule = remapProjectValidationRule(p, rule)
		if rule.Name != "" {
			object.ValidationRules = append(object.ValidationRules, rule)
		}
	}
	return object, nil
}

func loadField(path string) (Field, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Field{}, err
	}
	var raw customFieldXML
	if err := xml.Unmarshal(escapeBareAmpersands(data), &raw); err != nil {
		return Field{}, err
	}
	return fieldFromXML(raw, trimMetadataSuffix(filepath.Base(path), ".field-meta.xml")), nil
}

func fieldFromXML(raw customFieldXML, fallback string) Field {
	name := strings.TrimSpace(raw.FullName)
	if name == "" {
		name = fallback
	}
	relationshipName := raw.RelationshipName
	childRelationshipName := raw.ChildRelationshipName
	if strings.HasSuffix(name, "__c") && relationshipName != "" && !strings.HasSuffix(relationshipName, "__r") {
		if childRelationshipName == "" {
			childRelationshipName = relationshipName + "__r"
		}
		relationshipName = strings.TrimSuffix(name, "__c") + "__r"
	}
	return Field{
		Name:                  name,
		Label:                 raw.Label,
		InlineHelpText:        strings.TrimSpace(raw.InlineHelpText),
		Type:                  raw.Type,
		Length:                raw.Length,
		Precision:             raw.Precision,
		Scale:                 raw.Scale,
		ReferenceTo:           raw.ReferenceTo,
		RelationshipName:      relationshipName,
		ChildRelationshipName: childRelationshipName,
		DeleteConstraint:      raw.DeleteConstraint,
		DefaultValue:          strings.TrimSpace(raw.DefaultValue),
		Required:              raw.Required,
		ExternalID:            raw.ExternalID,
		IDLookup:              raw.IDLookup,
		Unique:                raw.Unique,
		Encrypted:             strings.EqualFold(raw.Type, "EncryptedText"),
		Formula:               strings.TrimSpace(raw.Formula),
		SummarizedField:       strings.TrimSpace(raw.SummarizedField),
		SummaryForeignKey:     strings.TrimSpace(raw.SummaryForeignKey),
		SummaryOperation:      strings.TrimSpace(raw.SummaryOperation),
		SummaryFilterItems:    summaryFiltersFromXML(raw.SummaryFilterItems),
		FilteredLookupInfo:    filteredLookupInfoFromXML(raw.LookupFilter),
		PicklistController:    strings.TrimSpace(raw.ValueSet.ControllingField),
		PicklistValueSettings: picklistSettings(raw.ValueSet.ValueSettings),
		ValueSetName:          strings.TrimSpace(raw.ValueSet.Name),
		RestrictedPicklist:    raw.ValueSet.Restricted,
		PicklistValues:        picklistValues(raw.ValueSet.Definition.Values),
	}
}

func remapProjectAPIName(p project.Project, name string) string {
	return namespaceremap.ApplyMetadataName(p.NamespaceRemaps, name)
}

func remapProjectField(p project.Project, field Field) Field {
	field.Name = remapProjectAPIName(p, field.Name)
	for i, referenceTo := range field.ReferenceTo {
		field.ReferenceTo[i] = remapProjectAPIName(p, referenceTo)
	}
	field.RelationshipName = remapProjectAPIName(p, field.RelationshipName)
	field.ChildRelationshipName = remapProjectAPIName(p, field.ChildRelationshipName)
	field.SummarizedField = remapProjectAPIName(p, field.SummarizedField)
	field.SummaryForeignKey = remapProjectAPIName(p, field.SummaryForeignKey)
	for i, filter := range field.SummaryFilterItems {
		filter.Field = remapProjectAPIName(p, filter.Field)
		field.SummaryFilterItems[i] = filter
	}
	for i, fieldName := range field.FilteredLookupInfo.ControllingFields {
		field.FilteredLookupInfo.ControllingFields[i] = remapProjectAPIName(p, fieldName)
	}
	field.PicklistController = remapProjectAPIName(p, field.PicklistController)
	field.ValueSetName = remapProjectAPIName(p, field.ValueSetName)
	return field
}

func remapProjectValidationRule(p project.Project, rule ValidationRule) ValidationRule {
	rule.Name = remapProjectAPIName(p, rule.Name)
	rule.Namespace = namespaceremap.ApplyNamespace(p.NamespaceRemaps, rule.Namespace)
	rule.ErrorDisplayField = remapProjectAPIName(p, rule.ErrorDisplayField)
	return rule
}

func remapProjectCustomMetadataRecord(p project.Project, record CustomMetadataRecord) CustomMetadataRecord {
	record.FullName = remapProjectAPIName(p, record.FullName)
	record.ObjectName = remapProjectAPIName(p, record.ObjectName)
	for i, value := range record.Values {
		value.Field = remapProjectAPIName(p, value.Field)
		record.Values[i] = value
	}
	return record
}

func namespaceObjectField(projectNamespace, objectName string, field Field) Field {
	namespace := namespaceFromAPIName(objectName)
	if projectNamespace != "" && namespace != "" && !strings.EqualFold(namespace, projectNamespace) {
		namespace = projectNamespace
	}
	if namespace == "" {
		return field
	}
	field.Name = namespaceAPIName(namespace, field.Name)
	for i, referenceTo := range field.ReferenceTo {
		field.ReferenceTo[i] = namespaceAPIName(namespace, referenceTo)
	}
	field.RelationshipName = namespaceAPIName(namespace, field.RelationshipName)
	field.ChildRelationshipName = namespaceAPIName(namespace, field.ChildRelationshipName)
	field.SummarizedField = namespaceAPIName(namespace, field.SummarizedField)
	field.SummaryForeignKey = namespaceAPIName(namespace, field.SummaryForeignKey)
	for i, filter := range field.SummaryFilterItems {
		filter.Field = namespaceAPIName(namespace, filter.Field)
		field.SummaryFilterItems[i] = filter
	}
	for i, fieldName := range field.FilteredLookupInfo.ControllingFields {
		field.FilteredLookupInfo.ControllingFields[i] = namespaceAPIName(namespace, fieldName)
	}
	field.PicklistController = namespaceAPIName(namespace, field.PicklistController)
	return field
}

func namespaceProjectObjectName(projectNamespace, objectName string) string {
	if projectNamespace == "" || objectName == "" || hasNamespaceToken(objectName) || !isCustomEntityName(objectName) {
		return objectName
	}
	return projectNamespace + "__" + objectName
}

func validationRuleSourceNamespace(projectNamespace, objectName string) string {
	if projectNamespace != "" {
		return projectNamespace
	}
	return namespaceFromAPIName(objectName)
}

func namespaceFromAPIName(name string) string {
	if !isCustomAPIName(name) {
		return ""
	}
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	if first <= 0 || first >= last {
		return ""
	}
	return name[:first]
}

func namespaceAPIName(namespace, name string) string {
	if namespace == "" || name == "" || !isCustomAPIName(name) || hasNamespaceToken(name) {
		return name
	}
	return namespace + "__" + name
}

func isCustomAPIName(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{"__c", "__r", "__e", "__mdt", "__b"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func hasNamespaceToken(name string) bool {
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	return first > 0 && first < last
}

func loadValueSets(p project.Project) (map[string][]PicklistValue, error) {
	sets := make(map[string][]PicklistValue)
	for _, path := range append(append([]string{}, p.GlobalValueSetFiles...), p.StandardValueSetFiles...) {
		values, err := loadValueSet(path)
		if err != nil {
			return nil, err
		}
		name := valueSetNameFromPath(path)
		if name == "" {
			continue
		}
		sets[strings.ToLower(name)] = values
	}
	return sets, nil
}

func loadValueSet(path string) ([]PicklistValue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw valueSetFileXML
	if err := xml.Unmarshal(escapeBareAmpersands(data), &raw); err != nil {
		return nil, err
	}
	values := picklistValues(raw.CustomValues)
	values = append(values, picklistValues(raw.StandardValues)...)
	return values, nil
}

func applyValueSet(field *Field, valueSets map[string][]PicklistValue) {
	if field == nil || len(field.PicklistValues) > 0 || strings.TrimSpace(field.ValueSetName) == "" {
		return
	}
	values := valueSets[strings.ToLower(field.ValueSetName)]
	if len(values) == 0 {
		return
	}
	field.PicklistValues = append([]PicklistValue(nil), values...)
}

func summaryFiltersFromXML(raw []summaryFilterXML) []SummaryFilter {
	out := make([]SummaryFilter, 0, len(raw))
	for _, item := range raw {
		field := strings.TrimSpace(item.Field)
		if field == "" {
			continue
		}
		out = append(out, SummaryFilter{
			Field:     field,
			Operation: strings.TrimSpace(item.Operation),
			Value:     strings.TrimSpace(item.Value),
		})
	}
	return out
}

func filteredLookupInfoFromXML(raw lookupFilterXML) FilteredLookupInfo {
	if raw.Active == nil && raw.IsOptional == nil && len(raw.FilterItems) == 0 {
		return FilteredLookupInfo{}
	}
	fields := make([]string, 0, len(raw.FilterItems))
	for _, item := range raw.FilterItems {
		field := strings.TrimSpace(item.Field)
		if field == "" || stringSliceContainsFold(fields, field) {
			continue
		}
		fields = append(fields, field)
	}
	info := FilteredLookupInfo{
		ControllingFields: fields,
		Dependent:         len(fields) > 0,
	}
	if raw.IsOptional != nil {
		info.OptionalFilter = *raw.IsOptional
	}
	return info
}

func stringSliceContainsFold(values []string, value string) bool {
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return true
		}
	}
	return false
}

func loadCustomMetadataRecord(path string) (CustomMetadataRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CustomMetadataRecord{}, err
	}
	var raw customMetadataXML
	if err := xml.Unmarshal(escapeBareAmpersands(data), &raw); err != nil {
		return CustomMetadataRecord{}, err
	}
	fullName := customMetadataFullName(path)
	objectName, developerName := customMetadataNames(fullName)
	values := make([]CustomMetadataValue, 0, len(raw.Values))
	for _, value := range raw.Values {
		field := strings.TrimSpace(value.Field)
		if field == "" {
			continue
		}
		text := strings.TrimSpace(value.Value.Text)
		isNil := value.Value.Nil
		if !isNil && text == "" {
			isNil = true
		}
		values = append(values, CustomMetadataValue{Field: field, Value: text, Nil: isNil})
	}
	return CustomMetadataRecord{
		FullName:      fullName,
		ObjectName:    objectName,
		DeveloperName: developerName,
		Label:         strings.TrimSpace(raw.Label),
		Protected:     raw.Protected,
		Values:        values,
		File:          path,
	}, nil
}

func picklistValues(values []picklistValueXML) []PicklistValue {
	out := make([]PicklistValue, 0, len(values))
	for _, value := range values {
		active := true
		if value.IsActive != nil {
			active = *value.IsActive
		}
		label := value.Label
		if label == "" {
			label = value.FullName
		}
		out = append(out, PicklistValue{FullName: value.FullName, Label: label, Default: value.Default, Active: active})
	}
	return out
}

func picklistSettings(values []valueSettingXML) []PicklistSetting {
	out := make([]PicklistSetting, 0, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value.ValueName)
		controllers := trimmedStrings(value.ControllingFieldValues)
		if name == "" && len(controllers) == 0 {
			continue
		}
		out = append(out, PicklistSetting{
			ValueName:              name,
			ControllingFieldValues: controllers,
		})
	}
	return out
}

func trimmedStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func trimMetadataSuffix(name, suffix string) string {
	if len(name) >= len(suffix) && strings.EqualFold(name[len(name)-len(suffix):], suffix) {
		return name[:len(name)-len(suffix)]
	}
	return strings.TrimSuffix(name, suffix)
}

func objectNameFromObjectPath(path string) string {
	name := filepath.Base(path)
	if strings.HasSuffix(strings.ToLower(name), ".object-meta.xml") {
		return trimMetadataSuffix(name, ".object-meta.xml")
	}
	return trimMetadataSuffix(name, ".object")
}

func customMetadataFullName(path string) string {
	name := filepath.Base(path)
	name = trimMetadataSuffix(name, ".md-meta.xml")
	name = trimMetadataSuffix(name, ".md")
	if strings.Contains(name, ".") {
		return name
	}
	typeName := nestedCustomMetadataTypeName(path)
	if typeName == "" {
		return name
	}
	return typeName + "." + name
}

func valueSetNameFromPath(path string) string {
	name := filepath.Base(path)
	name = trimMetadataSuffix(name, ".globalValueSet-meta.xml")
	return trimMetadataSuffix(name, ".standardValueSet-meta.xml")
}

func customMetadataNames(fullName string) (string, string) {
	parts := strings.SplitN(fullName, ".", 2)
	if len(parts) != 2 {
		return "", fullName
	}
	objectName := parts[0]
	if trimMetadataSuffix(objectName, "__mdt") == objectName {
		objectName += "__mdt"
	}
	return objectName, parts[1]
}

func nestedCustomMetadataTypeName(path string) string {
	recordsDir := filepath.Dir(path)
	if !strings.EqualFold(filepath.Base(recordsDir), "records") {
		return ""
	}
	typeName := filepath.Base(filepath.Dir(recordsDir))
	stripped := trimMetadataSuffix(typeName, "__mdt")
	if stripped == typeName {
		return ""
	}
	return stripped
}

func objectNameFromFieldPath(path string) string {
	dir := filepath.Dir(filepath.Dir(path))
	parent := filepath.Base(filepath.Dir(path))
	if parent == "fields" {
		return filepath.Base(dir)
	}
	if filepath.Base(dir) == "objects" {
		return parent
	}
	return ""
}

func objectNameFromRecordTypePath(path string) string {
	dir := filepath.Dir(filepath.Dir(path))
	if filepath.Base(filepath.Dir(path)) != "recordTypes" {
		return ""
	}
	return filepath.Base(dir)
}

func objectNameFromValidationRulePath(path string) string {
	dir := filepath.Dir(filepath.Dir(path))
	if filepath.Base(filepath.Dir(path)) != "validationRules" {
		return ""
	}
	return filepath.Base(dir)
}

var validEntityRE = regexp.MustCompile(`^(?:amp|lt|gt|quot|apos|#[0-9]+|#x[0-9a-fA-F]+);`)

func escapeBareAmpersands(data []byte) []byte {
	s := string(data)
	var out strings.Builder
	for {
		idx := strings.Index(s, "&")
		if idx == -1 {
			out.WriteString(s)
			break
		}
		out.WriteString(s[:idx])
		s = s[idx+1:]
		if validEntityRE.MatchString(s) {
			out.WriteString("&")
		} else {
			out.WriteString("&amp;")
		}
	}
	return []byte(out.String())
}
