package storage

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

const FixtureVersion = "glade.storage.v1"

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

	SystemTimestampBase     string `json:"-"`
	SystemTimestampSequence int64  `json:"-"`
	RuntimeSchemaStamp      string `json:"-"`

	objectNameCache *sync.Map
}

func (o *OrgState) ClearRuntimeSchemaStamp() {
	if o != nil {
		o.RuntimeSchemaStamp = ""
	}
}

type MetadataRegistry struct {
	Labels                 []LabelMetadata          `json:"labels,omitempty"`
	ManagedLabelNamespaces []string                 `json:"managedLabelNamespaces,omitempty"`
	Tabs                   []TabMetadata            `json:"tabs,omitempty"`
	DataCategoryGroups     []DataCategoryGroup      `json:"dataCategoryGroups,omitempty"`
	QuickActions           []QuickActionMetadata    `json:"quickActions,omitempty"`
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

type DataCategoryGroup struct {
	Name        string         `json:"name"`
	Label       string         `json:"label,omitempty"`
	Description string         `json:"description,omitempty"`
	SObjectName string         `json:"sObjectName,omitempty"`
	Categories  []DataCategory `json:"categories,omitempty"`
}

type DataCategory struct {
	Name     string         `json:"name"`
	Label    string         `json:"label,omitempty"`
	Children []DataCategory `json:"children,omitempty"`
}

type QuickActionMetadata struct {
	Name                  string                  `json:"name"`
	Label                 string                  `json:"label,omitempty"`
	Type                  string                  `json:"type,omitempty"`
	TargetObject          string                  `json:"targetObject,omitempty"`
	PredefinedFieldValues []QuickActionFieldValue `json:"predefinedFieldValues,omitempty"`
	File                  string                  `json:"file,omitempty"`
}

type QuickActionFieldValue struct {
	Field string `json:"field"`
	Value string `json:"value,omitempty"`
}

type FieldSetMetadata struct {
	ObjectName  string                   `json:"objectName,omitempty"`
	Namespace   string                   `json:"namespace,omitempty"`
	Name        string                   `json:"name"`
	Label       string                   `json:"label,omitempty"`
	Description string                   `json:"description,omitempty"`
	Fields      []FieldSetMemberMetadata `json:"fields,omitempty"`
	File        string                   `json:"file,omitempty"`
}

type FieldSetMemberMetadata struct {
	Field    string `json:"field"`
	Required bool   `json:"required,omitempty"`
}

type StaticResourceMetadata struct {
	Name            string            `json:"name"`
	NamespacePrefix string            `json:"namespacePrefix,omitempty"`
	ContentPath     string            `json:"contentPath,omitempty"`
	MetadataPath    string            `json:"metadataPath,omitempty"`
	Files           map[string]string `json:"files,omitempty"`
	Content         string            `json:"content,omitempty"`
	ContentType     string            `json:"contentType,omitempty"`
	CacheControl    string            `json:"cacheControl,omitempty"`
	Description     string            `json:"description,omitempty"`
	URL             string            `json:"url,omitempty"`
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
	Definition    ObjectDefinition    `json:"definition"`
	Records       map[ID]Record       `json:"records"`
	Indexes       map[string]IndexSet `json:"indexes,omitempty"`
	RecordsShared bool                `json:"-"`
	IndexesShared bool                `json:"-"`
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
	InlineHelpText        string              `json:"inlineHelpText,omitempty"`
	Type                  FieldType           `json:"type"`
	DisplayType           string              `json:"displayType,omitempty"`
	Length                int                 `json:"length,omitempty"`
	Precision             int                 `json:"precision,omitempty"`
	Scale                 int                 `json:"scale,omitempty"`
	Formula               string              `json:"formula,omitempty"`
	DefaultValue          string              `json:"defaultValue,omitempty"`
	CompoundFieldName     string              `json:"compoundFieldName,omitempty"`
	AutoNumber            bool                `json:"autoNumber,omitempty"`
	DisplayFormat         string              `json:"displayFormat,omitempty"`
	SummarizedField       string              `json:"summarizedField,omitempty"`
	SummaryForeignKey     string              `json:"summaryForeignKey,omitempty"`
	SummaryOperation      string              `json:"summaryOperation,omitempty"`
	SummaryFilterItems    []SummaryFilterItem `json:"summaryFilterItems,omitempty"`
	FilteredLookupInfo    FilteredLookupInfo  `json:"filteredLookupInfo,omitempty"`
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
	RestrictedPicklist    bool                `json:"restrictedPicklist,omitempty"`
	IDLookup              bool                `json:"idLookup,omitempty"`
	NamePointing          bool                `json:"namePointing,omitempty"`
	ReferenceTo           []string            `json:"referenceTo,omitempty"`
	RelationshipName      string              `json:"relationshipName,omitempty"`
	RelationshipOrder     *int                `json:"relationshipOrder,omitempty"`
	ChildRelationshipName string              `json:"childRelationshipName,omitempty"`
	PicklistController    string              `json:"picklistController,omitempty"`
	PicklistValueSettings []PicklistSetting   `json:"picklistValueSettings,omitempty"`
	PicklistValues        []PicklistValue     `json:"picklistValues,omitempty"`
}

type FilteredLookupInfo struct {
	ControllingFields []string `json:"controllingFields,omitempty"`
	Dependent         bool     `json:"dependent,omitempty"`
	OptionalFilter    bool     `json:"optionalFilter,omitempty"`
}

func BoolFlag(value bool) *bool {
	return &value
}

func IntFlag(value int) *int {
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

type PicklistSetting struct {
	ValueName              string   `json:"valueName,omitempty"`
	ControllingFieldValues []string `json:"controllingFieldValues,omitempty"`
}

type RecordTypeInfo struct {
	ID               ID                `json:"id,omitempty"`
	DeveloperName    string            `json:"developerName"`
	Name             string            `json:"name,omitempty"`
	Active           bool              `json:"active,omitempty"`
	Available        bool              `json:"available,omitempty"`
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

type WorkflowTask struct {
	Name             string `json:"name"`
	AssignedToType   string `json:"assignedToType,omitempty"`
	AssignedTo       string `json:"assignedTo,omitempty"`
	Description      string `json:"description,omitempty"`
	DueDateOffset    int    `json:"dueDateOffset,omitempty"`
	HasDueDateOffset bool   `json:"hasDueDateOffset,omitempty"`
	NotifyAssignee   bool   `json:"notifyAssignee,omitempty"`
	Priority         string `json:"priority,omitempty"`
	Status           string `json:"status,omitempty"`
	Subject          string `json:"subject,omitempty"`
	Protected        bool   `json:"protected,omitempty"`
}

type WorkflowRule struct {
	Name         string                 `json:"name"`
	Active       bool                   `json:"active,omitempty"`
	Formula      string                 `json:"formula,omitempty"`
	Criteria     []WorkflowCriteriaItem `json:"criteria,omitempty"`
	FieldUpdates []WorkflowFieldUpdate  `json:"fieldUpdates,omitempty"`
	EmailAlerts  []WorkflowEmailAlert   `json:"emailAlerts,omitempty"`
	Tasks        []WorkflowTask         `json:"tasks,omitempty"`
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
	TriggerOrder  int                    `json:"triggerOrder,omitempty"`
	RunInMode     string                 `json:"runInMode,omitempty"`
	Formula       string                 `json:"formula,omitempty"`
	Criteria      []WorkflowCriteriaItem `json:"criteria,omitempty"`
	Branches      []FlowBranch           `json:"branches,omitempty"`
	Steps         []FlowStep             `json:"steps,omitempty"`
	FieldUpdates  []WorkflowFieldUpdate  `json:"fieldUpdates,omitempty"`
	Actions       []FlowAction           `json:"actions,omitempty"`
	RecordLookups []FlowRecordLookup     `json:"recordLookups,omitempty"`
	RecordCreates []FlowRecordCreate     `json:"recordCreates,omitempty"`
	RecordDeletes []FlowRecordDelete     `json:"recordDeletes,omitempty"`
	CustomErrors  []FlowCustomError      `json:"customErrors,omitempty"`
	ApexPlugins   []FlowApexPluginCall   `json:"apexPlugins,omitempty"`
}

type FlowBranch struct {
	Name          string                 `json:"name"`
	Default       bool                   `json:"default,omitempty"`
	Formula       string                 `json:"formula,omitempty"`
	Criteria      []WorkflowCriteriaItem `json:"criteria,omitempty"`
	Steps         []FlowStep             `json:"steps,omitempty"`
	FieldUpdates  []WorkflowFieldUpdate  `json:"fieldUpdates,omitempty"`
	Actions       []FlowAction           `json:"actions,omitempty"`
	RecordLookups []FlowRecordLookup     `json:"recordLookups,omitempty"`
	RecordCreates []FlowRecordCreate     `json:"recordCreates,omitempty"`
	RecordDeletes []FlowRecordDelete     `json:"recordDeletes,omitempty"`
}

type FlowStep struct {
	Kind                string                  `json:"kind"`
	FaultTarget         string                  `json:"faultTarget,omitempty"`
	FaultBranch         []FlowStep              `json:"faultBranch,omitempty"`
	FieldUpdates        []WorkflowFieldUpdate   `json:"fieldUpdates,omitempty"`
	Action              FlowAction              `json:"action,omitempty"`
	RecordLookup        FlowRecordLookup        `json:"recordLookup,omitempty"`
	RecordCreate        FlowRecordCreate        `json:"recordCreate,omitempty"`
	RecordUpdate        FlowRecordUpdate        `json:"recordUpdate,omitempty"`
	RecordDelete        FlowRecordDelete        `json:"recordDelete,omitempty"`
	Subflow             FlowSubflow             `json:"subflow,omitempty"`
	Assignment          FlowAssignment          `json:"assignment,omitempty"`
	Transform           FlowTransform           `json:"transform,omitempty"`
	CollectionProcessor FlowCollectionProcessor `json:"collectionProcessor,omitempty"`
	CustomError         FlowCustomError         `json:"customError,omitempty"`
	Loop                FlowLoop                `json:"loop,omitempty"`
	Branches            []FlowBranch            `json:"branches,omitempty"`
}

type FlowAssignment struct {
	Name         string `json:"name"`
	Target       string `json:"target"`
	Operator     string `json:"operator,omitempty"`
	LiteralValue string `json:"literalValue,omitempty"`
	SourceField  string `json:"sourceField,omitempty"`
}

type FlowLoop struct {
	Name                 string     `json:"name"`
	CollectionReference  string     `json:"collectionReference"`
	CurrentItemReference string     `json:"currentItemReference,omitempty"`
	Steps                []FlowStep `json:"steps,omitempty"`
}

type FlowAction struct {
	Name       string                `json:"name"`
	ActionType string                `json:"actionType,omitempty"`
	ActionName string                `json:"actionName,omitempty"`
	ClassName  string                `json:"className,omitempty"`
	MethodName string                `json:"methodName,omitempty"`
	Inputs     []WorkflowFieldUpdate `json:"inputs,omitempty"`
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
	InputReference           string                `json:"inputReference,omitempty"`
	InputAssignments         []WorkflowFieldUpdate `json:"inputAssignments,omitempty"`
	StoreOutputAutomatically bool                  `json:"storeOutputAutomatically,omitempty"`
}

type FlowRecordUpdate struct {
	Name             string                 `json:"name"`
	ObjectName       string                 `json:"objectName,omitempty"`
	InputReference   string                 `json:"inputReference,omitempty"`
	Criteria         []WorkflowCriteriaItem `json:"criteria,omitempty"`
	InputAssignments []WorkflowFieldUpdate  `json:"inputAssignments,omitempty"`
}

type FlowRecordDelete struct {
	Name           string                 `json:"name"`
	ObjectName     string                 `json:"objectName"`
	InputReference string                 `json:"inputReference,omitempty"`
	Criteria       []WorkflowCriteriaItem `json:"criteria,omitempty"`
}

type FlowSubflow struct {
	Name              string                `json:"name"`
	FlowName          string                `json:"flowName"`
	InputAssignments  []WorkflowFieldUpdate `json:"inputAssignments,omitempty"`
	OutputAssignments []FlowAssignment      `json:"outputAssignments,omitempty"`
}

type FlowCustomError struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description,omitempty"`
	Messages    []FlowCustomErrorMessage `json:"messages,omitempty"`
}

type FlowCustomErrorMessage struct {
	Message string `json:"message"`
}

type FlowApexPluginCall struct {
	Name   string                `json:"name"`
	Class  string                `json:"class"`
	Inputs []WorkflowFieldUpdate `json:"inputs,omitempty"`
}

type FlowTransform struct {
	Name             string                `json:"name"`
	TransformType    string                `json:"transformType"`
	SourceCollection string                `json:"sourceCollection"`
	TargetCollection string                `json:"targetCollection"`
	FieldMappings    []WorkflowFieldUpdate `json:"fieldMappings,omitempty"`
	SumField         string                `json:"sumField,omitempty"`
}

type FlowCollectionProcessor struct {
	Name                string                 `json:"name"`
	ProcessorType       string                 `json:"processorType"`
	CollectionReference string                 `json:"collectionReference"`
	TargetCollection    string                 `json:"targetCollection,omitempty"`
	SortField           string                 `json:"sortField,omitempty"`
	SortOrder           string                 `json:"sortOrder,omitempty"`
	Criteria            []WorkflowCriteriaItem `json:"criteria,omitempty"`
	FieldMappings       []WorkflowFieldUpdate  `json:"fieldMappings,omitempty"`
}

type FieldType string

const (
	FieldAny           FieldType = "ANY"
	FieldID            FieldType = "ID"
	FieldString        FieldType = "STRING"
	FieldBoolean       FieldType = "BOOLEAN"
	FieldInteger       FieldType = "INTEGER"
	FieldDecimal       FieldType = "DECIMAL"
	FieldDate          FieldType = "DATE"
	FieldDateTime      FieldType = "DATETIME"
	FieldPicklist      FieldType = "PICKLIST"
	FieldMultiPicklist FieldType = "MULTIPICKLIST"
	FieldReference     FieldType = "REFERENCE"
	FieldBlob          FieldType = "BLOB"
	FieldAddress       FieldType = "ADDRESS"
	FieldLocation      FieldType = "LOCATION"
	FieldCalculated    FieldType = "CALCULATED"
	FieldSummary       FieldType = "SUMMARY"
)

type Relationship struct {
	Field               string   `json:"field"`
	ParentObjects       []string `json:"parentObjects"`
	ParentRelationship  string   `json:"parentRelationship,omitempty"`
	ChildRelationship   string   `json:"childRelationship,omitempty"`
	CascadeDelete       bool     `json:"cascadeDelete,omitempty"`
	RestrictedDelete    bool     `json:"restrictedDelete,omitempty"`
	DeprecatedAndHidden bool     `json:"deprecatedAndHidden,omitempty"`
	JunctionIDListNames []string `json:"junctionIdListNames,omitempty"`
	JunctionReferenceTo []string `json:"junctionReferenceTo,omitempty"`
	Polymorphic         bool     `json:"polymorphic,omitempty"`
	DeferredIntegrity   bool     `json:"deferredIntegrity,omitempty"`
}

type Record struct {
	ID                  ID                  `json:"id"`
	Object              string              `json:"object"`
	Fields              map[string]Value    `json:"fields,omitempty"`
	Children            map[string][]Record `json:"children,omitempty"`
	ParentRelationships map[string]Record   `json:"-"`
	ExplicitNulls       map[string]bool     `json:"explicitNulls,omitempty"`
	System              SystemFields        `json:"system,omitempty"`
}

func (r Record) GetField(name string) (Value, bool) {
	if r.Fields == nil {
		return Value{}, false
	}
	if v, ok := r.Fields[name]; ok {
		return v, true
	}
	for k, v := range r.Fields {
		if strings.EqualFold(k, name) {
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
	for k, v := range r.ExplicitNulls {
		if v && strings.EqualFold(k, name) {
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
	HiddenFromSOQL   bool   `json:"-"`
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
		if field.Type == FieldMultiPicklist {
			first := ""
			var joined strings.Builder
			for _, value := range field.PicklistValues {
				if !value.Default || strings.TrimSpace(value.Value) == "" {
					continue
				}
				if first == "" {
					first = value.Value
					continue
				}
				if joined.Len() == 0 {
					joined.Grow(len(first) + 1 + len(value.Value))
					joined.WriteString(first)
				}
				joined.WriteByte(';')
				joined.WriteString(value.Value)
			}
			if joined.Len() != 0 {
				return StringValue(joined.String()), true
			}
			if first != "" {
				return StringValue(first), true
			}
		}
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
	case FieldReference:
		return IDValue(ID(normalizeStringDefaultValue(raw))), true
	case FieldString, FieldPicklist, FieldMultiPicklist, FieldDate, FieldDateTime, FieldID, FieldAny:
		return StringValue(normalizeStringDefaultValue(raw)), true
	}
	return Value{}, false
}

func DefaultValueForRecordField(definition ObjectDefinition, record Record, field Field) (Value, bool) {
	raw := strings.TrimSpace(field.DefaultValue)
	formulaCallDefault := defaultValueLooksLikeFormulaCall(raw)
	if field.Type == FieldPicklist || field.Type == FieldMultiPicklist {
		if value, ok := defaultPicklistValueForRecordType(definition, record, field); ok {
			return value, true
		}
		if !formulaCallDefault {
			if value, ok := DefaultValueForField(field); ok {
				return value, true
			}
		}
	}
	if raw == "" {
		return Value{}, false
	}
	if raw == "$RecordType.Name" || raw == "$RecordType.DeveloperName" {
		value := defaultConditionValue(definition, record, raw)
		if value != "" {
			return defaultValueFromRaw(field, value)
		}
	}
	if !formulaCallDefault {
		if value, ok := DefaultValueForField(field); ok {
			return value, true
		}
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

func defaultValueLooksLikeFormulaCall(raw string) bool {
	raw = strings.TrimSpace(raw)
	open := strings.IndexByte(raw, '(')
	if open <= 0 {
		return false
	}
	for i := 0; i < open; i++ {
		c := raw[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

func defaultPicklistValueForRecordType(definition ObjectDefinition, record Record, field Field) (Value, bool) {
	if len(definition.RecordTypes) == 0 {
		return Value{}, false
	}
	recordTypeID := ""
	if value, ok := record.GetField("RecordTypeId"); ok {
		recordTypeID = strings.TrimSpace(defaultConditionScalar(value))
	}
	if recordTypeID == "" {
		return Value{}, false
	}
	for _, recordType := range definition.RecordTypes {
		if string(recordType.ID) != recordTypeID {
			continue
		}
		for name, value := range recordType.PicklistDefaults {
			if strings.EqualFold(name, field.APIName) && strings.TrimSpace(value) != "" {
				return StringValue(value), true
			}
		}
		return Value{}, false
	}
	return Value{}, false
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
	case FieldString, FieldPicklist, FieldMultiPicklist, FieldDate, FieldDateTime, FieldID, FieldAny:
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
		Objects:         make(map[string]ObjectState),
		IDSequences:     make(map[string]uint64),
		objectNameCache: &sync.Map{},
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
	cacheKey := strings.ToLower(strings.TrimSpace(org.Namespace)) + "|" + strconv.Itoa(len(org.Objects)) + "|" + strings.ToLower(strings.TrimSpace(name))
	if org.objectNameCache != nil {
		if cached, ok := org.objectNameCache.Load(cacheKey); ok {
			if resolved, resolvedOK := cached.(string); resolvedOK && resolved != "" {
				return resolved, true
			}
		}
	}
	if org.Namespace != "" && !hasNamespaceToken(name) && isCustomAPIName(name) {
		prefixed := NamespaceTokenName(org.Namespace, name)
		if prefixed != name {
			if _, ok := org.Objects[prefixed]; ok {
				cacheResolvedObjectName(org, cacheKey, prefixed)
				return prefixed, true
			}
		}
	}
	if exact, ok := org.Objects[name]; ok {
		if preferred, preferredOK := richerNamespacedObjectMatch(org, name, exact); preferredOK {
			cacheResolvedObjectName(org, cacheKey, preferred)
			return preferred, true
		}
		cacheResolvedObjectName(org, cacheKey, name)
		return name, true
	}
	prefixed := NamespaceTokenName(org.Namespace, name)
	if prefixed != name {
		if _, ok := org.Objects[prefixed]; ok {
			cacheResolvedObjectName(org, cacheKey, prefixed)
			return prefixed, true
		}
	}
	stripped := StripNamespaceToken(org.Namespace, name)
	if stripped != name {
		if _, ok := org.Objects[stripped]; ok {
			cacheResolvedObjectName(org, cacheKey, stripped)
			return stripped, true
		}
	}
	for candidate := range org.Objects {
		if strings.EqualFold(candidate, name) || strings.EqualFold(candidate, prefixed) || strings.EqualFold(candidate, stripped) {
			cacheResolvedObjectName(org, cacheKey, candidate)
			return candidate, true
		}
	}
	if hasNamespaceToken(name) {
		unqualified := StripAnyNamespaceToken(name)
		if unqualified != name {
			if _, ok := org.Objects[unqualified]; ok {
				cacheResolvedObjectName(org, cacheKey, unqualified)
				return unqualified, true
			}
			for candidate := range org.Objects {
				if strings.EqualFold(candidate, unqualified) {
					cacheResolvedObjectName(org, cacheKey, candidate)
					return candidate, true
				}
			}
		}
	}
	if !hasNamespaceToken(name) && isCustomAPIName(name) {
		var match string
		for candidate := range org.Objects {
			if strings.EqualFold(StripAnyNamespaceToken(candidate), name) {
				if match != "" {
					return "", false
				}
				match = candidate
			}
		}
		if match != "" {
			cacheResolvedObjectName(org, cacheKey, match)
			return match, true
		}
	}
	return "", false
}

func cacheResolvedObjectName(org OrgState, key, name string) {
	if org.objectNameCache == nil || key == "" || name == "" {
		return
	}
	org.objectNameCache.Store(key, name)
}

func richerNamespacedObjectMatch(org OrgState, name string, exact ObjectState) (string, bool) {
	if hasNamespaceToken(name) || !isCustomAPIName(name) {
		return "", false
	}
	if prefixed := NamespaceTokenName(org.Namespace, name); prefixed != name {
		if state, ok := org.Objects[prefixed]; ok && objectDefinitionRicher(state.Definition, exact.Definition) {
			return prefixed, true
		}
	}
	var match string
	var matched ObjectState
	for candidate, state := range org.Objects {
		if strings.EqualFold(candidate, name) || !strings.EqualFold(StripAnyNamespaceToken(candidate), name) {
			continue
		}
		if match != "" {
			return "", false
		}
		match = candidate
		matched = state
	}
	if match == "" {
		return "", false
	}
	if objectDefinitionRicher(matched.Definition, exact.Definition) {
		return match, true
	}
	return "", false
}

func objectDefinitionRicher(candidate, exact ObjectDefinition) bool {
	return len(candidate.Fields) > len(exact.Fields) || len(candidate.Relations) > len(exact.Relations) || len(candidate.RecordTypes) > len(exact.RecordTypes)
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
	if hasNamespaceToken(name) {
		unqualified := StripAnyNamespaceToken(name)
		if unqualified != name {
			if _, ok := definition.Fields[unqualified]; ok {
				return unqualified, true
			}
			for candidate := range definition.Fields {
				if strings.EqualFold(candidate, unqualified) {
					return candidate, true
				}
			}
		}
	}
	if !hasNamespaceToken(name) && isCustomAPIName(name) {
		var match string
		for candidate := range definition.Fields {
			if strings.EqualFold(StripAnyNamespaceToken(candidate), name) {
				if match != "" {
					return "", false
				}
				match = candidate
			}
		}
		if match != "" {
			return match, true
		}
	}
	return "", false
}

func resolveLocationComponentField(definition ObjectDefinition, namespace, name string) (string, bool) {
	for _, suffix := range []string{"__Latitude__s", "__Longitude__s"} {
		if !hasAPISuffix(name, suffix) {
			continue
		}
		base := name[:len(name)-len(suffix)] + "__c"
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
	if !hasAPISuffix(name, suffix) {
		return name
	}
	base := name[:len(name)-len(suffix)]
	if idx := strings.Index(base, "__"); idx > 0 && idx+2 < len(base) {
		base = base[idx+2:]
	}
	return base + suffix
}

type namespaceTokenNameCacheKey struct {
	namespace string
	name      string
}

const namespaceTokenNameCacheLimit = 8192

var namespaceTokenNameCache = struct {
	sync.RWMutex
	names map[namespaceTokenNameCacheKey]string
}{
	names: make(map[namespaceTokenNameCacheKey]string),
}

func NamespaceTokenName(namespace, name string) string {
	if namespace == "" || name == "" || !isCustomAPIName(name) || hasNamespaceToken(name) {
		return name
	}
	key := namespaceTokenNameCacheKey{namespace: namespace, name: name}
	namespaceTokenNameCache.RLock()
	if token, ok := namespaceTokenNameCache.names[key]; ok {
		namespaceTokenNameCache.RUnlock()
		return token
	}
	namespaceTokenNameCache.RUnlock()

	token := namespace + "__" + name
	namespaceTokenNameCache.Lock()
	if cached, ok := namespaceTokenNameCache.names[key]; ok {
		namespaceTokenNameCache.Unlock()
		return cached
	}
	if len(namespaceTokenNameCache.names) < namespaceTokenNameCacheLimit {
		namespaceTokenNameCache.names[key] = token
	}
	namespaceTokenNameCache.Unlock()
	return token
}

func StripAnyNamespaceToken(name string) string {
	if !isCustomAPIName(name) {
		return name
	}
	idx := strings.Index(name, "__")
	suffix := strings.LastIndex(name, "__")
	if idx <= 0 || idx >= suffix {
		return name
	}
	return name[idx+2:]
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
	return definition.Metadata != nil &&
		(definition.Metadata["kind"] == "customSetting" || strings.TrimSpace(definition.Metadata["customSettingsType"]) != "")
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
	if r.ParentRelationships != nil {
		out.ParentRelationships = make(map[string]Record, len(r.ParentRelationships))
		for name, record := range r.ParentRelationships {
			out.ParentRelationships[name] = record.Clone()
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

func ValuesEqual(left, right Value) bool {
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case ValueNull:
		return true
	case ValueString, ValueDate, ValueDateTime, ValueBlob:
		return left.String == right.String
	case ValueInteger:
		return left.Integer == right.Integer
	case ValueBoolean:
		return left.Boolean == right.Boolean
	case ValueDecimal:
		return left.Decimal == right.Decimal
	case ValueID:
		return IDsEqual(left.ID, right.ID)
	case ValueList:
		if len(left.List) != len(right.List) {
			return false
		}
		for i := range left.List {
			if !ValuesEqual(left.List[i], right.List[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
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
			field.PicklistValueSettings = clonePicklistSettings(field.PicklistValueSettings)
			out.Fields[name] = field
		}
	}
	out.Relations = append([]Relationship(nil), d.Relations...)
	for i := range out.Relations {
		out.Relations[i].ParentObjects = append([]string(nil), d.Relations[i].ParentObjects...)
		out.Relations[i].JunctionIDListNames = append([]string(nil), d.Relations[i].JunctionIDListNames...)
		out.Relations[i].JunctionReferenceTo = append([]string(nil), d.Relations[i].JunctionReferenceTo...)
	}
	out.RecordTypes = append([]RecordTypeInfo(nil), d.RecordTypes...)
	for i := range out.RecordTypes {
		if d.RecordTypes[i].PicklistDefaults == nil {
			continue
		}
		out.RecordTypes[i].PicklistDefaults = make(map[string]string, len(d.RecordTypes[i].PicklistDefaults))
		for field, value := range d.RecordTypes[i].PicklistDefaults {
			out.RecordTypes[i].PicklistDefaults[field] = value
		}
	}
	out.ValidationRules = append([]ValidationRule(nil), d.ValidationRules...)
	out.WorkflowRules = append([]WorkflowRule(nil), d.WorkflowRules...)
	for i := range out.WorkflowRules {
		out.WorkflowRules[i].Criteria = append([]WorkflowCriteriaItem(nil), d.WorkflowRules[i].Criteria...)
		out.WorkflowRules[i].FieldUpdates = append([]WorkflowFieldUpdate(nil), d.WorkflowRules[i].FieldUpdates...)
		out.WorkflowRules[i].EmailAlerts = append([]WorkflowEmailAlert(nil), d.WorkflowRules[i].EmailAlerts...)
		out.WorkflowRules[i].Tasks = append([]WorkflowTask(nil), d.WorkflowRules[i].Tasks...)
		for j := range out.WorkflowRules[i].EmailAlerts {
			out.WorkflowRules[i].EmailAlerts[j].Recipients = append([]WorkflowEmailRecipient(nil), d.WorkflowRules[i].EmailAlerts[j].Recipients...)
		}
	}
	out.FlowRules = append([]FlowRule(nil), d.FlowRules...)
	for i := range out.FlowRules {
		out.FlowRules[i].Criteria = append([]WorkflowCriteriaItem(nil), d.FlowRules[i].Criteria...)
		out.FlowRules[i].Steps = cloneFlowSteps(d.FlowRules[i].Steps)
		out.FlowRules[i].FieldUpdates = append([]WorkflowFieldUpdate(nil), d.FlowRules[i].FieldUpdates...)
		out.FlowRules[i].Actions = cloneFlowActions(d.FlowRules[i].Actions)
		out.FlowRules[i].Branches = cloneFlowBranches(d.FlowRules[i].Branches)
		out.FlowRules[i].RecordLookups = cloneFlowRecordLookups(d.FlowRules[i].RecordLookups)
		out.FlowRules[i].RecordCreates = cloneFlowRecordCreates(d.FlowRules[i].RecordCreates)
		out.FlowRules[i].RecordDeletes = cloneFlowRecordDeletes(d.FlowRules[i].RecordDeletes)
		out.FlowRules[i].CustomErrors = cloneFlowCustomErrors(d.FlowRules[i].CustomErrors)
		out.FlowRules[i].ApexPlugins = cloneFlowApexPlugins(d.FlowRules[i].ApexPlugins)
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

func cloneFlowSteps(steps []FlowStep) []FlowStep {
	out := append([]FlowStep(nil), steps...)
	for i := range out {
		out[i].FieldUpdates = append([]WorkflowFieldUpdate(nil), steps[i].FieldUpdates...)
		out[i].Action.Inputs = append([]WorkflowFieldUpdate(nil), steps[i].Action.Inputs...)
		out[i].RecordLookup.Criteria = append([]WorkflowCriteriaItem(nil), steps[i].RecordLookup.Criteria...)
		out[i].RecordCreate.InputAssignments = append([]WorkflowFieldUpdate(nil), steps[i].RecordCreate.InputAssignments...)
		out[i].RecordUpdate.Criteria = append([]WorkflowCriteriaItem(nil), steps[i].RecordUpdate.Criteria...)
		out[i].RecordUpdate.InputAssignments = append([]WorkflowFieldUpdate(nil), steps[i].RecordUpdate.InputAssignments...)
		out[i].RecordDelete.Criteria = append([]WorkflowCriteriaItem(nil), steps[i].RecordDelete.Criteria...)
		out[i].Subflow.InputAssignments = append([]WorkflowFieldUpdate(nil), steps[i].Subflow.InputAssignments...)
		out[i].Subflow.OutputAssignments = append([]FlowAssignment(nil), steps[i].Subflow.OutputAssignments...)
		out[i].Transform.FieldMappings = append([]WorkflowFieldUpdate(nil), steps[i].Transform.FieldMappings...)
		out[i].CollectionProcessor.Criteria = append([]WorkflowCriteriaItem(nil), steps[i].CollectionProcessor.Criteria...)
		out[i].CollectionProcessor.FieldMappings = append([]WorkflowFieldUpdate(nil), steps[i].CollectionProcessor.FieldMappings...)
		out[i].CustomError.Messages = append([]FlowCustomErrorMessage(nil), steps[i].CustomError.Messages...)
		out[i].FaultBranch = cloneFlowSteps(steps[i].FaultBranch)
		out[i].Loop.Steps = cloneFlowSteps(steps[i].Loop.Steps)
		out[i].Branches = cloneFlowBranches(steps[i].Branches)
	}
	return out
}

func cloneFlowBranches(branches []FlowBranch) []FlowBranch {
	out := append([]FlowBranch(nil), branches...)
	for i := range out {
		out[i].Criteria = append([]WorkflowCriteriaItem(nil), branches[i].Criteria...)
		out[i].Steps = cloneFlowSteps(branches[i].Steps)
		out[i].FieldUpdates = append([]WorkflowFieldUpdate(nil), branches[i].FieldUpdates...)
		out[i].Actions = cloneFlowActions(branches[i].Actions)
		out[i].RecordLookups = cloneFlowRecordLookups(branches[i].RecordLookups)
		out[i].RecordCreates = cloneFlowRecordCreates(branches[i].RecordCreates)
		out[i].RecordDeletes = cloneFlowRecordDeletes(branches[i].RecordDeletes)
	}
	return out
}

func cloneFlowActions(actions []FlowAction) []FlowAction {
	out := append([]FlowAction(nil), actions...)
	for i := range out {
		out[i].Inputs = append([]WorkflowFieldUpdate(nil), actions[i].Inputs...)
	}
	return out
}

func cloneFlowRecordLookups(lookups []FlowRecordLookup) []FlowRecordLookup {
	out := append([]FlowRecordLookup(nil), lookups...)
	for i := range out {
		out[i].Criteria = append([]WorkflowCriteriaItem(nil), lookups[i].Criteria...)
	}
	return out
}

func cloneFlowRecordCreates(creates []FlowRecordCreate) []FlowRecordCreate {
	out := append([]FlowRecordCreate(nil), creates...)
	for i := range out {
		out[i].InputAssignments = append([]WorkflowFieldUpdate(nil), creates[i].InputAssignments...)
	}
	return out
}

func cloneFlowRecordDeletes(deletes []FlowRecordDelete) []FlowRecordDelete {
	out := append([]FlowRecordDelete(nil), deletes...)
	for i := range out {
		out[i].Criteria = append([]WorkflowCriteriaItem(nil), deletes[i].Criteria...)
	}
	return out
}

func cloneFlowCustomErrors(errors []FlowCustomError) []FlowCustomError {
	out := append([]FlowCustomError(nil), errors...)
	for i := range out {
		out[i].Messages = append([]FlowCustomErrorMessage(nil), errors[i].Messages...)
	}
	return out
}

func cloneFlowApexPlugins(plugins []FlowApexPluginCall) []FlowApexPluginCall {
	out := append([]FlowApexPluginCall(nil), plugins...)
	for i := range out {
		out[i].Inputs = append([]WorkflowFieldUpdate(nil), plugins[i].Inputs...)
	}
	return out
}

func clonePicklistSettings(values []PicklistSetting) []PicklistSetting {
	out := append([]PicklistSetting(nil), values...)
	for i := range out {
		out[i].ControllingFieldValues = append([]string(nil), values[i].ControllingFieldValues...)
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
	out.objectNameCache = &sync.Map{}
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
// Records, indexes, definitions, sequences, and transaction frames are isolated
// so parallel test setup and execution can infer runtime metadata without
// sharing mutable maps.
func (o OrgState) CloneRuntime() OrgState {
	cloneStats.cloneRuntime.Add(1)
	out := o
	out.objectNameCache = &sync.Map{}
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

// CloneRuntimeFrozenDefinition returns an isolated org copy that SHARES object
// Definitions (immutable schema metadata) by reference instead of deep-cloning
// them. Records and Indexes (mutable data) are still cloned for isolation. All
// runtime definition mutations go through EnsureMutableObjectDefinition or an
// explicit Definition.Clone() before write, so sharing is safe and copy-on-write
// preserves per-clone isolation. This mirrors RuntimeTemplate.CloneRuntimeOrg.
func (o OrgState) CloneRuntimeFrozenDefinition() OrgState {
	cloneStats.cloneRuntime.Add(1)
	out := o
	out.objectNameCache = &sync.Map{}
	if o.Objects != nil {
		out.Objects = make(map[string]ObjectState, len(o.Objects))
		for name, object := range o.Objects {
			out.Objects[name] = object.CloneRuntimeFrozenDefinition()
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

// IsImmutableMetadataObject reports whether an object's records are compiled or
// setup metadata that no Apex test execution path mutates (mirroring how
// Salesforce keeps compiled metadata and describe-style setup data static across
// a test run). Records for these objects can be shared by reference across
// per-test org clones instead of deep-cloned, which is where most of the
// base-org record volume lives (FieldPermissions, ApexClass, Profile, etc.)
// under SeeAllData=false where business objects start empty. Custom metadata
// records stay cloned because package tests can use them as runtime fixtures.
//
// Mutable runtime/business objects (User, AsyncApexJob, PermissionSetAssignment,
// standard and custom data objects) are intentionally excluded so they continue
// to be deep-cloned for full per-test isolation, including read-path field
// mutation. DML to a shared metadata object still copy-on-writes through
// EnsureMutableObjectRecords, so insert/update of these types stays isolated.
func IsImmutableMetadataObject(objectName string) bool {
	name := strings.TrimSpace(objectName)
	if name == "" {
		return false
	}
	switch strings.ToLower(name) {
	case "apexclass", "apextrigger", "apexpage", "apexcomponent",
		"fieldpermissions", "objectpermissions", "setupentityaccess",
		"permissionset", "permissionsetgroup", "permissionsetgroupcomponent",
		"profile", "userrole",
		"recordtype", "layout", "staticresource",
		"customapplication", "apptabmember", "tabdefinition",
		"entitydefinition", "fielddefinition":
		return true
	default:
		return false
	}
}

// CloneRuntimeFrozenShared returns an isolated org copy that SHARES object
// Definitions (immutable schema metadata) by reference, and additionally shares
// Records/Indexes by reference for immutable metadata/setup objects (see
// IsImmutableMetadataObject). Mutable business and runtime objects keep their
// records deep-cloned for full per-test isolation. This mirrors how Salesforce
// keeps compiled metadata and existing setup data static across a test run while
// isolating mutable data per test.
//
// Sharing metadata records is safe because they are read-only on every test
// execution path; the rare DML against a shared metadata object copy-on-writes
// through EnsureMutableObjectRecords before mutating, leaving the shared base
// maps untouched. IDSequences and Transactions are cloned so each test gets
// private ID counters and DML scopes. The source org is never mutated, so
// parallel clones are race-safe.
func (o OrgState) CloneRuntimeFrozenShared() OrgState {
	cloneStats.cloneRuntime.Add(1)
	out := o
	out.objectNameCache = &sync.Map{}
	if o.Objects != nil {
		out.Objects = make(map[string]ObjectState, len(o.Objects))
		for name, object := range o.Objects {
			if IsImmutableMetadataObject(object.Definition.APIName) || IsImmutableMetadataObject(name) {
				out.Objects[name] = object.CloneRuntimeSnapshot()
			} else {
				out.Objects[name] = object.CloneRuntimeFrozenDefinition()
			}
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

// CloneRollbackSnapshot returns an isolated org copy for DML rollback scopes.
// It mirrors CloneRuntime for correctness.
func (o OrgState) CloneRollbackSnapshot() OrgState {
	cloneStats.cloneRollbackSnapshot.Add(1)
	return o.CloneRuntime()
}

func (o ObjectState) CloneRuntime() ObjectState {
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

func isRuntimeMutableDefinition(definition ObjectDefinition) bool {
	if hasRuntimeMutableMetadata(definition.Metadata) {
		return true
	}
	return isCustomAPIName(definition.APIName)
}

func hasRuntimeMutableMetadata(metadata map[string]string) bool {
	if len(metadata) == 0 {
		return false
	}
	for key := range metadata {
		if strings.HasPrefix(key, "__glade_") {
			continue
		}
		return true
	}
	return false
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
