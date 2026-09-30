package sobject

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
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
	EnableSearch    bool                           `json:"enableSearch,omitempty"`
	Metadata        map[string]string              `json:"metadata,omitempty"`
	Fields          map[string]DescribeFieldResult `json:"fields,omitempty"`
	Relationships   []storage.Relationship         `json:"relationships,omitempty"`
	RecordTypes     []DescribeRecordTypeInfo       `json:"recordTypes,omitempty"`
	ValidationRules []storage.ValidationRule       `json:"validationRules,omitempty"`
}

type DescribeFieldResult struct {
	Name                  string                      `json:"name"`
	Type                  storage.FieldType           `json:"type"`
	MasterDetail          bool                        `json:"masterDetail,omitempty"`
	DisplayType           string                      `json:"displayType,omitempty"`
	Label                 string                      `json:"label,omitempty"`
	InlineHelpText        string                      `json:"inlineHelpText,omitempty"`
	Length                int                         `json:"length,omitempty"`
	Precision             int                         `json:"precision,omitempty"`
	Scale                 int                         `json:"scale,omitempty"`
	ScaleSpecified        bool                        `json:"scaleSpecified,omitempty"`
	Formula               string                      `json:"formula,omitempty"`
	FormulaTreatBlanksAs  string                      `json:"formulaTreatBlanksAs,omitempty"`
	CompoundFieldName     string                      `json:"compoundFieldName,omitempty"`
	AutoNumber            bool                        `json:"autoNumber,omitempty"`
	DisplayFormat         string                      `json:"displayFormat,omitempty"`
	SummarizedField       string                      `json:"summarizedField,omitempty"`
	SummaryForeignKey     string                      `json:"summaryForeignKey,omitempty"`
	SummaryOperation      string                      `json:"summaryOperation,omitempty"`
	SummaryFilterItems    []storage.SummaryFilterItem `json:"summaryFilterItems,omitempty"`
	FilteredLookupInfo    storage.FilteredLookupInfo  `json:"filteredLookupInfo,omitempty"`
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
	RelationshipOrder     *int                        `json:"relationshipOrder,omitempty"`
	ReparentableMasterDetail bool                     `json:"reparentableMasterDetail,omitempty"`
	ChildRelationshipName string                      `json:"childRelationshipName,omitempty"`
	DeleteConstraint      string                      `json:"deleteConstraint,omitempty"`
	DefaultValue          string                      `json:"defaultValue,omitempty"`
	Required              bool                        `json:"required,omitempty"`
	ExternalID            bool                        `json:"externalId,omitempty"`
	Unique                bool                        `json:"unique,omitempty"`
	Encrypted             bool                        `json:"encrypted,omitempty"`
	CaseSensitive         bool                        `json:"caseSensitive,omitempty"`
	RestrictedPicklist    bool                        `json:"restrictedPicklist,omitempty"`
	IDLookup              bool                        `json:"idLookup,omitempty"`
	NamePointing          bool                        `json:"namePointing,omitempty"`
	PicklistController    string                      `json:"picklistController,omitempty"`
	PicklistValueSettings []storage.PicklistSetting   `json:"picklistValueSettings,omitempty"`
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
	baseObjectNames := objectNames(objects)
	objects = appendGeneratedShareObjects(objects)
	sort.Slice(objects, func(i, j int) bool { return objects[i].Name < objects[j].Name })
	baseExplicitPrefixes := make(map[string]string)
	customSettingIndex := 0
	for _, object := range objects {
		if object.CustomSettingsType == "" {
			continue
		}
		baseExplicitPrefixes[object.Name] = storage.CustomSettingPrefix(customSettingIndex)
		customSettingIndex++
	}
	// Allocate prefixes for source-backed objects before adding generated
	// share tables. A generated auxiliary object must not renumber the source
	// object's stable local ID prefix.
	basePrefixes := storage.AssignDeterministicPrefixes(baseObjectNames, baseExplicitPrefixes)
	explicitPrefixes := make(map[string]string, len(baseObjectNames))
	for _, name := range baseObjectNames {
		explicitPrefixes[name] = basePrefixes[name]
	}
	prefixes := storage.AssignDeterministicPrefixes(objectNames(objects), explicitPrefixes)

	registry := DescribeRegistry{Objects: make(map[string]DescribeSObjectResult, len(objects))}
	recordTypeIDs := storage.NewIDGenerator(map[string]string{"RecordType": storage.StandardKeyPrefix("RecordType")})
	for _, object := range objects {
		describe := DescribeSObjectResult{
			Name:         object.Name,
			Label:        object.Label,
			PluralLabel:  object.PluralLabel,
			KeyPrefix:    prefixes[object.Name],
			SharingModel: object.SharingModel,
			EnableSearch: object.EnableSearch,
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
			if strings.EqualFold(object.CustomSettingsType, "List") {
				nameField := describe.Fields["Name"]
				nameField.Required = true
				describe.Fields["Name"] = nameField
			}
			describe.Metadata = map[string]string{"kind": "customSetting", "customSettingsType": object.CustomSettingsType}
		}
		if object.PublishBehavior != "" {
			if describe.Metadata == nil {
				describe.Metadata = make(map[string]string)
			}
			describe.Metadata["publishBehavior"] = object.PublishBehavior
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
				Length:        nameFieldLength(object.NameField),
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
			autoNumber := strings.EqualFold(field.Type, "AutoNumber")
			var updateable *bool
			if strings.EqualFold(field.Type, "MasterDetail") {
				updateable = storage.BoolFlag(field.ReparentableMasterDetail)
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
				MasterDetail:          strings.EqualFold(field.Type, "MasterDetail"),
				DisplayType:           displayFieldType(field.Type),
				Label:                 labelOrName(field.Label, field.Name),
				InlineHelpText:        field.InlineHelpText,
				AutoNumber:            autoNumber,
				DisplayFormat:         field.DisplayFormat,
				Length:                field.Length,
				Precision:             field.Precision,
				Scale:                 field.Scale,
				ScaleSpecified:        field.ScaleSpecified,
				Formula:               field.Formula,
				FormulaTreatBlanksAs:  field.FormulaTreatBlanksAs,
				ReferenceTo:           referenceTargets(field.ReferenceTo),
				SummarizedField:       field.SummarizedField,
				SummaryForeignKey:     field.SummaryForeignKey,
				SummaryOperation:      field.SummaryOperation,
				SummaryFilterItems:    storageSummaryFilters(field.SummaryFilterItems),
				FilteredLookupInfo:    storageFilteredLookupInfo(field.FilteredLookupInfo),
				RelationshipOrder:     cloneIntPtr(field.RelationshipOrder),
				ReparentableMasterDetail: field.ReparentableMasterDetail,
				RelationshipName:      field.RelationshipName,
				ChildRelationshipName: childRelationshipName,
				DeleteConstraint:      field.DeleteConstraint,
				DefaultValue:          field.DefaultValue,
				Required:              field.Required || strings.EqualFold(field.Type, "MasterDetail"),
				ExternalID:            field.ExternalID,
				Unique:                field.Unique,
				IDLookup:              field.IDLookup || field.ExternalID,
				Encrypted:             field.Encrypted,
				RestrictedPicklist:    field.RestrictedPicklist,
				NamePointing:          len(references) > 1,
				Updateable:            updateable,
				PicklistController:    field.PicklistController,
				PicklistValueSettings: storagePicklistSettings(field.PicklistValueSettings),
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
					SetNullOnDelete:    strings.EqualFold(field.DeleteConstraint, "SetNull"),
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
				Namespace:             rule.Namespace,
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


// appendGeneratedShareObjects adds the describe shape Salesforce exposes for
// custom objects whose metadata enables record sharing. The share table is a
// platform-generated object, so it is not present in source metadata, but
// Apex can still resolve and query it through Schema.getGlobalDescribe().
func appendGeneratedShareObjects(objects []schema.Object) []schema.Object {
	seen := make(map[string]bool, len(objects))
	for _, object := range objects {
		seen[strings.ToLower(strings.TrimSpace(object.Name))] = true
	}
	out := append([]schema.Object(nil), objects...)
	for _, object := range objects {
		name := strings.TrimSpace(object.Name)
		if !strings.HasSuffix(strings.ToLower(name), "__c") {
			continue
		}
		if object.CustomSettingsType != "" {
			continue
		}
		// Metadata exported from Salesforce includes enableSharing. The
		// sharing-model fallback handles older/minimal metadata that records
		// a share-capable OWD without the platform flag. An empty model is
		// unknown metadata, not evidence that Salesforce omitted the generated
		// share table.
		if !object.EnableSharing && object.SharingModel != "" && !shareCapableSharingModel(object.SharingModel) {
			continue
		}
		shareName := name[:len(name)-3] + "__Share"
		key := strings.ToLower(shareName)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, schema.Object{
			Name:         shareName,
			Label:        labelOrName(object.Label, name) + " Share",
			PluralLabel:  labelOrName(object.PluralLabel, name) + " Shares",
			SharingModel: "ReadWrite",
			Fields: []schema.Field{
				{Name: "ParentId", Type: "Lookup", ReferenceTo: []string{name}, RelationshipName: "Parent"},
				{Name: "UserOrGroupId", Type: "Lookup", ReferenceTo: []string{"User", "Group"}, RelationshipName: "UserOrGroup"},
				{Name: "AccessLevel", Type: "Picklist", PicklistValues: []schema.PicklistValue{
					{FullName: "Read", Label: "Read", Active: true},
					{FullName: "Edit", Label: "Edit", Active: true},
					{FullName: "All", Label: "All", Active: true},
				}},
				{Name: "RowCause", Type: "Picklist", DefaultValue: "Manual", PicklistValues: append([]schema.PicklistValue{
					{FullName: "Manual", Label: "Manual", Default: true, Active: true},
					{FullName: "Owner", Label: "Owner", Active: true},
					{FullName: "Rule", Label: "Rule", Active: true},
					{FullName: "ImplicitChild", Label: "Implicit Child", Active: true},
					{FullName: "ImplicitParent", Label: "Implicit Parent", Active: true},
				}, sharingReasonPicklistValues(object.SharingReasons)...),
				},
			},
		})
	}
	return out
}

func sharingReasonPicklistValues(reasons []string) []schema.PicklistValue {
	values := make([]schema.PicklistValue, 0, len(reasons))
	for _, reason := range reasons {
		name := strings.TrimSpace(reason)
		if name == "" {
			continue
		}
		values = append(values, schema.PicklistValue{FullName: name, Label: name, Active: true})
	}
	return values
}

func stringSliceContainsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func shareCapableSharingModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "private", "read", "publicreadonly", "readwrite":
		return true
	default:
		return false
	}
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
	if overlay.EnableSharing {
		base.EnableSharing = true
	}
	for _, reason := range overlay.SharingReasons {
		if !stringSliceContainsFold(base.SharingReasons, reason) {
			base.SharingReasons = append(base.SharingReasons, reason)
		}
	}
	if overlay.CustomSettingsType != "" {
		base.CustomSettingsType = overlay.CustomSettingsType
	}
	if overlay.EnableSearch {
		base.EnableSearch = true
	}
	if overlay.NameField.Type != "" || overlay.NameField.Label != "" || overlay.NameField.DisplayFormat != "" || overlay.NameField.Length != 0 {
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
		out.Relationships[i].JunctionIDListNames = append([]string(nil), d.Relationships[i].JunctionIDListNames...)
		out.Relationships[i].JunctionReferenceTo = append([]string(nil), d.Relationships[i].JunctionReferenceTo...)
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
		EnableSearch:    describe.EnableSearch,
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
			InlineHelpText:        field.InlineHelpText,
			Type:                  field.Type,
			MasterDetail:          field.MasterDetail,
			DisplayType:           field.DisplayType,
			Length:                field.Length,
			Precision:             field.Precision,
			Scale:                 field.Scale,
			ScaleSpecified:        field.ScaleSpecified,
			Formula:               field.Formula,
			FormulaTreatBlanksAs:  field.FormulaTreatBlanksAs,
			CompoundFieldName:     field.CompoundFieldName,
			DefaultValue:          field.DefaultValue,
			AutoNumber:            field.AutoNumber,
			DisplayFormat:         field.DisplayFormat,
			SummarizedField:       field.SummarizedField,
			SummaryForeignKey:     field.SummaryForeignKey,
			SummaryOperation:      field.SummaryOperation,
			SummaryFilterItems:    append([]storage.SummaryFilterItem(nil), field.SummaryFilterItems...),
			FilteredLookupInfo:    cloneStorageFilteredLookupInfo(field.FilteredLookupInfo),
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
			RestrictedPicklist:    field.RestrictedPicklist,
			IDLookup:              field.IDLookup,
			NamePointing:          field.NamePointing,
			ReferenceTo:           append([]string(nil), field.ReferenceTo...),
			RelationshipName:      field.RelationshipName,
			RelationshipOrder:     cloneIntPtr(field.RelationshipOrder),
			ReparentableMasterDetail: field.ReparentableMasterDetail,
			ChildRelationshipName: field.ChildRelationshipName,
			PicklistController:    field.PicklistController,
			PicklistValueSettings: cloneStoragePicklistSettings(field.PicklistValueSettings),
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
		EnableSearch:    definition.EnableSearch,
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
			MasterDetail:          field.MasterDetail,
			DisplayType:           field.DisplayType,
			Label:                 labelOrName(field.Label, field.APIName),
			InlineHelpText:        field.InlineHelpText,
			Length:                field.Length,
			Precision:             field.Precision,
			Scale:                 field.Scale,
			ScaleSpecified:        field.ScaleSpecified,
			Formula:               field.Formula,
			FormulaTreatBlanksAs:  field.FormulaTreatBlanksAs,
			CompoundFieldName:     field.CompoundFieldName,
			AutoNumber:            field.AutoNumber,
			DisplayFormat:         field.DisplayFormat,
			SummarizedField:       field.SummarizedField,
			SummaryForeignKey:     field.SummaryForeignKey,
			SummaryOperation:      field.SummaryOperation,
			SummaryFilterItems:    append([]storage.SummaryFilterItem(nil), field.SummaryFilterItems...),
			FilteredLookupInfo:    cloneStorageFilteredLookupInfo(field.FilteredLookupInfo),
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
			RelationshipOrder:     cloneIntPtr(field.RelationshipOrder),
			ReparentableMasterDetail: field.ReparentableMasterDetail,
			ChildRelationshipName: field.ChildRelationshipName,
			DefaultValue:          field.DefaultValue,
			Required:              field.Required,
			ExternalID:            field.ExternalID,
			Unique:                field.Unique,
			Encrypted:             field.Encrypted,
			CaseSensitive:         field.CaseSensitive,
			RestrictedPicklist:    field.RestrictedPicklist,
			IDLookup:              field.IDLookup,
			NamePointing:          field.NamePointing,
			PicklistController:    field.PicklistController,
			PicklistValueSettings: cloneStoragePicklistSettings(field.PicklistValueSettings),
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

func nameFieldLength(field schema.NameField) int {
	if field.Length > 0 {
		return field.Length
	}
	if strings.EqualFold(field.Type, "Text") {
		return 80
	}
	return 0
}

func displayFieldType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "number":
		return "DOUBLE"
	case "currency":
		return "CURRENCY"
	case "percent":
		return "PERCENT"
	case "textarea", "longtextarea":
		return "TEXTAREA"
	case "html":
		return "RICHTEXTAREA"
	case "email":
		return "EMAIL"
	case "url":
		return "URL"
	case "autonumber":
		return "STRING"
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

func storagePicklistSettings(values []schema.PicklistSetting) []storage.PicklistSetting {
	out := make([]storage.PicklistSetting, 0, len(values))
	for _, value := range values {
		out = append(out, storage.PicklistSetting{
			ValueName:              value.ValueName,
			ControllingFieldValues: append([]string(nil), value.ControllingFieldValues...),
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

func storageFilteredLookupInfo(value schema.FilteredLookupInfo) storage.FilteredLookupInfo {
	var items []storage.LookupFilterItem
	for _, item := range value.FilterItems {
		items = append(items, storage.LookupFilterItem{Field: item.Field, Operation: item.Operation, Value: item.Value, ValueField: item.ValueField})
	}
	return storage.FilteredLookupInfo{
		Active: value.Active, BooleanFilter: value.BooleanFilter, ErrorMessage: value.ErrorMessage, FilterItems: items,
		ControllingFields: append([]string(nil), value.ControllingFields...),
		Dependent:         value.Dependent,
		OptionalFilter:    value.OptionalFilter,
	}
}

func cloneStorageFilteredLookupInfo(value storage.FilteredLookupInfo) storage.FilteredLookupInfo {
	value.FilterItems = append([]storage.LookupFilterItem(nil), value.FilterItems...)
	value.ControllingFields = append([]string(nil), value.ControllingFields...)
	return value
}

func cloneStoragePicklistSettings(values []storage.PicklistSetting) []storage.PicklistSetting {
	out := append([]storage.PicklistSetting(nil), values...)
	for i := range out {
		out[i].ControllingFieldValues = append([]string(nil), values[i].ControllingFieldValues...)
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

func cloneIntPtr(in *int) *int {
	if in == nil {
		return nil
	}
	out := *in
	return &out
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
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "text", "textarea", "longtextarea", "html", "email", "phone", "url", "encryptedtext", "autonumber":
		return storage.FieldString
	case "picklist":
		return storage.FieldPicklist
	case "multiselectpicklist":
		return storage.FieldMultiPicklist
	case "checkbox":
		return storage.FieldBoolean
	case "number", "currency", "percent":
		return storage.FieldDecimal
	case "date":
		return storage.FieldDate
	case "datetime":
		return storage.FieldDateTime
	case "time":
		return storage.FieldTime
	case "location":
		return storage.FieldLocation
	case "lookup", "masterdetail", "metadatarelationship":
		return storage.FieldReference
	case "id":
		return storage.FieldID
	case "base64":
		return storage.FieldBlob
	case "formula":
		return storage.FieldCalculated
	case "summary":
		return storage.FieldSummary
	default:
		return storage.FieldAny
	}
}
