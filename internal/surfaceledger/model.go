package surfaceledger

const SchemaVersion = 1

type SourceState string
type ShapeState string
type BehaviorState string
type EvidenceState string

const (
	SourceAbsent      SourceState = "absent"
	SourcePresent     SourceState = "present"
	SourceChanged     SourceState = "changed"
	SourceRemoved     SourceState = "removed"
	SourceDeprecated  SourceState = "deprecated"
	SourceUnavailable SourceState = "unavailable"
	SourceNotQueried  SourceState = "not-queried"
)

const (
	ShapeAbsent         ShapeState = "absent"
	ShapeTypeKnown      ShapeState = "type-known"
	ShapeSignatureKnown ShapeState = "signature-known"
	ShapeGenerated      ShapeState = "generated"
)

const (
	BehaviorNone        BehaviorState = "none"
	BehaviorPassive     BehaviorState = "passive"
	BehaviorUnsupported BehaviorState = "unsupported"
	BehaviorPartial     BehaviorState = "partial"
	BehaviorSupported   BehaviorState = "supported"
)

const (
	EvidenceNone             EvidenceState = "none"
	EvidenceDocs             EvidenceState = "docs"
	EvidenceFixture          EvidenceState = "fixture"
	EvidenceOracle           EvidenceState = "oracle"
	EvidenceFixtureAndOracle EvidenceState = "fixture-and-oracle"
	EvidenceCorpus           EvidenceState = "corpus"
)

const (
	ProductApex        = "apex"
	ProductTooling     = "tooling"
	ProductREST        = "rest"
	ProductVisualforce = "visualforce"
	ProductAura        = "lightning-aura"
	ProductLWC         = "lwc"
	ProductConnectAPI  = "connect-api"
	ProductDataRef     = "data-reference"
	ProductUnknown     = "unknown"
)

const (
	AreaRuntime = "runtime"
	AreaServer  = "server"
	AreaUI      = "ui"
	AreaData    = "data"
)

const (
	KindType      = "type"
	KindMethod    = "method"
	KindProperty  = "property"
	KindField     = "field"
	KindResource  = "resource"
	KindAttribute = "attribute"
	KindModule    = "module"
	KindGuide     = "guide"
)

const (
	GapMissingShape       = "missing-shape"
	GapMissingSignature   = "missing-signature"
	GapMissingBehavior    = "missing-behavior"
	GapMissingEvidence    = "missing-evidence"
	GapStaleGladeShape    = "stale-glade-shape"
	GapDocsOrgMismatch    = "docs-org-mismatch"
	GapSignatureChanged   = "signature-changed"
	GapPassiveServiceRisk = "passive-service-risk"
	GapAPIVersionChange   = "api-version-change"
)

const (
	BucketImplemented         = "implemented"
	BucketPartial             = "partial"
	BucketPassive             = "passive"
	BucketExplicitUnsupported = "explicitUnsupported"
	BucketGap                 = "gap"
	BucketFailure             = "failure"
)

type SurfaceLedger struct {
	SchemaVersion int                `json:"schemaVersion"`
	Rows          []SurfaceLedgerRow `json:"rows"`
	Summary       LedgerSummary      `json:"summary"`
}

type LedgerSummary struct {
	Implemented         int            `json:"implemented"`
	Partial             int            `json:"partial"`
	Passive             int            `json:"passive"`
	ExplicitUnsupported int            `json:"explicitUnsupported"`
	Gaps                map[string]int `json:"gaps"`
	Failures            map[string]int `json:"failures"`
	Total               int            `json:"total"`
}

type SurfaceLedgerRow struct {
	SurfaceID string `json:"surfaceId"`
	Product   string `json:"product"`
	Area      string `json:"area"`

	Namespace  string   `json:"namespace,omitempty"`
	TypeName   string   `json:"typeName,omitempty"`
	MemberName string   `json:"memberName,omitempty"`
	Resource   string   `json:"resource,omitempty"`
	FieldName  string   `json:"fieldName,omitempty"`
	Kind       string   `json:"kind"`
	Signature  string   `json:"signature,omitempty"`
	ReturnType string   `json:"returnType,omitempty"`
	Parameters []string `json:"parameters,omitempty"`

	Docs          SourceState   `json:"docs"`
	Org           SourceState   `json:"org"`
	GladeShape    ShapeState    `json:"gladeShape"`
	GladeBehavior BehaviorState `json:"gladeBehavior"`
	Evidence      EvidenceState `json:"evidence"`

	DocsSource string   `json:"docsSource,omitempty"`
	DocsTitle  string   `json:"docsTitle,omitempty"`
	APIVersion string   `json:"apiVersion,omitempty"`
	Sources    []string `json:"sources,omitempty"`

	SalesforceSurfaceFamily   string `json:"salesforceSurfaceFamily,omitempty"`
	GladeImplementationTarget string `json:"gladeImplementationTarget,omitempty"`
	ShapeSource               string `json:"shapeSource,omitempty"`
	BehaviorSource            string `json:"behaviorSource,omitempty"`
	ImplementationDecision    string `json:"implementationDecision,omitempty"`

	Owner    string `json:"owner,omitempty"`
	GapClass string `json:"gapClass,omitempty"`
	Bucket   string `json:"bucket,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

func RowFromDocs(row SurfaceLedgerRow) SurfaceLedgerRow {
	if row.Docs == "" {
		row.Docs = SourcePresent
	}
	if row.Org == "" {
		row.Org = SourceAbsent
	}
	return withDefaults(row)
}

func RowFromOrg(row SurfaceLedgerRow) SurfaceLedgerRow {
	if row.Org == "" {
		row.Org = SourcePresent
	}
	return withDefaults(row)
}

func RowFromGladeShape(row SurfaceLedgerRow) SurfaceLedgerRow {
	if row.GladeShape == "" || row.GladeShape == ShapeAbsent {
		if row.MemberName != "" || row.Signature != "" || len(row.Parameters) > 0 || row.Kind == KindMethod || row.Kind == KindProperty {
			row.GladeShape = ShapeSignatureKnown
		} else {
			row.GladeShape = ShapeTypeKnown
		}
	}
	return withDefaults(row)
}

func RowFromEvidence(row SurfaceLedgerRow) SurfaceLedgerRow {
	if row.Evidence == "" || row.Evidence == EvidenceNone {
		row.Evidence = EvidenceFixture
	}
	return withDefaults(row)
}

func withDefaults(row SurfaceLedgerRow) SurfaceLedgerRow {
	if row.Product == "" {
		row.Product = ProductUnknown
	}
	if row.Docs == "" {
		row.Docs = SourceAbsent
	}
	if row.Org == "" {
		row.Org = SourceAbsent
	}
	if row.GladeShape == "" {
		row.GladeShape = ShapeAbsent
	}
	if row.GladeBehavior == "" {
		row.GladeBehavior = BehaviorNone
	}
	if row.Evidence == "" {
		row.Evidence = EvidenceNone
	}
	if row.SalesforceSurfaceFamily == "" {
		row.SalesforceSurfaceFamily = surfaceFamilyForProduct(row.Product)
	}
	if row.GladeImplementationTarget == "" {
		row.GladeImplementationTarget = implementationTargetForRow(row)
	}
	if row.ShapeSource == "" && row.Docs == SourcePresent {
		row.ShapeSource = "reference"
	}
	if row.BehaviorSource == "" && row.Evidence != EvidenceNone {
		row.BehaviorSource = "fixture"
	}
	if row.ImplementationDecision == "" {
		row.ImplementationDecision = row.GladeImplementationTarget
	}
	return row
}

func surfaceFamilyForProduct(product string) string {
	switch product {
	case ProductApex:
		return "apex"
	case ProductREST:
		return "rest-api"
	case ProductTooling:
		return "tooling-api"
	case ProductVisualforce:
		return "visualforce"
	case ProductLWC:
		return "lwc"
	case ProductAura:
		return "aura"
	case ProductDataRef:
		return "data-reference"
	default:
		if product == "" {
			return ProductUnknown
		}
		return product
	}
}

func implementationTargetForRow(row SurfaceLedgerRow) string {
	switch row.Area {
	case AreaServer:
		return "server-or-explicit-unsupported"
	case AreaUI:
		return "ui-or-explicit-unsupported"
	case AreaRuntime:
		return "runtime"
	default:
		return "explicit-unsupported"
	}
}
