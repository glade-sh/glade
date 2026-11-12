package storage

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glade-sh/glade/internal/config"
)

const FixtureVersion = "glade.storage.v1"

// NormalizeRESTAPIVersion trims whitespace and strips an optional leading "v".
func NormalizeRESTAPIVersion(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimPrefix(s, "v")
}

// ResolveRESTAPIVersion returns a canonical supported REST API version.
func ResolveRESTAPIVersion(raw string) (string, error) {
	requested := strings.TrimSpace(raw)
	if requested == "" {
		return DefaultRESTAPIVersion, nil
	}
	parts := strings.Split(NormalizeRESTAPIVersion(requested), ".")
	if len(parts) != 2 {
		return "", unsupportedRESTAPIVersion(requested)
	}
	major, majorOK := unsignedRESTVersionComponent(parts[0])
	minor, minorOK := unsignedRESTVersionComponent(parts[1])
	if !majorOK || !minorOK || major < 1 || minor != 0 {
		return "", unsupportedRESTAPIVersion(requested)
	}
	version := fmt.Sprintf("%d.0", major)
	if !slices.Contains(SupportedRESTAPIVersions, version) {
		return "", unsupportedRESTAPIVersion(requested)
	}
	return version, nil
}

func IsSupportedRESTAPIVersion(raw string) bool {
	version, err := ResolveRESTAPIVersion(raw)
	return err == nil && slices.Contains(SupportedRESTAPIVersions, version)
}

func unsignedRESTVersionComponent(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return 0, false
		}
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil
}

func unsupportedRESTAPIVersion(requested string) error {
	return fmt.Errorf("unsupported REST API version %q; supported versions: %s", requested, strings.Join(SupportedRESTAPIVersions, ", "))
}

// NormalizeOrgDomainURL shares canonical-origin validation with config. Unset
// stays unset so callers can apply a seed before the VM's local default.
func NormalizeOrgDomainURL(raw string) (string, error) {
	return config.NormalizeOrgDomainURL(raw)
}

type OrgState struct {
	ChangeDataCaptureTestSetup *ChangeDataCaptureSnapshot `json:"-"`
	// Formula execution context is set on a per-query OrgState value copy.
	// It is neither persisted nor stored in query or schema caches.
	formulaUserID           ID
	formulaPermissionFields map[string]Value

	OrgID      string                 `json:"orgId,omitempty"`
	APIVersion string                 `json:"apiVersion,omitempty"`
	Namespace  string                 `json:"namespace,omitempty"`
	DomainURL  string                 `json:"domainUrl,omitempty"`
	Features   []string               `json:"features,omitzero"`
	Metadata   MetadataRegistry       `json:"metadata,omitempty"`
	Objects    map[string]ObjectState `json:"objects"`
	// StandardSpread marks an org whose standard catalog objects are installed
	// on first reference from a shared per-shape source (see
	// standard_spread_source.go). "plain" is before the org shape applied,
	// "shaped" after. Empty means the org holds every object it will hold.
	StandardSpread         string              `json:"standardSpread,omitempty"`
	IDSequences            map[string]uint64   `json:"idSequences,omitempty"`
	Transactions           []TransactionFrame  `json:"transactions,omitempty"`
	Now                    func() time.Time    `json:"-"`
	Today                  func() time.Time    `json:"-"` // Execution user's local calendar for formula TODAY().
	PlatformCache          *PlatformCacheState `json:"-"`
	PlatformCacheTestSetup *PlatformCacheState `json:"-"`

	SystemTimestampBase     string `json:"-"`
	SystemTimestampSequence int64  `json:"-"`
	RuntimeSchemaStamp      string `json:"-"`

	objectNameCache                 *sync.Map
	keyPrefixesValidated            bool
	lazyStandardSpread              bool
	spreadPrefixSeed                map[string]string
	keyPrefixesValidatedObjectCount int
	keyPrefixesValidatedPrefixes    []keyPrefixSnapshotEntry
	keyPrefixesValidatedGeneration  uint64
	schemaGeneration                uint64
	runtimeGeneration               *orgRuntimeGeneration

	// spreadPrefixesPlanned records that a source-backed lazy org has had its
	// prefixes validated. Clearing the schema stamp does not clear it: the cold
	// objects still take their prefixes from the unique assignment.
	spreadPrefixesPlanned bool
}

type orgRuntimeGeneration struct {
	id     uint64
	mu     sync.Mutex
	values map[string]any
}

var runtimeGenerationSequence atomic.Uint64

func newOrgRuntimeGeneration() *orgRuntimeGeneration {
	return &orgRuntimeGeneration{id: runtimeGenerationSequence.Add(1)}
}

func (o *OrgState) ensureRuntimeGeneration() *orgRuntimeGeneration {
	if o == nil {
		return nil
	}
	if o.runtimeGeneration == nil {
		o.runtimeGeneration = newOrgRuntimeGeneration()
	}
	return o.runtimeGeneration
}

// RuntimeGenerationID identifies org clones that share the same schema and
// metadata generation. ClearRuntimeSchemaStamp and explicit runtime invalidation
// move the org to a fresh generation.
func (o *OrgState) RuntimeGenerationID() uint64 {
	if generation := o.ensureRuntimeGeneration(); generation != nil {
		generation.mu.Lock()
		defer generation.mu.Unlock()
		return generation.id
	}
	return 0
}

// RuntimeGenerationCacheValue returns the cached value for a frozen runtime
// generation, building it once across all org clones. Unstamped orgs remain
// mutable and build their indexes locally.
func (o *OrgState) RuntimeGenerationCacheValue(key string, build func() any) any {
	if o == nil || strings.TrimSpace(o.RuntimeSchemaStamp) == "" {
		return build()
	}
	generation := o.ensureRuntimeGeneration()
	generation.mu.Lock()
	defer generation.mu.Unlock()
	if value, ok := generation.values[key]; ok {
		return value
	}
	value := build()
	if generation.values == nil {
		generation.values = make(map[string]any)
	}
	generation.values[key] = value
	return value
}

func (o *OrgState) LoadRuntimeGenerationCache(key string) (any, bool) {
	if o == nil || strings.TrimSpace(o.RuntimeSchemaStamp) == "" {
		return nil, false
	}
	generation := o.ensureRuntimeGeneration()
	generation.mu.Lock()
	defer generation.mu.Unlock()
	value, ok := generation.values[key]
	return value, ok
}

func (o *OrgState) StoreRuntimeGenerationCache(key string, value any) {
	if o == nil || strings.TrimSpace(o.RuntimeSchemaStamp) == "" {
		return
	}
	generation := o.ensureRuntimeGeneration()
	generation.mu.Lock()
	if generation.values == nil {
		generation.values = make(map[string]any)
	}
	generation.values[key] = value
	generation.mu.Unlock()
}

// InvalidateRuntimeGeneration detaches this org's derived indexes after an
// external SetOrg boundary that may have installed mutable schema maps.
// The template and sibling clones retain their shared generation and indexes.
func (o *OrgState) InvalidateRuntimeGeneration() {
	if o == nil {
		return
	}
	o.runtimeGeneration = newOrgRuntimeGeneration()
	// Resolved names, folded-name tables and misses are also schema-derived.
	// Detach the complete graph: a same-count replacement or a definition
	// change can alter lookup preference without changing any old cache key.
	if o.objectNameCache != nil {
		o.objectNameCache = newObjectNameCache(o.Objects)
	}
}

// SchemaGeneration counts ClearRuntimeSchemaStamp calls on this org value.
// Every helper that changes definitions in place (EnsureMutableObjectDefinition
// and the metadata paths) clears the stamp, so a cache built for an unstamped
// org stays valid while the org pointer and this generation are unchanged.
func (o *OrgState) SchemaGeneration() uint64 {
	if o == nil {
		return 0
	}
	return o.schemaGeneration
}

func (o *OrgState) ClearRuntimeSchemaStamp() {
	if o != nil {
		o.schemaGeneration++
		o.RuntimeSchemaStamp = ""
		o.keyPrefixesValidated = false
		o.keyPrefixesValidatedObjectCount = 0
		o.keyPrefixesValidatedPrefixes = nil
		o.keyPrefixesValidatedGeneration = 0
		o.InvalidateRuntimeGeneration()
	}
}

type MetadataRegistry struct {
	// ProjectFlowOwnershipVersion marks databases whose project-owned Flow
	// rules were tagged; omitted legacy metadata decodes as zero.
	ProjectFlowOwnershipVersion int `json:"projectFlowOwnershipVersion,omitempty"`
	// ProjectFlowRootID is an optional local filesystem identity for the bound
	// project directory, used to detect replacement before refreshing Flows.
	ProjectFlowRootID string `json:"projectFlowRootId,omitempty"`
	// AccountRecordTypeDefaults keeps the separate business/person defaults
	// declared by each profile. Runtime clones share this immutable metadata.
	AccountRecordTypeDefaults []AccountRecordTypeDefault `json:"accountRecordTypeDefaults,omitempty"`

	// ChangeEventEntities are selected by deployed ChangeEvents channel members.
	ChangeEventEntities []string `json:"changeEventEntities,omitempty"`

	Flows []FlowRule `json:"flows,omitempty"`
	// FlowDeclarations expose metadata without registering executable automation.
	FlowDeclarations       []FlowRule                    `json:"flowDeclarations,omitempty"`
	DataWeaveResources     []DataWeaveResourceMetadata   `json:"dataWeaveResources,omitempty"`
	Labels                 []LabelMetadata               `json:"labels,omitempty"`
	ValueSetTranslations   []StandardValueSetTranslation `json:"standardValueSetTranslations,omitempty"`
	ManagedLabelNamespaces []string                      `json:"managedLabelNamespaces,omitempty"`
	// PackageFieldsNarrow is set when the project declares package
	// dependencies with known namespaces. A namespaced custom field that a
	// standard object lacks is then accepted only for a namespace listed in
	// SourcelessPackageNamespaces (installed, source absent). When unset the
	// project declared nothing and every ns__X__c name stays accepted.
	PackageFieldsNarrow         bool                       `json:"packageFieldsNarrow,omitempty"`
	SourcelessPackageNamespaces []string                   `json:"sourcelessPackageNamespaces,omitempty"`
	Tabs                        []TabMetadata              `json:"tabs,omitempty"`
	Applications                []ApplicationMetadata      `json:"applications,omitempty"`
	DataCategoryGroups          []DataCategoryGroup        `json:"dataCategoryGroups,omitempty"`
	QuickActions                []QuickActionMetadata      `json:"quickActions,omitempty"`
	FieldSets                   []FieldSetMetadata         `json:"fieldSets,omitempty"`
	StaticResources             []StaticResourceMetadata   `json:"staticResources,omitempty"`
	ContentAssets               []ContentAssetMetadata     `json:"contentAssets,omitempty"`
	Endpoints                   []EndpointMetadata         `json:"endpoints,omitempty"`
	EmailTemplates              []EmailTemplateMetadata    `json:"emailTemplates,omitempty"`
	MessageChannels             []MessageChannelMetadata   `json:"messageChannels,omitempty"`
	FeatureParameters           []FeatureParameterMetadata `json:"featureParameters,omitempty"`
	Layouts                     []LayoutMetadata           `json:"layouts,omitempty"`
	ProjectReports              []ProjectReportMetadata    `json:"projectReports,omitempty"`
	ReportFolderShares          []ProjectReportFolderShare `json:"reportFolderShares,omitempty"`
}

// ProjectReportMetadata is the immutable project definition associated with a
// materialized Report row. Old org snapshots omit this optional field.
type ProjectReportFolderShare struct {
	FolderID     ID     `json:"folderId"`
	AccessLevel  string `json:"accessLevel"`
	SharedTo     string `json:"sharedTo"`
	SharedToType string `json:"sharedToType"`
}

type ProjectReportMetadata struct {
	FolderID       ID       `json:"folderId,omitempty"`
	ID             ID       `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	DeveloperName  string   `json:"developerName"`
	Format         string   `json:"format"`
	ReportType     string   `json:"reportType"`
	Scope          string   `json:"scope,omitempty"`
	ShowDetails    bool     `json:"showDetails,omitempty"`
	HideGrandTotal bool     `json:"hideGrandTotal,omitempty"`
	HideSubTotals  bool     `json:"hideSubTotals,omitempty"`
	DateColumn     string   `json:"dateColumn,omitempty"`
	StartDate      string   `json:"startDate,omitempty"`
	EndDate        string   `json:"endDate,omitempty"`
	Columns        []string `json:"columns,omitempty"`
	GroupingsDown  []string `json:"groupingsDown,omitempty"`
	Supported      bool     `json:"supported,omitempty"`
}

type AccountRecordTypeDefault struct {
	ProfileID     ID     `json:"profileId"`
	DeveloperName string `json:"developerName"`
	PersonAccount bool   `json:"personAccount,omitempty"`
}

type FeatureParameterMetadata struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	DataflowDirection string `json:"dataflowDirection"`
	Value             string `json:"value"`
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
	// TranslationSourceRaw retains source text that the legacy Value field
	// normalizes. Older and pathless registries do not have this provenance.
	TranslationSourceRaw *string `json:"translationSourceRaw,omitempty"`
}

// StandardValueSetTranslation is a project-authored translation of one
// standard picklist value. Runtime clones share this immutable metadata.
type StandardValueSetTranslation struct {
	ValueSet    string `json:"valueSet"`
	Language    string `json:"language"`
	MasterLabel string `json:"masterLabel"`
	Translation string `json:"translation"`
}

type TabMetadata struct {
	// ComponentBundleID is a deterministic local stand-in for a component bundle ID.
	ComponentBundleID string `json:"componentBundleId,omitempty"`
	// NavigationIdentity is the host-assigned objectApiName for a component
	// tab. It is not an SObjectName and does not make the tab an object tab.
	NavigationIdentity string `json:"navigationIdentity,omitempty"`
	Name               string `json:"name"`
	Label              string `json:"label,omitempty"`
	SObjectName        string `json:"sObjectName,omitempty"`
	Page               string `json:"page,omitempty"`
	URL                string `json:"url,omitempty"`
	Custom             bool   `json:"custom,omitempty"`
	Motif              string `json:"motif,omitempty"`
	Description        string `json:"description,omitempty"`
	File               string `json:"file,omitempty"`
}

type ApplicationMetadata struct {
	Name        string   `json:"name"`
	Label       string   `json:"label,omitempty"`
	Description string   `json:"description,omitempty"`
	Namespace   string   `json:"namespace,omitempty"`
	Logo        string   `json:"logo,omitempty"`
	Tabs        []string `json:"tabs,omitempty"`
	File        string   `json:"file,omitempty"`
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

// DataWeaveResourceMetadata retains the original script source and effective
// component API. ContentPath distinguishes a missing body from an empty script.
type DataWeaveResourceMetadata struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace,omitempty"`
	Content      string `json:"content"`
	APIVersion   string `json:"apiVersion,omitempty"`
	ContentPath  string `json:"contentPath,omitempty"`
	MetadataPath string `json:"metadataPath,omitempty"`
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

// OwnerSharingRule is a supported declared owner-based Read rule between public groups.
type OwnerSharingRule struct {
	Name        string `json:"name"`
	SourceGroup string `json:"sourceGroup"`
	TargetGroup string `json:"targetGroup"`
	AccessLevel string `json:"accessLevel"`
}

type ObjectDefinition struct {
	APIName           string             `json:"apiName"`
	Label             string             `json:"label,omitempty"`
	PluralLabel       string             `json:"pluralLabel,omitempty"`
	KeyPrefix         string             `json:"keyPrefix,omitempty"`
	OwnerSharingRules []OwnerSharingRule `json:"ownerSharingRules,omitempty"`
	SharingModel      string             `json:"sharingModel,omitempty"`
	EnableSearch      bool               `json:"enableSearch,omitempty"`
	Fields            map[string]Field   `json:"fields,omitempty"`
	Relations         []Relationship     `json:"relationships,omitempty"`
	RecordTypes       []RecordTypeInfo   `json:"recordTypes,omitempty"`
	ValidationRules   []ValidationRule   `json:"validationRules,omitempty"`
	AutoResponseRules []AutoResponseRule `json:"autoResponseRules,omitempty"`
	AssignmentRules   []AssignmentRule   `json:"assignmentRules,omitempty"`
	DuplicateRules    []DuplicateRule    `json:"duplicateRules,omitempty"`
	WorkflowRules     []WorkflowRule     `json:"workflowRules,omitempty"`
	FlowRules         []FlowRule         `json:"flowRules,omitempty"`
	Indexes           []IndexDefinition  `json:"indexes,omitempty"`
	Metadata          map[string]string  `json:"metadata,omitempty"`

	// ExternalSharingModel is the declared external organization-wide default
	// (CustomObject externalSharingModel); empty when the metadata omits it.
	ExternalSharingModel string `json:"externalSharingModel,omitempty"`
}

type Field struct {
	APIName        string    `json:"apiName"`
	Label          string    `json:"label,omitempty"`
	InlineHelpText string    `json:"inlineHelpText,omitempty"`
	Type           FieldType `json:"type"`
	MasterDetail   bool      `json:"masterDetail,omitempty"`
	DisplayType    string    `json:"displayType,omitempty"`
	// LongTextArea retains the declared metadata kind behind describe TEXTAREA.
	LongTextArea             bool                `json:"longTextArea,omitempty"`
	Length                   int                 `json:"length,omitempty"`
	Precision                int                 `json:"precision,omitempty"`
	Digits                   int                 `json:"digits,omitempty"`
	Scale                    int                 `json:"scale,omitempty"`
	ScaleSpecified           bool                `json:"scaleSpecified,omitempty"`
	Formula                  string              `json:"formula,omitempty"`
	FormulaTreatBlanksAs     string              `json:"formulaTreatBlanksAs,omitempty"`
	DefaultValue             string              `json:"defaultValue,omitempty"`
	DefaultValueFromMetadata bool                `json:"defaultValueFromMetadata,omitempty"`
	CompoundFieldName        string              `json:"compoundFieldName,omitempty"`
	AutoNumber               bool                `json:"autoNumber,omitempty"`
	DisplayFormat            string              `json:"displayFormat,omitempty"`
	SummarizedField          string              `json:"summarizedField,omitempty"`
	SummaryForeignKey        string              `json:"summaryForeignKey,omitempty"`
	SummaryOperation         string              `json:"summaryOperation,omitempty"`
	SummaryFilterItems       []SummaryFilterItem `json:"summaryFilterItems,omitempty"`
	FilteredLookupInfo       FilteredLookupInfo  `json:"filteredLookupInfo,omitempty"`
	Required                 bool                `json:"required,omitempty"`
	Nillable                 *bool               `json:"nillable,omitempty"`
	DefaultedOnCreate        *bool               `json:"defaultedOnCreate,omitempty"`
	Accessible               *bool               `json:"accessible,omitempty"`
	Createable               *bool               `json:"createable,omitempty"`
	Updateable               *bool               `json:"updateable,omitempty"`
	Filterable               *bool               `json:"filterable,omitempty"`
	Groupable                *bool               `json:"groupable,omitempty"`
	Sortable                 *bool               `json:"sortable,omitempty"`
	Aggregatable             *bool               `json:"aggregatable,omitempty"`
	Permissionable           *bool               `json:"permissionable,omitempty"`
	DeprecatedAndHidden      *bool               `json:"deprecatedAndHidden,omitempty"`
	ExternalID               bool                `json:"externalId,omitempty"`
	Unique                   bool                `json:"unique,omitempty"`
	Encrypted                bool                `json:"encrypted,omitempty"`
	CaseSensitive            bool                `json:"caseSensitive,omitempty"`
	// Describe-only observation; storage comparison semantics stay independent.
	DescribeCaseSensitive    *bool             `json:"describeCaseSensitive,omitempty"`
	RestrictedPicklist       bool              `json:"restrictedPicklist,omitempty"`
	IDLookup                 bool              `json:"idLookup,omitempty"`
	NamePointing             bool              `json:"namePointing,omitempty"`
	ReferenceTo              []string          `json:"referenceTo,omitempty"`
	RelationshipName         string            `json:"relationshipName,omitempty"`
	RelationshipOrder        *int              `json:"relationshipOrder,omitempty"`
	ReparentableMasterDetail bool              `json:"reparentableMasterDetail,omitempty"`
	WriteRequiresMasterRead  bool              `json:"writeRequiresMasterRead,omitempty"`
	ChildRelationshipName    string            `json:"childRelationshipName,omitempty"`
	PicklistController       string            `json:"picklistController,omitempty"`
	PicklistValueSettings    []PicklistSetting `json:"picklistValueSettings,omitempty"`
	PicklistValues           []PicklistValue   `json:"picklistValues,omitempty"`
	// Explicit org configuration can make a standard value set empty. It must
	// not be filled back in by a catalog overlay when the object is described.
	PicklistValuesConfigured bool `json:"picklistValuesConfigured,omitempty"`

	// Distinguish catalog describe restrictions from explicit project or native
	// validation rules. A missing marker preserves existing persisted rules.
	RestrictedPicklistFromCatalog bool `json:"restrictedPicklistFromCatalog,omitempty"`
	StandardValueSetApplied       bool `json:"standardValueSetApplied,omitempty"`

	MetadataRelationshipControllingField string `json:"metadataRelationshipControllingField,omitempty"`
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
	if definition == nil || len(definition.RecordTypes) == 0 || !StandardObjectRecordTypeFieldEnabled(*definition) {
		return
	}
	// This helper also runs before the standard overlay in describe/fixture
	// imports. Record the declared field set before synthesizing its lookup.
	snapshotConfiguredStandardPolicyFields(definition)
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
	// PersonAccount marks a record type declared under the PersonAccount
	// metadata object. Its developer name need not mention "person".
	PersonAccount bool `json:"personAccount,omitempty"`
	// Unavailable marks a record type the running user's profile does not
	// see (record_type_visibility rows): describe reports it unavailable and
	// USER_MODE DML rejects it.
	Unavailable bool `json:"unavailable,omitempty"`
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
	// FlowLiteralString marks a present string literal, including an empty one.
	FlowLiteralString bool   `json:"flowLiteralString,omitempty"`
	Formula           string `json:"formula,omitempty"`
	// FlowFrameFormula marks an individually selected record-triggered Flow formula.
	FlowFrameFormula bool   `json:"flowFrameFormula,omitempty"`
	SourceField      string `json:"sourceField,omitempty"`
	// ActionOutput names a Flow Apex action whose automatically stored
	// output supplies the value.
	ActionOutput string `json:"actionOutput,omitempty"`
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
	APIVersion                             string `json:"apiVersion,omitempty"`
	SourceStatus                           string `json:"sourceStatus,omitempty"`
	Label                                  string `json:"label,omitempty"`
	Name                                   string `json:"name"`
	File                                   string `json:"file,omitempty"`
	ProjectSourceOwned                     bool   `json:"projectSourceOwned,omitempty"`
	Active                                 bool   `json:"active,omitempty"`
	ProcessType                            string `json:"processType,omitempty"`
	TriggerType                            string `json:"triggerType,omitempty"`
	DoesRequireRecordChangedToMeetCriteria bool   `json:"doesRequireRecordChangedToMeetCriteria,omitempty"`
	RecordTriggerType                      string `json:"recordTriggerType,omitempty"`
	TriggerOrder                           int    `json:"triggerOrder,omitempty"`
	RunInMode                              string `json:"runInMode,omitempty"`
	Formula                                string `json:"formula,omitempty"`
	// CoreEntryFilterLogic distinguishes a generated start-filter formula from an arbitrary formula.
	CoreEntryFilterLogic bool              `json:"coreEntryFilterLogic,omitempty"`
	TextTemplates        map[string]string `json:"textTemplates,omitempty"`
	// ProcessStringConstants are available only to captured process subflow interviews.
	ProcessStringConstants map[string]string      `json:"processStringConstants,omitempty"`
	Criteria               []WorkflowCriteriaItem `json:"criteria,omitempty"`
	// CriteriaGroups holds start-filter logic other than AND as alternatives:
	// each group lists indexes into Criteria that must all match, and the
	// entry matches when any group does ("or", "1 OR (2 AND 3)").
	CriteriaGroups [][]int               `json:"criteriaGroups,omitempty"`
	Branches       []FlowBranch          `json:"branches,omitempty"`
	Steps          []FlowStep            `json:"steps,omitempty"`
	Paths          []FlowPath            `json:"paths,omitempty"`
	FieldUpdates   []WorkflowFieldUpdate `json:"fieldUpdates,omitempty"`
	Actions        []FlowAction          `json:"actions,omitempty"`
	RecordLookups  []FlowRecordLookup    `json:"recordLookups,omitempty"`
	RecordCreates  []FlowRecordCreate    `json:"recordCreates,omitempty"`
	RecordDeletes  []FlowRecordDelete    `json:"recordDeletes,omitempty"`
	CustomErrors   []FlowCustomError     `json:"customErrors,omitempty"`
	ApexPlugins    []FlowApexPluginCall  `json:"apexPlugins,omitempty"`
	Variables      []FlowVariable        `json:"variables,omitempty"`
	// StepTargets stores shared connector targets once. Reference steps name a
	// target instead of recursively embedding its continuation.
	StepTargets map[string][]FlowStep `json:"stepTargets,omitempty"`
}

// LinearRecordUpdates returns the flow's record updates in order when its steps
// are only record updates and references to them.
func (rule FlowRule) LinearRecordUpdates() ([]FlowRecordUpdate, bool) {
	if len(rule.Branches) > 0 {
		return nil, false
	}
	var updates []FlowRecordUpdate
	visited := make(map[string]bool)
	var walk func([]FlowStep) bool
	walk = func(steps []FlowStep) bool {
		for _, step := range steps {
			switch step.Kind {
			case "recordUpdate":
				updates = append(updates, step.RecordUpdate)
			case "reference":
				key := strings.ToLower(strings.TrimSpace(step.Target))
				if visited[key] {
					return false
				}
				visited[key] = true
				if !walk(rule.StepTargets[key]) {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	if !walk(rule.Steps) || len(updates) == 0 {
		return nil, false
	}
	return updates, true
}

// ProcessRecordUpdates recognizes the linear form and the captured single
// non-null-input decision that precedes one update in a process flow action.
// CapturedProcessActionSteps recognizes the two-invocable subflow graph
// observed in process_builder_review twoactions, without admitting other graphs.
func (rule FlowRule) CapturedProcessActionSteps() ([]FlowStep, bool) {
	if len(rule.Variables) != 1 || len(rule.Steps) != 4 || len(rule.Branches) != 0 || len(rule.StepTargets) != 0 || len(rule.Paths) != 0 {
		return nil, false
	}
	variable := rule.Variables[0]
	if !variable.IsInput || variable.IsOutput || variable.IsCollection || !strings.EqualFold(variable.DataType, "String") {
		return nil, false
	}
	for _, step := range rule.Steps {
		if step.FaultTarget != "" || len(step.FaultBranch) != 0 {
			return nil, false
		}
	}
	a := rule.Steps[0].Assignment
	if rule.Steps[0].Kind != "assignment" || !strings.EqualFold(a.Operator, "Assign") || !strings.EqualFold(a.Target, variable.Name) || !strings.EqualFold(a.SourceField, variable.Name) || a.Formula != "" || a.LiteralValue != "" {
		return nil, false
	}
	for _, step := range rule.Steps[1:3] {
		action := step.Action
		if step.Kind != "action" || !strings.EqualFold(action.ActionType, "apex") || len(action.Inputs) != 1 || len(action.Outputs) != 0 {
			return nil, false
		}
		input := action.Inputs[0]
		if input.Field == "" || !strings.EqualFold(input.SourceField, variable.Name) || input.Formula != "" || input.LiteralValue != "" || input.ActionOutput != "" {
			return nil, false
		}
	}
	if rule.Steps[1].Action.ActionName == "" || rule.Steps[2].Action.ActionName == "" || strings.EqualFold(rule.Steps[1].Action.ActionName, rule.Steps[2].Action.ActionName) {
		return nil, false
	}
	if rule.Steps[3].Kind != "recordUpdate" {
		return nil, false
	}
	return rule.Steps, true
}

// CapturedProcessLookupSteps recognizes the lookup/lookup/assignment/update
// graph observed in process_builder_review twolookups.
func (rule FlowRule) CapturedProcessLookupSteps() ([]FlowStep, bool) {
	if len(rule.Variables) != 3 || len(rule.Steps) != 4 || len(rule.Branches) != 0 || len(rule.StepTargets) != 0 || len(rule.Paths) != 0 {
		return nil, false
	}
	var input string
	for _, v := range rule.Variables {
		if v.IsOutput || v.IsCollection || !strings.EqualFold(v.DataType, "String") || (v.DefaultValue != nil && v.DefaultValue.Kind != ValueNull) {
			return nil, false
		}
		if v.IsInput {
			if input != "" {
				return nil, false
			}
			input = v.Name
		}
	}
	if input == "" {
		return nil, false
	}
	for _, step := range rule.Steps {
		if step.FaultTarget != "" || len(step.FaultBranch) != 0 {
			return nil, false
		}
	}
	for _, step := range rule.Steps[:2] {
		l := step.RecordLookup
		if step.Kind != "recordLookup" || !l.GetFirstRecordOnly || !l.StoreOutputAutomatically || l.AssignNullValuesIfNoRecordsFound || l.OutputReference != "" || len(l.QueriedFields) != 0 || len(l.OutputAssignments) != 0 || l.SortField != "" || l.SortOrder != "" || len(l.Criteria) != 1 || !strings.EqualFold(l.CriteriaLogic, "and") {
			return nil, false
		}
		c := l.Criteria[0]
		if !strings.EqualFold(c.Field, "Id") || !strings.EqualFold(c.Operation, "equals") || !strings.EqualFold(c.SourceField, input) || c.Value != "" {
			return nil, false
		}
	}
	a := rule.Steps[2].Assignment
	if rule.Steps[2].Kind != "assignment" || !strings.EqualFold(a.Operator, "Assign") || a.Target == "" || strings.EqualFold(a.Target, input) || a.LiteralValue == "" || a.Formula != "" || a.SourceField != "" {
		return nil, false
	}
	found := false
	for _, v := range rule.Variables {
		if strings.EqualFold(v.Name, a.Target) && !v.IsInput {
			found = true
		}
	}
	if !found || rule.Steps[3].Kind != "recordUpdate" || len(rule.Steps[3].RecordUpdate.InputAssignments) != 1 || !strings.EqualFold(rule.Steps[3].RecordUpdate.InputAssignments[0].SourceField, a.Target) {
		return nil, false
	}
	return rule.Steps, true
}

func (rule FlowRule) ProcessRecordUpdates() ([]FlowRecordUpdate, bool) {
	if updates, ok := rule.LinearRecordUpdates(); ok {
		return updates, true
	}
	if steps, ok := rule.CapturedProcessDependentLookupSteps(); ok {
		return []FlowRecordUpdate{steps[3].RecordUpdate}, true
	}
	if steps, ok := rule.CapturedProcessRecordActionSteps(); ok {
		return []FlowRecordUpdate{steps[2].RecordUpdate}, true
	}
	if steps, ok := rule.CapturedProcessActionSteps(); ok {
		return []FlowRecordUpdate{steps[3].RecordUpdate}, true
	}
	if steps, ok := rule.CapturedProcessLookupSteps(); ok {
		return []FlowRecordUpdate{steps[3].RecordUpdate}, true
	}
	if len(rule.Variables) != 1 || !rule.Variables[0].IsInput || rule.Variables[0].IsCollection || !strings.EqualFold(rule.Variables[0].DataType, "String") || len(rule.Branches) != 0 || len(rule.Steps) != 1 || len(rule.StepTargets) != 0 {
		return nil, false
	}
	step := rule.Steps[0]
	if step.Kind != "decision" || step.FaultTarget != "" || len(step.FaultBranch) != 0 || len(step.Branches) != 1 {
		return nil, false
	}
	branch := step.Branches[0]
	if branch.Default || branch.Formula != "" || len(branch.Criteria) != 1 || len(branch.Steps) != 1 {
		return nil, false
	}
	criterion := branch.Criteria[0]
	if !strings.EqualFold(criterion.Field, rule.Variables[0].Name) || !strings.EqualFold(criterion.Operation, "isNull") || !strings.EqualFold(criterion.Value, "false") || criterion.SourceField != "" {
		return nil, false
	}
	next := branch.Steps[0]
	if next.Kind != "recordUpdate" || next.FaultTarget != "" || len(next.FaultBranch) != 0 {
		return nil, false
	}
	return []FlowRecordUpdate{next.RecordUpdate}, true
}

// SimpleProcessAfterSaveAction recognizes the single record-input action
// form captured alongside a process in process_builder_review multiflow.
func (rule FlowRule) SimpleProcessAfterSaveAction() (FlowAction, bool) {
	if !rule.Active || !strings.EqualFold(rule.TriggerType, "RecordAfterSave") || !strings.EqualFold(rule.RecordTriggerType, "CreateAndUpdate") || rule.TriggerOrder != 0 || rule.Formula != "" || len(rule.Criteria) != 0 || len(rule.Branches) != 0 || len(rule.Paths) != 0 || len(rule.StepTargets) != 0 || len(rule.Steps) != 1 {
		return FlowAction{}, false
	}
	step := rule.Steps[0]
	if step.Kind != "action" || step.FaultTarget != "" || len(step.FaultBranch) != 0 || !strings.EqualFold(step.Action.ActionType, "apex") || len(step.Action.Inputs) != 1 || len(step.Action.Outputs) != 0 {
		return FlowAction{}, false
	}
	input := step.Action.Inputs[0]
	if !strings.EqualFold(input.SourceField, "$Record") || input.Field == "" || input.Formula != "" || input.LiteralValue != "" || input.ActionOutput != "" {
		return FlowAction{}, false
	}
	return step.Action, true
}

// FlowPath retains a deferred record-triggered entry independently of the
// rule's immediate graph. Kind is async or scheduled.
type FlowPath struct {
	Name         string                `json:"name"`
	Kind         string                `json:"kind"`
	Steps        []FlowStep            `json:"steps,omitempty"`
	StepTargets  map[string][]FlowStep `json:"stepTargets,omitempty"`
	OffsetNumber float64               `json:"offsetNumber,omitempty"`
	OffsetUnit   string                `json:"offsetUnit,omitempty"`
	TimeSource   string                `json:"timeSource,omitempty"`
	RecordField  string                `json:"recordField,omitempty"`
	MaxBatchSize int                   `json:"maxBatchSize,omitempty"`
}

// FlowVariable describes an autolaunched Flow interview variable. Keeping
// this small metadata projection on the rule lets local Apex code query the
// standard FlowDefinitionView/FlowVariableView surfaces without reparsing the
// source file during execution.
type FlowVariable struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	DataType     string `json:"dataType,omitempty"`
	ObjectType   string `json:"objectType,omitempty"`
	IsCollection bool   `json:"isCollection,omitempty"`
	IsInput      bool   `json:"isInput,omitempty"`
	IsOutput     bool   `json:"isOutput,omitempty"`
	DefaultValue *Value `json:"defaultValue,omitempty"`
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
	Target              string                  `json:"target,omitempty"`
	VisitKey            string                  `json:"visitKey,omitempty"`
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
	Formula      string `json:"formula,omitempty"`
	SourceField  string `json:"sourceField,omitempty"`
}

type FlowLoop struct {
	Name                 string     `json:"name"`
	CollectionReference  string     `json:"collectionReference"`
	CurrentItemReference string     `json:"currentItemReference,omitempty"`
	IterationOrder       string     `json:"iterationOrder,omitempty"`
	Steps                []FlowStep `json:"steps,omitempty"`
}

type FlowAction struct {
	Name       string                `json:"name"`
	ActionType string                `json:"actionType,omitempty"`
	ActionName string                `json:"actionName,omitempty"`
	ClassName  string                `json:"className,omitempty"`
	MethodName string                `json:"methodName,omitempty"`
	Inputs     []WorkflowFieldUpdate `json:"inputs,omitempty"`
	// Outputs maps an Apex action's returned fields to Flow variables
	// (actionCalls outputParameters with assignToReference).
	Outputs []FlowActionOutput `json:"outputs,omitempty"`
}

// FlowActionOutput assigns one returned field of an action to a Flow variable.
type FlowActionOutput struct {
	Name     string `json:"name"`
	AssignTo string `json:"assignTo"`
}

type FlowRecordLookup struct {
	Name                             string                 `json:"name"`
	ObjectName                       string                 `json:"objectName"`
	SortField                        string                 `json:"sortField,omitempty"`
	SortOrder                        string                 `json:"sortOrder,omitempty"`
	CriteriaLogic                    string                 `json:"criteriaLogic,omitempty"`
	Criteria                         []WorkflowCriteriaItem `json:"criteria,omitempty"`
	GetFirstRecordOnly               bool                   `json:"getFirstRecordOnly,omitempty"`
	StoreOutputAutomatically         bool                   `json:"storeOutputAutomatically,omitempty"`
	AssignNullValuesIfNoRecordsFound bool                   `json:"assignNullValuesIfNoRecordsFound,omitempty"`
	OutputReference                  string                 `json:"outputReference,omitempty"`
	QueriedFields                    []string               `json:"queriedFields,omitempty"`
	OutputAssignments                []FlowAssignment       `json:"outputAssignments,omitempty"`
}

type FlowRecordCreate struct {
	Name                      string                `json:"name"`
	ObjectName                string                `json:"objectName"`
	InputReference            string                `json:"inputReference,omitempty"`
	AssignRecordIDToReference string                `json:"assignRecordIdToReference,omitempty"`
	InputAssignments          []WorkflowFieldUpdate `json:"inputAssignments,omitempty"`
	StoreOutputAutomatically  bool                  `json:"storeOutputAutomatically,omitempty"`
}

type FlowRecordUpdate struct {
	Name             string                 `json:"name"`
	ObjectName       string                 `json:"objectName,omitempty"`
	InputReference   string                 `json:"inputReference,omitempty"`
	Criteria         []WorkflowCriteriaItem `json:"criteria,omitempty"`
	CriteriaLogic    string                 `json:"criteriaLogic,omitempty"`
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
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// APIVersion is the flow's source API version; DateTime and Time merge
	// fields render per version (flow_record_merge).
	APIVersion string                   `json:"apiVersion,omitempty"`
	Messages   []FlowCustomErrorMessage `json:"messages,omitempty"`
}

type FlowCustomErrorMessage struct {
	Message         string            `json:"message"`
	Formulas        map[string]string `json:"formulas,omitempty"`
	NumberConstants map[string]string `json:"numberConstants,omitempty"`
	// ResourceFormulas holds every flow formula resource a {!name} merge field
	// may name; Formulas keeps the captured frame-bound subset.
	ResourceFormulas map[string]string `json:"resourceFormulas,omitempty"`
	// Field names the record field of a field-level error (isFieldError).
	Field string `json:"field,omitempty"`
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
	ObjectType       string                `json:"objectType,omitempty"`
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
	FieldTime          FieldType = "TIME"
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
	parentLookup *relationshipParentLookup

	// InheritedFrom retains relationship provenance for reverse describe.
	InheritedFrom       string   `json:"inheritedFrom,omitempty"`
	Field               string   `json:"field"`
	ParentObjects       []string `json:"parentObjects"`
	ParentRelationship  string   `json:"parentRelationship,omitempty"`
	ChildRelationship   string   `json:"childRelationship,omitempty"`
	CascadeDelete       bool     `json:"cascadeDelete,omitempty"`
	RestrictedDelete    bool     `json:"restrictedDelete,omitempty"`
	SetNullOnDelete     bool     `json:"setNullOnDelete,omitempty"`
	DeprecatedAndHidden bool     `json:"deprecatedAndHidden,omitempty"`
	JunctionIDListNames []string `json:"junctionIdListNames,omitempty"`
	JunctionReferenceTo []string `json:"junctionReferenceTo,omitempty"`
	Polymorphic         bool     `json:"polymorphic,omitempty"`
	DeferredIntegrity   bool     `json:"deferredIntegrity,omitempty"`
}

// LookupHistoryScope identifies one transaction without exposing its identity
// to serialized records or Apex values.
type LookupHistoryScope struct{ identity byte }

func NewLookupHistoryScope() *LookupHistoryScope { return &LookupHistoryScope{} }

type Record struct {
	// triggerAssignments tracks explicit before-trigger assignments during DML.
	triggerAssignments uint8
	// dmlNameUnassigned is transient provenance for a queried, unassigned Name.
	// Unexported fields are not retained by JSON or gob snapshots.
	dmlNameUnassigned bool
	// dmlDefaultRecordTypeID is transient provenance for a VM-supplied default.
	// It is intentionally unexported so JSON and gob snapshots do not retain it.
	dmlDefaultRecordTypeID *ID
	// AssignmentRule is a transient per-record DML option, never persisted.
	AssignmentRule *AssignmentRuleSelection `json:"-"`
	// MetadataSourceUnit binds source-backed metadata collisions to their producing project.
	MetadataSourceUnit string `json:"metadataSourceUnit,omitempty"`
	// HierarchyNameUsesRecordID retains omitted Name/owner insert provenance.
	// Explicit-null Name can produce the same text through a different default.
	HierarchyNameUsesRecordID bool `json:"hierarchyNameUsesRecordId,omitempty"`
	// LoadedReferences is transient DML input provenance, never persisted.
	// Each canonical field retains the reference ID from an unchanged SOQL view.
	LoadedReferences map[string]ID `json:"-"`
	// triggerFormulasMaterialized marks a hydrated before-update record whose
	// formula values are in Fields and ExplicitNulls; triggerFormulas holds the
	// values that read differently on a Trigger.new record (the trigger
	// surface keeps whitespace and renders BR() as text). Both are transient
	// DML provenance, unexported so snapshots never retain them.
	triggerFormulasMaterialized bool
	triggerFormulas             map[string]Value
	// defaultedLineBreaks names the fields whose VM-supplied default value
	// holds a line feed; DML keeps it in a single-line Text field (org capture
	// flow_br_surfaces: a default formula with BR() stores a line feed, an
	// assigned line feed stores a space). Transient and unexported.
	defaultedLineBreaks map[string]bool
	// Cleared lookup history is transaction-local and remains unexported so
	// runtime snapshots and Gob payloads cannot persist a transaction marker.
	clearedLookupReferences map[string]ID       `json:"-"`
	clearedLookupScope      *LookupHistoryScope `json:"-"`
	ID                      ID                  `json:"id"`
	Object                  string              `json:"object"`
	Fields                  map[string]Value    `json:"fields,omitempty"`
	Children                map[string][]Record `json:"children,omitempty"`
	ParentRelationships     map[string]Record   `json:"-"`
	ExplicitNulls           map[string]bool     `json:"explicitNulls,omitempty"`
	System                  SystemFields        `json:"system,omitempty"`

	// ClearedReferences retains optional lookups cleared by parent deletion.
	ClearedReferences map[string]ClearedReference `json:"clearedReferences,omitempty"`
}

// MarkDefaultedLineBreak records that the default supplied for field holds a
// line break the single-line Text normalization must keep.
func (r *Record) MarkDefaultedLineBreak(field string) {
	if r.defaultedLineBreaks == nil {
		r.defaultedLineBreaks = make(map[string]bool)
	}
	r.defaultedLineBreaks[strings.ToLower(field)] = true
}

// KeepsDefaultedLineBreak reports a MarkDefaultedLineBreak for field.
func (r Record) KeepsDefaultedLineBreak(field string) bool {
	return r.defaultedLineBreaks[strings.ToLower(field)]
}

// MarkTriggerFormulasMaterialized records that the formula values in Fields
// and ExplicitNulls were computed for this record's Trigger.new view.
func (r *Record) MarkTriggerFormulasMaterialized() { r.triggerFormulasMaterialized = true }

// TriggerFormulasMaterialized reports whether the formula values in Fields and
// ExplicitNulls were computed for this record's Trigger.new view.
func (r Record) TriggerFormulasMaterialized() bool { return r.triggerFormulasMaterialized }

// SetTriggerFormula records a formula value that reads differently on the
// Trigger.new record than the value in Fields; a null result is a null Value.
func (r *Record) SetTriggerFormula(field string, value Value) {
	if r.triggerFormulas == nil {
		r.triggerFormulas = make(map[string]Value)
	}
	r.triggerFormulas[field] = value
}

// TriggerFormulas returns the recorded trigger-surface formula values, or nil.
func (r Record) TriggerFormulas() map[string]Value { return r.triggerFormulas }

// MarkDMLTriggerAssignment records a user assignment during a before trigger.
func (r *Record) MarkDMLTriggerAssignment(field string) {
	if strings.EqualFold(field, "Name") {
		r.triggerAssignments |= 1
	}
	if strings.EqualFold(field, "RecordTypeId") {
		r.triggerAssignments |= 2
	}
}

// DMLTriggerAssigned reports whether a before trigger assigned the field.
func (r Record) DMLTriggerAssigned(field string) bool {
	if strings.EqualFold(field, "Name") {
		return r.triggerAssignments&1 != 0
	}
	if strings.EqualFold(field, "RecordTypeId") {
		return r.triggerAssignments&2 != 0
	}
	return false
}

// ClearDMLTriggerAssignments removes transaction-local assignment provenance.
func (r *Record) ClearDMLTriggerAssignments() { r.triggerAssignments = 0 }

// MarkDMLNameUnassigned records a queried Name that was not assigned.
func (r *Record) MarkDMLNameUnassigned() { r.dmlNameUnassigned = true }

// DMLNameUnassigned reports queried, unassigned Name provenance.
func (r Record) DMLNameUnassigned() bool { return r.dmlNameUnassigned }

// ClearDMLNameUnassigned removes query provenance before storage.
func (r *Record) ClearDMLNameUnassigned() { r.dmlNameUnassigned = false }

// MarkDMLRecordTypeDefaulted records the current VM-supplied record type.
func (r *Record) MarkDMLRecordTypeDefaulted() {
	value, ok := r.GetField("RecordTypeId")
	if !ok {
		return
	}
	var id ID
	if value.Kind == ValueID {
		id = value.ID
	} else if value.Kind == ValueString {
		id = ID(value.String)
	}
	if id != "" {
		r.dmlDefaultRecordTypeID = &id
	}
}

// DMLRecordTypeDefaulted is false if the default was subsequently changed.
func (r Record) DMLRecordTypeDefaulted() bool {
	if r.dmlDefaultRecordTypeID == nil {
		return false
	}
	value, ok := r.GetField("RecordTypeId")
	if !ok {
		return false
	}
	if value.Kind == ValueID {
		return IDsEqual(*r.dmlDefaultRecordTypeID, value.ID)
	}
	return value.Kind == ValueString && IDsEqual(*r.dmlDefaultRecordTypeID, ID(value.String))
}

// ClearDMLRecordTypeDefaulted drops the provenance before a record is stored.
func (r *Record) ClearDMLRecordTypeDefaulted() {
	r.dmlDefaultRecordTypeID = nil
}

type ClearedReference struct {
	ParentObject string `json:"parentObject"`
	ParentID     ID     `json:"parentId"`
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
	CreatedByID            ID     `json:"createdById,omitempty"`
	CreatedDate            string `json:"createdDate,omitempty"`
	LastModifiedByID       ID     `json:"lastModifiedById,omitempty"`
	LastModifiedDate       string `json:"lastModifiedDate,omitempty"`
	SystemModstamp         string `json:"systemModstamp,omitempty"`
	OwnerID                ID     `json:"ownerId,omitempty"`
	IsDeleted              bool   `json:"isDeleted,omitempty"`
	RecycleBinEmptied      bool   `json:"recycleBinEmptied,omitempty"`
	CascadeDeletedByObject string `json:"cascadeDeletedByObject,omitempty"`
	CascadeDeletedByID     ID     `json:"cascadeDeletedById,omitempty"`
	Locked                 bool   `json:"locked,omitempty"`
	HiddenFromSOQL         bool   `json:"-"`
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
	case FieldString, FieldPicklist, FieldMultiPicklist, FieldDate, FieldDateTime, FieldTime, FieldID, FieldAny:
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
	case FieldString, FieldPicklist, FieldMultiPicklist, FieldDate, FieldDateTime, FieldTime, FieldID, FieldAny:
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
	objects := make(map[string]ObjectState)
	return OrgState{
		Objects:           objects,
		IDSequences:       make(map[string]uint64),
		objectNameCache:   newObjectNameCache(objects),
		runtimeGeneration: newOrgRuntimeGeneration(),
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
	if org.StandardSpread != "" {
		// Objects is shared by value copies, so this installs for the caller.
		installSpreadObject(&org, name)
	}
	org.objectNameCache = ownedObjectNameCache(org)
	cacheKey := strings.ToLower(strings.TrimSpace(org.Namespace)) + "|" + strconv.Itoa(len(org.Objects)) + "|" + strings.ToLower(strings.TrimSpace(name))
	if org.objectNameCache != nil {
		if cached, ok := org.objectNameCache.Load(cacheKey); ok {
			if resolved, resolvedOK := cached.(string); resolvedOK && resolved != "" {
				return resolved, true
			}
		}
	}
	resolved, ok := resolveObjectName(org, name)
	if ok {
		cacheResolvedObjectName(org, cacheKey, resolved)
	}
	return resolved, ok
}

func resolveObjectName(org OrgState, name string) (string, bool) {
	if org.Namespace != "" && !hasNamespaceToken(name) && isCustomAPIName(name) {
		prefixed := NamespaceTokenName(org.Namespace, name)
		if prefixed != name {
			if _, ok := org.Objects[prefixed]; ok {
				return prefixed, true
			}
		}
	}
	if exact, ok := org.Objects[name]; ok {
		if preferred, preferredOK := richerNamespacedObjectMatch(org, name, exact); preferredOK {
			return preferred, true
		}
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
	// The remaining matches ignore case or namespace. They come from the
	// name index, and a name that matches nothing is remembered as a miss.
	index := objectNameIndexFor(org)
	miss := objectNameMiss{namespace: org.Namespace, name: name}
	if index != nil {
		if _, missed := index.misses.Load(miss); missed {
			return "", false
		}
	}
	resolved, ok := resolveObjectNameFolding(org, index, name, prefixed, stripped)
	if !ok && index != nil {
		index.misses.Store(miss, struct{}{})
	}
	return resolved, ok
}

func resolveObjectNameFolding(org OrgState, index *objectNameIndex, name, prefixed, stripped string) (string, bool) {
	if candidate, ok := firstObjectNameFolding(org, index, name, prefixed, stripped); ok {
		return candidate, true
	}
	if hasNamespaceToken(name) {
		unqualified := StripAnyNamespaceToken(name)
		if unqualified != name {
			if _, ok := org.Objects[unqualified]; ok {
				return unqualified, true
			}
			if candidate, ok := firstObjectNameFolding(org, index, unqualified); ok {
				return candidate, true
			}
		}
	}
	if !hasNamespaceToken(name) && isCustomAPIName(name) {
		// More than one namespace match is ambiguous.
		if candidates := objectNamesStrippingTo(org, index, name); len(candidates) == 1 {
			return candidates[0], true
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
	for _, candidate := range objectNamesStrippingTo(org, objectNameIndexFor(org), name) {
		if strings.EqualFold(candidate, name) {
			continue
		}
		if match != "" {
			return "", false
		}
		match = candidate
	}
	if match == "" {
		return "", false
	}
	matched := org.Objects[match]
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
	// Custom Address components (<Base>__<Part>__s) carry the package
	// namespace of their compound field, like __c names.
	if namespace != "" && hasAPISuffix(name, "__s") && StripNamespaceToken(namespace, name) == name {
		component := namespace + "__" + name
		if _, ok := definition.Fields[component]; ok {
			return component, true
		}
		prefixed = component
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

// ResolveFieldDefinition returns the canonical field name and its metadata.
// Location component fields are exposed by Salesforce as virtual decimal
// fields even though source metadata declares only the compound Location field.
func ResolveFieldDefinition(definition ObjectDefinition, namespace, name string) (string, Field, bool) {
	canonical, ok := ResolveFieldName(definition, namespace, name)
	if !ok {
		return "", Field{}, false
	}
	if field, exists := definition.Fields[canonical]; exists {
		if field.APIName == "" {
			field.APIName = canonical
		}
		return canonical, field, true
	}
	component, ok := ResolveLocationComponentField(definition, namespace, name)
	if !ok {
		return "", Field{}, false
	}
	return component.APIName, component, true
}

// ResolveLocationComponentField returns the Salesforce descriptor for a
// virtual latitude or longitude component of a compound Location field.
func ResolveLocationComponentField(definition ObjectDefinition, namespace, name string) (Field, bool) {
	canonical, ok := resolveLocationComponentField(definition, namespace, name)
	if !ok {
		return Field{}, false
	}
	suffix := "__Latitude__s"
	labelSuffix := "Latitude"
	if strings.HasSuffix(strings.ToLower(canonical), strings.ToLower("__Longitude__s")) {
		suffix = "__Longitude__s"
		labelSuffix = "Longitude"
	}
	baseName := strings.TrimSuffix(canonical, suffix) + "__c"
	baseField, ok := definition.Fields[baseName]
	if !ok {
		for candidate, field := range definition.Fields {
			if strings.EqualFold(candidate, baseName) && field.Type == FieldLocation {
				baseName = candidate
				baseField = field
				ok = true
				break
			}
		}
	}
	if !ok || baseField.Type != FieldLocation {
		return Field{}, false
	}
	compoundName := baseField.APIName
	if compoundName == "" {
		compoundName = baseName
	}
	label := strings.TrimSpace(baseField.Label)
	if label == "" {
		label = strings.TrimSuffix(compoundName, "__c")
	}
	component := Field{
		APIName:             canonical,
		Label:               label + " " + labelSuffix,
		Type:                FieldDecimal,
		DisplayType:         "DOUBLE",
		Precision:           18,
		Scale:               15,
		CompoundFieldName:   compoundName,
		Nillable:            BoolFlag(true),
		Createable:          BoolFlag(true),
		Updateable:          BoolFlag(true),
		Filterable:          BoolFlag(true),
		Groupable:           BoolFlag(false),
		Sortable:            BoolFlag(true),
		Aggregatable:        BoolFlag(true),
		Permissionable:      baseField.Permissionable,
		DeprecatedAndHidden: baseField.DeprecatedAndHidden,
	}
	return component, true
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
	return hasAPISuffix(name, "__c") || hasAPISuffix(name, "__r") || hasAPISuffix(name, "__e") || hasAPISuffix(name, "__mdt") || hasAPISuffix(name, "__share")
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
	if r.AssignmentRule != nil {
		selection := *r.AssignmentRule
		out.AssignmentRule = &selection
	}
	if r.clearedLookupReferences != nil {
		out.clearedLookupReferences = make(map[string]ID, len(r.clearedLookupReferences))
		for field, id := range r.clearedLookupReferences {
			out.clearedLookupReferences[field] = id
		}
	}
	if r.LoadedReferences != nil {
		out.LoadedReferences = make(map[string]ID, len(r.LoadedReferences))
		for field, id := range r.LoadedReferences {
			out.LoadedReferences[field] = id
		}
	}
	if r.defaultedLineBreaks != nil {
		out.defaultedLineBreaks = make(map[string]bool, len(r.defaultedLineBreaks))
		for field := range r.defaultedLineBreaks {
			out.defaultedLineBreaks[field] = true
		}
	}
	if r.triggerFormulas != nil {
		out.triggerFormulas = make(map[string]Value, len(r.triggerFormulas))
		for field, value := range r.triggerFormulas {
			out.triggerFormulas[field] = value.Clone()
		}
	}
	if r.ClearedReferences != nil {
		out.ClearedReferences = make(map[string]ClearedReference, len(r.ClearedReferences))
		for field, reference := range r.ClearedReferences {
			out.ClearedReferences[field] = reference
		}
	}
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
			field.DescribeCaseSensitive = cloneBoolFlag(field.DescribeCaseSensitive)
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
	out.OwnerSharingRules = append([]OwnerSharingRule(nil), d.OwnerSharingRules...)
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
	out.DuplicateRules = CloneDuplicateRules(d.DuplicateRules)
	out.AssignmentRules = CloneAssignmentRules(d.AssignmentRules)
	out.AutoResponseRules = CloneAutoResponseRules(d.AutoResponseRules)
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
		out.FlowRules[i].TextTemplates = cloneStringMap(d.FlowRules[i].TextTemplates)
		out.FlowRules[i].ProcessStringConstants = cloneStringMap(d.FlowRules[i].ProcessStringConstants)
		out.FlowRules[i].Criteria = append([]WorkflowCriteriaItem(nil), d.FlowRules[i].Criteria...)
		out.FlowRules[i].Steps = cloneFlowSteps(d.FlowRules[i].Steps)
		out.FlowRules[i].StepTargets = cloneFlowStepTargets(d.FlowRules[i].StepTargets)
		out.FlowRules[i].Paths = cloneFlowPaths(d.FlowRules[i].Paths)
		out.FlowRules[i].FieldUpdates = append([]WorkflowFieldUpdate(nil), d.FlowRules[i].FieldUpdates...)
		out.FlowRules[i].Actions = cloneFlowActions(d.FlowRules[i].Actions)
		out.FlowRules[i].Branches = cloneFlowBranches(d.FlowRules[i].Branches)
		out.FlowRules[i].RecordLookups = cloneFlowRecordLookups(d.FlowRules[i].RecordLookups)
		out.FlowRules[i].RecordCreates = cloneFlowRecordCreates(d.FlowRules[i].RecordCreates)
		out.FlowRules[i].RecordDeletes = cloneFlowRecordDeletes(d.FlowRules[i].RecordDeletes)
		out.FlowRules[i].CustomErrors = cloneFlowCustomErrors(d.FlowRules[i].CustomErrors)
		out.FlowRules[i].ApexPlugins = cloneFlowApexPlugins(d.FlowRules[i].ApexPlugins)
		out.FlowRules[i].Variables = append([]FlowVariable(nil), d.FlowRules[i].Variables...)
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
		out[i].RecordLookup.QueriedFields = append([]string(nil), steps[i].RecordLookup.QueriedFields...)
		out[i].RecordLookup.OutputAssignments = append([]FlowAssignment(nil), steps[i].RecordLookup.OutputAssignments...)
		out[i].RecordCreate.InputAssignments = append([]WorkflowFieldUpdate(nil), steps[i].RecordCreate.InputAssignments...)
		out[i].RecordUpdate.Criteria = append([]WorkflowCriteriaItem(nil), steps[i].RecordUpdate.Criteria...)
		out[i].RecordUpdate.InputAssignments = append([]WorkflowFieldUpdate(nil), steps[i].RecordUpdate.InputAssignments...)
		out[i].RecordDelete.Criteria = append([]WorkflowCriteriaItem(nil), steps[i].RecordDelete.Criteria...)
		out[i].Subflow.InputAssignments = append([]WorkflowFieldUpdate(nil), steps[i].Subflow.InputAssignments...)
		out[i].Subflow.OutputAssignments = append([]FlowAssignment(nil), steps[i].Subflow.OutputAssignments...)
		out[i].Transform.FieldMappings = append([]WorkflowFieldUpdate(nil), steps[i].Transform.FieldMappings...)
		out[i].CollectionProcessor.Criteria = append([]WorkflowCriteriaItem(nil), steps[i].CollectionProcessor.Criteria...)
		out[i].CollectionProcessor.FieldMappings = append([]WorkflowFieldUpdate(nil), steps[i].CollectionProcessor.FieldMappings...)
		out[i].CustomError.Messages = cloneFlowCustomErrorMessages(steps[i].CustomError.Messages)
		out[i].FaultBranch = cloneFlowSteps(steps[i].FaultBranch)
		out[i].Loop.Steps = cloneFlowSteps(steps[i].Loop.Steps)
		out[i].Branches = cloneFlowBranches(steps[i].Branches)
	}
	return out
}

func cloneFlowRules(rules []FlowRule) []FlowRule {
	out := append([]FlowRule(nil), rules...)
	for i := range out {
		out[i].TextTemplates = cloneStringMap(rules[i].TextTemplates)
		out[i].ProcessStringConstants = cloneStringMap(rules[i].ProcessStringConstants)
		out[i].Criteria = append([]WorkflowCriteriaItem(nil), rules[i].Criteria...)
		out[i].Steps = cloneFlowSteps(rules[i].Steps)
		out[i].StepTargets = cloneFlowStepTargets(rules[i].StepTargets)
		out[i].Paths = cloneFlowPaths(rules[i].Paths)
		out[i].FieldUpdates = append([]WorkflowFieldUpdate(nil), rules[i].FieldUpdates...)
		out[i].Actions = cloneFlowActions(rules[i].Actions)
		out[i].Branches = cloneFlowBranches(rules[i].Branches)
		out[i].RecordLookups = cloneFlowRecordLookups(rules[i].RecordLookups)
		out[i].RecordCreates = cloneFlowRecordCreates(rules[i].RecordCreates)
		out[i].RecordDeletes = cloneFlowRecordDeletes(rules[i].RecordDeletes)
		out[i].CustomErrors = cloneFlowCustomErrors(rules[i].CustomErrors)
		out[i].ApexPlugins = cloneFlowApexPlugins(rules[i].ApexPlugins)
		out[i].Variables = append([]FlowVariable(nil), rules[i].Variables...)
	}
	return out
}

func cloneFlowPaths(paths []FlowPath) []FlowPath {
	out := append([]FlowPath(nil), paths...)
	for i := range out {
		out[i].Steps = cloneFlowSteps(paths[i].Steps)
		out[i].StepTargets = cloneFlowStepTargets(paths[i].StepTargets)
	}
	return out
}

func cloneFlowStepTargets(targets map[string][]FlowStep) map[string][]FlowStep {
	if targets == nil {
		return nil
	}
	out := make(map[string][]FlowStep, len(targets))
	for name, steps := range targets {
		out[name] = cloneFlowSteps(steps)
	}
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
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
		out[i].QueriedFields = append([]string(nil), lookups[i].QueriedFields...)
		out[i].OutputAssignments = append([]FlowAssignment(nil), lookups[i].OutputAssignments...)
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

func cloneFlowCustomErrorMessages(messages []FlowCustomErrorMessage) []FlowCustomErrorMessage {
	out := append([]FlowCustomErrorMessage(nil), messages...)
	for i := range out {
		if messages[i].NumberConstants != nil {
			out[i].NumberConstants = make(map[string]string, len(messages[i].NumberConstants))
			for k, v := range messages[i].NumberConstants {
				out[i].NumberConstants[k] = v
			}
		}
	}
	return out
}

func cloneFlowCustomErrors(errors []FlowCustomError) []FlowCustomError {
	out := append([]FlowCustomError(nil), errors...)
	for i := range out {
		out[i].Messages = cloneFlowCustomErrorMessages(errors[i].Messages)
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
	out.Features = slices.Clone(o.Features)
	out.Metadata.FeatureParameters = append([]FeatureParameterMetadata(nil), o.Metadata.FeatureParameters...)
	out.Metadata.Flows = cloneFlowRules(o.Metadata.Flows)
	out.Metadata.FlowDeclarations = cloneFlowRules(o.Metadata.FlowDeclarations)
	if o.Objects != nil {
		out.Objects = make(map[string]ObjectState, len(o.Objects))
		for name, object := range o.Objects {
			out.Objects[name] = object.Clone()
		}
	}
	out.objectNameCache = newObjectNameCacheFrom(o.objectNameCache, o.Objects, out.Objects)
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

// CloneObjectsShallow returns an org copy whose Objects map is its own, with
// room for extra entries, while every object state, record map and definition
// is shared with the receiver. It is for read-only views that add objects of
// their own beside the receiver's. Every copied state is marked
// RecordsShared and IndexesShared, so a writer that goes through
// EnsureMutableObjectRecords clones the maps first; definitions must go
// through EnsureMutableObjectDefinition. The copy gets an object name cache of
// its own, as Clone does.
func (o OrgState) CloneObjectsShallow(extra int) OrgState {
	out := o
	out.Features = slices.Clone(o.Features)
	if o.Objects != nil || extra > 0 {
		out.Objects = make(map[string]ObjectState, len(o.Objects)+extra)
		for name, object := range o.Objects {
			object.RecordsShared, object.IndexesShared = true, true
			out.Objects[name] = object
		}
	}
	out.objectNameCache = newObjectNameCacheFrom(o.objectNameCache, o.Objects, out.Objects)
	return out
}

// CloneRuntime returns an org copy suitable for isolated test/runtime execution.
// Records, indexes, definitions, sequences, and transaction frames are isolated
// so parallel test setup and execution can infer runtime metadata without
// sharing mutable maps.
func (o OrgState) CloneRuntime() OrgState {
	cloneStats.cloneRuntime.Add(1)
	out := o
	out.Features = slices.Clone(o.Features)
	out.Metadata.FeatureParameters = append([]FeatureParameterMetadata(nil), o.Metadata.FeatureParameters...)
	out.Metadata.Flows = cloneFlowRules(o.Metadata.Flows)
	out.Metadata.FlowDeclarations = cloneFlowRules(o.Metadata.FlowDeclarations)
	if o.Objects != nil {
		out.Objects = make(map[string]ObjectState, len(o.Objects))
		for name, object := range o.Objects {
			out.Objects[name] = object.CloneRuntime()
		}
	}
	out.objectNameCache = newObjectNameCacheFrom(o.objectNameCache, o.Objects, out.Objects)
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
	out.Features = slices.Clone(o.Features)
	out.Metadata.FeatureParameters = append([]FeatureParameterMetadata(nil), o.Metadata.FeatureParameters...)
	out.Metadata.Flows = cloneFlowRules(o.Metadata.Flows)
	out.Metadata.FlowDeclarations = cloneFlowRules(o.Metadata.FlowDeclarations)
	if o.Objects != nil {
		out.Objects = make(map[string]ObjectState, len(o.Objects))
		for name, object := range o.Objects {
			out.Objects[name] = object.CloneRuntimeFrozenDefinition()
		}
	}
	out.objectNameCache = newObjectNameCacheFrom(o.objectNameCache, o.Objects, out.Objects)
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
	return isImmutableMetadataObjectName(objectName)
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
	out.Features = slices.Clone(o.Features)
	out.Metadata.FeatureParameters = append([]FeatureParameterMetadata(nil), o.Metadata.FeatureParameters...)
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
	out.objectNameCache = newObjectNameCacheFrom(o.objectNameCache, o.Objects, out.Objects)
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

// PackageFieldNamespaceAllowed reports whether an undefined namespaced custom
// field of namespace may be tolerated on a standard object: the project
// declared nothing (narrow is false) or declared the namespace with its
// source absent.
func PackageFieldNamespaceAllowed(narrow bool, sourceless []string, namespace string) bool {
	if !narrow {
		return true
	}
	for _, candidate := range sourceless {
		if strings.EqualFold(candidate, namespace) {
			return true
		}
	}
	return false
}
