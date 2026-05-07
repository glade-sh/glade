package schema

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/project"
)

type Schema struct {
	Objects               []Object               `json:"objects"`
	CustomMetadataRecords []CustomMetadataRecord `json:"customMetadataRecords,omitempty"`
}

type Object struct {
	Name               string           `json:"name"`
	Label              string           `json:"label,omitempty"`
	PluralLabel        string           `json:"pluralLabel,omitempty"`
	SharingModel       string           `json:"sharingModel,omitempty"`
	CustomSettingsType string           `json:"customSettingsType,omitempty"`
	Fields             []Field          `json:"fields,omitempty"`
	RecordTypes        []RecordType     `json:"recordTypes,omitempty"`
	ValidationRules    []ValidationRule `json:"validationRules,omitempty"`
}

type Field struct {
	Name                  string          `json:"name"`
	Label                 string          `json:"label,omitempty"`
	Type                  string          `json:"type,omitempty"`
	ReferenceTo           []string        `json:"referenceTo,omitempty"`
	RelationshipName      string          `json:"relationshipName,omitempty"`
	ChildRelationshipName string          `json:"childRelationshipName,omitempty"`
	DeleteConstraint      string          `json:"deleteConstraint,omitempty"`
	DefaultValue          string          `json:"defaultValue,omitempty"`
	Required              bool            `json:"required,omitempty"`
	ExternalID            bool            `json:"externalId,omitempty"`
	Unique                bool            `json:"unique,omitempty"`
	Encrypted             bool            `json:"encrypted,omitempty"`
	Formula               string          `json:"formula,omitempty"`
	PicklistValues        []PicklistValue `json:"picklistValues,omitempty"`
}

type PicklistValue struct {
	FullName string `json:"fullName"`
	Label    string `json:"label,omitempty"`
	Default  bool   `json:"default,omitempty"`
	Active   bool   `json:"active,omitempty"`
}

type RecordType struct {
	DeveloperName string `json:"developerName"`
	Label         string `json:"label,omitempty"`
	Active        bool   `json:"active,omitempty"`
	Default       bool   `json:"default,omitempty"`
	Description   string `json:"description,omitempty"`
}

type ValidationRule struct {
	Name                  string `json:"name"`
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
}

type customObjectXML struct {
	XMLName            xml.Name            `xml:"CustomObject"`
	Label              string              `xml:"label"`
	PluralLabel        string              `xml:"pluralLabel"`
	SharingModel       string              `xml:"sharingModel"`
	CustomSettingsType string              `xml:"customSettingsType"`
	Fields             []customFieldXML    `xml:"fields"`
	RecordTypes        []recordTypeXML     `xml:"recordTypes"`
	ValidationRules    []validationRuleXML `xml:"validationRules"`
}

type customFieldXML struct {
	FullName              string      `xml:"fullName"`
	Label                 string      `xml:"label"`
	Type                  string      `xml:"type"`
	ReferenceTo           []string    `xml:"referenceTo"`
	RelationshipName      string      `xml:"relationshipName"`
	ChildRelationshipName string      `xml:"childRelationshipName"`
	DeleteConstraint      string      `xml:"deleteConstraint"`
	DefaultValue          string      `xml:"defaultValue"`
	Required              bool        `xml:"required"`
	ExternalID            bool        `xml:"externalId"`
	Unique                bool        `xml:"unique"`
	Formula               string      `xml:"formula"`
	ValueSet              valueSetXML `xml:"valueSet"`
}

type recordTypeXML struct {
	FullName    string `xml:"fullName"`
	Label       string `xml:"label"`
	Active      *bool  `xml:"active"`
	Default     bool   `xml:"default"`
	Description string `xml:"description"`
}

type validationRuleXML struct {
	FullName              string `xml:"fullName"`
	Active                bool   `xml:"active"`
	ErrorConditionFormula string `xml:"errorConditionFormula"`
	ErrorMessage          string `xml:"errorMessage"`
	ErrorDisplayField     string `xml:"errorDisplayField"`
}

type valueSetXML struct {
	Definition valueSetDefinitionXML `xml:"valueSetDefinition"`
}

type valueSetDefinitionXML struct {
	Values []picklistValueXML `xml:"value"`
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
	Field string `xml:"field"`
	Value string `xml:"value"`
}

func LoadProject(p project.Project) (Schema, error) {
	byName := make(map[string]*Object)

	for _, path := range p.ObjectFiles {
		object, err := loadObject(path)
		if err != nil {
			return Schema{}, err
		}
		byName[object.Name] = &object
	}

	for _, path := range p.FieldFiles {
		objectName := objectNameFromFieldPath(path)
		if objectName == "" {
			continue
		}
		field, err := loadField(path)
		if err != nil {
			return Schema{}, err
		}
		object := byName[objectName]
		if object == nil {
			object = &Object{Name: objectName}
			byName[objectName] = object
		}
		object.Fields = append(object.Fields, field)
	}

	for _, path := range p.RecordTypeFiles {
		objectName := objectNameFromRecordTypePath(path)
		if objectName == "" {
			continue
		}
		recordType, err := loadRecordType(path)
		if err != nil {
			return Schema{}, err
		}
		object := byName[objectName]
		if object == nil {
			object = &Object{Name: objectName}
			byName[objectName] = object
		}
		object.RecordTypes = append(object.RecordTypes, recordType)
	}

	for _, path := range p.ValidationRuleFiles {
		objectName := objectNameFromValidationRulePath(path)
		if objectName == "" {
			continue
		}
		rule, err := loadValidationRule(path)
		if err != nil {
			return Schema{}, err
		}
		object := byName[objectName]
		if object == nil {
			object = &Object{Name: objectName}
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
		byName[name] = &Object{Name: name}
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

func loadValidationRule(path string) (ValidationRule, error) {
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
		Active:                raw.Active,
		ErrorConditionFormula: strings.TrimSpace(raw.ErrorConditionFormula),
		ErrorMessage:          raw.ErrorMessage,
		ErrorDisplayField:     raw.ErrorDisplayField,
	}, nil
}

func validationRuleFromXML(raw validationRuleXML, fallback string) ValidationRule {
	name := strings.TrimSpace(raw.FullName)
	if name == "" {
		name = fallback
	}
	return ValidationRule{
		Name:                  name,
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
		DeveloperName: developerName,
		Label:         label,
		Active:        active,
		Default:       raw.Default,
		Description:   raw.Description,
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
		DeveloperName: developerName,
		Label:         label,
		Active:        active,
		Default:       raw.Default,
		Description:   raw.Description,
	}
}

func loadObject(path string) (Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Object{}, err
	}
	var raw customObjectXML
	if err := xml.Unmarshal(escapeBareAmpersands(data), &raw); err != nil {
		return Object{}, err
	}
	object := Object{
		Name:               objectNameFromObjectPath(path),
		Label:              raw.Label,
		PluralLabel:        raw.PluralLabel,
		SharingModel:       raw.SharingModel,
		CustomSettingsType: strings.TrimSpace(raw.CustomSettingsType),
	}
	for _, rawField := range raw.Fields {
		field := fieldFromXML(rawField, "")
		if field.Name != "" {
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
		rule := validationRuleFromXML(rawRule, "")
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
	return Field{
		Name:                  name,
		Label:                 raw.Label,
		Type:                  raw.Type,
		ReferenceTo:           raw.ReferenceTo,
		RelationshipName:      raw.RelationshipName,
		ChildRelationshipName: raw.ChildRelationshipName,
		DeleteConstraint:      raw.DeleteConstraint,
		DefaultValue:          strings.TrimSpace(raw.DefaultValue),
		Required:              raw.Required,
		ExternalID:            raw.ExternalID,
		Unique:                raw.Unique,
		Encrypted:             strings.EqualFold(raw.Type, "EncryptedText"),
		Formula:               strings.TrimSpace(raw.Formula),
		PicklistValues:        picklistValues(raw.ValueSet.Definition.Values),
	}
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
		values = append(values, CustomMetadataValue{Field: field, Value: strings.TrimSpace(value.Value)})
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
	return trimMetadataSuffix(name, ".md")
}

func customMetadataNames(fullName string) (string, string) {
	parts := strings.SplitN(fullName, ".", 2)
	if len(parts) != 2 {
		return "", fullName
	}
	return parts[0] + "__mdt", parts[1]
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
