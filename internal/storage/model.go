package storage

import (
	"strconv"
	"strings"
	"time"
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
	Now          func() time.Time       `json:"-"`
}

type MetadataRegistry struct {
	Labels                 []LabelMetadata          `json:"labels,omitempty"`
	ManagedLabelNamespaces []string                 `json:"managedLabelNamespaces,omitempty"`
	Tabs                   []TabMetadata            `json:"tabs,omitempty"`
	FieldSets              []FieldSetMetadata       `json:"fieldSets,omitempty"`
	StaticResources        []StaticResourceMetadata `json:"staticResources,omitempty"`
	ContentAssets          []ContentAssetMetadata   `json:"contentAssets,omitempty"`
	Endpoints              []EndpointMetadata       `json:"endpoints,omitempty"`
	EmailTemplates         []EmailTemplateMetadata  `json:"emailTemplates,omitempty"`
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

type TabMetadata struct {
	Name        string `json:"name"`
	Label       string `json:"label,omitempty"`
	SObjectName string `json:"sObjectName,omitempty"`
	Custom      bool   `json:"custom,omitempty"`
	Motif       string `json:"motif,omitempty"`
	Description string `json:"description,omitempty"`
	File        string `json:"file,omitempty"`
}

type FieldSetMetadata struct {
	ObjectName string                   `json:"objectName,omitempty"`
	Name       string                   `json:"name"`
	Label      string                   `json:"label,omitempty"`
	Fields     []FieldSetMemberMetadata `json:"fields,omitempty"`
	File       string                   `json:"file,omitempty"`
}

type FieldSetMemberMetadata struct {
	Field    string `json:"field"`
	Required bool   `json:"required,omitempty"`
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

type EmailTemplateMetadata struct {
	Name          string `json:"name"`
	DeveloperName string `json:"developerName,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	Subject       string `json:"subject,omitempty"`
	Body          string `json:"body,omitempty"`
	HTMLValue     string `json:"htmlValue,omitempty"`
	Markup        string `json:"markup,omitempty"`
	TemplateType  string `json:"templateType,omitempty"`
	TemplateStyle string `json:"templateStyle,omitempty"`
	Encoding      string `json:"encoding,omitempty"`
	Description   string `json:"description,omitempty"`
	FolderName    string `json:"folderName,omitempty"`
	Active        bool   `json:"active,omitempty"`
	File          string `json:"file,omitempty"`
	MetadataPath  string `json:"metadataPath,omitempty"`
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
	FlowRules       []FlowRule        `json:"flowRules,omitempty"`
	Indexes         []IndexDefinition `json:"indexes,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type Field struct {
	APIName               string              `json:"apiName"`
	Label                 string              `json:"label,omitempty"`
	Type                  FieldType           `json:"type"`
	DisplayType           string              `json:"displayType,omitempty"`
	Length                int                 `json:"length,omitempty"`
	Precision             int                 `json:"precision,omitempty"`
	Scale                 int                 `json:"scale,omitempty"`
	Formula               string              `json:"formula,omitempty"`
	DefaultValue          string              `json:"defaultValue,omitempty"`
	AutoNumber            bool                `json:"autoNumber,omitempty"`
	DisplayFormat         string              `json:"displayFormat,omitempty"`
	SummarizedField       string              `json:"summarizedField,omitempty"`
	SummaryForeignKey     string              `json:"summaryForeignKey,omitempty"`
	SummaryOperation      string              `json:"summaryOperation,omitempty"`
	SummaryFilterItems    []SummaryFilterItem `json:"summaryFilterItems,omitempty"`
	Required              bool                `json:"required,omitempty"`
	Nillable              *bool               `json:"nillable,omitempty"`
	DefaultedOnCreate     *bool               `json:"defaultedOnCreate,omitempty"`
	Accessible            *bool               `json:"accessible,omitempty"`
	Createable            *bool               `json:"createable,omitempty"`
	Updateable            *bool               `json:"updateable,omitempty"`
	Filterable            *bool               `json:"filterable,omitempty"`
	Groupable             *bool               `json:"groupable,omitempty"`
	Sortable              *bool               `json:"sortable,omitempty"`
	Aggregatable          *bool               `json:"aggregatable,omitempty"`
	Permissionable        *bool               `json:"permissionable,omitempty"`
	DeprecatedAndHidden   *bool               `json:"deprecatedAndHidden,omitempty"`
	ExternalID            bool                `json:"externalId,omitempty"`
	Unique                bool                `json:"unique,omitempty"`
	Encrypted             bool                `json:"encrypted,omitempty"`
	CaseSensitive         bool                `json:"caseSensitive,omitempty"`
	ReferenceTo           []string            `json:"referenceTo,omitempty"`
	RelationshipName      string              `json:"relationshipName,omitempty"`
	ChildRelationshipName string              `json:"childRelationshipName,omitempty"`
	PicklistValues        []PicklistValue     `json:"picklistValues,omitempty"`
}

func BoolFlag(value bool) *bool {
	return &value
}

func FieldFlagValue(flag *bool, fallback bool) bool {
	if flag == nil {
		return fallback
	}
	return *flag
}

type SummaryFilterItem struct {
	Field     string `json:"field,omitempty"`
	Operation string `json:"operation,omitempty"`
	Value     string `json:"value,omitempty"`
}

func EnsureRecordTypeIDField(definition *ObjectDefinition) {
	if definition == nil || len(definition.RecordTypes) == 0 {
		return
	}
	if definition.Fields == nil {
		definition.Fields = make(map[string]Field)
	}
	if _, ok := ResolveFieldName(*definition, "", "RecordTypeId"); ok {
		return
	}
	definition.Fields["RecordTypeId"] = Field{
		APIName:          "RecordTypeId",
		Label:            "Record Type ID",
		Type:             FieldReference,
		DisplayType:      string(FieldReference),
		ReferenceTo:      []string{"RecordType"},
		RelationshipName: "RecordType",
	}
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
	EmailAlerts  []WorkflowEmailAlert   `json:"emailAlerts,omitempty"`
}

type WorkflowCriteriaItem struct {
	Field       string `json:"field"`
	Operation   string `json:"operation,omitempty"`
	Value       string `json:"value,omitempty"`
	SourceField string `json:"sourceField,omitempty"`
}

type WorkflowFieldUpdate struct {
	Name         string `json:"name"`
	Field        string `json:"field"`
	LiteralValue string `json:"literalValue,omitempty"`
	Formula      string `json:"formula,omitempty"`
	SourceField  string `json:"sourceField,omitempty"`
}

type WorkflowEmailAlert struct {
	Name       string                   `json:"name"`
	Template   string                   `json:"template,omitempty"`
	Recipients []WorkflowEmailRecipient `json:"recipients,omitempty"`
}

type WorkflowEmailRecipient struct {
	Type      string `json:"type,omitempty"`
	Field     string `json:"field,omitempty"`
	Recipient string `json:"recipient,omitempty"`
}

type FlowRule struct {
	Name          string                 `json:"name"`
	File          string                 `json:"file,omitempty"`
	Active        bool                   `json:"active,omitempty"`
	ProcessType   string                 `json:"processType,omitempty"`
	TriggerType   string                 `json:"triggerType,omitempty"`
	Formula       string                 `json:"formula,omitempty"`
	Criteria      []WorkflowCriteriaItem `json:"criteria,omitempty"`
	Branches      []FlowBranch           `json:"branches,omitempty"`
	FieldUpdates  []WorkflowFieldUpdate  `json:"fieldUpdates,omitempty"`
	Actions       []FlowAction           `json:"actions,omitempty"`
	RecordLookups []FlowRecordLookup     `json:"recordLookups,omitempty"`
	RecordCreates []FlowRecordCreate     `json:"recordCreates,omitempty"`
}

type FlowBranch struct {
	Name          string                 `json:"name"`
	Default       bool                   `json:"default,omitempty"`
	Criteria      []WorkflowCriteriaItem `json:"criteria,omitempty"`
	FieldUpdates  []WorkflowFieldUpdate  `json:"fieldUpdates,omitempty"`
	Actions       []FlowAction           `json:"actions,omitempty"`
	RecordLookups []FlowRecordLookup     `json:"recordLookups,omitempty"`
	RecordCreates []FlowRecordCreate     `json:"recordCreates,omitempty"`
}

type FlowAction struct {
	Name       string `json:"name"`
	ActionType string `json:"actionType,omitempty"`
	ActionName string `json:"actionName,omitempty"`
	ClassName  string `json:"className,omitempty"`
	MethodName string `json:"methodName,omitempty"`
}

type FlowRecordLookup struct {
	Name                     string                 `json:"name"`
	ObjectName               string                 `json:"objectName"`
	Criteria                 []WorkflowCriteriaItem `json:"criteria,omitempty"`
	GetFirstRecordOnly       bool                   `json:"getFirstRecordOnly,omitempty"`
	StoreOutputAutomatically bool                   `json:"storeOutputAutomatically,omitempty"`
}

type FlowRecordCreate struct {
	Name                     string                `json:"name"`
	ObjectName               string                `json:"objectName"`
	InputAssignments         []WorkflowFieldUpdate `json:"inputAssignments,omitempty"`
	StoreOutputAutomatically bool                  `json:"storeOutputAutomatically,omitempty"`
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
	FieldSummary    FieldType = "SUMMARY"
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

func (r Record) GetField(name string) (Value, bool) {
	if r.Fields == nil {
		return Value{}, false
	}
	if v, ok := r.Fields[name]; ok {
		return v, true
	}
	lower := strings.ToLower(name)
	for k, v := range r.Fields {
		if strings.ToLower(k) == lower {
			return v, true
		}
	}
	return Value{}, false
}

func (r Record) HasExplicitNull(name string) bool {
	if r.ExplicitNulls == nil {
		return false
	}
	if r.ExplicitNulls[name] {
		return true
	}
	lower := strings.ToLower(name)
	for k, v := range r.ExplicitNulls {
		if strings.ToLower(k) == lower && v {
			return true
		}
	}
	return false
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
		if field.Type == FieldPicklist {
			for _, value := range field.PicklistValues {
				if value.Default && strings.TrimSpace(value.Value) != "" {
					return StringValue(value.Value), true
				}
			}
		}
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
		return StringValue(normalizeStringDefaultValue(raw)), true
	}
	return Value{}, false
}

func DefaultValueForRecordField(definition ObjectDefinition, record Record, field Field) (Value, bool) {
	if value, ok := DefaultValueForField(field); ok {
		return value, true
	}
	raw := strings.TrimSpace(field.DefaultValue)
	if raw == "" {
		return Value{}, false
	}
	condition, trueValue, falseValue, ok := splitDefaultIF(raw)
	if !ok {
		return Value{}, false
	}
	branch := falseValue
	if evaluateDefaultCondition(definition, record, condition) {
		branch = trueValue
	}
	return defaultValueFromRaw(field, branch)
}

func splitDefaultIF(raw string) (string, string, string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < len("IF()") || !strings.EqualFold(raw[:2], "IF") {
		return "", "", "", false
	}
	open := strings.IndexByte(raw, '(')
	if open < 0 || raw[len(raw)-1] != ')' {
		return "", "", "", false
	}
	parts := splitDefaultArgs(raw[open+1 : len(raw)-1])
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func splitDefaultArgs(raw string) []string {
	var out []string
	start := 0
	depth := 0
	inString := false
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '\'':
			if inString && i+1 < len(raw) && raw[i+1] == '\'' {
				i++
				continue
			}
			inString = !inString
		case '(':
			if !inString {
				depth++
			}
		case ')':
			if !inString && depth > 0 {
				depth--
			}
		case ',':
			if !inString && depth == 0 {
				out = append(out, strings.TrimSpace(raw[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(raw[start:]))
	return out
}

func evaluateDefaultCondition(definition ObjectDefinition, record Record, condition string) bool {
	left, right, ok := splitDefaultComparison(condition, "==")
	if ok {
		return strings.EqualFold(defaultConditionValue(definition, record, left), normalizeStringDefaultValue(right))
	}
	left, right, ok = splitDefaultComparison(condition, "!=")
	if ok {
		return !strings.EqualFold(defaultConditionValue(definition, record, left), normalizeStringDefaultValue(right))
	}
	return false
}

func splitDefaultComparison(condition, op string) (string, string, bool) {
	if idx := strings.Index(condition, op); idx >= 0 {
		return strings.TrimSpace(condition[:idx]), strings.TrimSpace(condition[idx+len(op):]), true
	}
	return "", "", false
}

func defaultConditionValue(definition ObjectDefinition, record Record, name string) string {
	name = strings.TrimSpace(name)
	switch name {
	case "$RecordType.Name", "$RecordType.DeveloperName":
		recordTypeID := ""
		if value, ok := record.GetField("RecordTypeId"); ok {
			recordTypeID = strings.TrimSpace(defaultConditionScalar(value))
		}
		for _, recordType := range definition.RecordTypes {
			if recordTypeID != "" && string(recordType.ID) != recordTypeID {
				continue
			}
			if name == "$RecordType.DeveloperName" {
				return recordType.DeveloperName
			}
			if recordType.Name != "" {
				return recordType.Name
			}
			return recordType.DeveloperName
		}
	}
	return ""
}

func defaultConditionScalar(value Value) string {
	switch value.Kind {
	case ValueString, ValueDate, ValueDateTime, ValueBlob:
		return value.String
	case ValueID:
		return string(value.ID)
	case ValueInteger:
		return strconv.FormatInt(value.Integer, 10)
	case ValueDecimal:
		return value.Decimal
	case ValueBoolean:
		if value.Boolean {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func defaultValueFromRaw(field Field, raw string) (Value, bool) {
	raw = strings.TrimSpace(raw)
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
		return StringValue(normalizeStringDefaultValue(raw)), true
	}
	return Value{}, false
}

func normalizeStringDefaultValue(raw string) string {
	if len(raw) >= 2 {
		if raw[0] == '\'' && raw[len(raw)-1] == '\'' {
			return strings.ReplaceAll(raw[1:len(raw)-1], "''", "'")
		}
		if raw[0] == '"' && raw[len(raw)-1] == '"' {
			return raw[1 : len(raw)-1]
		}
	}
	return raw
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
	if len(name) >= len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) {
		return name[len(prefix):]
	}
	return name
}

func ResolveObjectName(org OrgState, name string) (string, bool) {
	if _, ok := org.Objects[name]; ok {
		return name, true
	}
	prefixed := NamespaceTokenName(org.Namespace, name)
	if prefixed != name {
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
	for candidate := range org.Objects {
		if strings.EqualFold(candidate, name) || strings.EqualFold(candidate, prefixed) || strings.EqualFold(candidate, stripped) {
			return candidate, true
		}
	}
	return "", false
}

func ResolveFieldName(definition ObjectDefinition, namespace, name string) (string, bool) {
	if strings.EqualFold(name, "Id") {
		return "Id", true
	}
	prefixed := NamespaceTokenName(namespace, name)
	if prefixed != name {
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
	if resolved, ok := resolveLocationComponentField(definition, namespace, name); ok {
		return resolved, true
	}
	for candidate := range definition.Fields {
		if strings.EqualFold(candidate, name) || strings.EqualFold(candidate, prefixed) || strings.EqualFold(candidate, stripped) {
			return candidate, true
		}
	}
	return "", false
}

func resolveLocationComponentField(definition ObjectDefinition, namespace, name string) (string, bool) {
	for _, suffix := range []string{"__Latitude__s", "__Longitude__s"} {
		if !strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
			continue
		}
		base := strings.TrimSuffix(name, suffix) + "__c"
		for _, candidateBase := range []string{
			NamespaceTokenName(namespace, base),
			base,
			StripNamespaceToken(namespace, base),
		} {
			field, ok := definition.Fields[candidateBase]
			if !ok || field.Type != FieldLocation {
				continue
			}
			return strings.TrimSuffix(candidateBase, "__c") + suffix, true
		}
		for candidate := range definition.Fields {
			candidateBase := candidate
			field := definition.Fields[candidateBase]
			if field.Type != FieldLocation {
				continue
			}
			candidateComponent := strings.TrimSuffix(candidateBase, "__c") + suffix
			if strings.EqualFold(candidateComponent, name) ||
				strings.EqualFold(candidateComponent, NamespaceTokenName(namespace, name)) ||
				strings.EqualFold(candidateComponent, StripNamespaceToken(namespace, name)) ||
				strings.EqualFold(locationComponentLocalName(candidateComponent, suffix), locationComponentLocalName(name, suffix)) {
				return candidateComponent, true
			}
		}
	}
	return "", false
}

func locationComponentLocalName(name, suffix string) string {
	if !strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
		return name
	}
	base := strings.TrimSuffix(name, suffix)
	if idx := strings.Index(base, "__"); idx > 0 && idx+2 < len(base) {
		base = base[idx+2:]
	}
	return base + suffix
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
	return hasAPISuffix(name, "__c") || hasAPISuffix(name, "__r") || hasAPISuffix(name, "__e") || hasAPISuffix(name, "__mdt")
}

func hasAPISuffix(name, suffix string) bool {
	return len(name) >= len(suffix) && strings.EqualFold(name[len(name)-len(suffix):], suffix)
}

func IsCustomMetadataDefinition(definition ObjectDefinition) bool {
	if definition.Metadata != nil && definition.Metadata["kind"] == "customMetadata" {
		return true
	}
	return hasAPISuffix(definition.APIName, "__mdt")
}

func IsCustomSettingDefinition(definition ObjectDefinition) bool {
	return definition.Metadata != nil && definition.Metadata["kind"] == "customSetting"
}

func IsCustomMetadataObject(org OrgState, name string) bool {
	objectName, ok := ResolveObjectName(org, name)
	if !ok {
		return hasAPISuffix(StripNamespaceToken(org.Namespace, name), "__mdt")
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
		out.WorkflowRules[i].EmailAlerts = append([]WorkflowEmailAlert(nil), d.WorkflowRules[i].EmailAlerts...)
		for j := range out.WorkflowRules[i].EmailAlerts {
			out.WorkflowRules[i].EmailAlerts[j].Recipients = append([]WorkflowEmailRecipient(nil), d.WorkflowRules[i].EmailAlerts[j].Recipients...)
		}
	}
	out.FlowRules = append([]FlowRule(nil), d.FlowRules...)
	for i := range out.FlowRules {
		out.FlowRules[i].Criteria = append([]WorkflowCriteriaItem(nil), d.FlowRules[i].Criteria...)
		out.FlowRules[i].FieldUpdates = append([]WorkflowFieldUpdate(nil), d.FlowRules[i].FieldUpdates...)
		out.FlowRules[i].Actions = append([]FlowAction(nil), d.FlowRules[i].Actions...)
		out.FlowRules[i].RecordLookups = append([]FlowRecordLookup(nil), d.FlowRules[i].RecordLookups...)
		for j := range out.FlowRules[i].RecordLookups {
			out.FlowRules[i].RecordLookups[j].Criteria = append([]WorkflowCriteriaItem(nil), d.FlowRules[i].RecordLookups[j].Criteria...)
		}
		out.FlowRules[i].RecordCreates = append([]FlowRecordCreate(nil), d.FlowRules[i].RecordCreates...)
		for j := range out.FlowRules[i].RecordCreates {
			out.FlowRules[i].RecordCreates[j].InputAssignments = append([]WorkflowFieldUpdate(nil), d.FlowRules[i].RecordCreates[j].InputAssignments...)
		}
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

// CloneRuntime returns an org copy suitable for isolated test/runtime execution.
// Records, indexes, sequences, and transaction frames are isolated. Object
// definitions are shared because normal test execution treats metadata as
// read-only; runtime metadata mutation paths clone definitions before writing.
func (o OrgState) CloneRuntime() OrgState {
	out := o
	if o.Objects != nil {
		out.Objects = make(map[string]ObjectState, len(o.Objects))
		for name, object := range o.Objects {
			out.Objects[name] = object.CloneRuntime()
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

func (o ObjectState) CloneRuntime() ObjectState {
	out := o
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

func isRuntimeMutableDefinition(definition ObjectDefinition) bool {
	if definition.Metadata != nil {
		return true
	}
	return isCustomAPIName(definition.APIName)
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
