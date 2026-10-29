package project

import (
	"encoding/json"
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/config"
)

// OrgSetupSuggestion reports source-backed setup evidence and local config
// mappings. It is advice only: nothing here changes runtime behavior.
type OrgSetupSuggestion struct {
	// Kind identifies the setup evidence or configuration mapping.
	Kind string `json:"kind"`
	// Feature names the feature for "feature", "unsupportedFeature", or "scratchFeature".
	Feature string `json:"feature,omitempty"`
	// Names lists the API names for permission sets, record types, roles, or dependencies.
	Names []string `json:"names,omitempty"`
	// Reason states the evidence in plain words.
	Reason string `json:"reason"`
	// Files lists up to three project-relative files that hold the evidence.
	Files []string `json:"files,omitempty"`
	// YAML is the glade.yml block to add, a feature mapping, or a note.
	YAML string `json:"yaml,omitempty"`
}

type featureEvidence struct {
	feature            string
	reason             string
	covered            []string
	pattern            *regexp.Regexp
	unsupported        bool
	customPersonFields map[string]bool
}

var featureEvidenceRules = []featureEvidence{
	{
		feature: "PersonAccounts",
		reason:  "source uses Person Account fields (IsPersonAccount, PersonContactId, Person* contact fields, or __pc fields)",
		covered: []string{"PersonAccounts"},
		pattern: regexp.MustCompile(`(?i)\b` + personAccountFieldPattern + `\b`),
	},
	{
		feature: "Communities",
		reason:  "source uses Network or Experience Cloud users (IsPortalEnabled or an external UserType)",
		covered: []string{"Communities", "FlowSites", "Sites"},
	},
	{
		feature: "Sites",
		reason:  "source calls the Site class",
		covered: []string{"Sites"},
		pattern: regexp.MustCompile(`\bSite\.(getBaseUrl|getBaseSecureUrl|getBaseRequestUrl|getCurrentSiteUrl|getDomain|getErrorDescription|getErrorMessage|getOriginalUrl|getPathPrefix|getSiteId|getSiteType|getTemplate|isLoginEnabled|isPasswordExpired|isRegistrationEnabled|login|forgotPassword|changePassword|createExternalUser|createPersonAccountPortalUser)\b`),
	},
	{
		feature: "MultiCurrency",
		reason:  "source uses CurrencyIsoCode, CurrencyType, DatedConversionRate, or convertCurrency",
		covered: []string{"MultiCurrency"},
		pattern: regexp.MustCompile(`(?i)\bCurrencyIsoCode\b|\bFROM\s+(CurrencyType|DatedConversionRate)\b|\bconvertCurrency\s*\(`),
	},
	{
		feature:     "Territory2",
		reason:      "source uses Territory2; Enterprise Territory Management setup is not modelled by org.features",
		unsupported: true,
	},
}

var (
	suggestSkipDirs = map[string]bool{
		".git": true, "node_modules": true, ".glade": true, ".sfdx": true, ".sf": true,
		".vscode": true, ".husky": true, "dist": true, "coverage": true, "__tests__": true,
	}
	suggestExts = map[string]bool{
		".cls": true, ".trigger": true, ".page": true, ".component": true,
		".html": true, ".js": true,
	}
	suggestSuffixes          = []string{".flow-meta.xml", ".field-meta.xml", ".object-meta.xml", ".permissionset-meta.xml", ".app-meta.xml"}
	permSetNameRE            = regexp.MustCompile(`'([A-Za-z][A-Za-z0-9_]*)'`)
	soqlBlockRE              = regexp.MustCompile(`(?is)\[\s*SELECT\b([^\]]*)\]`)
	fromObjectRE             = regexp.MustCompile(`(?i)\bFROM\s+([A-Za-z_][A-Za-z0-9_]*)`)
	recordTypeFilterRE       = regexp.MustCompile(`(?i)\bRecordType\.(DeveloperName|Name)\s*=\s*'([^']+)'`)
	directRecordTypeFilterRE = regexp.MustCompile(`(?i)\b((?:[A-Za-z_][A-Za-z0-9_]*\s*\.\s*)*)(DeveloperName|Name)\s*=\s*'([^']+)'`)
	sobjectTypeFilterRE      = regexp.MustCompile(`(?i)\bSObjectType\s*=\s*'([^']+)'`)
	roleNameFilterRE         = regexp.MustCompile(`(?i)\b(?:DeveloperName|Name)\s*=\s*'([^']+)'`)
	userRoleCtorRE           = regexp.MustCompile(`(?is)\bnew\s+UserRole\s*\(([^)]*)\)`)
	roleFieldValueRE         = regexp.MustCompile(`(?i)\b(?:DeveloperName|Name)\s*=\s*'([^']+)'`)
	recordTypeDescribeRE     = regexp.MustCompile(`(?i)\bSchema\.SObjectType\.([A-Za-z_][A-Za-z0-9_]*)\s*(?:\.getDescribe\s*\(\s*\))?\.getRecordTypeInfosBy(DeveloperName|Name)\s*\(\s*\)\s*\.get\s*\(\s*'([^']+)'`)
	recordTypeSObjectRE      = regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)\.SObjectType\.getDescribe\s*\(\s*\)\.getRecordTypeInfosBy(DeveloperName|Name)\s*\(\s*\)\s*\.get\s*\(\s*'([^']+)'`)
)

var mappedScratchFeatures = map[string]bool{
	"personaccounts":             true,
	"multicurrency":              true,
	"stateandcountrypicklist":    true,
	"communities":                true,
	"flowsites":                  true,
	"sites":                      true,
	"contactstomultipleaccounts": true,
	"platformcache":              true,
	"providerfreeplatformcache":  true,
	"quotes":                     true,
	"healthcloud":                true,
	"chatter":                    true,
	"lightningexperience":        true,
	"enablesetpasswordinapi":     true,
	"analyticsadminperms":        true,
}

const suggestMaxFileBytes = 2 << 20

// SuggestOrgSetup scans project source and metadata for explicit setup evidence,
// and reports scratch features and dependency diagnostics without changing behavior.
func SuggestOrgSetup(root string) []OrgSetupSuggestion {
	loaded, loadErr := Load(root)
	customPersonFields := setupCustomPersonAccountFields(loaded)
	declared := map[string]bool{}
	effective := OrgShapeFeatures(root)
	if loadErr == nil {
		effective = EffectiveOrgShapeFeatures(loaded)
	}
	for _, f := range effective {
		declared[f] = true
	}
	var configured []string
	var assignedSets []string
	if cfg, _, err := config.LoadNearest(root); err == nil {
		configured = append(configured, cfg.Org.Features...)
		assignedSets = append(assignedSets, cfg.Test.PermissionSets...)
	}
	assigned := map[string]bool{}
	for _, n := range assignedSets {
		assigned[n] = true
	}

	files := map[string][]string{}
	permSets := map[string]bool{}
	var permSetFiles []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && suggestSkipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".permissionset-meta.xml") {
			permSets[strings.TrimSuffix(name, ".permissionset-meta.xml")] = true
		}
		if !suggestScannable(name) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > suggestMaxFileBytes {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(data)
		if ext := strings.ToLower(filepath.Ext(name)); ext == ".cls" || ext == ".trigger" {
			text = stripApexComments(text)
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		for _, rule := range featureEvidenceRules {
			rule.customPersonFields = customPersonFields
			if matchesSetupFeature(rule, text, strings.HasSuffix(name, ".xml")) {
				files[rule.feature] = append(files[rule.feature], rel)
			}
		}
		if strings.HasSuffix(name, ".cls") && strings.Contains(text, "PermissionSetAssignment") {
			permSetFiles = append(permSetFiles, rel+"\x00"+text)
		}
		return nil
	})

	var out []OrgSetupSuggestion
	var newFeatures []string
	var featureSuggestions []OrgSetupSuggestion
	var unsupportedSuggestions []OrgSetupSuggestion
	for _, rule := range featureEvidenceRules {
		hits := files[rule.feature]
		if len(hits) == 0 || anyDeclared(declared, rule.covered) {
			continue
		}
		sort.Strings(hits)
		if rule.unsupported {
			unsupportedSuggestions = append(unsupportedSuggestions, OrgSetupSuggestion{
				Kind: "unsupportedFeature", Feature: rule.feature, Reason: rule.reason,
				Files: firstN(hits, 3),
			})
			continue
		}
		featureSuggestions = append(featureSuggestions, OrgSetupSuggestion{
			Kind: "feature", Feature: rule.feature, Reason: rule.reason,
			Files: firstN(hits, 3),
		})
		newFeatures = append(newFeatures, rule.feature)
	}
	if len(featureSuggestions) > 0 {
		list := dedupeStrings(append(append([]string{}, configured...), newFeatures...))
		yaml := "org:\n  features: [" + strings.Join(list, ", ") + "]"
		for i := range featureSuggestions {
			featureSuggestions[i].YAML = yaml
		}
		out = append(out, featureSuggestions...)
	}
	out = append(out, unsupportedSuggestions...)

	needed := map[string][]string{}
	for _, entry := range permSetFiles {
		rel, text, _ := strings.Cut(entry, "\x00")
		for _, m := range permSetNameRE.FindAllStringSubmatch(text, -1) {
			if permSets[m[1]] && !assigned[m[1]] {
				needed[m[1]] = append(needed[m[1]], rel)
			}
		}
	}
	if len(needed) > 0 {
		var names, hits []string
		for n, fs := range needed {
			names = append(names, n)
			hits = append(hits, fs...)
		}
		sort.Strings(names)
		sort.Strings(hits)
		list := dedupeStrings(append(append([]string{}, assignedSets...), names...))
		out = append(out, OrgSetupSuggestion{
			Kind: "permissionSet", Names: names,
			Reason: "tests assign project permission sets by name; the running test user does not have them unless declared",
			Files:  firstN(dedupeStrings(hits), 3),
			YAML:   "test:\n  permissionSets: [" + strings.Join(list, ", ") + "]",
		})
	}

	if loadErr == nil {
		out = append(out, missingMetadataSuggestions(root, loaded)...)
		out = append(out, missingDependencySuggestions(loaded)...)
	}
	out = append(out, scratchFeatureSuggestions(root)...)
	return out
}

type setupNameReference struct {
	object string
	name   string
}

func missingMetadataSuggestions(root string, p Project) []OrgSetupSuggestion {
	if p.Root != "" {
		root = p.Root
	}
	recordTypeMetadata := recordTypeMetadataNames(root, p.RecordTypeFiles)
	roleMetadata := roleMetadataNames(p.RoleFiles)
	missingRecordTypes := map[string][]string{}
	missingRoles := map[string][]string{}
	recordTypeDisplay := map[string]string{}
	roleDisplay := map[string]string{}

	for _, sourcePath := range p.ApexFiles {
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			continue
		}
		source := stripApexComments(string(data))
		rel := suggestRelativePath(root, sourcePath)
		for _, ref := range recordTypeReferences(source) {
			if ref.object == "" || strings.EqualFold(ref.name, "Master") || hasMetadataName(recordTypeMetadata[normalizedSetupName(ref.object)], ref.name) {
				continue
			}
			label := ref.object + "." + ref.name
			key := normalizedSetupName(ref.object) + "\x00" + normalizedSetupName(ref.name)
			missingRecordTypes[key] = append(missingRecordTypes[key], rel)
			if recordTypeDisplay[key] == "" {
				recordTypeDisplay[key] = label
			}
		}
		for _, name := range roleReferences(source) {
			if hasMetadataName(roleMetadata, name) {
				continue
			}
			key := normalizedSetupName(name)
			missingRoles[key] = append(missingRoles[key], rel)
			if roleDisplay[key] == "" {
				roleDisplay[key] = name
			}
		}
	}

	var out []OrgSetupSuggestion
	if len(missingRecordTypes) > 0 {
		names, files := sortedMissingNames(missingRecordTypes, func(key string) string { return recordTypeDisplay[key] })
		out = append(out, OrgSetupSuggestion{
			Kind: "recordType", Names: names,
			Reason: "Apex source references record types whose metadata files are absent from the project.",
			Files:  firstN(files, 3),
		})
	}
	if len(missingRoles) > 0 {
		names, files := sortedMissingNames(missingRoles, func(key string) string { return roleDisplay[key] })
		out = append(out, OrgSetupSuggestion{
			Kind: "role", Names: names,
			Reason: "Apex source references roles whose metadata files are absent from the project.",
			Files:  firstN(files, 3),
		})
	}
	return out
}

func recordTypeReferences(source string) []setupNameReference {
	var out []setupNameReference
	for _, query := range soqlBlockRE.FindAllStringSubmatch(source, -1) {
		object := ""
		alias := ""
		filter := recordTypeFilterRE
		if match := setupQueryRootRE.FindStringSubmatch(query[1]); len(match) == 3 {
			object = match[1]
			alias = match[2]
			switch strings.ToUpper(alias) {
			case "WHERE", "WITH", "ORDER", "LIMIT", "OFFSET", "GROUP", "HAVING", "FOR":
				alias = ""
			}
		}
		if strings.EqualFold(object, "RecordType") {
			filter = directRecordTypeFilterRE
			if match := sobjectTypeFilterRE.FindStringSubmatch(query[1]); len(match) == 2 {
				object = match[1]
			} else {
				object = ""
			}
		}
		if object == "" {
			continue
		}
		for _, match := range filter.FindAllStringSubmatch(query[1], -1) {
			name := match[2]
			if filter == directRecordTypeFilterRE {
				qualifier := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(match[1]), "."))
				if qualifier != "" && !strings.EqualFold(qualifier, "RecordType") && !strings.EqualFold(qualifier, alias) {
					continue
				}
				name = match[3]
			}
			out = append(out, setupNameReference{object: object, name: strings.TrimSpace(name)})
		}
	}
	for _, pattern := range []*regexp.Regexp{recordTypeDescribeRE, recordTypeSObjectRE} {
		for _, match := range pattern.FindAllStringSubmatch(source, -1) {
			out = append(out, setupNameReference{object: match[1], name: strings.TrimSpace(match[3])})
		}
	}
	return out
}

func roleReferences(source string) []string {
	var out []string
	for _, query := range soqlBlockRE.FindAllStringSubmatch(source, -1) {
		match := fromObjectRE.FindStringSubmatch(query[1])
		if len(match) != 2 || !strings.EqualFold(match[1], "UserRole") {
			continue
		}
		for _, name := range roleNameFilterRE.FindAllStringSubmatch(query[1], -1) {
			out = append(out, strings.TrimSpace(name[1]))
		}
	}
	for _, constructor := range userRoleCtorRE.FindAllStringSubmatch(source, -1) {
		for _, name := range roleFieldValueRE.FindAllStringSubmatch(constructor[1], -1) {
			out = append(out, strings.TrimSpace(name[1]))
		}
	}
	return dedupeStrings(out)
}

func recordTypeMetadataNames(root string, paths []string) map[string]map[string]bool {
	index := map[string]map[string]bool{}
	for _, path := range paths {
		object := recordTypeObjectFromPath(root, path)
		if object == "" {
			continue
		}
		base := metadataBaseName(path, ".recordtype-meta.xml")
		var data struct {
			FullName string `xml:"fullName"`
			Label    string `xml:"label"`
		}
		content, err := os.ReadFile(path)
		if err == nil {
			_ = xml.Unmarshal(content, &data)
		}
		for _, name := range []string{base, data.FullName, data.Label} {
			if strings.TrimSpace(name) != "" {
				addMetadataName(index, object, name)
			}
		}
	}
	return index
}

func recordTypeObjectFromPath(root, path string) string {
	rel := suggestRelativePath(root, path)
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i := 0; i+2 < len(parts); i++ {
		if strings.EqualFold(parts[i], "objects") && strings.EqualFold(parts[i+2], "recordTypes") {
			return parts[i+1]
		}
	}
	return ""
}

func roleMetadataNames(paths []string) map[string]bool {
	index := map[string]bool{}
	for _, path := range paths {
		base := metadataBaseName(path, ".role-meta.xml")
		var data struct {
			FullName string `xml:"fullName"`
			Name     string `xml:"name"`
		}
		content, err := os.ReadFile(path)
		if err == nil {
			_ = xml.Unmarshal(content, &data)
		}
		for _, name := range []string{base, data.FullName, data.Name} {
			if strings.TrimSpace(name) != "" {
				index[normalizedSetupName(name)] = true
			}
		}
	}
	return index
}

func metadataBaseName(path, suffix string) string {
	base := filepath.Base(path)
	if len(base) >= len(suffix) && strings.EqualFold(base[len(base)-len(suffix):], suffix) {
		return base[:len(base)-len(suffix)]
	}
	return base
}

func addMetadataName(index map[string]map[string]bool, object, name string) {
	key := normalizedSetupName(object)
	if index[key] == nil {
		index[key] = map[string]bool{}
	}
	index[key][normalizedSetupName(name)] = true
}

func hasMetadataName(index map[string]bool, name string) bool {
	return index[normalizedSetupName(name)]
}

func normalizedSetupName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func sortedMissingNames(missing map[string][]string, display func(string) string) ([]string, []string) {
	keys := make([]string, 0, len(missing))
	for key := range missing {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	names := make([]string, 0, len(keys))
	var files []string
	for _, key := range keys {
		names = append(names, display(key))
		files = append(files, missing[key]...)
	}
	return names, dedupeStrings(sortedStrings(files))
}

func missingDependencySuggestions(p Project) []OrgSetupSuggestion {
	missing := map[string]bool{}
	for _, diagnostic := range p.DependencyDiagnostics {
		if diagnostic.Code == "dependency_missing" && strings.TrimSpace(diagnostic.Namespace) != "" {
			missing[strings.TrimSpace(diagnostic.Namespace)] = true
		}
	}
	if len(missing) == 0 {
		return nil
	}
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return []OrgSetupSuggestion{{
		Kind: "dependency", Names: names,
		Reason: "sfdx-project.json declares these package dependencies, but their local source was not found; configure a dependency path.",
		Files:  []string{"sfdx-project.json"},
		YAML:   "project:\n  dependencyPaths: [\"../path/to/dependencies\"]",
	}}
}

func scratchFeatureSuggestions(root string) []OrgSetupSuggestion {
	features := map[string][]string{}
	for _, path := range scratchOrgConfigFiles(root) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var definition scratchOrgDefinition
		if json.Unmarshal(data, &definition) != nil {
			continue
		}
		rel := suggestRelativePath(root, path)
		for _, feature := range definition.Features {
			feature = strings.TrimSpace(feature)
			if feature != "" {
				features[feature] = append(features[feature], rel)
			}
		}
	}
	if len(features) == 0 {
		return nil
	}

	var configured []string
	if cfg, _, err := config.LoadNearest(root); err == nil {
		configured = append(configured, cfg.Org.Features...)
	}
	names := make([]string, 0, len(features))
	for name := range features {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []OrgSetupSuggestion
	for _, name := range names {
		mapping := "not modelled"
		if isMappedScratchFeature(name) {
			lineFeatures := append([]string(nil), configured...)
			if !containsFolded(lineFeatures, name) {
				lineFeatures = append(lineFeatures, name)
			}
			lineFeatures = dedupeStrings(lineFeatures)
			mapping = "org:\n  features: [" + strings.Join(lineFeatures, ", ") + "]"
		}
		reason := "scratch definition requests this feature"
		if mapping == "not modelled" {
			reason += "; Glade does not model it"
		} else if containsFolded(configured, name) {
			reason += "; it is already present in glade.yml"
		} else {
			reason += "; Glade maps it through org.features"
		}
		out = append(out, OrgSetupSuggestion{
			Kind: "scratchFeature", Feature: name, Reason: reason,
			Files: firstN(dedupeStrings(sortedStrings(features[name])), 3), YAML: mapping,
		})
	}
	return out
}

func isMappedScratchFeature(feature string) bool {
	name := strings.ToLower(strings.TrimSpace(feature))
	if mappedScratchFeatures[name] {
		return true
	}
	if strings.HasPrefix(name, "healthcloud") {
		return true
	}
	const customAppsPrefix = "addcustomapps:"
	if !strings.HasPrefix(name, customAppsPrefix) || len(name) == len(customAppsPrefix) {
		return false
	}
	for _, char := range name[len(customAppsPrefix):] {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func containsFolded(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

func suggestRelativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func stripApexComments(source string) string {
	data := []byte(source)
	const (
		normal = iota
		singleQuoted
		doubleQuoted
		lineComment
		blockComment
	)
	state := normal
	for i := 0; i < len(data); i++ {
		switch state {
		case normal:
			if data[i] == '/' && i+1 < len(data) && data[i+1] == '/' {
				data[i], data[i+1] = ' ', ' '
				i++
				state = lineComment
			} else if data[i] == '/' && i+1 < len(data) && data[i+1] == '*' {
				data[i], data[i+1] = ' ', ' '
				i++
				state = blockComment
			} else if data[i] == '\'' {
				state = singleQuoted
			} else if data[i] == '"' {
				state = doubleQuoted
			}
		case singleQuoted, doubleQuoted:
			if data[i] == '\\' {
				i++
			} else if (state == singleQuoted && data[i] == '\'') || (state == doubleQuoted && data[i] == '"') {
				state = normal
			}
		case lineComment:
			if data[i] == '\n' {
				state = normal
			} else {
				data[i] = ' '
			}
		case blockComment:
			if data[i] == '*' && i+1 < len(data) && data[i+1] == '/' {
				data[i], data[i+1] = ' ', ' '
				i++
				state = normal
			} else if data[i] != '\n' {
				data[i] = ' '
			}
		}
	}
	return string(data)
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func suggestScannable(name string) bool {
	if suggestExts[strings.ToLower(filepath.Ext(name))] {
		return true
	}
	for _, s := range suggestSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

func anyDeclared(declared map[string]bool, names []string) bool {
	for _, n := range names {
		if declared[n] {
			return true
		}
	}
	return false
}

func firstN(values []string, n int) []string {
	if len(values) > n {
		return values[:n]
	}
	return values
}
