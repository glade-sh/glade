package projectscan

import (
	"bufio"
	"encoding/xml"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/automation"
	metadatapkg "github.com/open-aer/oaer/internal/metadata"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/resource"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/uicontroller"
	"github.com/open-aer/oaer/internal/visualforce"
)

type Report struct {
	Project     string       `json:"project"`
	Summary     Summary      `json:"summary"`
	Surfaces    []Surface    `json:"surfaces"`
	Findings    []Finding    `json:"findings"`
	TopBlockers []TopBlocker `json:"topBlockers"`
}

type Summary struct {
	FilesScanned         int `json:"filesScanned"`
	Findings             int `json:"findings"`
	TestBlockingFindings int `json:"testBlockingFindings"`
	Surfaces             int `json:"surfaces"`
}

type Surface struct {
	Capability          string    `json:"capability"`
	Title               string    `json:"title"`
	Area                string    `json:"area"`
	Stage               string    `json:"stage"`
	Status              string    `json:"status"`
	TestBlocking        bool      `json:"testBlocking"`
	Count               int       `json:"count"`
	AffectedFiles       int       `json:"affectedFiles"`
	MetadataTypes       []string  `json:"metadataTypes,omitempty"`
	SuggestedCapability string    `json:"suggestedCapability"`
	Examples            []Example `json:"examples,omitempty"`
}

type Example struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Symbol   string `json:"symbol,omitempty"`
	Evidence string `json:"evidence,omitempty"`
}

type Finding struct {
	Capability          string `json:"capability"`
	File                string `json:"file"`
	Line                int    `json:"line,omitempty"`
	MetadataType        string `json:"metadataType"`
	Stage               string `json:"stage"`
	Symbol              string `json:"symbol,omitempty"`
	Evidence            string `json:"evidence,omitempty"`
	SuggestedCapability string `json:"suggestedCapability"`
	TestBlocking        bool   `json:"testBlocking"`
}

type TopBlocker struct {
	Capability    string `json:"capability"`
	Title         string `json:"title"`
	Count         int    `json:"count"`
	AffectedFiles int    `json:"affectedFiles"`
}

type surfaceDef struct {
	capability          string
	title               string
	area                string
	stage               string
	status              string
	testBlocking        bool
	suggestedCapability string
}

var surfaceDefs = map[string]surfaceDef{
	"visualforce.controller-test": {
		capability:          "visualforce.controller-test",
		title:               "Visualforce page/controller test support",
		area:                "local-test-ui-controllers",
		stage:               "resolve",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "visualforce.controller-test",
	},
	"visualforce.component-test": {
		capability:          "visualforce.component-test",
		title:               "Visualforce component metadata",
		area:                "local-test-ui-controllers",
		stage:               "resolve",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "visualforce.component-test",
	},
	"aura.controller-test": {
		capability:          "aura.controller-test",
		title:               "Aura controller action discovery",
		area:                "local-test-ui-controllers",
		stage:               "resolve",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "aura.controller-test",
	},
	"lwc.controller-test": {
		capability:          "lwc.controller-test",
		title:               "LWC Apex import and wire discovery",
		area:                "local-test-ui-controllers",
		stage:               "resolve",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "lwc.controller-test",
	},
	"workflow.save-order": {
		capability:          "workflow.save-order",
		title:               "Workflow rule save-order side effects",
		area:                "declarative-automation",
		stage:               "execute",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "workflow.save-order",
	},
	"flow.save-order": {
		capability:          "flow.save-order",
		title:               "Flow and Process Builder save-order side effects",
		area:                "declarative-automation",
		stage:               "execute",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "flow.save-order",
	},
	"labels.localization": {
		capability:          "labels.localization",
		title:               "Custom label and translation resolution",
		area:                "metadata-localization",
		stage:               "resolve",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "labels.localization",
	},
	"email.templates": {
		capability:          "email.templates",
		title:               "Email template metadata and merge support",
		area:                "declarative-side-effects",
		stage:               "execute",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "email.templates",
	},
	"metadata.legacy-source": {
		capability:          "metadata.legacy-source",
		title:               "Legacy Metadata API source format loading",
		area:                "metadata-loading",
		stage:               "load",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "metadata.legacy-source",
	},
	"custommetadata.legacy-records": {
		capability:          "custommetadata.legacy-records",
		title:               "Legacy custom metadata records",
		area:                "metadata-loading",
		stage:               "load",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "custommetadata.legacy-records",
	},
	"metadata.apex-deploy": {
		capability:          "metadata.apex-deploy",
		title:               "Apex Metadata API deploy/mutation behavior",
		area:                "platform-apis",
		stage:               "execute",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "metadata.apex-deploy",
	},
	"site.community-context": {
		capability:          "site.community-context",
		title:               "Site, community, and network test context",
		area:                "platform-context",
		stage:               "resolve",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "site.community-context",
	},
	"platform.cache-connectapi": {
		capability:          "platform.cache-connectapi",
		title:               "Platform Cache and ConnectApi org settings",
		area:                "platform-apis",
		stage:               "execute",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "platform.cache-connectapi",
	},
	"platform.auth-context": {
		capability:          "platform.auth-context",
		title:               "Auth namespace and authentication context",
		area:                "platform-apis",
		stage:               "resolve",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "platform.auth-context",
	},
	"apex.callable-stub": {
		capability:          "apex.callable-stub",
		title:               "System.Callable and Stub API compatibility",
		area:                "platform-apis",
		stage:               "execute",
		status:              "partial",
		testBlocking:        false,
		suggestedCapability: "apex.callable-stub",
	},
	"endpoint.metadata": {
		capability:          "endpoint.metadata",
		title:               "Named credential and remote site metadata",
		area:                "callout-metadata",
		stage:               "resolve",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "endpoint.metadata",
	},
	"ui.presentation-metadata": {
		capability:          "ui.presentation-metadata",
		title:               "UI and org presentation metadata",
		area:                "metadata-loading",
		stage:               "load",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "ui.presentation-metadata",
	},
	"staticresources.urlfor": {
		capability:          "staticresources.urlfor",
		title:               "Static resources, content assets, and URLFOR",
		area:                "metadata-resources",
		stage:               "resolve",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "staticresources.urlfor",
	},
	"files.binary-content": {
		capability:          "files.binary-content",
		title:               "Files, attachments, documents, and binary content",
		area:                "data-side-effects",
		stage:               "execute",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "files.binary-content",
	},
}

type patternDef struct {
	capability   string
	metadataType string
	re           *regexp.Regexp
	symbolGroup  int
}

type scanContext struct {
	org          storage.OrgState
	metadata     storage.MetadataRegistry
	vf           visualforce.Index
	vfComponents map[string]bool
	pages        map[string]string
	uiApex       map[string]bool
	aura         map[string]bool
	types        map[string]typesys.TypeSymbol
	present      map[string]bool
	loadedFiles  map[string]bool
	automation   map[string]bool
	namespaces   map[string]bool
}

var textPatterns = []patternDef{
	{"visualforce.controller-test", "ApexClass", regexp.MustCompile(`\b(ApexPages\.|PageReference\b|Page\.[A-Za-z_][A-Za-z0-9_]*|StandardController\b|StandardSetController\b)`), 1},
	{"labels.localization", "ApexClass", regexp.MustCompile(`\b(?:System\.)?Label\.([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?)`), 1},
	{"metadata.apex-deploy", "ApexClass", regexp.MustCompile(`\b(Metadata\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"site.community-context", "ApexClass", regexp.MustCompile(`\b(Site\.|Network\.|Community__mdt\b)`), 1},
	{"platform.cache-connectapi", "ApexClass", regexp.MustCompile(`\b(Cache\.|ConnectApi\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"platform.auth-context", "ApexClass", regexp.MustCompile(`\b(Auth\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"apex.callable-stub", "ApexClass", regexp.MustCompile(`\b(System\.Callable|Callable\b|System\.StubProvider|Test\.createStub|handleMethodCall\b)`), 1},
	{"endpoint.metadata", "ApexClass", regexp.MustCompile(`callout:([A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"files.binary-content", "ApexClass", regexp.MustCompile(`\b(ContentVersion\b|ContentDocument\b|ContentDocumentLink\b|Attachment\b|Document\b)`), 1},
	{"custommetadata.legacy-records", "ApexClass", regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*__mdt)\b`), 1},
	{"lwc.controller-test", "LWCJavaScript", regexp.MustCompile(`@salesforce/apex/([A-Za-z_][A-Za-z0-9_.]*\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"lwc.controller-test", "LWCJavaScript", regexp.MustCompile(`\b@wire\s*\(\s*([A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"labels.localization", "LWCJavaScript", regexp.MustCompile(`@salesforce/label/([A-Za-z0-9_./]+)`), 1},
	{"staticresources.urlfor", "LWCJavaScript", regexp.MustCompile(`@salesforce/resourceUrl/([A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"ui.presentation-metadata", "LWCJavaScript", regexp.MustCompile(`@salesforce/schema/([A-Za-z0-9_./]+)|lightning/(navigation|uiRecordApi|uiObjectInfoApi)`), 1},
	{"staticresources.urlfor", "Visualforce", regexp.MustCompile(`\$Resource\.([A-Za-z_][A-Za-z0-9_]*)|URLFOR\s*\(\s*\$Resource\.([A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"site.community-context", "Visualforce", regexp.MustCompile(`(\$Site\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"labels.localization", "Visualforce", regexp.MustCompile(`(\$Label(?:\.[A-Za-z_][A-Za-z0-9_]*)+)`), 1},
	{"ui.presentation-metadata", "Visualforce", regexp.MustCompile(`(\$ObjectType(?:\.[A-Za-z_][A-Za-z0-9_]*)+|\$Component(?:\.[A-Za-z_][A-Za-z0-9_]*)*)`), 1},
	{"visualforce.controller-test", "Visualforce", regexp.MustCompile(`\b(controller|standardController|extensions|action|recordSetVar)=["']([^"']+)["']`), 2},
}

func Scan(root string) (Report, error) {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return Report{}, err
	}
	if !info.IsDir() {
		return Report{}, errors.New("project scan root must be a directory")
	}

	ctx := loadScanContext(absRoot)
	var findings []Finding
	filesScanned := 0
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != absRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldSkipFile(d.Name()) {
			return nil
		}
		filesScanned++
		rel := slashRel(absRoot, path)
		findings = append(findings, classifyByPath(rel, path, &ctx)...)
		if isTextScannable(rel) {
			lineFindings, err := scanTextFile(path, rel, &ctx)
			if err != nil {
				return err
			}
			findings = append(findings, lineFindings...)
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	report := buildReport(absRoot, filesScanned, findings)
	return report, nil
}

func loadScanContext(absRoot string) scanContext {
	ctx := scanContext{org: storage.NewOrgState()}
	proj, err := project.Load(absRoot)
	if err != nil {
		return ctx
	}
	ctx.org.Namespace = proj.Namespace
	sch, err := schema.LoadProject(proj)
	if err != nil {
		return ctx
	}
	typeIndex := typesys.Build(proj, sch)
	ctx.types = make(map[string]typesys.TypeSymbol, len(typeIndex.Types))
	for _, typ := range typeIndex.Types {
		ctx.types[strings.ToLower(typ.Name)] = typ
	}
	if ui, err := uicontroller.Build(proj, typeIndex); err == nil {
		ctx.uiApex = make(map[string]bool)
		for _, method := range ui.ApexMethods {
			if method.Resolved {
				ctx.uiApex[strings.ToLower(method.ClassName+"."+method.MethodName)] = true
				ctx.uiApex[strings.ToLower(stripDotNamespace(method.ClassName)+"."+method.MethodName)] = true
			}
		}
		ctx.aura = resolvedAuraFiles(ui, &ctx)
	}
	if metadata, err := resource.LoadProject(proj); err == nil {
		ctx.metadata = metadata
	}
	ctx.automation = resolvedAutomationFiles(proj)
	ctx.loadedFiles = make(map[string]bool)
	for _, path := range proj.ObjectFiles {
		ctx.loadedFiles[filepath.Clean(path)] = true
	}
	for _, path := range proj.CustomMetadataFiles {
		ctx.loadedFiles[filepath.Clean(path)] = true
	}
	var metadataIndex *metadatapkg.Index
	if idx, err := metadatapkg.LoadProject(proj); err == nil {
		metadataIndex = &idx
		ctx.present = make(map[string]bool)
		ctx.addPresentationAssetFiles(idx.Layouts)
		ctx.addPresentationAssetFiles(idx.CompactLayouts)
		ctx.addPresentationAssetFiles(idx.Tabs)
		ctx.addPresentationAssetFiles(idx.WebLinks)
		ctx.addPresentationAssetFiles(idx.QuickActions)
		ctx.addPresentationAssetFiles(idx.GlobalValueSets)
		ctx.addPresentationAssetFiles(idx.StandardValueSets)
		ctx.addPresentationAssetFiles(idx.FlexiPages)
		ctx.addPresentationAssetFiles(idx.Applications)
		for _, profile := range idx.Profiles {
			ctx.present[filepath.Clean(profile.File)] = true
		}
		for _, permissionSet := range idx.PermissionSets {
			ctx.present[filepath.Clean(permissionSet.File)] = true
		}
	}
	ctx.namespaces = namespaceAliases(proj, sch, metadataIndex)
	ctx.vf = visualforce.LoadProjectBestEffort(proj)
	ctx.vfComponents = make(map[string]bool, len(proj.VisualforceComponentFiles))
	for _, path := range proj.VisualforceComponentFiles {
		ctx.vfComponents[filepath.Clean(path)] = true
	}
	ctx.pages = make(map[string]string, len(proj.VisualforcePageFiles))
	for _, path := range proj.VisualforcePageFiles {
		name := baseNoExt(path)
		ctx.pages[strings.ToLower(name)] = name
	}
	for _, object := range sch.Objects {
		definition := storage.ObjectDefinition{
			APIName:     object.Name,
			Label:       object.Label,
			PluralLabel: object.PluralLabel,
			Fields:      make(map[string]storage.Field, len(object.Fields)),
		}
		for _, field := range object.Fields {
			definition.Fields[field.Name] = storage.Field{
				APIName:          field.Name,
				Label:            field.Label,
				Type:             storage.FieldAny,
				ReferenceTo:      append([]string(nil), field.ReferenceTo...),
				RelationshipName: field.RelationshipName,
			}
		}
		storage.EnsureStandardObjectFields(&definition)
		ctx.org.Objects[definition.APIName] = storage.ObjectState{Definition: definition}
	}
	if metadataIndex != nil {
		ctx.addPresentationFields(*metadataIndex, proj)
	}
	return ctx
}

func (ctx *scanContext) addPresentationAssetFiles(assets []metadatapkg.NamedAsset) {
	for _, asset := range assets {
		ctx.present[filepath.Clean(asset.File)] = true
	}
}

func (ctx *scanContext) addPresentationFields(idx metadatapkg.Index, proj project.Project) {
	add := func(objectName, fieldName string) {
		objectName = strings.TrimSpace(objectName)
		fieldName = strings.TrimSpace(fieldName)
		if objectName == "" || fieldName == "" || strings.Contains(fieldName, ".") {
			return
		}
		resolved, ok := storage.ResolveObjectName(ctx.org, objectName)
		if !ok {
			return
		}
		state := ctx.org.Objects[resolved]
		if state.Definition.Fields == nil {
			state.Definition.Fields = make(map[string]storage.Field)
		}
		if _, ok := state.Definition.Fields[fieldName]; !ok {
			state.Definition.Fields[fieldName] = storage.Field{APIName: fieldName, Type: storage.FieldAny}
			ctx.org.Objects[resolved] = state
		}
	}
	for _, fieldSet := range idx.FieldSets {
		for _, member := range fieldSet.Fields {
			add(fieldSet.ObjectName, member.Field)
		}
	}
	for _, path := range proj.CompactLayoutFiles {
		objectName := objectNameFromObjectMetadataPath(path, "compactLayouts")
		for _, fieldName := range loadPresentationFields(path) {
			add(objectName, fieldName)
		}
	}
	for _, path := range proj.LayoutFiles {
		objectName := objectNameFromLayoutPath(path)
		for _, fieldName := range loadPresentationFields(path) {
			add(objectName, fieldName)
		}
	}
	for _, path := range proj.QuickActionFiles {
		objectName := objectNameFromQuickActionPath(path)
		fields, target := loadQuickActionPresentationFields(path)
		if target != "" {
			objectName = target
		}
		for _, fieldName := range fields {
			add(objectName, fieldName)
		}
	}
}

func resolvedAutomationFiles(proj project.Project) map[string]bool {
	resolved := make(map[string]bool)
	idx, err := automation.LoadProject(proj)
	if err != nil {
		return resolved
	}
	diagnosticFiles := make(map[string]bool)
	for _, diag := range idx.Diagnostics {
		if diag.File != "" {
			diagnosticFiles[filepath.Clean(diag.File)] = true
		}
	}
	for _, workflow := range idx.Workflows {
		clean := filepath.Clean(workflow.File)
		resolved[clean] = !diagnosticFiles[clean]
	}
	for _, flow := range idx.Flows {
		clean := filepath.Clean(flow.File)
		resolved[clean] = !diagnosticFiles[clean]
	}
	return resolved
}

func (ctx *scanContext) resolvesAutomationFile(path string) bool {
	if ctx == nil || ctx.automation == nil {
		return false
	}
	return ctx.automation[filepath.Clean(path)]
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".sfdx", ".sf", ".claude", "node_modules", ".idea", ".vscode", "__tests__":
		return true
	default:
		return false
	}
}

func shouldSkipFile(name string) bool {
	return name == ".DS_Store"
}

func slashRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func classifyByPath(rel, path string, ctx *scanContext) []Finding {
	lower := strings.ToLower(rel)
	var findings []Finding
	add := func(capability, metadataType, symbol string) {
		findings = append(findings, makeFinding(capability, rel, 0, metadataType, symbol, "metadata file"))
	}

	switch {
	case strings.HasSuffix(lower, ".component"):
		if ctx != nil && ctx.resolvesVisualforceComponentMetadata(path) {
			return findings
		}
		add("visualforce.component-test", "VisualforceComponent", baseNoExt(path))
	case strings.Contains(lower, "/aura/"):
		if isPassiveAuraArtifact(lower) {
			return findings
		}
		if ctx != nil && ctx.resolvesAuraFile(path) {
			return findings
		}
		add("aura.controller-test", "AuraBundle", auraOrLWCBundle(rel, "aura"))
	case strings.HasSuffix(lower, ".workflow-meta.xml"), strings.HasSuffix(lower, ".workflow"):
		if ctx != nil && ctx.resolvesAutomationFile(path) {
			return findings
		}
		add("workflow.save-order", "Workflow", baseNoExt(path))
	case strings.HasSuffix(lower, ".flow-meta.xml"), strings.HasSuffix(lower, ".flow"):
		if ctx != nil && ctx.resolvesAutomationFile(path) {
			return findings
		}
		add("flow.save-order", "Flow", baseNoExt(path))
	case strings.HasSuffix(lower, ".email"), strings.HasSuffix(lower, ".email-meta.xml"):
		if ctx != nil && ctx.resolvesEmailTemplate(baseNoExt(path), path) {
			return findings
		}
		add("email.templates", "EmailTemplate", baseNoExt(path))
	case strings.HasSuffix(lower, ".object"):
		if ctx != nil && ctx.loadedFiles[filepath.Clean(path)] {
			return findings
		}
		add("metadata.legacy-source", "LegacyObject", baseNoExt(path))
	case hasAnySuffix(lower, ".layout", ".layout-meta.xml", ".profile", ".profile-meta.xml", ".permissionset", ".permissionset-meta.xml", ".tab", ".tab-meta.xml", ".weblink", ".weblink-meta.xml", ".quickaction-meta.xml", ".globalvalueset-meta.xml", ".standardvalueset-meta.xml", ".flexipage", ".flexipage-meta.xml", ".application", ".app-meta.xml"):
		if ctx != nil && ctx.present[filepath.Clean(path)] {
			return findings
		}
		add("ui.presentation-metadata", "UIPresentationMetadata", baseNoExt(path))
	}
	return findings
}

func hasAnySuffix(value string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func isPassiveAuraArtifact(path string) bool {
	return hasAnySuffix(path, ".intf", ".intf-meta.xml", ".tokens", ".tokens-meta.xml")
}

func isCustomMetadataPath(path string) bool {
	return strings.Contains(path, "/custommetadata/")
}

func baseNoExt(path string) string {
	base := filepath.Base(path)
	for _, suffix := range []string{".object-meta.xml", ".field-meta.xml", ".recordType-meta.xml", ".validationRule-meta.xml", ".workflow-meta.xml", ".flow-meta.xml", ".labels-meta.xml", ".email-meta.xml", ".namedCredential-meta.xml", ".remoteSite-meta.xml", ".staticResource-meta.xml", ".asset-meta.xml", ".layout-meta.xml", ".profile-meta.xml", ".permissionset-meta.xml", ".tab-meta.xml", ".webLink-meta.xml", ".quickAction-meta.xml", ".globalValueSet-meta.xml", ".standardValueSet-meta.xml", ".flexipage-meta.xml", ".app-meta.xml"} {
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix)
		}
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func objectNameFromObjectMetadataPath(path, container string) string {
	dir := filepath.Dir(filepath.Dir(path))
	if filepath.Base(filepath.Dir(path)) != container {
		return ""
	}
	return filepath.Base(dir)
}

func objectNameFromLayoutPath(path string) string {
	name := baseNoExt(path)
	if dash := strings.Index(name, "-"); dash > 0 {
		return name[:dash]
	}
	return ""
}

func objectNameFromQuickActionPath(path string) string {
	name := baseNoExt(path)
	if dot := strings.Index(name, "."); dot > 0 {
		return name[:dot]
	}
	return ""
}

func loadPresentationFields(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return trimmedNonEmpty(xmlElementTexts(data, "fields"))
}

func loadQuickActionPresentationFields(path string) ([]string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ""
	}
	fields := trimmedNonEmpty(xmlElementTexts(data, "field"))
	targets := trimmedNonEmpty(xmlElementTexts(data, "targetObject"))
	if len(targets) == 0 {
		return fields, ""
	}
	return fields, targets[0]
}

func xmlElementTexts(data []byte, local string) []string {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	var values []string
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != local {
			continue
		}
		var value string
		if err := decoder.DecodeElement(&value, &start); err != nil {
			continue
		}
		values = append(values, value)
	}
	return values
}

func trimmedNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[strings.ToLower(value)] {
			continue
		}
		seen[strings.ToLower(value)] = true
		out = append(out, value)
	}
	return out
}

func auraOrLWCBundle(rel, marker string) string {
	parts := strings.Split(rel, "/")
	for i := 0; i+1 < len(parts); i++ {
		if strings.EqualFold(parts[i], marker) {
			return parts[i+1]
		}
	}
	return ""
}

func isTextScannable(rel string) bool {
	lower := strings.ToLower(rel)
	return hasAnySuffix(lower, ".cls", ".trigger", ".page", ".component", ".cmp", ".app", ".evt", ".design", ".js", ".html", ".xml")
}

func scanTextFile(path, rel string, ctx *scanContext) ([]Finding, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	metadataType := metadataTypeForText(rel)
	var findings []Finding
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "//") || strings.HasPrefix(trimmedLine, "*") {
			continue
		}
		if beforeComment, _, ok := strings.Cut(line, "//"); ok {
			line = beforeComment
		}
		for _, pattern := range textPatterns {
			if pattern.metadataType != metadataType && !(metadataType == "ApexClass" && pattern.metadataType == "ApexClass") {
				continue
			}
			scanLine := lineForPattern(line, pattern)
			matches := pattern.re.FindAllStringSubmatch(scanLine, -1)
			for _, match := range matches {
				symbol := patternSymbol(pattern, match)
				if ctx != nil && ctx.resolvesFinding(pattern.capability, symbol, rel) {
					continue
				}
				if suppressSupportedFinding(pattern.capability, symbol, line) {
					continue
				}
				findings = append(findings, makeFinding(pattern.capability, rel, lineNo, metadataType, symbol, strings.TrimSpace(line)))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return findings, nil
}

func lineForPattern(line string, pattern patternDef) string {
	if pattern.capability == "custommetadata.legacy-records" && pattern.metadataType == "ApexClass" {
		return stripApexCommentsAndStrings(line)
	}
	return line
}

func stripApexCommentsAndStrings(line string) string {
	if strings.HasPrefix(strings.TrimSpace(line), "*") {
		return ""
	}
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = line[:idx]
	}
	if idx := strings.Index(line, "/*"); idx >= 0 {
		line = line[:idx]
	}
	var b strings.Builder
	inString := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inString {
			if ch == '\\' && i+1 < len(line) {
				i++
				continue
			}
			if ch == '\'' {
				inString = false
			}
			b.WriteByte(' ')
			continue
		}
		if ch == '\'' {
			inString = true
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func patternSymbol(pattern patternDef, match []string) string {
	if pattern.symbolGroup > 0 && pattern.symbolGroup < len(match) {
		if symbol := strings.TrimSpace(match[pattern.symbolGroup]); symbol != "" {
			return symbol
		}
	}
	for i := 1; i < len(match); i++ {
		if symbol := strings.TrimSpace(match[i]); symbol != "" {
			return symbol
		}
	}
	if len(match) > 0 {
		return strings.TrimSpace(match[0])
	}
	return ""
}

func (ctx *scanContext) resolvesFinding(capability, symbol, rel string) bool {
	switch capability {
	case "ui.presentation-metadata":
		if isRecognizedLightningClientModule(symbol) {
			return true
		}
		if strings.HasPrefix(strings.TrimSpace(symbol), "$Component") {
			return true
		}
		if schemaRef, ok := schemaReferenceSymbol(symbol); ok {
			return ctx.resolvesSchemaReference(schemaRef)
		}
		if ctx.resolvesSchemaReference(symbol) {
			return true
		}
	case "labels.localization":
		if namespace, label, ok := labelReferenceSymbol(symbol); ok {
			return ctx.resolvesLabel(namespace, label)
		}
	case "staticresources.urlfor":
		return ctx.resolvesResource(symbol)
	case "endpoint.metadata":
		return ctx.resolvesEndpoint(symbol)
	case "custommetadata.legacy-records":
		return ctx.resolvesCustomMetadataObject(symbol)
	case "visualforce.controller-test":
		return ctx.resolvesVisualforceControllerReference(symbol, rel)
	case "aura.controller-test":
		return ctx.resolvesAuraControllerReference(symbol)
	case "lwc.controller-test":
		return ctx.resolvesLWCControllerReference(symbol)
	}
	return false
}

func isRecognizedLightningClientModule(symbol string) bool {
	switch strings.TrimSpace(symbol) {
	case "navigation", "uiRecordApi", "uiObjectInfoApi":
		return true
	default:
		return false
	}
}

func (ctx *scanContext) resolvesLWCControllerReference(symbol string) bool {
	symbol = strings.TrimSpace(symbol)
	if !strings.Contains(symbol, ".") {
		return false
	}
	if ctx.uiApex[strings.ToLower(symbol)] {
		return true
	}
	lastDot := strings.LastIndex(symbol, ".")
	if lastDot > 0 {
		className := symbol[:lastDot]
		methodName := symbol[lastDot+1:]
		return ctx.uiApex[strings.ToLower(stripDotNamespace(className)+"."+methodName)]
	}
	return false
}

func (ctx *scanContext) resolvesAuraFile(path string) bool {
	if ctx == nil || ctx.aura == nil {
		return false
	}
	cleanPath := filepath.Clean(path)
	if resolved, ok := ctx.aura[cleanPath]; ok {
		return resolved
	}
	resolved, ok := ctx.aura[filepath.Dir(cleanPath)]
	return ok && resolved
}

func (ctx *scanContext) resolvesAuraControllerReference(symbol string) bool {
	return ctx.resolvesApexType(symbol)
}

func (ctx *scanContext) resolvesVisualforceControllerReference(symbol, rel string) bool {
	symbol = strings.TrimSpace(symbol)
	switch symbol {
	case "ApexPages.", "PageReference", "StandardController", "StandardSetController":
		return true
	}
	if strings.HasPrefix(symbol, "Page.") {
		pageName := strings.TrimPrefix(symbol, "Page.")
		if ctx.vf.HasPageReference(pageName) {
			return true
		}
		_, ok := ctx.pages[strings.ToLower(pageName)]
		return ok
	}
	if ctx.resolvesApexType(symbol) {
		return true
	}
	if _, ok := ctx.objectDefinition(symbol); ok {
		return true
	}
	if ctx.resolvesVisualforceActionReference(symbol, rel) {
		return true
	}
	return false
}

func (ctx *scanContext) resolvesVisualforceActionReference(symbol, rel string) bool {
	expr, ok := visualforceActionExpression(symbol)
	if !ok {
		return false
	}
	name := baseNoExt(rel)
	if strings.HasSuffix(strings.ToLower(rel), ".component") {
		if component, ok := ctx.vf.Component(name); ok {
			if visualforceComponentHasAttribute(component, expr) {
				return true
			}
			return ctx.visualforceTypesHaveMethod([]string{component.Controller}, component.Extensions, expr)
		}
	}
	if strings.HasSuffix(strings.ToLower(rel), ".page") {
		if page, ok := ctx.vf.Page(name); ok {
			return ctx.visualforceTypesHaveMethod([]string{page.Controller}, page.Extensions, expr)
		}
	}
	return false
}

func visualforceActionExpression(symbol string) (string, bool) {
	symbol = strings.TrimSpace(symbol)
	if !strings.HasPrefix(symbol, "{!") || !strings.HasSuffix(symbol, "}") {
		return "", false
	}
	expr := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(symbol, "{!"), "}"))
	if expr == "" || strings.ContainsAny(expr, " ()+-*/?:=<>!&|,") {
		return "", false
	}
	return expr, true
}

func visualforceComponentHasAttribute(component visualforce.Component, expr string) bool {
	if strings.Contains(expr, ".") {
		return false
	}
	for _, attr := range component.Attributes {
		if strings.EqualFold(attr.Name, expr) {
			return true
		}
	}
	return false
}

func (ctx *scanContext) resolvesVisualforceComponentMetadata(path string) bool {
	if ctx == nil {
		return false
	}
	cleanPath := filepath.Clean(path)
	name := baseNoExt(path)
	component, ok := ctx.vf.Component(name)
	if !ok {
		return false
	}
	if filepath.Clean(component.File) != cleanPath {
		return false
	}
	if ctx.vfComponents != nil && !ctx.vfComponents[cleanPath] {
		return false
	}
	return true
}

func (ctx *scanContext) visualforceTypesHaveMethod(primary, extensions []string, expr string) bool {
	methodName := expr
	if strings.Contains(methodName, ".") {
		parts := strings.Split(methodName, ".")
		methodName = parts[len(parts)-1]
	}
	for _, typeName := range append(primary, extensions...) {
		if ctx.apexTypeHasMethod(typeName, methodName) {
			return true
		}
	}
	return false
}

func (ctx *scanContext) resolvesApexType(symbol string) bool {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return false
	}
	if _, ok := ctx.types[strings.ToLower(symbol)]; ok {
		return true
	}
	if stripped := stripDotNamespace(symbol); stripped != symbol {
		_, ok := ctx.types[strings.ToLower(stripped)]
		return ok
	}
	stripped := stripAnyNamespaceToken(symbol)
	if stripped != symbol {
		_, ok := ctx.types[strings.ToLower(stripped)]
		return ok
	}
	return false
}

func (ctx *scanContext) apexTypeHasMethod(typeName, methodName string) bool {
	typ, ok := ctx.lookupApexType(typeName)
	if !ok {
		return false
	}
	for _, member := range typ.Members {
		if member.Kind == "method" && strings.EqualFold(member.Name, methodName) {
			return true
		}
	}
	return false
}

func (ctx *scanContext) lookupApexType(typeName string) (typesys.TypeSymbol, bool) {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typesys.TypeSymbol{}, false
	}
	if typ, ok := ctx.types[strings.ToLower(typeName)]; ok {
		return typ, true
	}
	if stripped := stripDotNamespace(typeName); stripped != typeName {
		typ, ok := ctx.types[strings.ToLower(stripped)]
		return typ, ok
	}
	stripped := stripAnyNamespaceToken(typeName)
	if stripped != typeName {
		typ, ok := ctx.types[strings.ToLower(stripped)]
		return typ, ok
	}
	return typesys.TypeSymbol{}, false
}

func (ctx *scanContext) resolvesResource(symbol string) bool {
	name := strings.TrimSpace(symbol)
	if name == "" {
		return false
	}
	_, ok := resource.URLForStaticResource(ctx.metadata, name, "")
	return ok
}

func (ctx *scanContext) resolvesEndpoint(symbol string) bool {
	name := strings.TrimSpace(symbol)
	if name == "" {
		return false
	}
	_, ok := resource.ResolveEndpoint(ctx.metadata, "callout:"+name)
	return ok
}

func (ctx *scanContext) resolvesEmailTemplate(symbol, path string) bool {
	symbol = strings.TrimSpace(symbol)
	cleanPath := filepath.Clean(path)
	for _, template := range ctx.metadata.EmailTemplates {
		if filepath.Clean(template.File) == cleanPath || filepath.Clean(template.MetadataPath) == cleanPath {
			return true
		}
		for _, candidate := range []string{template.DeveloperName, template.Name} {
			if strings.EqualFold(candidate, symbol) {
				return true
			}
			if slash := strings.LastIndex(candidate, "/"); slash >= 0 && strings.EqualFold(candidate[slash+1:], symbol) {
				return true
			}
		}
	}
	return false
}

func labelReferenceSymbol(symbol string) (string, string, bool) {
	symbol = strings.TrimSpace(symbol)
	symbol = strings.TrimPrefix(symbol, "System.")
	symbol = strings.TrimPrefix(symbol, "Label.")
	symbol = strings.TrimPrefix(symbol, "$Label.")
	symbol = strings.TrimPrefix(symbol, "@salesforce/label/")
	symbol = strings.ReplaceAll(symbol, "/", ".")
	if symbol == "" {
		return "", "", false
	}
	parts := strings.Split(symbol, ".")
	if len(parts) > 1 && isLabelStringMethod(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	switch len(parts) {
	case 1:
		return "", parts[0], true
	default:
		namespace := parts[len(parts)-2]
		label := parts[len(parts)-1]
		if strings.EqualFold(namespace, "c") {
			namespace = ""
		}
		return namespace, label, label != ""
	}
}

func (ctx *scanContext) resolvesLabel(namespace, label string) bool {
	if _, found := resource.LookupLabel(ctx.metadata, namespace, label); found {
		return true
	}
	if namespace == "" || !ctx.namespaces[strings.ToLower(namespace)] {
		return false
	}
	if ctx.org.Namespace != "" {
		if _, found := resource.LookupLabel(ctx.metadata, ctx.org.Namespace, label); found {
			return true
		}
	}
	_, found := resource.LookupLabel(ctx.metadata, "", label)
	return found
}

func namespaceAliases(proj project.Project, sch schema.Schema, idx *metadatapkg.Index) map[string]bool {
	aliases := make(map[string]bool)
	add := func(name string) {
		token := namespaceToken(name)
		if token != "" {
			aliases[strings.ToLower(token)] = true
		}
	}
	add(proj.Namespace)
	for _, object := range sch.Objects {
		add(object.Name)
		for _, field := range object.Fields {
			add(field.Name)
			add(field.RelationshipName)
			for _, target := range field.ReferenceTo {
				add(target)
			}
		}
	}
	if idx != nil {
		for _, fieldSet := range idx.FieldSets {
			add(fieldSet.ObjectName)
			for _, field := range fieldSet.Fields {
				add(field.Field)
			}
		}
		for _, profile := range idx.Profiles {
			for _, permission := range profile.ObjectPermissions {
				add(permission.Object)
			}
			for _, permission := range profile.FieldPermissions {
				add(permission.Field)
			}
		}
		for _, permissionSet := range idx.PermissionSets {
			for _, permission := range permissionSet.ObjectPermissions {
				add(permission.Object)
			}
			for _, permission := range permissionSet.FieldPermissions {
				add(permission.Field)
			}
		}
	}
	if len(aliases) == 0 {
		return nil
	}
	return aliases
}

func namespaceToken(name string) string {
	name = strings.TrimSpace(name)
	idx := strings.Index(name, "__")
	if idx <= 0 {
		return ""
	}
	token := name[:idx]
	if strings.Contains(token, ".") {
		token = token[strings.LastIndex(token, ".")+1:]
	}
	return token
}

func isLabelStringMethod(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "abbreviate", "capitalize", "center", "contains", "containsany", "containsignorecase",
		"containsnone", "deletewhitespace", "difference", "endswith", "equals", "equalsignorecase",
		"escapeecmascript", "escapehtml3", "escapehtml4", "escapesinglequotes", "escapeunicode",
		"indexof", "isempty", "isblank", "isnotblank", "isnotempty", "join", "left", "leftpad",
		"length", "mid", "normalize", "overlay", "remove", "removeend", "removeendignorecase",
		"removestart", "removestartignorecase", "repeat", "replace", "replaceall", "replacefirst",
		"reverse", "right", "rightpad", "split", "startswith", "substring", "substringafter",
		"substringafterlast", "substringbefore", "substringbeforelast", "swapcase", "tolowercase",
		"touppercase", "trim", "unescapeecmascript", "unescapehtml3", "unescapehtml4", "uncapitalize":
		return true
	default:
		return false
	}
}

func schemaReferenceSymbol(symbol string) (string, bool) {
	symbol = strings.TrimSpace(symbol)
	if strings.HasPrefix(symbol, "@salesforce/schema/") {
		return strings.TrimPrefix(symbol, "@salesforce/schema/"), true
	}
	if strings.HasPrefix(symbol, "$ObjectType.") {
		return strings.TrimPrefix(symbol, "$ObjectType."), true
	}
	if strings.Contains(symbol, ".") && !strings.HasPrefix(symbol, "lightning/") {
		return symbol, true
	}
	return "", false
}

func (ctx *scanContext) resolvesCustomMetadataObject(symbol string) bool {
	objectName := strings.TrimSpace(symbol)
	if objectName == "" || !strings.HasSuffix(objectName, "__mdt") {
		return false
	}
	if _, ok := storage.ResolveObjectName(ctx.org, objectName); ok {
		return true
	}
	stripped := stripAnyNamespaceToken(objectName)
	if stripped != objectName {
		_, ok := storage.ResolveObjectName(ctx.org, stripped)
		return ok
	}
	return false
}

func (ctx *scanContext) resolvesSchemaReference(ref string) bool {
	parts := schemaReferenceTokens(ref)
	if len(parts) == 0 {
		return false
	}
	definition, ok := ctx.objectDefinition(parts[0])
	if !ok {
		return false
	}
	return ctx.resolvesSchemaPath(definition, parts[1:])
}

func schemaReferenceTokens(ref string) []string {
	raw := strings.Split(strings.ReplaceAll(strings.TrimSpace(ref), "/", "."), ".")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func (ctx *scanContext) resolvesSchemaPath(definition storage.ObjectDefinition, parts []string) bool {
	if len(parts) == 0 {
		return true
	}
	if strings.EqualFold(parts[0], "Fields") {
		if len(parts) == 1 {
			return true
		}
		return ctx.resolvesSchemaFieldPath(definition, parts[1], parts[2:])
	}
	if isTerminalSchemaProperty(parts[0]) {
		return true
	}
	return ctx.resolvesSchemaFieldPath(definition, parts[0], parts[1:])
}

func (ctx *scanContext) resolvesSchemaFieldPath(definition storage.ObjectDefinition, fieldOrRelationship string, rest []string) bool {
	if field, ok := ctx.resolveSchemaField(definition, fieldOrRelationship); ok {
		if len(rest) == 0 || isTerminalSchemaProperty(rest[0]) {
			return true
		}
		return ctx.resolvesReferenceTargets(field.ReferenceTo, rest)
	}
	for _, field := range definition.Fields {
		if !fieldRelationshipTokenMatches(field, fieldOrRelationship) {
			continue
		}
		if len(rest) == 0 {
			return true
		}
		if ctx.resolvesReferenceTargets(field.ReferenceTo, rest) {
			return true
		}
		if len(field.ReferenceTo) == 0 {
			if related, ok := ctx.objectDefinition(fieldOrRelationship); ok && ctx.resolvesSchemaPath(related, rest) {
				return true
			}
		}
	}
	for _, relation := range definition.Relations {
		if !sameSchemaToken(relation.ParentRelationship, fieldOrRelationship) {
			continue
		}
		if len(rest) == 0 {
			return true
		}
		if ctx.resolvesReferenceTargets(relation.ParentObjects, rest) {
			return true
		}
	}
	return false
}

func fieldRelationshipTokenMatches(field storage.Field, token string) bool {
	if sameSchemaToken(field.RelationshipName, token) {
		return true
	}
	apiName := strings.TrimSpace(field.APIName)
	if strings.HasSuffix(strings.ToLower(apiName), "id") && len(apiName) > 2 {
		return sameSchemaToken(apiName[:len(apiName)-2], token)
	}
	return false
}

func (ctx *scanContext) resolveSchemaField(definition storage.ObjectDefinition, fieldName string) (storage.Field, bool) {
	resolved, ok := storage.ResolveFieldName(definition, ctx.org.Namespace, fieldName)
	if !ok {
		stripped := stripAnyNamespaceToken(fieldName)
		if stripped == fieldName {
			return storage.Field{}, false
		}
		resolved, ok = storage.ResolveFieldName(definition, ctx.org.Namespace, stripped)
	}
	if !ok {
		if strings.EqualFold(fieldName, "Id") || strings.EqualFold(fieldName, "Name") {
			return storage.Field{APIName: fieldName}, true
		}
		return storage.Field{}, false
	}
	field, ok := definition.Fields[resolved]
	if !ok && strings.EqualFold(resolved, "Id") {
		return storage.Field{APIName: "Id"}, true
	}
	return field, ok
}

func (ctx *scanContext) resolvesReferenceTargets(targets []string, rest []string) bool {
	if len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		definition, ok := ctx.objectDefinition(target)
		if ok && ctx.resolvesSchemaPath(definition, rest) {
			return true
		}
	}
	return false
}

func sameSchemaToken(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if strings.EqualFold(a, b) {
		return true
	}
	return strings.EqualFold(stripAnyNamespaceToken(a), stripAnyNamespaceToken(b))
}

func schemaReferenceParts(ref string) (string, string) {
	parts := schemaReferenceTokens(ref)
	if len(parts) == 0 {
		return "", ""
	}
	objectName := parts[0]
	if len(parts) == 1 {
		return objectName, ""
	}
	if len(parts) >= 2 && strings.EqualFold(parts[1], "Fields") {
		if len(parts) >= 3 {
			return objectName, parts[2]
		}
		return objectName, ""
	}
	if len(parts) >= 2 && isTerminalSchemaProperty(parts[1]) {
		return objectName, ""
	}
	return objectName, parts[1]
}

func isTerminalSchemaProperty(part string) bool {
	switch strings.ToLower(part) {
	case "bytelength", "calculatedformula", "controller", "defaultvalue", "digits", "inlinehelptext",
		"label", "labelplural", "length", "localname", "name", "precision", "relationshipname",
		"scale", "soaptype", "type", "keyprefix":
		return true
	default:
		return false
	}
}

func (ctx *scanContext) objectDefinition(objectName string) (storage.ObjectDefinition, bool) {
	if resolved, ok := storage.ResolveObjectName(ctx.org, objectName); ok {
		return ctx.org.Objects[resolved].Definition, true
	}
	if stripped := stripAnyNamespaceToken(objectName); stripped != objectName {
		if resolved, ok := storage.ResolveObjectName(ctx.org, stripped); ok {
			return ctx.org.Objects[resolved].Definition, true
		}
	}
	if !storage.IsKnownStandardObject(objectName) {
		return storage.ObjectDefinition{}, false
	}
	storage.EnsureStandardObject(&ctx.org, objectName)
	return ctx.org.Objects[objectName].Definition, true
}

func stripAnyNamespaceToken(name string) string {
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	if first <= 0 || first >= last {
		return name
	}
	return name[first+2:]
}

func stripDotNamespace(name string) string {
	if dot := strings.LastIndex(name, "."); dot >= 0 && dot < len(name)-1 {
		return name[dot+1:]
	}
	return name
}

func resolvedAuraFiles(ui uicontroller.Index, ctx *scanContext) map[string]bool {
	resolved := make(map[string]bool)
	for _, bundle := range ui.AuraBundles {
		bundleResolved := true
		for _, controller := range bundle.ControllerReferences {
			if !ctx.resolvesAuraControllerReference(controller.Name) {
				bundleResolved = false
				break
			}
		}
		if bundleResolved {
			for _, action := range bundle.ActionReferences {
				if action.ClassName == "" || !action.Resolved {
					bundleResolved = false
					break
				}
			}
		}
		for _, file := range bundle.Files {
			resolved[filepath.Clean(file.Path)] = bundleResolved
		}
		resolved[filepath.Clean(bundle.Dir)] = bundleResolved
	}
	return resolved
}

func suppressSupportedFinding(capability, symbol, evidence string) bool {
	switch capability {
	case "files.binary-content":
		return supportedFileSymbol(symbol, evidence)
	case "site.community-context":
		return supportedSiteCommunitySymbol(symbol, evidence)
	case "platform.cache-connectapi":
		return supportedCacheConnectAPISymbol(symbol, evidence)
	case "platform.auth-context":
		return supportedAuthSymbol(symbol, evidence)
	case "metadata.apex-deploy":
		return supportedMetadataAPISymbol(symbol, evidence)
	case "apex.callable-stub":
		return supportedCallableStubSymbol(symbol, evidence)
	default:
		return false
	}
}

func supportedCallableStubSymbol(symbol, evidence string) bool {
	switch strings.TrimSpace(symbol) {
	case "System.Callable", "Callable", "System.StubProvider":
		return true
	case "handleMethodCall":
		return strings.Contains(evidence, "handleMethodCall(")
	case "Test.createStub":
		return !createStubSecondArgIsNull(evidence)
	default:
		return false
	}
}

func createStubSecondArgIsNull(evidence string) bool {
	idx := strings.Index(evidence, "Test.createStub")
	if idx < 0 {
		return false
	}
	open := strings.IndexByte(evidence[idx:], '(')
	if open < 0 {
		return false
	}
	argsText := evidence[idx+open+1:]
	args, ok := splitCallArgs(argsText)
	if !ok || len(args) < 2 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(args[1]), "null")
}

func splitCallArgs(argsText string) ([]string, bool) {
	var args []string
	start := 0
	depth := 0
	inString := false
	for i := 0; i < len(argsText); i++ {
		ch := argsText[i]
		if inString {
			if ch == '\\' && i+1 < len(argsText) {
				i++
				continue
			}
			if ch == '\'' {
				inString = false
			}
			continue
		}
		switch ch {
		case '\'':
			inString = true
		case '(':
			depth++
		case ')':
			if depth == 0 {
				args = append(args, strings.TrimSpace(argsText[start:i]))
				return args, true
			}
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(argsText[start:i]))
				start = i + 1
			}
		}
	}
	return args, false
}

func supportedFileSymbol(symbol, evidence string) bool {
	switch strings.TrimSpace(symbol) {
	case "Blob", "base64Encode", "base64Decode", "Attachment", "Document", "ContentVersion", "ContentDocument", "ContentDocumentLink":
		return true
	default:
		return false
	}
}

func suppressSupportedMetadataFinding(capability, metadataType, symbol, file string) bool {
	switch capability {
	case "labels.localization", "staticresources.urlfor", "endpoint.metadata", "email.templates", "custommetadata.legacy-records", "metadata.legacy-source":
		return true
	default:
		return false
	}
}

func supportedSiteCommunitySymbol(symbol, evidence string) bool {
	if strings.Contains(symbol, "Community__mdt") {
		return true
	}
	for _, needle := range []string{
		"$Site.", "Label.Site.",
		"Site.SObjectType", "Site.UrlRewriter", "Site.getSiteId", "Site.getBaseUrl",
		"Site.getPathPrefix", "Site.getAdminEmail", "Site.getAdminId",
		"Site.getMasterLabel", "Site.isRegistrationEnabled", "Site.getErrorMessage",
		"Site.getErrorDescription", "Site.forgotPassword", "Site.login",
		"Site.changePassword", "Site.validatePassword", "Site.createExternalUser",
		"Site.createPortalUser", "Site.isValidUsername", "Site.setExperienceId",
		"Site.isLoginEnabled", "Site.ExternalUserCreateException", "Site.Id",
		"Site.MasterLabel", "Site.Name", "Site testSite", "(Site)JSON.deserialize",
		"Network.getNetworkId", "Network.getLoginUrl", "Network.communitiesLanding",
		"Network.sObjectType", "Network.Id", "Network.Name", "Network.SelfRegProfileId",
		"Network mockNetwork", "(Network)",
	} {
		if strings.Contains(evidence, needle) {
			return true
		}
	}
	return false
}

func supportedCacheConnectAPISymbol(symbol, evidence string) bool {
	for _, needle := range []string{
		"Cache.", "ConnectApi.Organization.getSettings", "ConnectApi.Communities.getCommunity",
		"ConnectApi.OrganizationSettings", "ConnectApi.UserSettings", "ConnectApi.TimeZone",
		"ConnectApi.UserProfiles.setPhoto", "ConnectApi.UserProfiles.deletePhoto",
	} {
		if strings.Contains(evidence, needle) {
			return true
		}
	}
	return false
}

func supportedAuthSymbol(symbol, evidence string) bool {
	for _, needle := range []string{
		"Auth.UserData", "Auth.VerificationResult", "Auth.VerificationMethod",
		"Auth.JWT ", "new Auth.JWT", "(Auth.JWT)",
		"Auth.RegistrationHandler", "Auth.User", "Auth.CommunitiesUtil.isGuestUser",
		"Auth.AuthToken.revokeAccess", "Auth.SessionManagement.getCurrentSession",
		"Auth.AuthConfiguration",
	} {
		if strings.Contains(evidence, needle) {
			return true
		}
	}
	return false
}

func supportedMetadataAPISymbol(symbol, evidence string) bool {
	for _, needle := range []string{
		"Metadata.CustomMetadata", "Metadata.CustomMetadataValue",
		"Metadata.DeployCallback", "Metadata.DeployCallBack", "Metadata.DeployCallbackContext",
		"Metadata.DeployContainer", "Metadata.DeployDetails", "Metadata.DeployMessage",
		"Metadata.DeployResult", "Metadata.DeployStatus", "Metadata.Metadata", "Metadata.MetadataType",
		"Metadata.Operations.enqueueDeployment", "Metadata.Operations.retrieve",
	} {
		if strings.Contains(symbol, needle) || strings.Contains(evidence, needle) {
			return true
		}
	}
	return false
}

func metadataTypeForText(rel string) string {
	lower := strings.ToLower(rel)
	switch {
	case strings.HasSuffix(lower, ".page"), strings.HasSuffix(lower, ".component"):
		return "Visualforce"
	case strings.Contains(lower, "/lwc/") && (strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".html")):
		return "LWCJavaScript"
	case strings.HasSuffix(lower, ".cls"), strings.HasSuffix(lower, ".trigger"):
		return "ApexClass"
	default:
		return "MetadataXML"
	}
}

func makeFinding(capability, file string, line int, metadataType, symbol, evidence string) Finding {
	def := surfaceDefs[capability]
	return Finding{
		Capability:          capability,
		File:                file,
		Line:                line,
		MetadataType:        metadataType,
		Stage:               def.stage,
		Symbol:              symbol,
		Evidence:            evidence,
		SuggestedCapability: def.suggestedCapability,
		TestBlocking:        def.testBlocking,
	}
}

func buildReport(projectRoot string, filesScanned int, findings []Finding) Report {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Capability != findings[j].Capability {
			return findings[i].Capability < findings[j].Capability
		}
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Symbol < findings[j].Symbol
	})

	type agg struct {
		count     int
		files     map[string]struct{}
		metaTypes map[string]struct{}
		examples  []Example
	}
	aggs := map[string]*agg{}
	testBlockingFindings := 0
	for _, finding := range findings {
		def := surfaceDefs[finding.Capability]
		if def.testBlocking {
			testBlockingFindings++
		}
		a := aggs[finding.Capability]
		if a == nil {
			a = &agg{files: map[string]struct{}{}, metaTypes: map[string]struct{}{}}
			aggs[finding.Capability] = a
		}
		a.count++
		a.files[finding.File] = struct{}{}
		if finding.MetadataType != "" {
			a.metaTypes[finding.MetadataType] = struct{}{}
		}
		if len(a.examples) < 5 {
			a.examples = append(a.examples, Example{
				File:     finding.File,
				Line:     finding.Line,
				Symbol:   finding.Symbol,
				Evidence: finding.Evidence,
			})
		}
	}

	caps := make([]string, 0, len(aggs))
	for capability := range aggs {
		caps = append(caps, capability)
	}
	sort.Strings(caps)

	surfaces := make([]Surface, 0, len(caps))
	for _, capability := range caps {
		def := surfaceDefs[capability]
		a := aggs[capability]
		surfaces = append(surfaces, Surface{
			Capability:          capability,
			Title:               def.title,
			Area:                def.area,
			Stage:               def.stage,
			Status:              def.status,
			TestBlocking:        def.testBlocking,
			Count:               a.count,
			AffectedFiles:       len(a.files),
			MetadataTypes:       sortedKeys(a.metaTypes),
			SuggestedCapability: def.suggestedCapability,
			Examples:            a.examples,
		})
	}

	top := make([]TopBlocker, 0, len(surfaces))
	for _, surface := range surfaces {
		if !surface.TestBlocking {
			continue
		}
		top = append(top, TopBlocker{
			Capability:    surface.Capability,
			Title:         surface.Title,
			Count:         surface.Count,
			AffectedFiles: surface.AffectedFiles,
		})
	}
	sort.Slice(top, func(i, j int) bool {
		if top[i].Count != top[j].Count {
			return top[i].Count > top[j].Count
		}
		if top[i].AffectedFiles != top[j].AffectedFiles {
			return top[i].AffectedFiles > top[j].AffectedFiles
		}
		return top[i].Capability < top[j].Capability
	})
	if len(top) > 10 {
		top = top[:10]
	}

	return Report{
		Project:     projectRoot,
		Surfaces:    surfaces,
		Findings:    findings,
		TopBlockers: top,
		Summary: Summary{
			FilesScanned:         filesScanned,
			Findings:             len(findings),
			TestBlockingFindings: testBlockingFindings,
			Surfaces:             len(surfaces),
		},
	}
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
