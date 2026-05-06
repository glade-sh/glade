package storage

import (
	"sort"
	"strconv"
	"strings"
)

const FixtureVersion = "oaer.storage.v1"

// DefaultRESTAPIVersion is the REST API release string advertised by local HTTP
// surfaces when [OrgState.APIVersion] is empty (no leading "v").
const DefaultRESTAPIVersion = "65.0"

// NormalizeRESTAPIVersion trims whitespace and strips an optional leading "v".
func NormalizeRESTAPIVersion(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimPrefix(s, "v")
}

// EffectiveRESTAPIVersion returns the canonical REST API release (no leading "v")
// for persisted org version s, or [DefaultRESTAPIVersion] when s is blank.
func EffectiveRESTAPIVersion(s string) string {
	v := NormalizeRESTAPIVersion(s)
	if v != "" {
		return v
	}
	return DefaultRESTAPIVersion
}

type OrgState struct {
	OrgID        string                 `json:"orgId,omitempty"`
	APIVersion   string                 `json:"apiVersion,omitempty"`
	Namespace    string                 `json:"namespace,omitempty"`
	Metadata     MetadataRegistry       `json:"metadata,omitempty"`
	Objects      map[string]ObjectState `json:"objects"`
	IDSequences  map[string]uint64      `json:"idSequences,omitempty"`
	Transactions []TransactionFrame     `json:"transactions,omitempty"`
}

type MetadataRegistry struct {
	Labels          []LabelMetadata          `json:"labels,omitempty"`
	StaticResources []StaticResourceMetadata `json:"staticResources,omitempty"`
	ContentAssets   []ContentAssetMetadata   `json:"contentAssets,omitempty"`
	Endpoints       []EndpointMetadata       `json:"endpoints,omitempty"`
}

type LabelMetadata struct {
	Name             string `json:"name"`
	Namespace        string `json:"namespace,omitempty"`
	Language         string `json:"language,omitempty"`
	Value            string `json:"value,omitempty"`
	Protected        bool   `json:"protected,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	Categories       string `json:"categories,omitempty"`
	File             string `json:"file,omitempty"`
}

type StaticResourceMetadata struct {
	Name         string `json:"name"`
	ContentPath  string `json:"contentPath,omitempty"`
	MetadataPath string `json:"metadataPath,omitempty"`
	Content      string `json:"content,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	CacheControl string `json:"cacheControl,omitempty"`
	Description  string `json:"description,omitempty"`
	URL          string `json:"url,omitempty"`
}

type ContentAssetMetadata struct {
	Name         string `json:"name"`
	ContentPath  string `json:"contentPath,omitempty"`
	MetadataPath string `json:"metadataPath,omitempty"`
	Content      string `json:"content,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	Description  string `json:"description,omitempty"`
	URL          string `json:"url,omitempty"`
}

type EndpointMetadata struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	URL           string `json:"url,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	PrincipalType string `json:"principalType,omitempty"`
	Active        bool   `json:"active,omitempty"`
	File          string `json:"file,omitempty"`
}

type ObjectState struct {
	Definition ObjectDefinition    `json:"definition"`
	Records    map[ID]Record       `json:"records"`
	Indexes    map[string]IndexSet `json:"indexes,omitempty"`
}

type ObjectDefinition struct {
	APIName         string            `json:"apiName"`
	Label           string            `json:"label,omitempty"`
	PluralLabel     string            `json:"pluralLabel,omitempty"`
	KeyPrefix       string            `json:"keyPrefix,omitempty"`
	SharingModel    string            `json:"sharingModel,omitempty"`
	Fields          map[string]Field  `json:"fields,omitempty"`
	Relations       []Relationship    `json:"relationships,omitempty"`
	RecordTypes     []RecordTypeInfo  `json:"recordTypes,omitempty"`
	ValidationRules []ValidationRule  `json:"validationRules,omitempty"`
	WorkflowRules   []WorkflowRule    `json:"workflowRules,omitempty"`
	Indexes         []IndexDefinition `json:"indexes,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type Field struct {
	APIName          string          `json:"apiName"`
	Label            string          `json:"label,omitempty"`
	Type             FieldType       `json:"type"`
	DisplayType      string          `json:"displayType,omitempty"`
	DefaultValue     string          `json:"defaultValue,omitempty"`
	Required         bool            `json:"required,omitempty"`
	ExternalID       bool            `json:"externalId,omitempty"`
	Unique           bool            `json:"unique,omitempty"`
	Encrypted        bool            `json:"encrypted,omitempty"`
	CaseSensitive    bool            `json:"caseSensitive,omitempty"`
	ReferenceTo      []string        `json:"referenceTo,omitempty"`
	RelationshipName string          `json:"relationshipName,omitempty"`
	PicklistValues   []PicklistValue `json:"picklistValues,omitempty"`
}

type PicklistValue struct {
	Value   string `json:"value"`
	Label   string `json:"label,omitempty"`
	Default bool   `json:"default,omitempty"`
	Active  bool   `json:"active,omitempty"`
}

type RecordTypeInfo struct {
	ID            ID     `json:"id,omitempty"`
	DeveloperName string `json:"developerName"`
	Name          string `json:"name,omitempty"`
	Active        bool   `json:"active,omitempty"`
	Available     bool   `json:"available,omitempty"`
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

type WorkflowRule struct {
	Name         string                 `json:"name"`
	Active       bool                   `json:"active,omitempty"`
	Formula      string                 `json:"formula,omitempty"`
	Criteria     []WorkflowCriteriaItem `json:"criteria,omitempty"`
	FieldUpdates []WorkflowFieldUpdate  `json:"fieldUpdates,omitempty"`
}

type WorkflowCriteriaItem struct {
	Field     string `json:"field"`
	Operation string `json:"operation,omitempty"`
	Value     string `json:"value,omitempty"`
}

type WorkflowFieldUpdate struct {
	Name         string `json:"name"`
	Field        string `json:"field"`
	LiteralValue string `json:"literalValue,omitempty"`
	Formula      string `json:"formula,omitempty"`
	SourceField  string `json:"sourceField,omitempty"`
}

type FieldType string

const (
	FieldAny        FieldType = "ANY"
	FieldID         FieldType = "ID"
	FieldString     FieldType = "STRING"
	FieldBoolean    FieldType = "BOOLEAN"
	FieldInteger    FieldType = "INTEGER"
	FieldDecimal    FieldType = "DECIMAL"
	FieldDate       FieldType = "DATE"
	FieldDateTime   FieldType = "DATETIME"
	FieldPicklist   FieldType = "PICKLIST"
	FieldReference  FieldType = "REFERENCE"
	FieldBlob       FieldType = "BLOB"
	FieldAddress    FieldType = "ADDRESS"
	FieldLocation   FieldType = "LOCATION"
	FieldCalculated FieldType = "CALCULATED"
)

type Relationship struct {
	Field              string   `json:"field"`
	ParentObjects      []string `json:"parentObjects"`
	ParentRelationship string   `json:"parentRelationship,omitempty"`
	ChildRelationship  string   `json:"childRelationship,omitempty"`
	CascadeDelete      bool     `json:"cascadeDelete,omitempty"`
	RestrictedDelete   bool     `json:"restrictedDelete,omitempty"`
	Polymorphic        bool     `json:"polymorphic,omitempty"`
	DeferredIntegrity  bool     `json:"deferredIntegrity,omitempty"`
}

type Record struct {
	ID            ID                  `json:"id"`
	Object        string              `json:"object"`
	Fields        map[string]Value    `json:"fields,omitempty"`
	Children      map[string][]Record `json:"children,omitempty"`
	ExplicitNulls map[string]bool     `json:"explicitNulls,omitempty"`
	System        SystemFields        `json:"system,omitempty"`
}

type SystemFields struct {
	CreatedByID      ID     `json:"createdById,omitempty"`
	CreatedDate      string `json:"createdDate,omitempty"`
	LastModifiedByID ID     `json:"lastModifiedById,omitempty"`
	LastModifiedDate string `json:"lastModifiedDate,omitempty"`
	SystemModstamp   string `json:"systemModstamp,omitempty"`
	OwnerID          ID     `json:"ownerId,omitempty"`
	IsDeleted        bool   `json:"isDeleted,omitempty"`
	Locked           bool   `json:"locked,omitempty"`
}

type Value struct {
	Kind    ValueKind `json:"kind"`
	String  string    `json:"string,omitempty"`
	Integer int64     `json:"integer,omitempty"`
	Boolean bool      `json:"boolean,omitempty"`
	Decimal string    `json:"decimal,omitempty"`
	ID      ID        `json:"id,omitempty"`
	List    []Value   `json:"list,omitempty"`
}

type ValueKind string

const (
	ValueNull     ValueKind = "null"
	ValueString   ValueKind = "string"
	ValueInteger  ValueKind = "integer"
	ValueBoolean  ValueKind = "boolean"
	ValueDecimal  ValueKind = "decimal"
	ValueDate     ValueKind = "date"
	ValueDateTime ValueKind = "datetime"
	ValueID       ValueKind = "id"
	ValueBlob     ValueKind = "blob"
	ValueList     ValueKind = "list"
)

func NullValue() Value {
	return Value{Kind: ValueNull}
}

func StringValue(v string) Value {
	return Value{Kind: ValueString, String: v}
}

func IntegerValue(v int64) Value {
	return Value{Kind: ValueInteger, Integer: v}
}

func BooleanValue(v bool) Value {
	return Value{Kind: ValueBoolean, Boolean: v}
}

func DecimalValue(v string) Value {
	return Value{Kind: ValueDecimal, Decimal: v}
}

func DateValue(v string) Value {
	return Value{Kind: ValueDate, String: v}
}

func DateTimeValue(v string) Value {
	return Value{Kind: ValueDateTime, String: v}
}

func IDValue(v ID) Value {
	return Value{Kind: ValueID, ID: v}
}

func BlobValue(v string) Value {
	return Value{Kind: ValueBlob, String: v}
}

func ListValue(values ...Value) Value {
	return Value{Kind: ValueList, List: append([]Value(nil), values...)}
}

func DefaultValueForField(field Field) (Value, bool) {
	raw := strings.TrimSpace(field.DefaultValue)
	if raw == "" {
		return Value{}, false
	}
	switch field.Type {
	case FieldBoolean:
		if strings.EqualFold(raw, "true") {
			return BooleanValue(true), true
		}
		if strings.EqualFold(raw, "false") {
			return BooleanValue(false), true
		}
	case FieldInteger:
		value, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			return IntegerValue(value), true
		}
	case FieldDecimal:
		if _, err := strconv.ParseFloat(raw, 64); err == nil {
			return DecimalValue(raw), true
		}
	case FieldString, FieldPicklist, FieldDate, FieldDateTime, FieldID, FieldAny:
		return StringValue(strings.Trim(raw, `"`)), true
	}
	return Value{}, false
}

type IndexDefinition struct {
	Name          string   `json:"name"`
	Object        string   `json:"object"`
	Fields        []string `json:"fields"`
	Unique        bool     `json:"unique,omitempty"`
	ExternalID    bool     `json:"externalId,omitempty"`
	CaseSensitive bool     `json:"caseSensitive,omitempty"`
	Sparse        bool     `json:"sparse,omitempty"`
}

type IndexSet struct {
	Definition IndexDefinition `json:"definition"`
	Entries    map[string][]ID `json:"entries,omitempty"`
	Dirty      bool            `json:"dirty,omitempty"`
}

type TransactionFrame struct {
	ID        string     `json:"id"`
	Name      string     `json:"name,omitempty"`
	Depth     int        `json:"depth"`
	Mutations []Mutation `json:"mutations,omitempty"`
}

type Mutation struct {
	Op     MutationOp `json:"op"`
	Object string     `json:"object"`
	ID     ID         `json:"id"`
	Before *Record    `json:"before,omitempty"`
	After  *Record    `json:"after,omitempty"`
}

type MutationOp string

const (
	MutationInsert   MutationOp = "insert"
	MutationUpdate   MutationOp = "update"
	MutationDelete   MutationOp = "delete"
	MutationUndelete MutationOp = "undelete"
	MutationMerge    MutationOp = "merge"
)

func NewOrgState() OrgState {
	return OrgState{
		Objects:     make(map[string]ObjectState),
		IDSequences: make(map[string]uint64),
	}
}

func StripNamespaceToken(namespace, name string) string {
	if namespace == "" || name == "" {
		return name
	}
	prefix := namespace + "__"
	if strings.HasPrefix(name, prefix) {
		return strings.TrimPrefix(name, prefix)
	}
	return name
}

func ResolveObjectName(org OrgState, name string) (string, bool) {
	if _, ok := org.Objects[name]; ok {
		return name, true
	}
	if prefixed := NamespaceTokenName(org.Namespace, name); prefixed != name {
		if _, ok := org.Objects[prefixed]; ok {
			return prefixed, true
		}
	}
	stripped := StripNamespaceToken(org.Namespace, name)
	if stripped != name {
		if _, ok := org.Objects[stripped]; ok {
			return stripped, true
		}
	}
	candidates := make([]string, 0, len(org.Objects))
	for candidate := range org.Objects {
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)
	for _, candidate := range candidates {
		if strings.EqualFold(candidate, name) || strings.EqualFold(candidate, stripped) {
			return candidate, true
		}
	}
	return "", false
}

func ResolveFieldName(definition ObjectDefinition, namespace, name string) (string, bool) {
	if strings.EqualFold(name, "Id") {
		return "Id", true
	}
	if prefixed := NamespaceTokenName(namespace, name); prefixed != name {
		if _, ok := definition.Fields[prefixed]; ok {
			return prefixed, true
		}
	}
	if _, ok := definition.Fields[name]; ok {
		return name, true
	}
	stripped := StripNamespaceToken(namespace, name)
	if stripped != name {
		if _, ok := definition.Fields[stripped]; ok {
			return stripped, true
		}
	}
	candidates := make([]string, 0, len(definition.Fields))
	for candidate := range definition.Fields {
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)
	for _, candidate := range candidates {
		if strings.EqualFold(candidate, name) || strings.EqualFold(candidate, stripped) {
			return candidate, true
		}
	}
	return "", false
}

func NamespaceTokenName(namespace, name string) string {
	if namespace == "" || name == "" || !isCustomAPIName(name) || hasNamespaceToken(name) {
		return name
	}
	return namespace + "__" + name
}

func hasNamespaceToken(name string) bool {
	idx := strings.Index(name, "__")
	suffix := strings.LastIndex(name, "__")
	return idx > 0 && idx < suffix
}

func isCustomAPIName(name string) bool {
	return strings.HasSuffix(name, "__c") || strings.HasSuffix(name, "__r") || strings.HasSuffix(name, "__e") || strings.HasSuffix(name, "__mdt")
}

func IsCustomMetadataDefinition(definition ObjectDefinition) bool {
	if definition.Metadata != nil && definition.Metadata["kind"] == "customMetadata" {
		return true
	}
	return strings.HasSuffix(definition.APIName, "__mdt")
}

func IsCustomSettingDefinition(definition ObjectDefinition) bool {
	return definition.Metadata != nil && definition.Metadata["kind"] == "customSetting"
}

func IsCustomMetadataObject(org OrgState, name string) bool {
	objectName, ok := ResolveObjectName(org, name)
	if !ok {
		return strings.HasSuffix(StripNamespaceToken(org.Namespace, name), "__mdt")
	}
	return IsCustomMetadataDefinition(org.Objects[objectName].Definition)
}

func IsCustomSettingObject(org OrgState, name string) bool {
	objectName, ok := ResolveObjectName(org, name)
	if !ok {
		return false
	}
	return IsCustomSettingDefinition(org.Objects[objectName].Definition)
}

func (r Record) Clone() Record {
	out := r
	if r.Fields != nil {
		out.Fields = make(map[string]Value, len(r.Fields))
		for name, value := range r.Fields {
			out.Fields[name] = value.Clone()
		}
	}
	if r.ExplicitNulls != nil {
		out.ExplicitNulls = make(map[string]bool, len(r.ExplicitNulls))
		for name, value := range r.ExplicitNulls {
			out.ExplicitNulls[name] = value
		}
	}
	if r.Children != nil {
		out.Children = make(map[string][]Record, len(r.Children))
		for name, records := range r.Children {
			out.Children[name] = make([]Record, len(records))
			for i, record := range records {
				out.Children[name][i] = record.Clone()
			}
		}
	}
	return out
}

func (v Value) Clone() Value {
	out := v
	if v.List != nil {
		out.List = make([]Value, len(v.List))
		for i, value := range v.List {
			out.List[i] = value.Clone()
		}
	}
	return out
}

func (o ObjectState) Clone() ObjectState {
	out := o
	out.Definition = o.Definition.Clone()
	if o.Records != nil {
		out.Records = make(map[ID]Record, len(o.Records))
		for id, record := range o.Records {
			out.Records[id] = record.Clone()
		}
	}
	if o.Indexes != nil {
		out.Indexes = make(map[string]IndexSet, len(o.Indexes))
		for name, index := range o.Indexes {
			out.Indexes[name] = index.Clone()
		}
	}
	return out
}

func (d ObjectDefinition) Clone() ObjectDefinition {
	out := d
	if d.Fields != nil {
		out.Fields = make(map[string]Field, len(d.Fields))
		for name, field := range d.Fields {
			field.ReferenceTo = append([]string(nil), field.ReferenceTo...)
			out.Fields[name] = field
		}
	}
	out.Relations = append([]Relationship(nil), d.Relations...)
	for i := range out.Relations {
		out.Relations[i].ParentObjects = append([]string(nil), d.Relations[i].ParentObjects...)
	}
	out.RecordTypes = append([]RecordTypeInfo(nil), d.RecordTypes...)
	out.ValidationRules = append([]ValidationRule(nil), d.ValidationRules...)
	out.WorkflowRules = append([]WorkflowRule(nil), d.WorkflowRules...)
	for i := range out.WorkflowRules {
		out.WorkflowRules[i].Criteria = append([]WorkflowCriteriaItem(nil), d.WorkflowRules[i].Criteria...)
		out.WorkflowRules[i].FieldUpdates = append([]WorkflowFieldUpdate(nil), d.WorkflowRules[i].FieldUpdates...)
	}
	out.Indexes = append([]IndexDefinition(nil), d.Indexes...)
	for i := range out.Indexes {
		out.Indexes[i].Fields = append([]string(nil), d.Indexes[i].Fields...)
	}
	if d.Metadata != nil {
		out.Metadata = make(map[string]string, len(d.Metadata))
		for key, value := range d.Metadata {
			out.Metadata[key] = value
		}
	}
	return out
}

func (i IndexSet) Clone() IndexSet {
	out := i
	out.Definition.Fields = append([]string(nil), i.Definition.Fields...)
	if i.Entries != nil {
		out.Entries = make(map[string][]ID, len(i.Entries))
		for key, ids := range i.Entries {
			out.Entries[key] = append([]ID(nil), ids...)
		}
	}
	return out
}

func (o OrgState) Clone() OrgState {
	out := o
	if o.Objects != nil {
		out.Objects = make(map[string]ObjectState, len(o.Objects))
		for name, object := range o.Objects {
			out.Objects[name] = object.Clone()
		}
	}
	if o.IDSequences != nil {
		out.IDSequences = make(map[string]uint64, len(o.IDSequences))
		for object, sequence := range o.IDSequences {
			out.IDSequences[object] = sequence
		}
	}
	if o.Transactions != nil {
		out.Transactions = make([]TransactionFrame, len(o.Transactions))
		for i, transaction := range o.Transactions {
			out.Transactions[i] = transaction.Clone()
		}
	}
	return out
}

func (t TransactionFrame) Clone() TransactionFrame {
	out := t
	if t.Mutations != nil {
		out.Mutations = make([]Mutation, len(t.Mutations))
		for i, mutation := range t.Mutations {
			out.Mutations[i] = mutation.Clone()
		}
	}
	return out
}

func (m Mutation) Clone() Mutation {
	out := m
	if m.Before != nil {
		before := m.Before.Clone()
		out.Before = &before
	}
	if m.After != nil {
		after := m.After.Clone()
		out.After = &after
	}
	return out
}
