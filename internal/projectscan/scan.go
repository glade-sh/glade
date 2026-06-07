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

	"github.com/glade-sh/glade/internal/automation"
	metadatapkg "github.com/glade-sh/glade/internal/metadata"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/uicontroller"
	"github.com/glade-sh/glade/internal/visualforce"
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
	Reports              int `json:"reports"`
	Dashboards           int `json:"dashboards"`
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
	"visualforce.page-metadata": {
		capability:          "visualforce.page-metadata",
		title:               "Visualforce page metadata dependency",
		area:                "metadata-loading",
		stage:               "load",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "visualforce.page-metadata",
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
	"aura.action-metadata": {
		capability:          "aura.action-metadata",
		title:               "Aura server action metadata dependency",
		area:                "metadata-loading",
		stage:               "load",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "aura.action-metadata",
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
	"analytics.report-execution": {
		capability:          "analytics.report-execution",
		title:               "Local report and analytics execution",
		area:                "analytics-metadata",
		stage:               "execute",
		status:              "unsupported",
		testBlocking:        true,
		suggestedCapability: "analytics.report-execution",
	},
}

type patternDef struct {
	capability   string
	metadataType string
	re           *regexp.Regexp
	symbolGroup  int
}

type scanContext struct {
	org                storage.OrgState
	metadata           storage.MetadataRegistry
	vf                 visualforce.Index
	vfComponents       map[string]bool
	pages              map[string]string
	uiApex             map[string]bool
	aura               map[string]bool
	auraActionMetadata map[string]bool
	types              map[string]typesys.TypeSymbol
	apexMetadataNames  map[string]bool
	apexMetadataFiles  map[string][]string
	apexMethodCache    map[string]bool
	present            map[string]bool
	presentationPaths  map[string]bool
	fieldSets          map[string]bool
	customMetadata     map[string]bool
	loadedFiles        map[string]bool
	automation         map[string]bool
	namespaces         map[string]bool
	reports            map[string]bool
	dashboards         map[string]bool
}

var textPatterns = []patternDef{
	{"visualforce.controller-test", "ApexClass", regexp.MustCompile(`\b(ApexPages\.|PageReference\b|Page\.[A-Za-z_][A-Za-z0-9_]*|StandardController\b|StandardSetController\b)`), 1},
	{"labels.localization", "ApexClass", regexp.MustCompile(`(^|[^A-Za-z0-9_.])(?:System\.)?Label\.([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?)`), 2},
	{"metadata.apex-deploy", "ApexClass", regexp.MustCompile(`\b(Metadata\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"site.community-context", "ApexClass", regexp.MustCompile(`\b(Site\.|Network\.|Community__mdt\b)`), 1},
	{"platform.cache-connectapi", "ApexClass", regexp.MustCompile(`\b(Cache\.|ConnectApi\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"platform.auth-context", "ApexClass", regexp.MustCompile(`\b(Auth\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"apex.callable-stub", "ApexClass", regexp.MustCompile(`\b(System\.Callable|Callable\b|System\.StubProvider|Test\.createStub|handleMethodCall\b)`), 1},
	{"endpoint.metadata", "ApexClass", regexp.MustCompile(`callout:([A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"files.binary-content", "ApexClass", regexp.MustCompile(`\b(ContentVersion\b|ContentDocument\b|ContentDocumentLink\b|Attachment\b|Document\b)`), 1},
	{"analytics.report-execution", "ApexClass", regexp.MustCompile(`\b((?:Reports|Analytics)\.[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?)`), 1},
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
	{"visualforce.controller-test", "Visualforce", regexp.MustCompile(`(?:^|[\s<])(controller|standardController|extensions|action|recordSetVar)=["']([^"']+)["']`), 2},
}

var visualforceControllerAttrRE = regexp.MustCompile(`(?i)\b(controller|extensions)\s*=\s*["']([^"']+)["']`)

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

	report := buildReport(absRoot, filesScanned, findings, ctx)
	return report, nil
}

func loadScanContext(absRoot string) scanContext {
	ctx := scanContext{org: storage.NewOrgState()}
	proj, err := project.Load(absRoot)
	if err != nil {
		return ctx
	}
	scanned, err := scanMetadataProjectWithAnalytics(absRoot, proj)
	if err != nil {
		return ctx
	}
	scanProj := scanned.project
	ctx.reports = scanned.reports
	ctx.dashboards = scanned.dashboards
	ctx.org.Namespace = proj.Namespace
	sch, err := schema.LoadProject(scanProj)
	if err != nil {
		return ctx
	}
	ctx.customMetadata = make(map[string]bool)
	for _, record := range sch.CustomMetadataRecords {
		ctx.customMetadata[schemaPathKey([]string{record.ObjectName})] = true
		if stripped := stripAnyNamespaceToken(record.ObjectName); stripped != record.ObjectName {
			ctx.customMetadata[schemaPathKey([]string{stripped})] = true
		}
	}
	typeIndex := typesys.Build(scanProj, sch)
	ctx.types = make(map[string]typesys.TypeSymbol, len(typeIndex.Types))
	for _, typ := range typeIndex.Types {
		ctx.types[strings.ToLower(typ.Name)] = typ
	}
	ctx.apexMetadataNames = make(map[string]bool, len(scanProj.ApexFiles))
	ctx.apexMetadataFiles = make(map[string][]string, len(scanProj.ApexFiles))
	for _, path := range scanProj.ApexFiles {
		name := strings.ToLower(baseNoExt(path))
		ctx.apexMetadataNames[name] = true
		ctx.apexMetadataFiles[name] = append(ctx.apexMetadataFiles[name], path)
	}
	if ui, err := uicontroller.Build(scanProj, typeIndex); err == nil {
		ctx.uiApex = make(map[string]bool)
		for _, method := range ui.ApexMethods {
			if method.Resolved {
				ctx.uiApex[strings.ToLower(method.ClassName+"."+method.MethodName)] = true
				ctx.uiApex[strings.ToLower(stripDotNamespace(method.ClassName)+"."+method.MethodName)] = true
			}
		}
		ctx.aura, ctx.auraActionMetadata = resolvedAuraFiles(ui, &ctx)
	}
	if metadata, err := resource.LoadProject(scanProj); err == nil {
		ctx.metadata = metadata
	}
	ctx.automation = resolvedAutomationFiles(proj)
	ctx.loadedFiles = make(map[string]bool)
	for _, path := range scanProj.ObjectFiles {
		ctx.loadedFiles[filepath.Clean(path)] = true
	}
	for _, path := range scanProj.CustomMetadataFiles {
		ctx.loadedFiles[filepath.Clean(path)] = true
	}
	var metadataIndex *metadatapkg.Index
	if idx, err := metadatapkg.LoadProject(scanProj); err == nil {
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
	ctx.namespaces = namespaceAliases(scanProj, sch, metadataIndex)
	ctx.vf = visualforce.LoadProjectBestEffort(scanProj)
	ctx.vfComponents = make(map[string]bool, len(scanProj.VisualforceComponentFiles))
	for _, path := range scanProj.VisualforceComponentFiles {
		ctx.vfComponents[filepath.Clean(path)] = true
	}
	ctx.pages = make(map[string]string, len(scanProj.VisualforcePageFiles))
	for _, path := range scanProj.VisualforcePageFiles {
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
		ctx.addPresentationFields(*metadataIndex, scanProj)
	}
	return ctx
}

type scanProjectWithAnalytics struct {
	project    project.Project
	reports    map[string]bool
	dashboards map[string]bool
}

func loadScanProjectWithAnalytics(absRoot string) (scanProjectWithAnalytics, error) {
	proj, err := project.Load(absRoot)
	if err != nil {
		return scanProjectWithAnalytics{}, err
	}
	return scanMetadataProjectWithAnalytics(absRoot, proj)
}

func scanMetadataProjectWithAnalytics(absRoot string, proj project.Project) (scanProjectWithAnalytics, error) {
	out := proj
	seen := scanProjectSeenFiles(out)
	scanned := scanProjectWithAnalytics{
		project:    out,
		reports:    make(map[string]bool),
		dashboards: make(map[string]bool),
	}
	err := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
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
		addScanMetadataFile(path, &scanned.project, seen)
		switch {
		case isReportMetadataPath(path):
			scanned.reports[filepath.Clean(path)] = true
		case isDashboardMetadataPath(path):
			scanned.dashboards[filepath.Clean(path)] = true
		}
		return nil
	})
	if err != nil {
		return scanProjectWithAnalytics{}, err
	}
	sortScanProjectFiles(&scanned.project)
	return scanned, nil
}

func scanProjectSeenFiles(proj project.Project) map[string]bool {
	seen := make(map[string]bool)
	addAll := func(paths []string) {
		for _, path := range paths {
			seen[filepath.Clean(path)] = true
		}
	}
	addAll(proj.ObjectFiles)
	addAll(proj.FieldFiles)
	addAll(proj.FieldSetFiles)
	addAll(proj.RecordTypeFiles)
	addAll(proj.ValidationRuleFiles)
	addAll(proj.LabelFiles)
	addAll(proj.TranslationFiles)
	addAll(proj.StaticResourceFiles)
	addAll(proj.StaticResourceMetas)
	addAll(proj.ContentAssetFiles)
	addAll(proj.ContentAssetMetas)
	addAll(proj.EmailTemplateFiles)
	addAll(proj.NamedCredentialFiles)
	addAll(proj.RemoteSiteFiles)
	addAll(proj.CustomMetadataFiles)
	addAll(proj.WorkflowFiles)
	addAll(proj.FlowFiles)
	addAll(proj.ProfileFiles)
	addAll(proj.PermissionSetFiles)
	addAll(proj.PermissionAssignmentFiles)
	addAll(proj.ListViewFiles)
	addAll(proj.LayoutFiles)
	addAll(proj.CompactLayoutFiles)
	addAll(proj.TabFiles)
	addAll(proj.WebLinkFiles)
	addAll(proj.QuickActionFiles)
	addAll(proj.GlobalValueSetFiles)
	addAll(proj.StandardValueSetFiles)
	addAll(proj.FlexiPageFiles)
	addAll(proj.ApplicationFiles)
	addAll(proj.VisualforcePageFiles)
	addAll(proj.VisualforceComponentFiles)
	return seen
}

func sortScanProjectFiles(proj *project.Project) {
	lists := []*[]string{
		&proj.ObjectFiles, &proj.FieldFiles, &proj.FieldSetFiles, &proj.RecordTypeFiles,
		&proj.ValidationRuleFiles, &proj.LabelFiles, &proj.TranslationFiles,
		&proj.StaticResourceFiles, &proj.StaticResourceMetas, &proj.ContentAssetFiles,
		&proj.ContentAssetMetas, &proj.EmailTemplateFiles, &proj.NamedCredentialFiles,
		&proj.RemoteSiteFiles, &proj.CustomMetadataFiles, &proj.WorkflowFiles,
		&proj.FlowFiles, &proj.ProfileFiles, &proj.PermissionSetFiles,
		&proj.PermissionAssignmentFiles, &proj.ListViewFiles, &proj.LayoutFiles,
		&proj.CompactLayoutFiles, &proj.TabFiles, &proj.WebLinkFiles,
		&proj.QuickActionFiles, &proj.GlobalValueSetFiles, &proj.StandardValueSetFiles,
		&proj.FlexiPageFiles, &proj.ApplicationFiles, &proj.VisualforcePageFiles,
		&proj.VisualforceComponentFiles,
	}
	for _, list := range lists {
		sort.Strings(*list)
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

func isReportMetadataPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(lower, "/reports/") && hasAnySuffix(lower, ".report", ".report-meta.xml")
}

func isDashboardMetadataPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(lower, "/dashboards/") && hasAnySuffix(lower, ".dashboard", ".dashboard-meta.xml")
}

func hasAnySuffix(value string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func hasPrefixFold(value, prefix string) bool {
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
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
	inHTMLComment := false
	inApexBlockComment := false
	guardContext := ""
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "//") || strings.HasPrefix(trimmedLine, "*") {
			continue
		}
		if metadataType == "ApexClass" {
			line = stripApexBlockComments(line, &inApexBlockComment)
			if strings.TrimSpace(line) == "" {
				continue
			}
		}
		if beforeComment, _, ok := strings.Cut(line, "//"); ok {
			line = beforeComment
		}
		if metadataType == "Visualforce" {
			line = stripHTMLComments(line, &inHTMLComment)
			if strings.TrimSpace(line) == "" {
				continue
			}
		}
		for _, pattern := range textPatterns {
			if pattern.metadataType != metadataType && !(metadataType == "ApexClass" && pattern.metadataType == "ApexClass") {
				continue
			}
			scanLine := lineForPattern(line, pattern)
			matches := pattern.re.FindAllStringSubmatch(scanLine, -1)
			for _, match := range matches {
				symbol := patternSymbol(pattern, match)
				if suppressVisualforceControllerAttributeFinding(pattern, match) {
					continue
				}
				if ctx != nil && ctx.resolvesFinding(pattern.capability, symbol, rel, path) {
					continue
				}
				evidence := line
				if guardContext != "" {
					evidence = guardContext + " " + line
				}
				if suppressSupportedFinding(pattern.capability, symbol, evidence, rel) {
					continue
				}
				if suppressApexStubSelfReference(pattern.capability, rel, symbol, line) {
					continue
				}
				capability := pattern.capability
				if capability == "visualforce.controller-test" && metadataType == "ApexClass" && strings.HasPrefix(symbol, "Page.") {
					capability = "visualforce.page-metadata"
				}
				findings = append(findings, makeFinding(capability, rel, lineNo, metadataType, symbol, strings.TrimSpace(evidence)))
			}
		}
		guardContext = nextTestGuardContext(line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return findings, nil
}

func stripApexBlockComments(line string, inComment *bool) string {
	if inComment == nil {
		return line
	}
	if !*inComment && !strings.Contains(line, "/*") {
		return line
	}
	var b strings.Builder
	for {
		if *inComment {
			end := strings.Index(line, "*/")
			if end < 0 {
				return b.String()
			}
			line = line[end+len("*/"):]
			*inComment = false
			continue
		}
		start := strings.Index(line, "/*")
		if start < 0 {
			b.WriteString(line)
			return b.String()
		}
		b.WriteString(line[:start])
		line = line[start+len("/*"):]
		*inComment = true
	}
}

func nextTestGuardContext(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(strings.ToLower(trimmed), "if") || !callGuardedOutOfTests(trimmed) || strings.Contains(trimmed, "||") {
		return ""
	}
	return trimmed
}

func suppressVisualforceControllerAttributeFinding(pattern patternDef, match []string) bool {
	if pattern.capability != "visualforce.controller-test" || pattern.metadataType != "Visualforce" || len(match) < 3 {
		return false
	}
	attrName := strings.TrimSpace(match[1])
	symbol := strings.TrimSpace(match[2])
	switch strings.ToLower(attrName) {
	case "recordsetvar":
		return true
	case "action":
		return !visualforceActionAttributeCanInvokeController(symbol)
	default:
		return false
	}
}

func lineForPattern(line string, pattern patternDef) string {
	if pattern.metadataType == "Visualforce" && pattern.capability == "visualforce.controller-test" {
		return visualforceControllerTagLine(line)
	}
	if pattern.metadataType == "ApexClass" && (pattern.capability == "custommetadata.legacy-records" || pattern.capability == "visualforce.controller-test" || pattern.capability == "labels.localization" || pattern.capability == "metadata.apex-deploy") {
		return stripApexCommentsAndStrings(line)
	}
	return line
}

func visualforceControllerTagLine(line string) string {
	var b strings.Builder
	remaining := line
	for {
		start := strings.IndexByte(remaining, '<')
		if start < 0 {
			break
		}
		if !hasPrefixFold(remaining[start:], "<apex:") {
			remaining = remaining[start+1:]
			continue
		}
		remaining = remaining[start:]
		end := strings.Index(remaining, ">")
		if end < 0 {
			b.WriteString(remaining)
			break
		}
		b.WriteString(remaining[:end+1])
		b.WriteByte(' ')
		remaining = remaining[end+1:]
	}
	return b.String()
}

func stripHTMLComments(line string, inComment *bool) string {
	if inComment == nil {
		return line
	}
	if !*inComment && !strings.Contains(line, "<!--") {
		return line
	}
	var b strings.Builder
	for {
		if *inComment {
			end := strings.Index(line, "-->")
			if end < 0 {
				return b.String()
			}
			line = line[end+len("-->"):]
			*inComment = false
			continue
		}
		start := strings.Index(line, "<!--")
		if start < 0 {
			b.WriteString(line)
			return b.String()
		}
		b.WriteString(line[:start])
		line = line[start+len("<!--"):]
		*inComment = true
	}
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

func (ctx *scanContext) resolvesFinding(capability, symbol, rel, path string) bool {
	switch capability {
	case "ui.presentation-metadata":
		if strings.EqualFold(metadataTypeForText(rel), "Visualforce") && ctx.resolvesManagedVisualforceSchemaReference(path, symbol) {
			return true
		}
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
		for _, ref := range labelReferenceSymbols(symbol) {
			if ctx.resolvesLabel(ref.namespace, ref.label) {
				return true
			}
		}
	case "staticresources.urlfor":
		return ctx.resolvesResource(symbol)
	case "endpoint.metadata":
		return ctx.resolvesEndpoint(symbol)
	case "custommetadata.legacy-records":
		return ctx.resolvesCustomMetadataObject(symbol)
	case "visualforce.controller-test":
		return ctx.resolvesVisualforceControllerReference(symbol, rel, path)
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

func (ctx *scanContext) hasAuraActionMetadata(path string) bool {
	if ctx == nil || ctx.auraActionMetadata == nil {
		return false
	}
	cleanPath := filepath.Clean(path)
	if ctx.auraActionMetadata[cleanPath] {
		return true
	}
	return ctx.auraActionMetadata[filepath.Dir(cleanPath)]
}

func (ctx *scanContext) resolvesAuraControllerReference(symbol string) bool {
	return ctx.resolvesApexType(symbol)
}

func (ctx *scanContext) resolvesVisualforceControllerReference(symbol, rel, path string) bool {
	symbol = strings.TrimSpace(symbol)
	if strings.Contains(symbol, ",") {
		return ctx.resolvesVisualforceControllerList(symbol)
	}
	switch symbol {
	case "ApexPages.", "PageReference", "StandardController", "StandardSetController":
		return true
	}
	if strings.HasPrefix(symbol, "Page.") {
		pageName := strings.TrimPrefix(symbol, "Page.")
		if ctx.vf.HasPageReference(pageName) {
			return true
		}
		if stripped := stripAnyNamespaceToken(pageName); stripped != pageName && ctx.vf.HasPageReference(stripped) {
			return true
		}
		_, ok := ctx.pages[strings.ToLower(pageName)]
		if ok {
			return true
		}
		stripped := stripAnyNamespaceToken(pageName)
		if stripped != pageName {
			_, ok = ctx.pages[strings.ToLower(stripped)]
		}
		return ok
	}
	if ctx.resolvesApexType(symbol) {
		return true
	}
	if ctx.resolvesExternalManagedSymbol(symbol) {
		return true
	}
	if _, ok := ctx.objectDefinition(symbol); ok {
		return true
	}
	if ctx.resolvesVisualforceActionReference(symbol, rel, path) {
		return true
	}
	return false
}

func (ctx *scanContext) resolvesVisualforceControllerList(symbol string) bool {
	parts := strings.Split(symbol, ",")
	found := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		found = true
		if !ctx.resolvesVisualforceControllerReference(part, "", "") {
			return false
		}
	}
	return found
}

func (ctx *scanContext) resolvesVisualforceActionReference(symbol, rel, path string) bool {
	expr, ok := visualforceActionExpression(symbol)
	if !ok {
		return false
	}
	if strings.Contains(expr, ".") && visualforceStandardControllerAction(expr) {
		return true
	}
	name := baseNoExt(rel)
	if strings.HasSuffix(strings.ToLower(rel), ".component") {
		component, ok := ctx.vf.ComponentFile(path)
		if !ok {
			component, ok = ctx.vf.Component(name)
		}
		if ok {
			if visualforceComponentHasAttribute(component, expr) {
				return true
			}
			return ctx.visualforceTypesHaveMethod([]string{component.Controller}, component.Extensions, expr)
		}
	}
	if strings.HasSuffix(strings.ToLower(rel), ".page") {
		if ctx.visualforceFileHasExternalManagedController(path) {
			return true
		}
		page, ok := ctx.vf.PageFile(path)
		if !ok {
			page, ok = ctx.vf.Page(name)
		}
		if ok {
			if page.StandardController != "" && visualforceStandardControllerAction(expr) {
				return true
			}
			return ctx.visualforceTypesHaveMethod([]string{page.Controller}, page.Extensions, expr)
		}
	}
	return false
}

func visualforceActionAttributeCanInvokeController(symbol string) bool {
	expr, ok := visualforceActionExpression(symbol)
	if !ok {
		return false
	}
	if visualforceStandardControllerAction(expr) {
		return true
	}
	return !strings.Contains(expr, ".")
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

func visualforceStandardControllerAction(expr string) bool {
	methodName := expr
	if strings.Contains(methodName, ".") {
		parts := strings.Split(methodName, ".")
		methodName = parts[len(parts)-1]
	}
	switch strings.ToLower(strings.TrimSpace(methodName)) {
	case "save", "quicksave", "edit", "delete", "cancel", "list", "view",
		"first", "previous", "next", "last":
		return true
	default:
		return false
	}
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
	component, ok := ctx.vf.ComponentFile(cleanPath)
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
		if ctx.namespacedApexTypeHasMethod(typeName, methodName) {
			return true
		}
		if ctx.apexMetadataFileHasMethod(typeName, methodName) {
			return true
		}
		if _, ok := ctx.lookupApexType(typeName); !ok && ctx.resolvesApexMetadataName(typeName) {
			return true
		}
	}
	return false
}

func (ctx *scanContext) namespacedApexTypeHasMethod(typeName, methodName string) bool {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" || strings.Contains(typeName, ".") || namespaceToken(typeName) != "" {
		return false
	}
	for namespace := range ctx.namespaces {
		candidate := namespace + "__" + typeName
		if ctx.apexTypeHasMethod(candidate, methodName) {
			return true
		}
	}
	return false
}

func (ctx *scanContext) apexMetadataFileHasMethod(typeName, methodName string) bool {
	methodName = strings.TrimSpace(methodName)
	if methodName == "" {
		return false
	}
	for _, candidate := range ctx.apexMetadataCandidateNames(typeName) {
		key := strings.ToLower(candidate) + "." + strings.ToLower(methodName)
		if cached, ok := ctx.apexMethodCache[key]; ok {
			return cached
		}
		found := false
		for _, path := range ctx.apexMetadataFiles[strings.ToLower(candidate)] {
			if apexFileDeclaresMethod(path, methodName) {
				found = true
				break
			}
		}
		if ctx.apexMethodCache == nil {
			ctx.apexMethodCache = make(map[string]bool)
		}
		ctx.apexMethodCache[key] = found
		if found {
			return true
		}
	}
	return false
}

func (ctx *scanContext) apexMetadataFileHasAuraMethod(typeName, methodName string) bool {
	methodName = strings.TrimSpace(methodName)
	if methodName == "" {
		return false
	}
	for _, candidate := range ctx.apexMetadataCandidateNames(typeName) {
		key := "aura:" + strings.ToLower(candidate) + "." + strings.ToLower(methodName)
		if cached, ok := ctx.apexMethodCache[key]; ok {
			return cached
		}
		found := false
		for _, path := range ctx.apexMetadataFiles[strings.ToLower(candidate)] {
			if apexFileDeclaresAuraMethod(path, methodName) {
				found = true
				break
			}
		}
		if ctx.apexMethodCache == nil {
			ctx.apexMethodCache = make(map[string]bool)
		}
		ctx.apexMethodCache[key] = found
		if found {
			return true
		}
	}
	return false
}

func (ctx *scanContext) apexMetadataCandidateNames(typeName string) []string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, name)
	}
	add(typeName)
	add(dotNamespaceToMetadataName(typeName))
	add(stripDotNamespace(typeName))
	add(stripAnyNamespaceToken(typeName))
	if !strings.Contains(typeName, ".") && namespaceToken(typeName) == "" {
		for namespace := range ctx.namespaces {
			add(namespace + "__" + typeName)
		}
	}
	return out
}

func apexFileDeclaresMethod(path, methodName string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if apexLineDeclaresMethod(line, methodName) {
			return true
		}
	}
	return false
}

func apexFileDeclaresAuraMethod(path, methodName string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pendingAuraEnabled := false
	for _, line := range strings.Split(string(data), "\n") {
		clean := strings.TrimSpace(stripApexCommentsAndStrings(line))
		if clean == "" {
			continue
		}
		if strings.Contains(strings.ToLower(clean), "@auraenabled") {
			pendingAuraEnabled = true
			continue
		}
		if apexLineDeclaresAuraMethod(line, methodName, pendingAuraEnabled) {
			return true
		}
		if strings.Contains(clean, "(") || strings.HasSuffix(clean, ";") || strings.HasSuffix(clean, "{") {
			pendingAuraEnabled = false
		}
	}
	return false
}

func apexLineDeclaresMethod(line, methodName string) bool {
	if before, _, ok := strings.Cut(line, "//"); ok {
		line = before
	}
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "/*") {
		return false
	}
	idx := strings.Index(strings.ToLower(line), strings.ToLower(methodName))
	if idx < 0 {
		return false
	}
	after := strings.TrimLeft(line[idx+len(methodName):], " \t")
	if !strings.HasPrefix(after, "(") {
		return false
	}
	before := strings.ToLower(line[:idx])
	for _, token := range []string{"public", "global", "private", "protected", "webservice"} {
		if strings.Contains(before, token) {
			return true
		}
	}
	return false
}

func apexLineDeclaresAuraMethod(line, methodName string, auraEnabled bool) bool {
	if before, _, ok := strings.Cut(line, "//"); ok {
		line = before
	}
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "/*") {
		return false
	}
	lower := strings.ToLower(line)
	if strings.Contains(lower, "@auraenabled") {
		auraEnabled = true
	}
	if !auraEnabled || !strings.Contains(lower, " static ") {
		return false
	}
	idx := strings.Index(lower, strings.ToLower(methodName))
	if idx < 0 {
		return false
	}
	after := strings.TrimLeft(line[idx+len(methodName):], " \t")
	if !strings.HasPrefix(after, "(") {
		return false
	}
	before := lower[:idx]
	return strings.Contains(before, "public") || strings.Contains(before, "global")
}

func (ctx *scanContext) resolvesApexType(symbol string) bool {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return false
	}
	if _, ok := ctx.types[strings.ToLower(symbol)]; ok {
		return true
	}
	if namespaced := dotNamespaceToMetadataName(symbol); namespaced != symbol {
		if _, ok := ctx.types[strings.ToLower(namespaced)]; ok {
			return true
		}
	}
	if stripped := stripDotNamespace(symbol); stripped != symbol {
		if _, ok := ctx.types[strings.ToLower(stripped)]; ok {
			return true
		}
	}
	stripped := stripAnyNamespaceToken(symbol)
	if stripped != symbol {
		if _, ok := ctx.types[strings.ToLower(stripped)]; ok {
			return true
		}
	}
	if ctx.resolvesNamespacedApexTypeName(symbol) {
		return true
	}
	return ctx.resolvesApexMetadataName(symbol)
}

func (ctx *scanContext) resolvesNamespacedApexTypeName(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" || strings.Contains(typeName, ".") || namespaceToken(typeName) != "" {
		return false
	}
	for namespace := range ctx.namespaces {
		candidate := namespace + "__" + typeName
		if _, ok := ctx.types[strings.ToLower(candidate)]; ok {
			return true
		}
		if ctx.apexMetadataNames[strings.ToLower(candidate)] {
			return true
		}
	}
	return false
}

func (ctx *scanContext) resolvesExternalManagedSymbol(symbol string) bool {
	namespace := managedNamespaceFromSymbol(symbol)
	if namespace == "" {
		return false
	}
	if ctx.org.Namespace != "" && strings.EqualFold(namespace, ctx.org.Namespace) {
		return false
	}
	return ctx.namespaces[strings.ToLower(namespace)]
}

func (ctx *scanContext) resolvesApexMetadataName(symbol string) bool {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return false
	}
	if ctx.apexMetadataNames[strings.ToLower(symbol)] {
		return true
	}
	if namespaced := dotNamespaceToMetadataName(symbol); namespaced != symbol && ctx.apexMetadataNames[strings.ToLower(namespaced)] {
		return true
	}
	if stripped := stripDotNamespace(symbol); stripped != symbol && ctx.apexMetadataNames[strings.ToLower(stripped)] {
		return true
	}
	if stripped := stripAnyNamespaceToken(symbol); stripped != symbol && ctx.apexMetadataNames[strings.ToLower(stripped)] {
		return true
	}
	return false
}

func managedNamespaceFromSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if dot := strings.Index(symbol, "."); dot > 0 {
		return symbol[:dot]
	}
	if namespace := metadataNamespacePrefix(symbol); namespace != "" {
		return namespace
	}
	return namespaceToken(symbol)
}

func dotNamespaceToMetadataName(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	dot := strings.Index(symbol, ".")
	if dot <= 0 || dot >= len(symbol)-1 {
		return symbol
	}
	return symbol[:dot] + "__" + symbol[dot+1:]
}

func (ctx *scanContext) resolvesManagedVisualforceSchemaReference(path, symbol string) bool {
	schemaRef, ok := schemaReferenceSymbol(symbol)
	if !ok {
		schemaRef = symbol
	}
	parts := schemaReferenceTokens(schemaRef)
	if len(parts) == 0 {
		return false
	}
	objectName := parts[0]
	if !strings.HasSuffix(strings.ToLower(objectName), "__c") && !strings.HasSuffix(strings.ToLower(objectName), "__mdt") {
		return false
	}
	namespace := ctx.visualforceExternalManagedNamespace(path)
	if namespace == "" {
		return false
	}
	if namespaceToken(objectName) != "" {
		if _, ok := ctx.objectDefinition(objectName); ok {
			return false
		}
		return strings.EqualFold(namespaceToken(objectName), namespace)
	}
	return true
}

func (ctx *scanContext) visualforceExternalManagedNamespace(path string) string {
	if path == "" {
		return ""
	}
	page, ok := ctx.vf.PageFile(filepath.Clean(path))
	if !ok {
		page, ok = ctx.vf.Page(baseNoExt(path))
	}
	if ok {
		if namespace := ctx.externalManagedNamespace(page.Controller); namespace != "" {
			return namespace
		}
		for _, extension := range page.Extensions {
			if namespace := ctx.externalManagedNamespace(extension); namespace != "" {
				return namespace
			}
		}
	}
	return ctx.visualforceFileDeclaresExternalManagedNamespace(path)
}

func (ctx *scanContext) externalManagedNamespace(symbol string) string {
	namespace := managedNamespaceFromSymbol(symbol)
	if namespace == "" {
		return ""
	}
	if ctx.org.Namespace != "" && strings.EqualFold(namespace, ctx.org.Namespace) {
		return ""
	}
	return namespace
}

func (ctx *scanContext) visualforceFileHasExternalManagedController(path string) bool {
	if path == "" {
		return false
	}
	page, ok := ctx.vf.PageFile(filepath.Clean(path))
	if !ok {
		page, ok = ctx.vf.Page(baseNoExt(path))
		if !ok {
			return ctx.visualforceFileDeclaresExternalManagedController(path)
		}
	}
	if ctx.resolvesExternalManagedSymbol(page.Controller) {
		return true
	}
	for _, extension := range page.Extensions {
		if ctx.resolvesExternalManagedSymbol(extension) {
			return true
		}
	}
	return ctx.visualforceFileDeclaresExternalManagedController(path)
}

func (ctx *scanContext) visualforceFileDeclaresExternalManagedNamespace(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	matches := visualforceControllerAttrRE.FindAllStringSubmatch(string(data), -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		for _, symbol := range strings.Split(match[2], ",") {
			if namespace := ctx.externalManagedNamespace(symbol); namespace != "" {
				return namespace
			}
		}
	}
	return ""
}

func (ctx *scanContext) visualforceFileDeclaresExternalManagedController(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	source := string(data)
	lower := strings.ToLower(source)
	start := strings.Index(lower, "<apex:page")
	if start < 0 {
		return false
	}
	end := strings.Index(source[start:], ">")
	if end < 0 {
		return false
	}
	tag := source[start : start+end+1]
	for _, match := range visualforceControllerAttrRE.FindAllStringSubmatch(tag, -1) {
		if len(match) < 3 {
			continue
		}
		for _, symbol := range strings.Split(match[2], ",") {
			if ctx.resolvesExternalManagedSymbol(symbol) {
				return true
			}
		}
	}
	return false
}

func (ctx *scanContext) apexTypeHasMethod(typeName, methodName string) bool {
	return ctx.apexTypeHasMethodSeen(typeName, methodName, nil)
}

func (ctx *scanContext) apexTypeHasMethodSeen(typeName, methodName string, seen map[string]bool) bool {
	typ, ok := ctx.lookupApexType(typeName)
	if !ok {
		return false
	}
	key := strings.ToLower(typ.Name)
	if seen == nil {
		seen = make(map[string]bool)
	}
	if seen[key] {
		return false
	}
	seen[key] = true
	for _, member := range typ.Members {
		if member.Kind == "method" && strings.EqualFold(member.Name, methodName) {
			return true
		}
	}
	return ctx.apexTypeHasMethodSeen(typ.SuperClass, methodName, seen)
}

func (ctx *scanContext) lookupApexType(typeName string) (typesys.TypeSymbol, bool) {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typesys.TypeSymbol{}, false
	}
	if typ, ok := ctx.types[strings.ToLower(typeName)]; ok {
		return typ, true
	}
	if namespaced := dotNamespaceToMetadataName(typeName); namespaced != typeName {
		if typ, ok := ctx.types[strings.ToLower(namespaced)]; ok {
			return typ, true
		}
	}
	if stripped := stripDotNamespace(typeName); stripped != typeName {
		if typ, ok := ctx.types[strings.ToLower(stripped)]; ok {
			return typ, true
		}
	}
	stripped := stripAnyNamespaceToken(typeName)
	if stripped != typeName {
		if typ, ok := ctx.types[strings.ToLower(stripped)]; ok {
			return typ, true
		}
	}
	return typesys.TypeSymbol{}, false
}

func (ctx *scanContext) resolvesResource(symbol string) bool {
	name := strings.TrimSpace(symbol)
	if name == "" {
		return false
	}
	_, ok := resource.URLForStaticResource(ctx.metadata, name, "")
	if ok {
		return true
	}
	namespace := namespaceToken(name)
	if namespace == "" {
		namespace = metadataNamespacePrefix(name)
	}
	if namespace == "" {
		return false
	}
	if ctx.org.Namespace != "" && strings.EqualFold(namespace, ctx.org.Namespace) {
		return false
	}
	return ctx.namespaces[strings.ToLower(namespace)]
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

type labelReference struct {
	namespace string
	label     string
}

func labelReferenceSymbols(symbol string) []labelReference {
	symbol = strings.TrimSpace(symbol)
	symbol = strings.TrimPrefix(symbol, "System.")
	symbol = strings.TrimPrefix(symbol, "Label.")
	symbol = strings.TrimPrefix(symbol, "$Label.")
	symbol = strings.TrimPrefix(symbol, "@salesforce/label/")
	symbol = strings.ReplaceAll(symbol, "/", ".")
	if symbol == "" {
		return nil
	}
	parts := strings.Split(symbol, ".")
	refs := make([]labelReference, 0, 2)
	addParts := func(parts []string) {
		if len(parts) == 0 {
			return
		}
		ref := labelReference{}
		switch len(parts) {
		case 1:
			ref.label = parts[0]
		default:
			ref.namespace = parts[len(parts)-2]
			ref.label = parts[len(parts)-1]
			if strings.EqualFold(ref.namespace, "c") {
				ref.namespace = ""
			}
		}
		if ref.label == "" {
			return
		}
		for _, existing := range refs {
			if strings.EqualFold(existing.namespace, ref.namespace) && strings.EqualFold(existing.label, ref.label) {
				return
			}
		}
		refs = append(refs, ref)
	}
	addParts(parts)
	if len(parts) == 1 {
		if namespace, label, ok := labelNamespaceToken(parts[0]); ok {
			addParts([]string{namespace, label})
		}
	}
	if len(parts) > 1 && isLabelStringMethod(parts[len(parts)-1]) {
		addParts(parts[:len(parts)-1])
	}
	return refs
}

func labelReferenceSymbol(symbol string) (string, string, bool) {
	refs := labelReferenceSymbols(symbol)
	if len(refs) == 0 {
		return "", "", false
	}
	return refs[0].namespace, refs[0].label, true
}

func labelNamespaceToken(name string) (string, string, bool) {
	name = strings.TrimSpace(name)
	idx := strings.Index(name, "__")
	if idx <= 0 || idx+2 >= len(name) {
		return "", "", false
	}
	return name[:idx], name[idx+2:], true
}

func (ctx *scanContext) resolvesLabel(namespace, label string) bool {
	if _, status := resource.ResolveLabel(ctx.metadata, ctx.org.Namespace, namespace, label); status != resource.LabelLookupMissing {
		return true
	}
	if namespace == "" || !ctx.namespaces[strings.ToLower(namespace)] {
		return false
	}
	if ctx.org.Namespace == "" || !strings.EqualFold(namespace, ctx.org.Namespace) {
		return true
	}
	if ctx.org.Namespace != "" {
		if _, found := resource.LookupLabel(ctx.metadata, ctx.org.Namespace, label); found {
			return true
		}
	}
	_, found := resource.LookupLabel(ctx.metadata, "", label)
	return found
}

func namespaceToken(name string) string {
	name = strings.TrimSpace(name)
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	if first <= 0 || first >= last {
		return ""
	}
	token := name[:first]
	if strings.Contains(token, ".") {
		token = token[strings.LastIndex(token, ".")+1:]
	}
	return token
}

func metadataNamespacePrefix(name string) string {
	name = strings.TrimSpace(name)
	idx := strings.Index(name, "__")
	if idx <= 0 {
		return ""
	}
	return name[:idx]
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
	if ctx.customMetadata[schemaPathKey([]string{objectName})] {
		return true
	}
	stripped := stripAnyNamespaceToken(objectName)
	if stripped != objectName {
		_, ok := storage.ResolveObjectName(ctx.org, stripped)
		if ok {
			return true
		}
		if ctx.customMetadata[schemaPathKey([]string{stripped})] {
			return true
		}
	}
	return isExternalManagedPackageObjectName(ctx.org.Namespace, objectName)
}

func (ctx *scanContext) resolvesSchemaReference(ref string) bool {
	parts := schemaReferenceTokens(ref)
	if len(parts) == 0 {
		return false
	}
	if ctx.presentationPaths[schemaPathKey(parts)] {
		return true
	}
	if len(parts) >= 3 && strings.EqualFold(parts[1], "FieldSets") && ctx.resolvesFieldSet(parts[0], parts[2]) {
		return true
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

func schemaPathKey(parts []string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			normalized = append(normalized, part)
		}
	}
	return strings.Join(normalized, ".")
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
	if strings.EqualFold(parts[0], "FieldSets") {
		if len(parts) == 1 {
			return true
		}
		return ctx.resolvesFieldSet(definition.APIName, parts[1])
	}
	if isTerminalSchemaProperty(parts[0]) {
		return true
	}
	return ctx.resolvesSchemaFieldPath(definition, parts[0], parts[1:])
}

func (ctx *scanContext) resolvesFieldSet(objectName, fieldSetName string) bool {
	if ctx.fieldSets == nil {
		return false
	}
	if ctx.fieldSets[schemaPathKey([]string{objectName, fieldSetName})] {
		return true
	}
	stripped := stripAnyNamespaceToken(objectName)
	if stripped != objectName && ctx.fieldSets[schemaPathKey([]string{stripped, fieldSetName})] {
		return true
	}
	return false
}

func (ctx *scanContext) resolvesSchemaFieldPath(definition storage.ObjectDefinition, fieldOrRelationship string, rest []string) bool {
	if field, ok := ctx.resolveSchemaField(definition, fieldOrRelationship); ok {
		if len(rest) == 0 || isTerminalSchemaProperty(rest[0]) {
			return true
		}
		return ctx.resolvesReferenceTargets(field.ReferenceTo, rest)
	}
	if field, ok := knownStandardRelationshipField(fieldOrRelationship); ok {
		if len(rest) == 0 {
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
	if ctx.resolvesExternalManagedPackageFieldPath(definition, fieldOrRelationship, rest) {
		return true
	}
	return false
}

func (ctx *scanContext) resolvesExternalManagedPackageFieldPath(definition storage.ObjectDefinition, fieldOrRelationship string, rest []string) bool {
	objectNamespace := externalManagedNamespaceToken(definition.APIName)
	fieldNamespace := externalManagedNamespaceToken(fieldOrRelationship)
	externalObject := isExternalManagedPackageObjectName(ctx.org.Namespace, definition.APIName)
	if objectNamespace == "" && fieldNamespace == "" {
		return false
	}
	for _, namespace := range []string{objectNamespace, fieldNamespace} {
		if namespace == "" {
			continue
		}
		if ctx.org.Namespace != "" && strings.EqualFold(namespace, ctx.org.Namespace) {
			continue
		}
		if ctx.namespaces[strings.ToLower(namespace)] || externalObject {
			lower := strings.ToLower(strings.TrimSpace(fieldOrRelationship))
			if strings.HasSuffix(lower, "__c") {
				return len(rest) == 0 || isTerminalSchemaProperty(rest[0])
			}
			if strings.HasSuffix(lower, "__r") {
				if len(rest) == 0 {
					return true
				}
				if len(rest) == 1 {
					if isTerminalSchemaProperty(rest[0]) {
						return true
					}
					_, ok := knownStandardField(rest[0])
					return ok
				}
				if hasNamespaceToken(rest[0]) && strings.HasSuffix(strings.ToLower(rest[0]), "__r") {
					return ctx.resolvesExternalManagedPackageFieldPath(definition, rest[0], rest[1:])
				}
				return len(rest) == 2 && strings.EqualFold(rest[0], "Fields") && isTerminalSchemaProperty(rest[1])
			}
			return len(rest) == 0 || isTerminalSchemaProperty(rest[len(rest)-1])
		}
	}
	return false
}

func externalManagedNamespaceToken(name string) string {
	name = strings.TrimSpace(name)
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	if first <= 0 || first >= last {
		return ""
	}
	return name[:first]
}

func hasNamespaceToken(name string) bool {
	return externalManagedNamespaceToken(name) != ""
}

func fieldRelationshipTokenMatches(field storage.Field, token string) bool {
	if sameSchemaToken(field.RelationshipName, token) {
		return true
	}
	apiName := strings.TrimSpace(field.APIName)
	if strings.HasSuffix(strings.ToLower(apiName), "id") && len(apiName) > 2 {
		return sameSchemaToken(apiName[:len(apiName)-2], token)
	}
	if strings.HasSuffix(strings.ToLower(apiName), "__c") && len(apiName) > 3 {
		return sameSchemaToken(apiName[:len(apiName)-3]+"__r", token)
	}
	return false
}

func (ctx *scanContext) resolveSchemaField(definition storage.ObjectDefinition, fieldName string) (storage.Field, bool) {
	resolved, ok := storage.ResolveFieldName(definition, ctx.org.Namespace, fieldName)
	if !ok {
		stripped := stripAnyNamespaceToken(fieldName)
		if stripped == fieldName {
			if field, ok := knownStandardField(fieldName); ok {
				return field, true
			}
			return storage.Field{}, false
		}
		resolved, ok = storage.ResolveFieldName(definition, ctx.org.Namespace, stripped)
	}
	if !ok {
		if field, ok := knownStandardField(fieldName); ok {
			return field, true
		}
		return storage.Field{}, false
	}
	field, ok := definition.Fields[resolved]
	if !ok && strings.EqualFold(resolved, "Id") {
		return storage.Field{APIName: "Id"}, true
	}
	return field, ok
}

func knownStandardField(fieldName string) (storage.Field, bool) {
	switch strings.ToLower(strings.TrimSpace(fieldName)) {
	case "id":
		return storage.Field{APIName: "Id", Label: "Record ID", Type: storage.FieldID}, true
	case "name":
		return storage.Field{APIName: "Name", Label: "Name", Type: storage.FieldString}, true
	case "firstname":
		return storage.Field{APIName: "FirstName", Label: "First Name", Type: storage.FieldString}, true
	case "lastname":
		return storage.Field{APIName: "LastName", Label: "Last Name", Type: storage.FieldString}, true
	case "personbirthdate":
		return storage.Field{APIName: "PersonBirthdate", Label: "Birthdate", Type: storage.FieldDate}, true
	case "createddate":
		return storage.Field{APIName: "CreatedDate", Label: "Created Date", Type: storage.FieldDateTime}, true
	case "createdbyid":
		return storage.Field{APIName: "CreatedById", Label: "Created By ID", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy"}, true
	case "lastmodifieddate":
		return storage.Field{APIName: "LastModifiedDate", Label: "Last Modified Date", Type: storage.FieldDateTime}, true
	case "lastmodifiedbyid":
		return storage.Field{APIName: "LastModifiedById", Label: "Last Modified By ID", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "LastModifiedBy"}, true
	case "systemmodstamp":
		return storage.Field{APIName: "SystemModstamp", Label: "System Modstamp", Type: storage.FieldDateTime}, true
	case "lastvieweddate":
		return storage.Field{APIName: "LastViewedDate", Label: "Last Viewed Date", Type: storage.FieldDateTime}, true
	case "lastreferenceddate":
		return storage.Field{APIName: "LastReferencedDate", Label: "Last Referenced Date", Type: storage.FieldDateTime}, true
	case "ownerid":
		return storage.Field{APIName: "OwnerId", Label: "Owner ID", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner"}, true
	case "isdeleted":
		return storage.Field{APIName: "IsDeleted", Label: "Deleted", Type: storage.FieldBoolean}, true
	default:
		return storage.Field{}, false
	}
}

func knownStandardRelationshipField(token string) (storage.Field, bool) {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "createdby":
		return storage.Field{APIName: "CreatedById", Label: "Created By ID", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy"}, true
	case "lastmodifiedby":
		return storage.Field{APIName: "LastModifiedById", Label: "Last Modified By ID", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "LastModifiedBy"}, true
	case "owner":
		return storage.Field{APIName: "OwnerId", Label: "Owner ID", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner"}, true
	default:
		return storage.Field{}, false
	}
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

func isExternalManagedPackageObjectName(projectNamespace, objectName string) bool {
	objectName = strings.TrimSpace(objectName)
	token := externalManagedNamespaceToken(objectName)
	if token == "" {
		return false
	}
	if projectNamespace != "" && strings.EqualFold(token, projectNamespace) {
		return false
	}
	lower := strings.ToLower(objectName)
	return strings.HasSuffix(lower, "__c") || strings.HasSuffix(lower, "__mdt") || strings.HasSuffix(lower, "__e")
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
		"scale", "soaptype", "type", "keyprefix", "accessible", "createable", "creatable",
		"deleteable", "deletable", "filterable", "groupable", "nillable", "queryable",
		"sortable", "updateable", "updatable":
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
	if isExternalManagedPackageObjectName(ctx.org.Namespace, objectName) {
		return storage.ObjectDefinition{APIName: objectName, Fields: map[string]storage.Field{}}, true
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

func resolvedAuraFiles(ui uicontroller.Index, ctx *scanContext) (map[string]bool, map[string]bool) {
	resolved := make(map[string]bool)
	actionMetadata := make(map[string]bool)
	for _, bundle := range ui.AuraBundles {
		controllerActionResolved := make(map[string]bool)
		for _, action := range bundle.ActionReferences {
			if action.ClassName == "" {
				continue
			}
			if action.Resolved || ctx.apexMetadataFileHasAuraMethod(action.ClassName, action.Name) {
				controllerActionResolved[strings.ToLower(action.ClassName)] = true
			}
		}
		controllersResolved := true
		for _, controller := range bundle.ControllerReferences {
			if !ctx.resolvesAuraControllerReference(controller.Name) && !controllerActionResolved[strings.ToLower(controller.Name)] {
				controllersResolved = false
				break
			}
		}
		bundleResolved := controllersResolved
		missingActions := false
		if controllersResolved {
			for _, action := range bundle.ActionReferences {
				if action.ClassName == "" || (!action.Resolved && !ctx.apexMetadataFileHasAuraMethod(action.ClassName, action.Name)) {
					bundleResolved = false
					if action.ClassName != "" {
						missingActions = true
					}
					break
				}
			}
		}
		for _, file := range bundle.Files {
			cleanPath := filepath.Clean(file.Path)
			resolved[cleanPath] = bundleResolved || (controllersResolved && missingActions)
			if controllersResolved && missingActions {
				actionMetadata[cleanPath] = true
			}
		}
		cleanDir := filepath.Clean(bundle.Dir)
		resolved[cleanDir] = bundleResolved || (controllersResolved && missingActions)
		if controllersResolved && missingActions {
			actionMetadata[cleanDir] = true
		}
	}
	return resolved, actionMetadata
}

func suppressSupportedFinding(capability, symbol, evidence, file string) bool {
	switch capability {
	case "labels.localization":
		return supportedLabelSymbol(symbol, evidence)
	case "endpoint.metadata":
		return supportedEndpointSymbol(symbol, evidence)
	case "files.binary-content":
		return supportedFileSymbol(symbol, evidence)
	case "site.community-context":
		return supportedSiteCommunitySymbol(symbol, evidence)
	case "platform.cache-connectapi":
		return supportedCacheConnectAPISymbol(symbol, evidence, file)
	case "platform.auth-context":
		return supportedAuthSymbol(symbol, evidence)
	case "metadata.apex-deploy":
		return supportedMetadataAPISymbol(symbol, evidence)
	case "apex.callable-stub":
		return supportedCallableStubSymbol(symbol, evidence)
	case "analytics.report-execution":
		return supportedAnalyticsSymbol(symbol, evidence)
	default:
		return false
	}
}

func supportedEndpointSymbol(symbol, evidence string) bool {
	if strings.TrimSpace(symbol) == "" {
		return false
	}
	if callGuardedOutOfTests(evidence) {
		return true
	}
	if strings.Contains(evidence, "getEndpoint()") && !strings.Contains(evidence, ".setEndpoint(") {
		return true
	}
	for _, needle := range []string{
		".setStaticResource(", "MultiStaticResourceCalloutMock", "StaticResourceCalloutMock",
	} {
		if strings.Contains(evidence, needle) {
			return true
		}
	}
	return false
}

func supportedAnalyticsSymbol(symbol, evidence string) bool {
	if !analyticsNamespaceSymbol(symbol) {
		return true
	}
	for _, needle := range []string{
		"Reports.ReportFormat.", "reports.ReportFormat.",
	} {
		if strings.Contains(evidence, needle) {
			return true
		}
	}
	return false
}

func analyticsNamespaceSymbol(symbol string) bool {
	parts := strings.Split(strings.TrimSpace(symbol), ".")
	if len(parts) < 2 {
		return false
	}
	if parts[0] != "Reports" && parts[0] != "Analytics" {
		return false
	}
	member := strings.TrimSpace(parts[1])
	return member != "" && member[0] >= 'A' && member[0] <= 'Z'
}

func apexIdentifierStartsUpper(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}

func supportedLabelSymbol(symbol, evidence string) bool {
	symbol = strings.TrimSpace(symbol)
	evidence = strings.TrimSpace(evidence)
	return strings.EqualFold(symbol, "get") && (strings.Contains(evidence, "System.Label.get(") || strings.Contains(evidence, "Label.get("))
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

func suppressApexStubSelfReference(capability, rel, symbol, evidence string) bool {
	if capability != "custommetadata.legacy-records" {
		return false
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "stubs/apex-sobject-stubs/") {
		return false
	}
	symbol = strings.TrimSpace(symbol)
	if symbol == "" || baseNoExt(rel) != symbol {
		return false
	}
	trimmed := strings.TrimSpace(evidence)
	return strings.Contains(trimmed, " class "+symbol+" ") ||
		strings.HasPrefix(trimmed, "public "+symbol+"(") ||
		strings.HasPrefix(trimmed, "global "+symbol+"(")
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
		"Site.getDomain", "Site.getName", "Site.getPrefix",
		"Site.getMasterLabel", "Site.isRegistrationEnabled", "Site.getErrorMessage",
		"Site.getErrorDescription", "Site.forgotPassword", "Site.login",
		"Site.changePassword", "Site.validatePassword", "Site.createExternalUser",
		"Site.createPortalUser", "Site.isValidUsername", "Site.setExperienceId",
		"Site.isLoginEnabled", "Site.ExternalUserCreateException", "Site.Id",
		"Site.MasterLabel", "Site.Name", "Site testSite", "(Site)JSON.deserialize",
		"Network.getNetworkId", "Network.getNetworkID", "Network.getLoginUrl", "Network.communitiesLanding",
		"Network.getLogoutUrl", "Network.getSelfRegUrl", "Network.createExternalUserAsync",
		"System.Network.getLogoutUrl", "System.Network.getSelfRegUrl", "System.Network.createExternalUserAsync",
		"Network.forwardToAuthPage", "System.Network.forwardToAuthPage",
		"FROM Network", "From Network", "from Network", "Network.Status",
		"Network.sObjectType", "Network.Id", "Network.Name", "Network.SelfRegProfileId",
		"Network mockNetwork", "(Network)",
	} {
		if strings.Contains(evidence, needle) {
			return true
		}
	}
	return false
}

func supportedCacheConnectAPISymbol(symbol, evidence, file string) bool {
	if strings.HasPrefix(strings.TrimSpace(symbol), "ConnectApi.") && callGuardedOutOfTests(evidence) {
		return true
	}
	for _, needle := range []string{
		"Cache.", "ConnectApi.Organization.getSettings", "ConnectApi.Communities.getCommunity", "ConnectApi.Communities.getCommunities",
		"ConnectApi.OrganizationSettings", "ConnectApi.UserSettings", "ConnectApi.TimeZone",
		"ConnectApi.ChatterUsers.getFollowings", "ConnectApi.NamedCredentials.getExternalCredential",
		"ConnectApi.UserProfiles.setPhoto", "ConnectApi.UserProfiles.deletePhoto",
	} {
		if strings.Contains(evidence, needle) {
			return true
		}
	}
	if supportedConnectAPIMockWrapperMutation(file, evidence) {
		return true
	}
	if strings.HasPrefix(symbol, "ConnectApi.") && !connectAPIMethodCallEvidence(symbol, evidence) {
		return true
	}
	return false
}

func callGuardedOutOfTests(evidence string) bool {
	if strings.Contains(evidence, "||") {
		return false
	}
	return containsFoldIgnoringSpaces(evidence, "if(!test.isrunningtest())") ||
		containsFoldIgnoringSpaces(evidence, "if(!system.test.isrunningtest())")
}

func containsFoldIgnoringSpaces(text, needle string) bool {
	if needle == "" {
		return true
	}
	for start := 0; start < len(text); start++ {
		ti := start
		ni := 0
		for ni < len(needle) {
			for ti < len(text) && text[ti] == ' ' {
				ti++
			}
			if ti >= len(text) || lowerASCII(text[ti]) != lowerASCII(needle[ni]) {
				break
			}
			ti++
			ni++
		}
		if ni == len(needle) {
			return true
		}
	}
	return false
}

func lowerASCII(ch byte) byte {
	if ch >= 'A' && ch <= 'Z' {
		return ch + ('a' - 'A')
	}
	return ch
}

func supportedConnectAPIMockWrapperMutation(file, evidence string) bool {
	if !strings.EqualFold(filepath.Base(file), "ConnectApiWrapper.cls") {
		return false
	}
	for _, needle := range []string{
		"ConnectApi.NamedCredentials.createExternalCredential(",
		"ConnectApi.NamedCredentials.createNamedCredential(",
	} {
		if strings.Contains(evidence, needle) {
			return true
		}
	}
	return false
}

func connectAPIMethodCallEvidence(symbol, evidence string) bool {
	if symbol == "" || evidence == "" {
		return false
	}
	re := regexp.MustCompile(regexp.QuoteMeta(symbol) + `\.[A-Za-z_][A-Za-z0-9_]*\s*\(`)
	return re.FindStringIndex(evidence) != nil
}

func supportedAuthSymbol(symbol, evidence string) bool {
	for _, needle := range []string{
		"Auth.UserData", "Auth.VerificationResult", "Auth.VerificationMethod",
		"Auth.JWT ", "new Auth.JWT", "(Auth.JWT)",
		"Auth.RegistrationHandler", "Auth.User", "Auth.CommunitiesUtil.isGuestUser",
		"Auth.AuthToken.revokeAccess", "Auth.SessionManagement.getCurrentSession",
		"Auth.AuthConfiguration", "Auth.SamlJitHandler", "Auth.AuthProviderPluginClass",
		"Auth.AuthProviderPlugin", "Auth.AuthProviderCallbackState", "Auth.AuthProviderTokenResponse",
		"Auth.OAuthRefreshResult",
	} {
		if strings.Contains(evidence, needle) {
			return true
		}
	}
	return false
}

func supportedMetadataAPISymbol(symbol, evidence string) bool {
	if supportedMetadataGeneratedEnumDTO(symbol, evidence) {
		return true
	}
	for _, needle := range []string{
		"Metadata.CustomMetadata", "Metadata.CustomMetadataValue",
		"Metadata.CustomObject", "Metadata.CustomField",
		"Metadata.DeployCallback", "Metadata.DeployCallBack", "Metadata.DeployCallbackContext",
		"Metadata.DeployContainer", "Metadata.DeployDetails", "Metadata.DeployMessage",
		"Metadata.DeployResult", "Metadata.DeployStatus", "Metadata.Metadata", "Metadata.MetadataType",
		"Metadata.Operations.enqueueDeployment", "Metadata.Operations.checkDeployStatus", "Metadata.Operations.retrieve",
	} {
		if strings.Contains(symbol, needle) || strings.Contains(evidence, needle) {
			return true
		}
	}
	return false
}

func supportedMetadataGeneratedEnumDTO(symbol, evidence string) bool {
	if symbol == "" || !strings.HasPrefix(symbol, "Metadata.") {
		return false
	}
	trimmed := strings.TrimSpace(evidence)
	return strings.Contains(trimmed, " "+symbol+" valueOf(String") ||
		strings.Contains(trimmed, " List<"+symbol+"> values()") ||
		strings.Contains(trimmed, " List<"+symbol+"> values(")
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

func buildReport(projectRoot string, filesScanned int, findings []Finding, ctx scanContext) Report {
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
			Reports:              len(ctx.reports),
			Dashboards:           len(ctx.dashboards),
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
