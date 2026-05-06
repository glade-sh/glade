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
	Objects []Object `json:"objects"`
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

type customObjectXML struct {
	XMLName            xml.Name `xml:"CustomObject"`
	Label              string   `xml:"label"`
	PluralLabel        string   `xml:"pluralLabel"`
	SharingModel       string   `xml:"sharingModel"`
	CustomSettingsType string   `xml:"customSettingsType"`
}

type customFieldXML struct {
	XMLName               xml.Name    `xml:"CustomField"`
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
	XMLName     xml.Name `xml:"RecordType"`
	FullName    string   `xml:"fullName"`
	Label       string   `xml:"label"`
	Active      *bool    `xml:"active"`
	Default     bool     `xml:"default"`
	Description string   `xml:"description"`
}

type validationRuleXML struct {
	XMLName               xml.Name `xml:"ValidationRule"`
	FullName              string   `xml:"fullName"`
	Active                bool     `xml:"active"`
	ErrorConditionFormula string   `xml:"errorConditionFormula"`
	ErrorMessage          string   `xml:"errorMessage"`
	ErrorDisplayField     string   `xml:"errorDisplayField"`
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

	out := Schema{Objects: make([]Object, 0, len(byName))}
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

func loadObject(path string) (Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Object{}, err
	}
	var raw customObjectXML
	if err := xml.Unmarshal(escapeBareAmpersands(data), &raw); err != nil {
		return Object{}, err
	}
	return Object{
		Name:               trimMetadataSuffix(filepath.Base(path), ".object-meta.xml"),
		Label:              raw.Label,
		PluralLabel:        raw.PluralLabel,
		SharingModel:       raw.SharingModel,
		CustomSettingsType: strings.TrimSpace(raw.CustomSettingsType),
	}, nil
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
	name := raw.FullName
	if name == "" {
		name = trimMetadataSuffix(filepath.Base(path), ".field-meta.xml")
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

func objectNameFromFieldPath(path string) string {
	dir := filepath.Dir(filepath.Dir(path))
	if filepath.Base(filepath.Dir(path)) != "fields" {
		return ""
	}
	return filepath.Base(dir)
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
