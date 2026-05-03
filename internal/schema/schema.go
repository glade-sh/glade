package schema

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/project"
)

type Schema struct {
	Objects []Object `json:"objects"`
}

type Object struct {
	Name         string  `json:"name"`
	Label        string  `json:"label,omitempty"`
	PluralLabel  string  `json:"pluralLabel,omitempty"`
	SharingModel string  `json:"sharingModel,omitempty"`
	Fields       []Field `json:"fields,omitempty"`
}

type Field struct {
	Name                  string          `json:"name"`
	Label                 string          `json:"label,omitempty"`
	Type                  string          `json:"type,omitempty"`
	ReferenceTo           string          `json:"referenceTo,omitempty"`
	RelationshipName      string          `json:"relationshipName,omitempty"`
	ChildRelationshipName string          `json:"childRelationshipName,omitempty"`
	DeleteConstraint      string          `json:"deleteConstraint,omitempty"`
	Required              bool            `json:"required,omitempty"`
	ExternalID            bool            `json:"externalId,omitempty"`
	Unique                bool            `json:"unique,omitempty"`
	PicklistValues        []PicklistValue `json:"picklistValues,omitempty"`
}

type PicklistValue struct {
	FullName string `json:"fullName"`
	Label    string `json:"label,omitempty"`
	Default  bool   `json:"default,omitempty"`
	Active   bool   `json:"active,omitempty"`
}

type customObjectXML struct {
	XMLName      xml.Name `xml:"CustomObject"`
	Label        string   `xml:"label"`
	PluralLabel  string   `xml:"pluralLabel"`
	SharingModel string   `xml:"sharingModel"`
}

type customFieldXML struct {
	XMLName               xml.Name    `xml:"CustomField"`
	FullName              string      `xml:"fullName"`
	Label                 string      `xml:"label"`
	Type                  string      `xml:"type"`
	ReferenceTo           string      `xml:"referenceTo"`
	RelationshipName      string      `xml:"relationshipName"`
	ChildRelationshipName string      `xml:"childRelationshipName"`
	DeleteConstraint      string      `xml:"deleteConstraint"`
	Required              bool        `xml:"required"`
	ExternalID            bool        `xml:"externalId"`
	Unique                bool        `xml:"unique"`
	ValueSet              valueSetXML `xml:"valueSet"`
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

	out := Schema{Objects: make([]Object, 0, len(byName))}
	for _, object := range byName {
		sort.Slice(object.Fields, func(i, j int) bool {
			return object.Fields[i].Name < object.Fields[j].Name
		})
		out.Objects = append(out.Objects, *object)
	}
	sort.Slice(out.Objects, func(i, j int) bool {
		return out.Objects[i].Name < out.Objects[j].Name
	})
	return out, nil
}

func loadObject(path string) (Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Object{}, err
	}
	var raw customObjectXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return Object{}, err
	}
	return Object{
		Name:         strings.TrimSuffix(filepath.Base(path), ".object-meta.xml"),
		Label:        raw.Label,
		PluralLabel:  raw.PluralLabel,
		SharingModel: raw.SharingModel,
	}, nil
}

func loadField(path string) (Field, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Field{}, err
	}
	var raw customFieldXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return Field{}, err
	}
	name := raw.FullName
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".field-meta.xml")
	}
	return Field{
		Name:                  name,
		Label:                 raw.Label,
		Type:                  raw.Type,
		ReferenceTo:           raw.ReferenceTo,
		RelationshipName:      raw.RelationshipName,
		ChildRelationshipName: raw.ChildRelationshipName,
		DeleteConstraint:      raw.DeleteConstraint,
		Required:              raw.Required,
		ExternalID:            raw.ExternalID,
		Unique:                raw.Unique,
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

func objectNameFromFieldPath(path string) string {
	dir := filepath.Dir(filepath.Dir(path))
	if filepath.Base(filepath.Dir(path)) != "fields" {
		return ""
	}
	return filepath.Base(dir)
}
