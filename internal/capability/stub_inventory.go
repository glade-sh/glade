package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

const StubInventorySchemaVersion = 1

type StubInventoryReport struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Source        StubInventorySource    `json:"source"`
	Generated     StubInventoryGenerated `json:"generated"`
	Active        StubInventoryActive    `json:"active"`
	Gaps          StubInventoryGaps      `json:"gaps"`
	Namespaces    []StubInventoryCount   `json:"namespaces,omitempty"`
	Warnings      []string               `json:"warnings,omitempty"`
}

type StubInventorySource struct {
	Root                       string `json:"root"`
	SystemStubClasses          int    `json:"systemStubClasses"`
	SystemStubMethods          int    `json:"systemStubMethods"`
	SystemStubProperties       int    `json:"systemStubProperties"`
	SystemStubConstructors     int    `json:"systemStubConstructors"`
	SObjectStubClasses         int    `json:"sobjectStubClasses"`
	SObjectStubFieldTokens     int    `json:"sobjectStubFieldTokens"`
	SObjectStubApexProperties  int    `json:"sobjectStubApexProperties"`
	SObjectStubParentRelations int    `json:"sobjectStubParentRelationships"`
	SObjectStubChildRelations  int    `json:"sobjectStubChildRelationships"`
}

type StubInventoryGenerated struct {
	PlatformTypes        int `json:"platformTypes"`
	PlatformMethods      int `json:"platformMethods"`
	PlatformProperties   int `json:"platformProperties"`
	PlatformConstructors int `json:"platformConstructors"`
	PlatformInterfaces   int `json:"platformInterfaces"`
	PlatformEnums        int `json:"platformEnums"`
}

type StubInventoryActive struct {
	StandardObjects                  int `json:"standardObjects"`
	StandardObjectFields             int `json:"standardObjectFields"`
	StandardObjectFieldsWithFeatures int `json:"standardObjectFieldsWithFeatures"`
	StandardRelationships            int `json:"standardRelationships"`
	StandardKeyPrefixes              int `json:"standardKeyPrefixes"`
	StandardRecordTypes              int `json:"standardRecordTypes"`
}

type StubInventoryGaps struct {
	SystemSourceMissingGeneratedTypeCount     int      `json:"systemSourceMissingGeneratedTypeCount"`
	SystemSourceMissingGeneratedTypeSample    []string `json:"systemSourceMissingGeneratedTypeSample,omitempty"`
	SObjectSourceMissingActiveCount           int      `json:"sobjectSourceMissingActiveCount"`
	SObjectSourceMissingActiveSample          []string `json:"sobjectSourceMissingActiveSample,omitempty"`
	SObjectFieldMissingActiveCount            int      `json:"sobjectFieldMissingActiveCount"`
	SObjectFieldMissingActiveSample           []string `json:"sobjectFieldMissingActiveSample,omitempty"`
	SObjectFieldMissingFeatureGatedCount      int      `json:"sobjectFieldMissingFeatureGatedCount"`
	SObjectFieldMissingFeatureGatedSample     []string `json:"sobjectFieldMissingFeatureGatedSample,omitempty"`
	SObjectFieldMissingSupportedFeatureCount  int      `json:"sobjectFieldMissingSupportedFeatureCount"`
	SObjectFieldMissingSupportedFeatureSample []string `json:"sobjectFieldMissingSupportedFeatureSample,omitempty"`
}

type StubInventoryCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func BuildStubInventoryReport(sourceRoot string) (StubInventoryReport, error) {
	sourceRoot = strings.TrimSpace(sourceRoot)
	report := StubInventoryReport{SchemaVersion: StubInventorySchemaVersion}
	report.Source.Root = sourceRoot

	var systemFiles, sobjectFiles []string
	systemRoot := ""
	sobjectRoot := ""
	if sourceRoot == "" {
		report.Warnings = append(report.Warnings, "stub source root not provided; source counts omitted")
	} else {
		systemRoot = filepath.Join(sourceRoot, "apex-system-stubs")
		var err error
		systemFiles, err = collectStubFiles(systemRoot)
		if err != nil {
			report.Warnings = append(report.Warnings, err.Error())
		}
		sobjectRoot = filepath.Join(sourceRoot, "apex-sobject-stubs")
		sobjectFiles, err = collectStubFiles(sobjectRoot)
		if err != nil {
			report.Warnings = append(report.Warnings, err.Error())
		}
	}

	systemSourceNames := map[string]string{}
	namespaceCounts := map[string]int{}
	for _, file := range systemFiles {
		rel, _ := filepath.Rel(systemRoot, file)
		namespace := strings.Split(rel, string(filepath.Separator))[0]
		name := strings.TrimSuffix(filepath.Base(file), ".cls")
		if namespace == "" || namespace == "." {
			namespace = "(root)"
		}
		namespaceCounts[namespace]++
		if strings.EqualFold(namespace, "System") {
			systemSourceNames[normalizeInventoryKey(name)] = name
		} else {
			systemSourceNames[normalizeInventoryKey(namespace+"."+name)] = namespace + "." + name
		}
		methods, properties, constructors := countApexStubSurface(file)
		report.Source.SystemStubMethods += methods
		report.Source.SystemStubProperties += properties
		report.Source.SystemStubConstructors += constructors
	}
	report.Source.SystemStubClasses = len(systemFiles)
	report.Namespaces = sortedInventoryCounts(namespaceCounts)

	sobjectSourceNames := map[string]string{}
	sobjectSourceFields := map[string]map[string]string{}
	for _, file := range sobjectFiles {
		name := strings.TrimSuffix(filepath.Base(file), ".cls")
		sobjectSourceNames[normalizeInventoryKey(name)] = name
		fields, properties, parents, children := countSObjectStubSurface(file)
		report.Source.SObjectStubFieldTokens += fields
		report.Source.SObjectStubApexProperties += properties
		report.Source.SObjectStubParentRelations += parents
		report.Source.SObjectStubChildRelations += children
		sobjectSourceFields[name] = sobjectStubFieldNames(file)
	}
	report.Source.SObjectStubClasses = len(sobjectFiles)

	generatedNames := map[string]string{}
	for _, symbol := range typesys.StandardPlatformSymbols() {
		fullName := symbol.Name
		if symbol.Namespace != "" {
			fullName = symbol.Namespace + "." + symbol.Name
		}
		generatedNames[normalizeInventoryKey(fullName)] = fullName
		report.Generated.PlatformTypes++
		switch symbol.Kind {
		case apexast.DeclarationInterface:
			report.Generated.PlatformInterfaces++
		case apexast.DeclarationEnum:
			report.Generated.PlatformEnums++
		}
		for _, member := range symbol.Members {
			switch member.Kind {
			case apexast.DeclarationConstructor:
				report.Generated.PlatformConstructors++
			case apexast.DeclarationMethod:
				report.Generated.PlatformMethods++
			case apexast.DeclarationProperty:
				report.Generated.PlatformProperties++
			}
		}
	}

	activeObjects := BuildStandardObjectCoverageReport()
	report.Active.StandardObjects = activeObjects.Totals.Objects
	report.Active.StandardObjectFields = activeObjects.Totals.Fields
	report.Active.StandardRelationships = activeObjects.Totals.Relationships
	report.Active.StandardKeyPrefixes = activeObjects.Totals.KeyPrefixes
	report.Active.StandardRecordTypes = activeObjects.Totals.RecordTypes
	activeObjectNames := map[string]string{}
	activeFields := map[string]map[string]string{}
	featureActiveFields := map[string]map[string]string{}
	for _, object := range activeObjects.Objects {
		activeObjectNames[normalizeInventoryKey(object.Object)] = object.Object
		definition, ok := storage.StandardObjectDefinition(object.Object)
		if !ok {
			continue
		}
		activeFields[object.Object] = map[string]string{}
		for fieldName := range definition.Fields {
			activeFields[object.Object][normalizeInventoryKey(fieldName)] = fieldName
		}
		featureDefinition := storage.ObjectDefinition{APIName: object.Object}
		storage.EnsureStandardObjectFieldsForFeatures(&featureDefinition, stubInventoryAllFeatures())
		featureActiveFields[object.Object] = map[string]string{}
		for fieldName := range featureDefinition.Fields {
			featureActiveFields[object.Object][normalizeInventoryKey(fieldName)] = fieldName
		}
		report.Active.StandardObjectFieldsWithFeatures += len(featureDefinition.Fields)
	}

	missingSystem := missingInventoryNames(systemSourceNames, generatedNames)
	report.Gaps.SystemSourceMissingGeneratedTypeCount = len(missingSystem)
	report.Gaps.SystemSourceMissingGeneratedTypeSample = firstInventoryNames(missingSystem, 25)
	missingSObjects := missingInventoryNames(sobjectSourceNames, activeObjectNames)
	report.Gaps.SObjectSourceMissingActiveCount = len(missingSObjects)
	report.Gaps.SObjectSourceMissingActiveSample = firstInventoryNames(missingSObjects, 25)
	missingActiveFields := missingSObjectFields(sobjectSourceFields, activeObjectNames, activeFields)
	report.Gaps.SObjectFieldMissingActiveCount = len(missingActiveFields)
	report.Gaps.SObjectFieldMissingActiveSample = firstInventoryNames(missingActiveFields, 25)
	missingSupportedFeatureFields := missingSObjectFields(sobjectSourceFields, activeObjectNames, featureActiveFields)
	missingFeatureGatedFields := inventoryNameDifference(missingActiveFields, missingSupportedFeatureFields)
	report.Gaps.SObjectFieldMissingFeatureGatedCount = len(missingFeatureGatedFields)
	report.Gaps.SObjectFieldMissingFeatureGatedSample = firstInventoryNames(missingFeatureGatedFields, 25)
	report.Gaps.SObjectFieldMissingSupportedFeatureCount = len(missingSupportedFeatureFields)
	report.Gaps.SObjectFieldMissingSupportedFeatureSample = firstInventoryNames(missingSupportedFeatureFields, 25)

	return report, nil
}

func WriteStubInventoryJSON(w io.Writer, report StubInventoryReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteStubInventoryMarkdown(w io.Writer, report StubInventoryReport) error {
	if _, err := fmt.Fprintln(w, "# Stub Inventory"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\nSource: `%s`\n", report.Source.Root); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\n- System stub classes: %d\n", report.Source.SystemStubClasses); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Generated platform types: %d\n", report.Generated.PlatformTypes); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- SObject stub classes: %d\n", report.Source.SObjectStubClasses); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Active standard objects: %d\n", report.Active.StandardObjects); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- System source types missing generated type: %d\n", report.Gaps.SystemSourceMissingGeneratedTypeCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- SObject source classes missing active object: %d\n", report.Gaps.SObjectSourceMissingActiveCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- SObject fields missing active field: %d\n", report.Gaps.SObjectFieldMissingActiveCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- SObject fields missing only default feature gate: %d\n", report.Gaps.SObjectFieldMissingFeatureGatedCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- SObject fields missing supported-feature field: %d\n", report.Gaps.SObjectFieldMissingSupportedFeatureCount); err != nil {
		return err
	}
	return nil
}

func stubInventoryAllFeatures() []string {
	return []string{
		"PersonAccounts",
		"MultiCurrency",
		"Sites",
		"Communities",
		"StateAndCountryPicklist",
		"ContactsToMultipleAccounts",
		"PlatformCache",
		"EnableSetPasswordInApi",
		"AddCustomApps",
		"AnalyticsAdminPerms",
		"HealthCloud",
		"LightningExperience",
		"Chatter",
	}
}

func collectStubFiles(root string) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".cls") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, err
}

var (
	stubConstructorPattern = regexp.MustCompile(`^(?:global|public)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	stubMethodPattern      = regexp.MustCompile(`^(?:global|public)\s+(?:static\s+)?[A-Za-z_][A-Za-z0-9_.]*(?:<[^;{}()]+>)?\s+[A-Za-z_][A-Za-z0-9_]*\s*\(`)
	stubPropertyPattern    = regexp.MustCompile(`^(?:global|public)\s+(?:static\s+)?(?:(.*?)\s+)?[A-Za-z_][A-Za-z0-9_]*\s*\{\s*get;`)
	sobjectFieldPattern    = regexp.MustCompile(`\bSObjectField\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
)

func countApexStubSurface(file string) (methods, properties, constructors int) {
	source, err := os.ReadFile(file)
	if err != nil {
		return 0, 0, 0
	}
	className := strings.TrimSuffix(filepath.Base(file), ".cls")
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimSpace(line)
		if match := stubConstructorPattern.FindStringSubmatch(line); match != nil && match[1] == className {
			constructors++
			continue
		}
		if stubMethodPattern.MatchString(line) {
			methods++
			continue
		}
		if stubPropertyPattern.MatchString(line) {
			properties++
		}
	}
	return methods, properties, constructors
}

func countSObjectStubSurface(file string) (fields, properties, parents, children int) {
	source, err := os.ReadFile(file)
	if err != nil {
		return 0, 0, 0, 0
	}
	text := string(source)
	fields = len(sobjectFieldPattern.FindAllString(text, -1))
	properties = len(stubPropertyPattern.FindAllString(text, -1))
	parents = strings.Count(text, "Parent relationship for")
	children = strings.Count(text, "Child relationship")
	return fields, properties, parents, children
}

func sobjectStubFieldNames(file string) map[string]string {
	source, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	fields := map[string]string{}
	for _, match := range sobjectFieldPattern.FindAllStringSubmatch(string(source), -1) {
		if len(match) > 1 {
			fields[normalizeInventoryKey(match[1])] = match[1]
		}
	}
	return fields
}

func sortedInventoryCounts(counts map[string]int) []StubInventoryCount {
	out := make([]StubInventoryCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, StubInventoryCount{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func missingInventoryNames(source map[string]string, generated map[string]string) []string {
	var missing []string
	for key, name := range source {
		if _, ok := generated[key]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func missingSObjectFields(sourceFields map[string]map[string]string, activeObjects map[string]string, activeFields map[string]map[string]string) []string {
	var missing []string
	for objectName, fields := range sourceFields {
		activeObjectName, ok := activeObjects[normalizeInventoryKey(objectName)]
		if !ok {
			continue
		}
		objectActiveFields := activeFields[activeObjectName]
		for fieldKey, fieldName := range fields {
			if _, ok := objectActiveFields[fieldKey]; !ok {
				missing = append(missing, objectName+"."+fieldName)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

func inventoryNameDifference(names []string, excluded []string) []string {
	excludedSet := map[string]struct{}{}
	for _, name := range excluded {
		excludedSet[name] = struct{}{}
	}
	var out []string
	for _, name := range names {
		if _, ok := excludedSet[name]; !ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func firstInventoryNames(names []string, limit int) []string {
	if len(names) <= limit {
		return names
	}
	return append([]string(nil), names[:limit]...)
}

func normalizeInventoryKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
