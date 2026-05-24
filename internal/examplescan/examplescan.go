package examplescan

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/projectscan"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Report is the top-level machine-readable example-project inventory.
type Report struct {
	Name         string                `json:"name"`
	Root         string                `json:"root"`
	SourceLayout string                `json:"sourceLayout"`
	Counts       AssetCounts           `json:"counts"`
	Constructs   ApexConstructs        `json:"constructs"`
	RuntimeUsage RuntimeUsage          `json:"runtimeUsage"`
	Diagnostics  DiagnosticBreakdown   `json:"diagnostics"`
	TopBlockers  []Blocker             `json:"topBlockers"`
	Surfaces     []projectscan.Surface `json:"surfaces,omitempty"`
}

// AssetCounts holds metadata asset counts by type.
type AssetCounts struct {
	ApexClasses           int `json:"apexClasses"`
	ApexTriggers          int `json:"apexTriggers"`
	TestClasses           int `json:"testClasses"`
	VisualforcePages      int `json:"visualforcePages"`
	VisualforceComponents int `json:"visualforceComponents"`
	AuraComponents        int `json:"auraComponents"`
	LWCComponents         int `json:"lwcComponents"`
	Objects               int `json:"objects"`
	Fields                int `json:"fields"`
	FieldSets             int `json:"fieldSets"`
	RecordTypes           int `json:"recordTypes"`
	ValidationRules       int `json:"validationRules"`
	Workflows             int `json:"workflows"`
	Flows                 int `json:"flows"`
	CustomMetadata        int `json:"customMetadata"`
	NamedCredentials      int `json:"namedCredentials"`
	RemoteSites           int `json:"remoteSites"`
	StaticResources       int `json:"staticResources"`
	EmailTemplates        int `json:"emailTemplates"`
	Labels                int `json:"labels"`
	Profiles              int `json:"profiles"`
	PermissionSets        int `json:"permissionSets"`
	PermissionAssignments int `json:"permissionAssignments"`
}

// ApexConstructs inventories language constructs.
type ApexConstructs struct {
	Classes           int      `json:"classes"`
	Interfaces        int      `json:"interfaces"`
	Enums             int      `json:"enums"`
	NestedTypes       int      `json:"nestedTypes"`
	Annotations       []string `json:"annotations"`
	SharingModes      []string `json:"sharingModes"`
	GlobalClasses     int      `json:"globalClasses"`
	WebServiceMethods int      `json:"webServiceMethods"`
	AsyncInterfaces   []string `json:"asyncInterfaces"`
}

// RuntimeUsage inventories SOQL, DML, trigger, and platform API references.
type RuntimeUsage struct {
	SOQLFeatures      []string `json:"soqlFeatures"`
	DMLFeatures       []string `json:"dmlFeatures"`
	TriggerOperations []string `json:"triggerOperations"`
	NamespaceRefs     []string `json:"namespaceReferences"`
}

// DiagnosticBreakdown groups diagnostics by category.
type DiagnosticBreakdown struct {
	ObservedBlockers         []DiagnosticSummary `json:"observedBlockers"`
	ObservedRuntimeGaps      []DiagnosticSummary `json:"observedRuntimeGaps"`
	ObservedParityGaps       []DiagnosticSummary `json:"observedParityGaps"`
	UnobservedParityFollowup []DiagnosticSummary `json:"unobservedParityFollowup"`
}

// DiagnosticSummary is a rolled-up diagnostic entry.
type DiagnosticSummary struct {
	CapabilityID string   `json:"capabilityId,omitempty"`
	Code         string   `json:"code"`
	Message      string   `json:"message"`
	Count        int      `json:"count"`
	Files        []string `json:"files,omitempty"`
}

// Blocker represents a top blocking capability.
type Blocker struct {
	CapabilityID  string `json:"capabilityId"`
	Title         string `json:"title"`
	Count         int    `json:"count"`
	AffectedFiles int    `json:"affectedFiles"`
}

// Options controls the scan behavior.
type Options struct {
	Name           string
	RunSema        bool
	RunSurfaceScan bool
	SkipDirNames   map[string]bool
}

// Scan discovers and inventories a project at root.
func Scan(root string, opts Options) (Report, error) {
	if opts.Name == "" {
		opts.Name = filepath.Base(root)
	}
	if opts.SkipDirNames == nil {
		opts.SkipDirNames = map[string]bool{
			".git": true, ".sfdx": true, ".sf": true,
			".claude": true, "node_modules": true,
			".idea": true, ".vscode": true,
		}
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}

	// Load via project package.
	proj, err := project.Load(absRoot)
	if err != nil {
		return Report{}, err
	}

	layout := detectLayout(absRoot, proj)

	report := Report{
		Name:         opts.Name,
		Root:         absRoot,
		SourceLayout: layout,
	}

	// Populate counts from project.Load results.
	report.Counts.ApexClasses = countClasses(proj)
	report.Counts.ApexTriggers = countTriggers(proj)
	report.Counts.Objects = len(proj.ObjectFiles)
	report.Counts.Fields = len(proj.FieldFiles)
	report.Counts.FieldSets = len(proj.FieldSetFiles)
	report.Counts.RecordTypes = len(proj.RecordTypeFiles)
	report.Counts.ValidationRules = len(proj.ValidationRuleFiles)
	report.Counts.StaticResources = len(proj.StaticResourceFiles) + len(proj.StaticResourceMetas)
	report.Counts.NamedCredentials = len(proj.NamedCredentialFiles)
	report.Counts.RemoteSites = len(proj.RemoteSiteFiles)
	report.Counts.CustomMetadata = len(proj.CustomMetadataFiles)
	report.Counts.Workflows = len(proj.WorkflowFiles)
	report.Counts.Flows = len(proj.FlowFiles)
	report.Counts.Profiles = len(proj.ProfileFiles)
	report.Counts.PermissionSets = len(proj.PermissionSetFiles)
	report.Counts.PermissionAssignments = len(proj.PermissionAssignmentFiles)
	report.Counts.VisualforcePages = len(proj.VisualforcePageFiles)
	report.Counts.VisualforceComponents = len(proj.VisualforceComponentFiles)
	report.Counts.Labels = len(proj.LabelFiles)
	report.Counts.AuraComponents = countAuraBundles(proj)
	report.Counts.LWCComponents = countLWCBundles(proj)
	report.Counts.EmailTemplates = countEmailTemplates(absRoot, opts.SkipDirNames)
	scanDiagnostics := make([]diagnostic.Diagnostic, 0)

	// Walk and inventory Apex constructs.
	constructs, runtime, testClasses := inventoryApex(absRoot, proj, opts.SkipDirNames)
	report.Constructs = constructs
	report.RuntimeUsage = runtime
	report.Counts.TestClasses = testClasses

	// Surface scan.
	if opts.RunSurfaceScan {
		surfReport, err := projectscan.Scan(absRoot)
		if err != nil {
			scanDiagnostics = append(scanDiagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADEEXAMPLE001",
				Message:  fmt.Sprintf("surface scan failed: %v", err),
			})
		} else {
			report.Surfaces = surfReport.Surfaces
			report.TopBlockers = convertTopBlockers(surfReport.TopBlockers)
		}
	}

	// Semantic analysis.
	if opts.RunSema {
		idx, err := buildIndex(proj)
		if err != nil {
			scanDiagnostics = append(scanDiagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADEEXAMPLE002",
				Message:  fmt.Sprintf("metadata schema load failed: %v", err),
			})
		} else {
			semaResult := sema.Analyze(idx)
			scanDiagnostics = append(scanDiagnostics, semaResult.Diagnostics...)
		}
	}
	if len(scanDiagnostics) > 0 {
		report.Diagnostics = categorizeDiagnostics(scanDiagnostics, report.Surfaces)
		if len(report.TopBlockers) == 0 {
			report.TopBlockers = buildTopBlockersFromDiagnostics(report.Diagnostics)
		}
	}

	return report, nil
}

// WriteJSON writes the report as indented JSON.
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func detectLayout(root string, proj project.Project) string {
	sfdxPath := filepath.Join(root, "sfdx-project.json")
	if _, err := os.Stat(sfdxPath); err == nil {
		if hasLegacyDirs(root) {
			return "mixed"
		}
		return "sfdx"
	}
	if hasLegacyDirs(root) {
		return "legacy"
	}
	return "unknown"
}

func hasLegacyDirs(root string) bool {
	legacy := []string{"src/classes", "src/triggers", "src/objects", "src/pages"}
	for _, d := range legacy {
		if fi, err := os.Stat(filepath.Join(root, d)); err == nil && fi.IsDir() {
			return true
		}
	}
	return false
}

func countClasses(proj project.Project) int {
	c := 0
	for _, f := range proj.ApexFiles {
		if strings.HasSuffix(strings.ToLower(f), ".cls") {
			c++
		}
	}
	return c
}

func countTriggers(proj project.Project) int {
	c := 0
	for _, f := range proj.ApexFiles {
		if strings.HasSuffix(strings.ToLower(f), ".trigger") {
			c++
		}
	}
	return c
}

func countAuraBundles(proj project.Project) int {
	seen := make(map[string]bool)
	for _, f := range proj.AuraFiles {
		dir := filepath.Dir(f)
		seen[dir] = true
	}
	return len(seen)
}

func countLWCBundles(proj project.Project) int {
	seen := make(map[string]bool)
	for _, f := range proj.LWCFiles {
		dir := filepath.Dir(f)
		seen[dir] = true
	}
	return len(seen)
}

func countEmailTemplates(root string, skip map[string]bool) int {
	c := 0
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && skip[d.Name()] && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		lower := strings.ToLower(d.Name())
		if strings.HasSuffix(lower, ".email") || strings.HasSuffix(lower, ".email-meta.xml") {
			c++
		}
		return nil
	})
	return c
}

func countFlows(root string, skip map[string]bool) int {
	c := 0
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && skip[d.Name()] && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		lower := strings.ToLower(d.Name())
		if strings.HasSuffix(lower, ".flow-meta.xml") || strings.HasSuffix(lower, ".flow") {
			c++
		}
		return nil
	})
	return c
}

// Apex inventory regexes.
var (
	reClassDecl     = regexp.MustCompile(`\b(class|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	reAnnotation    = regexp.MustCompile(`@\s*([A-Za-z_][A-Za-z0-9_]*)`)
	reSharing       = regexp.MustCompile(`\b(with sharing|without sharing|inherited sharing)\b`)
	reGlobal        = regexp.MustCompile(`\bglobal\b`)
	reWebService    = regexp.MustCompile(`\bwebservice\b`)
	reAsync         = regexp.MustCompile(`\b(Queueable|Batchable|Schedulable|Database.Batchable|Database.AllowsCallouts)\b`)
	reSOQL          = regexp.MustCompile(`\[\s*(?:SELECT|FIND)\b`)
	reSOQLAggregate = regexp.MustCompile(`\b(COUNT\s*\(|COUNT_DISTINCT|AVG|SUM|MIN|MAX)\b`)
	reSOQLSecurity  = regexp.MustCompile(`\bWITH\s+(SECURITY_ENFORCED|USER_MODE|SYSTEM_MODE)\b`)
	reSOQLChild     = regexp.MustCompile(`\(\s*SELECT\b`)
	reSOSL          = regexp.MustCompile(`\[\s*FIND\b`)
	reDML           = regexp.MustCompile(`\b(insert|update|upsert|delete|undelete|merge)\s+`)
	reDatabaseDML   = regexp.MustCompile(`\bDatabase\.(insert|update|upsert|delete|undelete|merge|query|countQuery|getQueryLocator)\b`)
	reTriggerCtx    = regexp.MustCompile(`\bTrigger\.(new|old|newMap|oldMap|isBefore|isAfter|isInsert|isUpdate|isDelete|isUndelete|size|operationType)\b`)
	reNamespaceRef  = regexp.MustCompile(`\b(System|Schema|ApexPages|PageReference|Messaging|Http|JSON|Dom|XmlStreamReader|XmlStreamWriter|ContentVersion|Attachment|Auth|ConnectApi|EventBus|UserInfo|Limits|Test|Database|Search|Approval|CronTrigger|AsyncApexJob)\b`)
)

func inventoryApex(root string, proj project.Project, skip map[string]bool) (ApexConstructs, RuntimeUsage, int) {
	var constructs ApexConstructs
	var runtime RuntimeUsage
	testClasses := 0

	seenAnnotations := make(map[string]bool)
	seenSharing := make(map[string]bool)
	seenAsync := make(map[string]bool)
	seenSOQL := make(map[string]bool)
	seenDML := make(map[string]bool)
	seenTriggerOps := make(map[string]bool)
	seenNamespaces := make(map[string]bool)

	apexFiles := append([]string{}, proj.ApexFiles...)

	for _, path := range apexFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		lower := strings.ToLower(text)

		// Test class detection.
		if strings.Contains(lower, "@istest") {
			testClasses++
		}

		// Declarations.
		matches := reClassDecl.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			kind := strings.ToLower(m[1])
			switch kind {
			case "class":
				constructs.Classes++
			case "interface":
				constructs.Interfaces++
			case "enum":
				constructs.Enums++
			}
		}
		// Nested types: heuristic based on indentation/inside class.
		// For now, count all declarations beyond the first as nested if file contains more than one.
		// A better heuristic: count inner class-like declarations after the first top-level one.
		// Simplified: if there are >1 declarations and file isn't a trigger, guess nested.
		if len(matches) > 1 && !strings.HasSuffix(strings.ToLower(path), ".trigger") {
			constructs.NestedTypes += len(matches) - 1
		}

		// Annotations.
		for _, m := range reAnnotation.FindAllStringSubmatch(text, -1) {
			ann := m[1]
			if !seenAnnotations[ann] {
				seenAnnotations[ann] = true
				constructs.Annotations = append(constructs.Annotations, ann)
			}
		}

		// Sharing.
		for _, m := range reSharing.FindAllStringSubmatch(text, -1) {
			mode := m[1]
			if !seenSharing[mode] {
				seenSharing[mode] = true
				constructs.SharingModes = append(constructs.SharingModes, mode)
			}
		}

		// Global.
		if reGlobal.MatchString(text) {
			constructs.GlobalClasses++
		}

		// Webservice.
		if reWebService.MatchString(text) {
			constructs.WebServiceMethods++
		}

		// Async.
		for _, m := range reAsync.FindAllStringSubmatch(text, -1) {
			iface := m[1]
			if !seenAsync[iface] {
				seenAsync[iface] = true
				constructs.AsyncInterfaces = append(constructs.AsyncInterfaces, iface)
			}
		}

		// SOQL features.
		if reSOQL.MatchString(text) {
			seenSOQL["static-query"] = true
		}
		if reSOQLAggregate.MatchString(text) {
			seenSOQL["aggregate"] = true
		}
		if reSOQLSecurity.MatchString(text) {
			seenSOQL["security-clause"] = true
		}
		if reSOQLChild.MatchString(text) {
			seenSOQL["child-subquery"] = true
		}
		if reSOSL.MatchString(text) {
			seenSOQL["sosl"] = true
		}

		// DML features.
		for _, m := range reDML.FindAllStringSubmatch(text, -1) {
			stmt := m[1]
			if !seenDML[stmt] {
				seenDML[stmt] = true
				runtime.DMLFeatures = append(runtime.DMLFeatures, stmt)
			}
		}
		for _, m := range reDatabaseDML.FindAllStringSubmatch(text, -1) {
			call := "Database." + m[1]
			if !seenDML[call] {
				seenDML[call] = true
				runtime.DMLFeatures = append(runtime.DMLFeatures, call)
			}
		}

		// Trigger operations.
		for _, m := range reTriggerCtx.FindAllStringSubmatch(text, -1) {
			op := m[1]
			if !seenTriggerOps[op] {
				seenTriggerOps[op] = true
				runtime.TriggerOperations = append(runtime.TriggerOperations, op)
			}
		}

		// Namespace references.
		for _, m := range reNamespaceRef.FindAllStringSubmatch(text, -1) {
			ns := m[1]
			if !seenNamespaces[ns] {
				seenNamespaces[ns] = true
				runtime.NamespaceRefs = append(runtime.NamespaceRefs, ns)
			}
		}
	}

	for k := range seenSOQL {
		runtime.SOQLFeatures = append(runtime.SOQLFeatures, k)
	}
	sort.Strings(runtime.SOQLFeatures)
	sort.Strings(runtime.DMLFeatures)
	sort.Strings(runtime.TriggerOperations)
	sort.Strings(runtime.NamespaceRefs)
	sort.Strings(constructs.Annotations)
	sort.Strings(constructs.SharingModes)
	sort.Strings(constructs.AsyncInterfaces)

	return constructs, runtime, testClasses
}

func buildIndex(proj project.Project) (typesys.Index, error) {
	s, err := gladeschema.LoadProject(proj)
	if err != nil {
		return typesys.Index{}, err
	}
	return typesys.Build(proj, s), nil
}

func categorizeDiagnostics(diagnostics []diagnostic.Diagnostic, surfaces []projectscan.Surface) DiagnosticBreakdown {
	var breakdown DiagnosticBreakdown

	// Map surface capability -> title for quick lookup.
	surfTitle := make(map[string]string)
	for _, s := range surfaces {
		surfTitle[s.Capability] = s.Title
	}

	// Roll up diagnostics by code+message prefix.
	keyMap := make(map[string]*DiagnosticSummary)
	for _, d := range diagnostics {
		k := d.Code + "|" + d.Message
		if existing, ok := keyMap[k]; ok {
			existing.Count++
			if d.File != "" && !contains(existing.Files, d.File) {
				existing.Files = append(existing.Files, d.File)
			}
			continue
		}
		summary := DiagnosticSummary{
			Code:    d.Code,
			Message: d.Message,
			Count:   1,
			Files:   []string{},
		}
		if d.File != "" {
			summary.Files = append(summary.Files, d.File)
		}
		keyMap[k] = &summary
	}

	// Categorize.
	for _, summary := range keyMap {
		cat := classifyDiagnostic(summary.Code, summary.Message)
		summary.CapabilityID = inferCapabilityID(summary.Code, summary.Message, surfaces)
		switch cat {
		case "observed-blocker":
			breakdown.ObservedBlockers = append(breakdown.ObservedBlockers, *summary)
		case "observed-runtime-gap":
			breakdown.ObservedRuntimeGaps = append(breakdown.ObservedRuntimeGaps, *summary)
		case "observed-parity-gap":
			breakdown.ObservedParityGaps = append(breakdown.ObservedParityGaps, *summary)
		default:
			breakdown.UnobservedParityFollowup = append(breakdown.UnobservedParityFollowup, *summary)
		}
	}

	sortSummaries(breakdown.ObservedBlockers)
	sortSummaries(breakdown.ObservedRuntimeGaps)
	sortSummaries(breakdown.ObservedParityGaps)
	sortSummaries(breakdown.UnobservedParityFollowup)

	return breakdown
}

func classifyDiagnostic(code, message string) string {
	lower := strings.ToLower(message)
	if strings.Contains(lower, "unsupported") || strings.Contains(lower, "not supported") {
		return "observed-runtime-gap"
	}
	if strings.Contains(lower, "unknown type") || strings.Contains(lower, "unknown sobject") || strings.Contains(lower, "references unknown") {
		return "observed-blocker"
	}
	if strings.Contains(lower, "panic") || strings.Contains(code, "GLADESEMA000") {
		return "observed-blocker"
	}
	if strings.HasPrefix(code, "GLADEEXAMPLE") {
		return "observed-blocker"
	}
	if strings.Contains(lower, "parity") || strings.Contains(lower, "differs") {
		return "observed-parity-gap"
	}
	if strings.HasPrefix(code, "GLADESEMA") || strings.HasPrefix(code, "GLADEAST") {
		return "observed-blocker"
	}
	return "unobserved-parity-followup"
}

func inferCapabilityID(code, message string, surfaces []projectscan.Surface) string {
	lower := strings.ToLower(message)
	for _, s := range surfaces {
		if strings.Contains(lower, strings.ToLower(s.Title)) {
			return s.Capability
		}
	}
	// Fallbacks based on message content.
	if strings.Contains(lower, "soql") || strings.Contains(lower, "query") {
		return "soql.apex"
	}
	if strings.Contains(lower, "dml") || strings.Contains(lower, "insert") || strings.Contains(lower, "update") {
		return "dml.apex"
	}
	if strings.Contains(lower, "trigger") {
		return "trigger.apex"
	}
	if strings.Contains(lower, "visualforce") || strings.Contains(lower, "apexpages") {
		return "visualforce.controller-test"
	}
	if strings.Contains(lower, "async") || strings.Contains(lower, "future") || strings.Contains(lower, "queueable") || strings.Contains(lower, "batchable") {
		return "async.apex"
	}
	if strings.Contains(lower, "http") || strings.Contains(lower, "callout") {
		return "http.callout"
	}
	if strings.Contains(lower, "json") {
		return "json.apex"
	}
	if strings.Contains(lower, "xml") {
		return "xml.apex"
	}
	if strings.Contains(lower, "security") || strings.Contains(lower, "sharing") {
		return "security.sharing"
	}
	if strings.Contains(lower, "email") || strings.Contains(lower, "messaging") {
		return "email.apex"
	}
	return ""
}

func buildTopBlockersFromDiagnostics(dbreak DiagnosticBreakdown) []Blocker {
	// Convert observed blockers into Blocker structs.
	var blockers []Blocker
	for _, b := range dbreak.ObservedBlockers {
		blockers = append(blockers, Blocker{
			CapabilityID:  b.CapabilityID,
			Title:         b.Message,
			Count:         b.Count,
			AffectedFiles: len(b.Files),
		})
	}
	// Sort by count desc.
	sort.Slice(blockers, func(i, j int) bool {
		return blockers[i].Count > blockers[j].Count
	})
	return blockers
}

func convertTopBlockers(tb []projectscan.TopBlocker) []Blocker {
	var out []Blocker
	for _, b := range tb {
		out = append(out, Blocker{
			CapabilityID:  b.Capability,
			Title:         b.Title,
			Count:         b.Count,
			AffectedFiles: b.AffectedFiles,
		})
	}
	return out
}

func contains(sl []string, s string) bool {
	for _, v := range sl {
		if v == s {
			return true
		}
	}
	return false
}

func sortSummaries(sl []DiagnosticSummary) {
	sort.Slice(sl, func(i, j int) bool {
		if sl[i].Count != sl[j].Count {
			return sl[i].Count > sl[j].Count
		}
		return sl[i].Code < sl[j].Code
	})
}
