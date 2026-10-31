package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"maps"
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
	Name    string `json:"name"`
	Partial bool   `json:"partial,omitempty"`
	// FieldsComplete is an explicit snapshot assertion that the object's
	// field list is complete. Omission keeps unknown fields open.
	FieldsComplete bool `json:"fieldsComplete,omitempty"`
	// CompleteSnapshot binds a verified assertion to this schema generation.
	// Both flags are required so older inferred markers remain open.
	CompleteSnapshot       bool              `json:"completeSnapshot,omitempty"`
	Label                  string            `json:"label,omitempty"`
	PluralLabel            string            `json:"pluralLabel,omitempty"`
	SharingModel           string            `json:"sharingModel,omitempty"`
	EnableActivities       bool              `json:"enableActivities,omitempty"`
	EnableHistory          bool              `json:"enableHistory,omitempty"`
	GeneratedHistoryParent string            `json:"-"`
	EnableSharing          bool              `json:"enableSharing,omitempty"`
	SharingReasons         []string          `json:"sharingReasons,omitempty"`
	SharingReasonLabels    map[string]string `json:"sharingReasonLabels,omitempty"`
	CustomSettingsType     string            `json:"customSettingsType,omitempty"`
	PublishBehavior        string            `json:"publishBehavior,omitempty"`
	EnableSearch           bool              `json:"enableSearch,omitempty"`
	Triggerable            *bool             `json:"triggerable,omitempty"`
	NameField              NameField         `json:"nameField,omitempty"`
	Fields                 []Field           `json:"fields,omitempty"`
	RecordTypes            []RecordType      `json:"recordTypes,omitempty"`
	ValidationRules        []ValidationRule  `json:"validationRules,omitempty"`
}

// SharingReasonLabel returns the authored label without substituting its token.
func (o Object) SharingReasonLabel(name string) string {
	if label, ok := o.SharingReasonLabels[name]; ok {
		return label
	}
	for token, label := range o.SharingReasonLabels {
		if strings.EqualFold(token, name) {
			return label
		}
	}
	return ""
}

// SupportsTriggers reports whether the object accepts Apex triggers, and
// whether a describe source stated the capability at all. Local metadata does
// not carry the flag, so an absent value must never be read as a rejection.
func (o Object) SupportsTriggers() (supported, known bool) {
	if o.Triggerable == nil {
		return false, false
	}
	return *o.Triggerable, true
}

type NameField struct {
	Label         string `json:"label,omitempty"`
	Type          string `json:"type,omitempty"`
	DisplayFormat string `json:"displayFormat,omitempty"`
	Length        int    `json:"length,omitempty"`
}

type Field struct {
	Name string `json:"name"`
	// File is the declaring metadata source, retained through schema projections.
	File                          string             `json:"-"`
	GeneratedRecentAccess         bool               `json:"generatedRecentAccess,omitempty"`
	Label                         string             `json:"label,omitempty"`
	InlineHelpText                string             `json:"inlineHelpText,omitempty"`
	Type                          string             `json:"type,omitempty"`
	DisplayFormat                 string             `json:"displayFormat,omitempty"`
	Length                        int                `json:"length,omitempty"`
	Precision                     int                `json:"precision,omitempty"`
	Scale                         int                `json:"scale,omitempty"`
	ScaleSpecified                bool               `json:"scaleSpecified,omitempty"`
	ReferenceTo                   []string           `json:"referenceTo,omitempty"`
	RelationshipName              string             `json:"relationshipName,omitempty"`
	RelationshipOrder             *int               `json:"relationshipOrder,omitempty"`
	ReparentableMasterDetail      bool               `json:"reparentableMasterDetail,omitempty"`
	ChildRelationshipName         string             `json:"childRelationshipName,omitempty"`
	ChildRelationshipNameInferred bool               `json:"-"`
	DeleteConstraint              string             `json:"deleteConstraint,omitempty"`
	DefaultValue                  string             `json:"defaultValue,omitempty"`
	Required                      bool               `json:"required,omitempty"`
	Nillable                      *bool              `json:"nillable,omitempty"`
	DefaultedOnCreate             *bool              `json:"defaultedOnCreate,omitempty"`
	Createable                    *bool              `json:"createable,omitempty"`
	Updateable                    *bool              `json:"updateable,omitempty"`
	ExternalID                    bool               `json:"externalId,omitempty"`
	IDLookup                      bool               `json:"idLookup,omitempty"`
	Filterable                    *bool              `json:"filterable,omitempty"`
	Groupable                     *bool              `json:"groupable,omitempty"`
	Sortable                      *bool              `json:"sortable,omitempty"`
	Aggregatable                  *bool              `json:"aggregatable,omitempty"`
	Unique                        bool               `json:"unique,omitempty"`
	CaseSensitive                 bool               `json:"caseSensitive,omitempty"`
	DescribeCaseSensitive         *bool              `json:"describeCaseSensitive,omitempty"`
	Encrypted                     bool               `json:"encrypted,omitempty"`
	Formula                       string             `json:"formula,omitempty"`
	FormulaTreatBlanksAs          string             `json:"formulaTreatBlanksAs,omitempty"`
	SummarizedField               string             `json:"summarizedField,omitempty"`
	SummaryForeignKey             string             `json:"summaryForeignKey,omitempty"`
	SummaryOperation              string             `json:"summaryOperation,omitempty"`
	SummaryFilterItems            []SummaryFilter    `json:"summaryFilterItems,omitempty"`
	FilteredLookupInfo            FilteredLookupInfo `json:"filteredLookupInfo,omitempty"`
	PicklistController            string             `json:"picklistController,omitempty"`
	PicklistValueSettings         []PicklistSetting  `json:"picklistValueSettings,omitempty"`
	ValueSetName                  string             `json:"valueSetName,omitempty"`
	StandardValueSetApplied       bool               `json:"standardValueSetApplied,omitempty"`
	RestrictedPicklist            bool               `json:"restrictedPicklist,omitempty"`
	PicklistValues                []PicklistValue    `json:"picklistValues,omitempty"`

	MetadataRelationshipControllingField string `json:"metadataRelationshipControllingField,omitempty"`

	// CompoundFieldName names the custom Address field that generated this
	// component field; source metadata never declares components.
	CompoundFieldName string `json:"compoundFieldName,omitempty"`
}

type FilteredLookupInfo struct {
	ControllingFields []string           `json:"controllingFields,omitempty"`
	Dependent         bool               `json:"dependent,omitempty"`
	OptionalFilter    bool               `json:"optionalFilter,omitempty"`
	Active            bool               `json:"active,omitempty"`
	BooleanFilter     string             `json:"booleanFilter,omitempty"`
	ErrorMessage      string             `json:"errorMessage,omitempty"`
	FilterItems       []LookupFilterItem `json:"filterItems,omitempty"`
}

type LookupFilterItem struct {
	Field      string `json:"field,omitempty"`
	Operation  string `json:"operation,omitempty"`
	Value      string `json:"value,omitempty"`
	ValueField string `json:"valueField,omitempty"`
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
	SourceUnit string `json:"sourceUnit,omitempty"`
	// OwnerNamespace comes from the producing project, independently of the type.
	// Nil preserves the legacy interpretation for records without provenance.
	OwnerNamespace *string               `json:"ownerNamespace,omitempty"`
	FullName       string                `json:"fullName"`
	ObjectName     string                `json:"objectName"`
	DeveloperName  string                `json:"developerName"`
	Label          string                `json:"label,omitempty"`
	Protected      bool                  `json:"protected,omitempty"`
	Values         []CustomMetadataValue `json:"values,omitempty"`
	File           string                `json:"file,omitempty"`
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
	EnableActivities   bool                `xml:"enableActivities"`
	EnableSharing      bool                `xml:"enableSharing"`
	EnableHistory      bool                `xml:"enableHistory"`
	CustomSettingsType string              `xml:"customSettingsType"`
	PublishBehavior    string              `xml:"publishBehavior"`
	EnableSearch       bool                `xml:"enableSearch"`
	NameField          nameFieldXML        `xml:"nameField"`
	Fields             []customFieldXML    `xml:"fields"`
	RecordTypes        []recordTypeXML     `xml:"recordTypes"`
	ValidationRules    []validationRuleXML `xml:"validationRules"`
}

type nameFieldXML struct {
	Label         string `xml:"label"`
	Type          string `xml:"type"`
	DisplayFormat string `xml:"displayFormat"`
	Length        int    `xml:"length"`
}

type customFieldXML struct {
	FullName                 string             `xml:"fullName"`
	Label                    string             `xml:"label"`
	InlineHelpText           string             `xml:"inlineHelpText"`
	Type                     string             `xml:"type"`
	DisplayFormat            string             `xml:"displayFormat"`
	Length                   int                `xml:"length"`
	Precision                int                `xml:"precision"`
	Scale                    *int               `xml:"scale"`
	ReferenceTo              []string           `xml:"referenceTo"`
	RelationshipName         string             `xml:"relationshipName"`
	RelationshipOrder        *int               `xml:"relationshipOrder"`
	ReparentableMasterDetail bool               `xml:"reparentableMasterDetail"`
	ChildRelationshipName    string             `xml:"childRelationshipName"`
	DeleteConstraint         string             `xml:"deleteConstraint"`
	DefaultValue             string             `xml:"defaultValue"`
	Required                 bool               `xml:"required"`
	ExternalID               bool               `xml:"externalId"`
	IDLookup                 bool               `xml:"idLookup"`
	Unique                   bool               `xml:"unique"`
	CaseSensitive            *bool              `xml:"caseSensitive"`
	Formula                  string             `xml:"formula"`
	FormulaTreatBlanksAs     string             `xml:"formulaTreatBlanksAs"`
	SummarizedField          string             `xml:"summarizedField"`
	SummaryForeignKey        string             `xml:"summaryForeignKey"`
	SummaryOperation         string             `xml:"summaryOperation"`
	SummaryFilterItems       []summaryFilterXML `xml:"summaryFilterItems"`
	LookupFilter             lookupFilterXML    `xml:"lookupFilter"`
	ValueSet                 valueSetXML        `xml:"valueSet"`

	MetadataRelationshipControllingField string `xml:"metadataRelationshipControllingField"`
}

type lookupFilterXML struct {
	BooleanFilter string                `xml:"booleanFilter"`
	ErrorMessage  string                `xml:"errorMessage"`
	Active        *bool                 `xml:"active"`
	FilterItems   []lookupFilterItemXML `xml:"filterItems"`
	IsOptional    *bool                 `xml:"isOptional"`
}

type lookupFilterItemXML struct {
	Field      string `xml:"field"`
	Operation  string `xml:"operation"`
	Value      string `xml:"value"`
	ValueField string `xml:"valueField"`
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

type sharingReasonXML struct {
	FullName string `xml:"fullName"`
	Label    string `xml:"label"`
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

	snapshotObjects, err := loadPinnedSchemaSnapshot(p)
	if err != nil {
		return Schema{}, err
	}
	for i := range snapshotObjects {
		object := snapshotObjects[i]
		// The hash verifies integrity, not completeness. Only an explicit
		// assertion on a non-partial snapshot establishes authority.
		object.CompleteSnapshot = object.FieldsComplete && !object.Partial
		if existing := byName[object.Name]; existing != nil {
			mergeObjectMetadata(existing, object)
			existing.FieldsComplete = existing.FieldsComplete || object.FieldsComplete
			existing.CompleteSnapshot = existing.CompleteSnapshot || object.CompleteSnapshot
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
		if i := fieldIndex(object.Fields, field.Name); i >= 0 {
			object.Fields[i] = field
		} else {
			object.Fields = append(object.Fields, field)
		}
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
		if i := recordTypeIndex(object.RecordTypes, recordType.DeveloperName); i >= 0 {
			object.RecordTypes[i] = recordType
		} else {
			object.RecordTypes = append(object.RecordTypes, recordType)
		}
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
		if i := validationRuleIndex(object.ValidationRules, rule.Name); i >= 0 {
			object.ValidationRules[i] = rule
		} else {
			object.ValidationRules = append(object.ValidationRules, rule)
		}
	}

	if err := addCustomTabAccessFields(p, byName); err != nil {
		return Schema{}, err
	}
	addCustomAddressComponentFields(byName)
	addPersonContactAliases(byName, project.OrgShapeFeatures(p.Root))
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

	bindProjectStandardValueSets(p, byName, valueSets)
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

func loadPinnedSchemaSnapshot(p project.Project) ([]Object, error) {
	path := strings.TrimSpace(p.SchemaSnapshot)
	configured := strings.ToLower(strings.TrimSpace(p.SchemaSnapshotSHA256))
	if path == "" && configured == "" {
		return nil, nil
	}
	if path == "" || configured == "" {
		return nil, errors.New("project.schemaSnapshot and project.schemaSnapshotSHA256 must be configured together")
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is an explicit project configuration value bound to a required SHA-256.
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if configured != actual {
		return nil, fmt.Errorf("schema snapshot SHA-256 mismatch: configured %s, actual %s", configured, actual)
	}
	var snapshot Schema
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("load schema snapshot %q: %w", path, err)
	}
	return snapshot.Objects, nil
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
	if src.EnableSharing {
		dst.EnableSharing = true
	}
	if src.EnableActivities {
		dst.EnableActivities = true
	}
	if src.EnableHistory {
		dst.EnableHistory = true
	}
	for _, reason := range src.SharingReasons {
		if !stringSliceContainsFold(dst.SharingReasons, reason) {
			dst.SharingReasons = append(dst.SharingReasons, reason)
		}
	}
	if len(dst.SharingReasonLabels)+len(src.SharingReasonLabels) > 0 {
		labels := maps.Clone(dst.SharingReasonLabels)
		if labels == nil {
			labels = make(map[string]string, len(src.SharingReasonLabels))
		}
		for name, label := range src.SharingReasonLabels {
			if dst.SharingReasonLabel(name) != "" {
				continue
			}
			for _, token := range dst.SharingReasons {
				if strings.EqualFold(token, name) {
					name = token
					break
				}
			}
			labels[name] = label
		}
		dst.SharingReasonLabels = labels
	}
	if dst.PublishBehavior == "" {
		dst.PublishBehavior = src.PublishBehavior
	}
	if dst.CustomSettingsType == "" {
		dst.CustomSettingsType = src.CustomSettingsType
	}
	if src.EnableSearch {
		dst.EnableSearch = true
	}
	if dst.NameField == (NameField{}) {
		dst.NameField = src.NameField
	}
	for _, field := range src.Fields {
		if fieldIndex(dst.Fields, field.Name) < 0 {
			dst.Fields = append(dst.Fields, field)
		}
	}
	for _, recordType := range src.RecordTypes {
		if recordTypeIndex(dst.RecordTypes, recordType.DeveloperName) < 0 {
			dst.RecordTypes = append(dst.RecordTypes, recordType)
		}
	}
	for _, rule := range src.ValidationRules {
		if validationRuleIndex(dst.ValidationRules, rule.Name) < 0 {
			dst.ValidationRules = append(dst.ValidationRules, rule)
		}
	}
}

func fieldIndex(fields []Field, name string) int {
	for i, field := range fields {
		if strings.EqualFold(field.Name, name) {
			return i
		}
	}
	return -1
}

func recordTypeIndex(recordTypes []RecordType, name string) int {
	for i, recordType := range recordTypes {
		if strings.EqualFold(recordType.DeveloperName, name) {
			return i
		}
	}
	return -1
}

func validationRuleIndex(rules []ValidationRule, name string) int {
	for i, rule := range rules {
		if strings.EqualFold(rule.Name, name) {
			return i
		}
	}
	return -1
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
		EnableActivities:   raw.EnableActivities,
		EnableSharing:      raw.EnableSharing,
		EnableHistory:      raw.EnableHistory,
		CustomSettingsType: strings.TrimSpace(raw.CustomSettingsType),
		PublishBehavior:    strings.TrimSpace(raw.PublishBehavior),
		EnableSearch:       raw.EnableSearch,
		NameField: NameField{
			Label:         strings.TrimSpace(raw.NameField.Label),
			Type:          strings.TrimSpace(raw.NameField.Type),
			DisplayFormat: strings.TrimSpace(raw.NameField.DisplayFormat),
			Length:        raw.NameField.Length,
		},
	}
	object.SharingReasons, object.SharingReasonLabels = loadSharingReasons(path, p)
	for _, rawField := range raw.Fields {
		field := fieldFromXML(rawField, "")
		field.File = path
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

func loadSharingReasons(objectPath string, p project.Project) ([]string, map[string]string) {
	dir := filepath.Join(filepath.Dir(objectPath), "sharingReasons")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	reasons := make([]string, 0, len(entries))
	var labels map[string]string
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".sharingreason-meta.xml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var raw sharingReasonXML
		if err := xml.Unmarshal(escapeBareAmpersands(data), &raw); err != nil {
			continue
		}
		name := strings.TrimSpace(raw.FullName)
		if name == "" {
			name = trimMetadataSuffix(entry.Name(), ".sharingReason-meta.xml")
		}
		name = remapProjectAPIName(p, name)
		name = namespaceAPIName(p.Namespace, name)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		reasons = append(reasons, name)
		if label := strings.TrimSpace(raw.Label); label != "" {
			if labels == nil {
				labels = make(map[string]string)
			}
			labels[name] = label
		}
	}
	sort.Strings(reasons)
	return reasons, labels
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
	field := fieldFromXML(raw, trimMetadataSuffix(filepath.Base(path), ".field-meta.xml"))
	field.File = path
	return field, nil
}

func fieldFromXML(raw customFieldXML, fallback string) Field {
	name := strings.TrimSpace(raw.FullName)
	if name == "" {
		name = fallback
	}
	length := raw.Length
	if length == 0 && strings.EqualFold(strings.TrimSpace(raw.Type), "Url") {
		length = 255
	}
	relationshipName := raw.RelationshipName
	childRelationshipName := raw.ChildRelationshipName
	childRelationshipNameInferred := false
	if strings.HasSuffix(name, "__c") && relationshipName != "" && !strings.HasSuffix(relationshipName, "__r") {
		if childRelationshipName == "" {
			childRelationshipName = relationshipName + "__r"
			childRelationshipNameInferred = true
		}
		relationshipName = strings.TrimSuffix(name, "__c") + "__r"
	}
	// Only custom fields can be master-detail. Retrieved source can still
	// carry <type>MasterDetail</type> on a standard relationship field such as
	// ContactPointAddress.ParentId; Salesforce keeps that field lookup-shaped
	// and optional, so the declared type must not change it.
	fieldType := raw.Type
	if strings.EqualFold(fieldType, "MasterDetail") && !strings.Contains(name, "__") {
		fieldType = ""
		raw.ReparentableMasterDetail = false
	}
	return Field{
		Name:                          name,
		Label:                         raw.Label,
		InlineHelpText:                strings.TrimSpace(raw.InlineHelpText),
		Type:                          fieldType,
		DisplayFormat:                 strings.TrimSpace(raw.DisplayFormat),
		Length:                        length,
		Precision:                     raw.Precision,
		Scale:                         metadataScale(raw.Scale),
		ScaleSpecified:                raw.Scale != nil,
		ReferenceTo:                   raw.ReferenceTo,
		RelationshipName:              relationshipName,
		RelationshipOrder:             raw.RelationshipOrder,
		ReparentableMasterDetail:      raw.ReparentableMasterDetail,
		ChildRelationshipName:         childRelationshipName,
		ChildRelationshipNameInferred: childRelationshipNameInferred,
		DeleteConstraint:              raw.DeleteConstraint,
		DefaultValue:                  strings.TrimSpace(raw.DefaultValue),
		Required:                      raw.Required,
		ExternalID:                    raw.ExternalID,
		IDLookup:                      raw.IDLookup,
		Unique:                        raw.Unique,
		CaseSensitive:                 fieldKeyCaseSensitive(raw),
		DescribeCaseSensitive:         fieldKeyDescribeCaseSensitive(raw),
		Encrypted:                     strings.EqualFold(raw.Type, "EncryptedText"),
		Formula:                       strings.TrimSpace(raw.Formula),
		FormulaTreatBlanksAs:          strings.TrimSpace(raw.FormulaTreatBlanksAs),
		SummarizedField:               strings.TrimSpace(raw.SummarizedField),
		SummaryForeignKey:             strings.TrimSpace(raw.SummaryForeignKey),
		SummaryOperation:              strings.TrimSpace(raw.SummaryOperation),
		SummaryFilterItems:            summaryFiltersFromXML(raw.SummaryFilterItems),
		FilteredLookupInfo:            filteredLookupInfoFromXML(raw.LookupFilter),
		PicklistController:            strings.TrimSpace(raw.ValueSet.ControllingField),
		PicklistValueSettings:         picklistSettings(raw.ValueSet.ValueSettings),
		ValueSetName:                  strings.TrimSpace(raw.ValueSet.Name),
		RestrictedPicklist:            raw.ValueSet.Restricted,
		PicklistValues:                picklistValues(raw.ValueSet.Definition.Values),

		MetadataRelationshipControllingField: raw.MetadataRelationshipControllingField,
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
	for i, item := range field.FilteredLookupInfo.FilterItems {
		item.Field = mapLookupFilterPath(item.Field, func(name string) string { return remapProjectAPIName(p, name) })
		item.ValueField = mapLookupFilterPath(item.ValueField, func(name string) string { return remapProjectAPIName(p, name) })
		field.FilteredLookupInfo.FilterItems[i] = item
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
	namespace := namespaceremap.ApplyNamespace(p.NamespaceRemaps, p.Namespace)
	record.OwnerNamespace = &namespace
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
	// The producing project owns extension metadata even when its standard
	// object name supplies no namespace. Explicit foreign qualifiers survive.
	if projectNamespace != "" && (namespace == "" || !strings.EqualFold(namespace, projectNamespace)) {
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
		field.FilteredLookupInfo.ControllingFields[i] = mapLookupFilterPath(fieldName, func(name string) string { return namespaceAPIName(namespace, name) })
	}
	for i, item := range field.FilteredLookupInfo.FilterItems {
		item.Field = mapLookupFilterPath(item.Field, func(name string) string { return namespaceAPIName(namespace, name) })
		item.ValueField = mapLookupFilterPath(item.ValueField, func(name string) string { return namespaceAPIName(namespace, name) })
		field.FilteredLookupInfo.FilterItems[i] = item
	}
	field.PicklistController = namespaceAPIName(namespace, field.PicklistController)
	return field
}

func mapLookupFilterPath(path string, convert func(string) string) string {
	parts := strings.Split(path, ".")
	for i, part := range parts {
		parts[i] = convert(part)
	}
	return strings.Join(parts, ".")
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
	if field == nil || strings.TrimSpace(field.ValueSetName) == "" {
		return
	}
	values, ok := valueSets[strings.ToLower(field.ValueSetName)]
	if !ok || len(values) == 0 {
		return
	}
	field.PicklistValues = make([]PicklistValue, len(values))
	copy(field.PicklistValues, values)
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
	if raw.Active != nil {
		info.Active = *raw.Active
	}
	info.BooleanFilter = strings.TrimSpace(raw.BooleanFilter)
	info.ErrorMessage = raw.ErrorMessage
	for _, item := range raw.FilterItems {
		info.FilterItems = append(info.FilterItems, LookupFilterItem{Field: strings.TrimSpace(item.Field), Operation: strings.TrimSpace(item.Operation), Value: item.Value, ValueField: strings.TrimSpace(item.ValueField)})
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
	fieldDir := filepath.Dir(path)
	if strings.EqualFold(filepath.Base(filepath.Dir(fieldDir)), "objects") {
		return filepath.Base(fieldDir)
	}
	for dir := fieldDir; ; dir = filepath.Dir(dir) {
		objectDir := filepath.Dir(dir)
		if strings.EqualFold(filepath.Base(dir), "fields") && strings.EqualFold(filepath.Base(filepath.Dir(objectDir)), "objects") {
			return filepath.Base(objectDir)
		}
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
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

func metadataScale(scale *int) int {
	if scale == nil {
		return 0
	}
	return *scale
}
