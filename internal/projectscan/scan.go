package projectscan

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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
		status:              "unsupported",
		testBlocking:        true,
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

var textPatterns = []patternDef{
	{"visualforce.controller-test", "ApexClass", regexp.MustCompile(`\b(ApexPages\.|PageReference\b|Page\.[A-Za-z_][A-Za-z0-9_]*|StandardController\b|StandardSetController\b)`), 1},
	{"labels.localization", "ApexClass", regexp.MustCompile(`\b(System\.Label|Label\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"metadata.apex-deploy", "ApexClass", regexp.MustCompile(`\b(Metadata\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"site.community-context", "ApexClass", regexp.MustCompile(`\b(Site\.|Network\.|Community__mdt\b)`), 1},
	{"platform.cache-connectapi", "ApexClass", regexp.MustCompile(`\b(Cache\.|ConnectApi\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"platform.auth-context", "ApexClass", regexp.MustCompile(`\b(Auth\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"apex.callable-stub", "ApexClass", regexp.MustCompile(`\b(System\.Callable|Callable\b|System\.StubProvider|Test\.createStub|handleMethodCall\b)`), 1},
	{"files.binary-content", "ApexClass", regexp.MustCompile(`\b(ContentVersion\b|ContentDocument\b|ContentDocumentLink\b|Attachment\b|Document\b|Blob\b|base64Encode|base64Decode)`), 1},
	{"custommetadata.legacy-records", "ApexClass", regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*__mdt)\b`), 1},
	{"lwc.controller-test", "LWCJavaScript", regexp.MustCompile(`@salesforce/apex/([A-Za-z_][A-Za-z0-9_]*\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"lwc.controller-test", "LWCJavaScript", regexp.MustCompile(`\b@wire\s*\(\s*([A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"labels.localization", "LWCJavaScript", regexp.MustCompile(`@salesforce/label/([A-Za-z0-9_./]+)`), 1},
	{"staticresources.urlfor", "LWCJavaScript", regexp.MustCompile(`@salesforce/resourceUrl/([A-Za-z_][A-Za-z0-9_]*)`), 1},
	{"ui.presentation-metadata", "LWCJavaScript", regexp.MustCompile(`@salesforce/schema/([A-Za-z0-9_./]+)|lightning/(navigation|uiRecordApi|uiObjectInfoApi)`), 1},
	{"staticresources.urlfor", "Visualforce", regexp.MustCompile(`(\$Resource\.[A-Za-z_][A-Za-z0-9_]*|URLFOR\s*\(\s*\$Resource\.[A-Za-z_][A-Za-z0-9_]*)`), 1},
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
		findings = append(findings, classifyByPath(rel, path)...)
		if isTextScannable(rel) {
			lineFindings, err := scanTextFile(path, rel)
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

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".sfdx", ".sf", ".claude", "node_modules", ".idea", ".vscode":
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

func classifyByPath(rel, path string) []Finding {
	lower := strings.ToLower(rel)
	var findings []Finding
	add := func(capability, metadataType, symbol string) {
		findings = append(findings, makeFinding(capability, rel, 0, metadataType, symbol, "metadata file"))
	}

	switch {
	case strings.HasSuffix(lower, ".page"):
		add("visualforce.controller-test", "VisualforcePage", baseNoExt(path))
	case strings.HasSuffix(lower, ".component"):
		add("visualforce.component-test", "VisualforceComponent", baseNoExt(path))
	case strings.Contains(lower, "/aura/"):
		add("aura.controller-test", "AuraBundle", auraOrLWCBundle(rel, "aura"))
	case strings.Contains(lower, "/lwc/"):
		add("lwc.controller-test", "LWCBundle", auraOrLWCBundle(rel, "lwc"))
	case strings.HasSuffix(lower, ".workflow-meta.xml"), strings.HasSuffix(lower, ".workflow"):
		add("workflow.save-order", "Workflow", baseNoExt(path))
	case strings.HasSuffix(lower, ".flow-meta.xml"), strings.HasSuffix(lower, ".flow"):
		add("flow.save-order", "Flow", baseNoExt(path))
	case strings.HasSuffix(lower, ".labels-meta.xml"), strings.HasSuffix(lower, ".labels"):
		add("labels.localization", "CustomLabels", baseNoExt(path))
	case strings.HasSuffix(lower, ".email"), strings.HasSuffix(lower, ".email-meta.xml"):
		add("email.templates", "EmailTemplate", baseNoExt(path))
	case strings.HasSuffix(lower, ".object"):
		add("metadata.legacy-source", "LegacyObject", baseNoExt(path))
	case strings.HasSuffix(lower, ".md") && isCustomMetadataPath(lower):
		add("custommetadata.legacy-records", "LegacyCustomMetadata", baseNoExt(path))
	case strings.HasSuffix(lower, ".resource"), strings.HasSuffix(lower, ".resource-meta.xml"), strings.HasSuffix(lower, ".staticresource-meta.xml"):
		add("staticresources.urlfor", "StaticResource", baseNoExt(path))
	case strings.HasSuffix(lower, ".asset"), strings.HasSuffix(lower, ".asset-meta.xml"):
		add("staticresources.urlfor", "ContentAsset", baseNoExt(path))
	case strings.HasSuffix(lower, ".namedcredential"), strings.HasSuffix(lower, ".namedcredential-meta.xml"):
		add("endpoint.metadata", "NamedCredential", baseNoExt(path))
	case strings.HasSuffix(lower, ".remotesite"), strings.HasSuffix(lower, ".remotesite-meta.xml"):
		add("endpoint.metadata", "RemoteSiteSetting", baseNoExt(path))
	case hasAnySuffix(lower, ".layout", ".layout-meta.xml", ".profile", ".profile-meta.xml", ".permissionset", ".permissionset-meta.xml", ".tab", ".tab-meta.xml", ".weblink", ".weblink-meta.xml", ".quickaction-meta.xml", ".globalvalueset-meta.xml", ".standardvalueset-meta.xml", ".flexipage", ".flexipage-meta.xml", ".application", ".app-meta.xml"):
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

func scanTextFile(path, rel string) ([]Finding, error) {
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
		for _, pattern := range textPatterns {
			if pattern.metadataType != metadataType && !(metadataType == "ApexClass" && pattern.metadataType == "ApexClass") {
				continue
			}
			matches := pattern.re.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				symbol := ""
				if pattern.symbolGroup > 0 && pattern.symbolGroup < len(match) {
					symbol = strings.TrimSpace(match[pattern.symbolGroup])
				}
				if symbol == "" && len(match) > 0 {
					symbol = strings.TrimSpace(match[0])
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

func metadataTypeForText(rel string) string {
	lower := strings.ToLower(rel)
	switch {
	case strings.HasSuffix(lower, ".page"), strings.HasSuffix(lower, ".component"):
		return "Visualforce"
	case strings.Contains(lower, "/lwc/") && (strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".html")):
		return "LWCJavaScript"
	default:
		return "ApexClass"
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
