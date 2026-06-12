package enterpriseassess

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/enterprise"
	"github.com/glade-sh/glade/internal/enterprisegraph"
)

type Options struct {
	IncludeMetadata bool
	IncludeTests    bool
	Strict          bool
}

func Assess(ctx enterprise.Context, graph enterprisegraph.Graph, opts Options) enterprise.Report {
	if len(graph.Nodes) == 0 && len(graph.Edges) == 0 {
		if built, err := enterprisegraph.Build(ctx); err == nil {
			graph = built
		}
	}
	report := enterprise.NewReport("glade enterprise assess", ctx.Summary())
	stats := inspectGraph(graph)
	sources := sourceFiles(ctx, opts.IncludeMetadata, opts.IncludeTests)
	patterns := ReviewPatterns(sources)

	report.Sections = []enterprise.Section{
		inventorySection(ctx, opts),
		topRisksSection(ctx, stats),
		triggerMapSection(ctx),
		soqlDMLSection(sources),
		asyncCalloutSection(sources),
		fflibSection(ctx, graph),
		testHealthSection(ctx),
		limitationsSection(),
	}
	report.Findings = append(report.Findings, patternFindings(patterns)...)
	if opts.Strict {
		report.Findings = append(report.Findings, diagnosticFindings(ctx)...)
	}
	report.Limitations = limitations()
	report.RefreshSummary()
	return report
}

func inventorySection(ctx enterprise.Context, opts Options) enterprise.Section {
	summary := ctx.Summary()
	items := []enterprise.SectionItem{
		{Label: "Apex classes", Value: fmt.Sprint(summary.ApexClasses)},
		{Label: "Triggers", Value: fmt.Sprint(summary.Triggers)},
		{Label: "Tests", Value: fmt.Sprint(summary.Tests)},
	}
	if opts.IncludeMetadata {
		items = append(items, enterprise.SectionItem{Label: "Metadata files", Value: fmt.Sprint(summary.MetadataFiles)})
	}
	return enterprise.Section{ID: "inventory", Title: "Inventory", Summary: "Static project inventory from the enterprise context.", Items: items}
}

func topRisksSection(ctx enterprise.Context, stats graphStats) enterprise.Section {
	type riskItem struct {
		item  enterprise.SectionItem
		score int
	}
	var scored []riskItem
	testNames := testNameSet(ctx)
	triggerTargets := triggerTargetNames(ctx)
	for _, typ := range ctx.Index.Types {
		if typ.Dependency || typ.IsTest {
			continue
		}
		source := readText(typ.File)
		refs := DetectDynamicReferenceText(source)
		in := RiskInputs{
			Symbol:           typ.Name,
			TriggerPath:      triggerTargets[typ.Name] || strings.Contains(strings.ToLower(typ.Name), "triggerhandler"),
			FanOut:           stats.fanOut[typ.Name],
			FanIn:            stats.fanIn[typ.Name],
			DMLOperations:    countDML(source),
			SOQLStatements:   countSOQL(source),
			PublicOrGlobal:   hasPublicOrGlobal(typ.Modifiers),
			HasTestIndicator: testNames[strings.ToLower(typ.Name)] || testNames[strings.ToLower(typ.Name+"Test")],
			DynamicReference: refs,
		}
		score := ScoreNode(in)
		if score.Severity == enterprise.SeverityInfo {
			continue
		}
		scored = append(scored, riskItem{
			item: enterprise.SectionItem{
				Label: typ.Name,
				Value: fmt.Sprintf("%s score=%d", score.Severity, score.Score),
				Details: map[string]any{
					"score":        score.Score,
					"severity":     score.Severity,
					"explanations": score.Explanations,
				},
			},
			score: score.Score,
		})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].item.Label < scored[j].item.Label
		}
		return scored[i].score > scored[j].score
	})
	if len(scored) > 10 {
		scored = scored[:10]
	}
	items := make([]enterprise.SectionItem, 0, len(scored))
	for _, entry := range scored {
		items = append(items, entry.item)
	}
	return enterprise.Section{ID: "top_risks", Title: "Top Risks", Summary: "Risk score uses static graph and source indicators.", Items: items}
}

func triggerMapSection(ctx enterprise.Context) enterprise.Section {
	items := make([]enterprise.SectionItem, 0, len(ctx.Index.Triggers))
	for _, trigger := range ctx.Index.Triggers {
		items = append(items, enterprise.SectionItem{Label: trigger.Name, Value: trigger.ObjectName})
	}
	return enterprise.Section{ID: "trigger_map", Title: "Trigger Map", Summary: "Triggers grouped by target object.", Items: items}
}

func soqlDMLSection(files []SourceFile) enterprise.Section {
	soql, dml := 0, 0
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file.Path), ".cls") || strings.HasSuffix(strings.ToLower(file.Path), ".trigger") {
			soql += countSOQL(file.Text)
			dml += countDML(file.Text)
		}
	}
	return enterprise.Section{ID: "soql_dml_map", Title: "SOQL/DML Map", Summary: "Source text counts for query and mutation surface.", Items: []enterprise.SectionItem{{Label: "SOQL statements", Value: fmt.Sprint(soql)}, {Label: "DML operations", Value: fmt.Sprint(dml)}}}
}

func asyncCalloutSection(files []SourceFile) enterprise.Section {
	async, callouts := 0, 0
	for _, file := range files {
		text := strings.ToLower(file.Text)
		async += strings.Count(text, "@future") + strings.Count(text, "queueable") + strings.Count(text, "batchable") + strings.Count(text, "schedulable")
		callouts += strings.Count(text, "http.send") + strings.Count(text, "httprequest")
	}
	return enterprise.Section{ID: "async_callout_surface", Title: "Async/Callout Surface", Summary: "Conservative source text scan for async and callout patterns.", Items: []enterprise.SectionItem{{Label: "Async indicators", Value: fmt.Sprint(async)}, {Label: "Callout indicators", Value: fmt.Sprint(callouts)}}}
}

func fflibSection(ctx enterprise.Context, graph enterprisegraph.Graph) enterprise.Section {
	inventory := enterprisegraph.DetectFFLib(ctx)
	count := len(inventory.Domains) + len(inventory.Selectors) + len(inventory.Services) + len(inventory.UnitOfWorkUsers) + len(inventory.Factories)
	if count == 0 && strings.Contains(strings.ToLower(fmt.Sprint(graph)), "fflib") {
		count = 1
	}
	items := []enterprise.SectionItem{
		{Label: "Detected entries", Value: fmt.Sprint(count)},
		{Label: "Domains", Value: strings.Join(inventory.Domains, ", ")},
		{Label: "Selectors", Value: strings.Join(inventory.Selectors, ", ")},
		{Label: "Services", Value: strings.Join(inventory.Services, ", ")},
	}
	return enterprise.Section{ID: "fflib_inventory", Title: "FFLib Inventory", Summary: "Known FFLib naming and graph hints.", Items: items}
}

func testHealthSection(ctx enterprise.Context) enterprise.Section {
	summary := ctx.Summary()
	ratio := "0"
	if summary.ApexClasses > 0 {
		ratio = fmt.Sprintf("%.2f", float64(summary.Tests)/float64(summary.ApexClasses))
	}
	return enterprise.Section{ID: "test_health", Title: "Test Health", Summary: "Static test class count against Apex class count.", Items: []enterprise.SectionItem{{Label: "Test classes", Value: fmt.Sprint(summary.Tests)}, {Label: "Tests per class", Value: ratio}}}
}

func limitationsSection() enterprise.Section {
	items := make([]enterprise.SectionItem, 0, len(limitations()))
	for _, limitation := range limitations() {
		items = append(items, enterprise.SectionItem{Label: limitation})
	}
	return enterprise.Section{ID: "limitations", Title: "Limitations", Summary: "Boundary statements for this static report.", Items: items}
}

func limitations() []string {
	return []string{
		"static graph references are conservative",
		"dynamic Apex/custom metadata routing reduce confidence",
		"report does not claim full Salesforce parity; it names local evidence only",
		"support-map generation is plugin-owned",
	}
}

func patternFindings(patterns []PatternFinding) []enterprise.Finding {
	out := make([]enterprise.Finding, 0, len(patterns))
	for i, pattern := range patterns {
		out = append(out, enterprise.Finding{
			ID:             fmt.Sprintf("%s.%03d", pattern.ID, i+1),
			Category:       enterprise.CategoryArchitecture,
			Severity:       enterprise.SeverityLow,
			Confidence:     enterprise.ConfidenceMedium,
			Title:          pattern.Title,
			Summary:        pattern.Summary,
			Location:       enterprise.Location{File: pattern.File, LineStart: pattern.Line},
			Evidence:       []enterprise.Evidence{{Type: enterprise.EvidenceHeuristic, Message: pattern.Summary}},
			Recommendation: "Review the source pattern before modernization work.",
			Tags:           []string{pattern.ID},
		})
	}
	return out
}

func sourceFiles(ctx enterprise.Context, includeMetadata, includeTests bool) []SourceFile {
	seen := map[string]bool{}
	var paths []string
	for _, typ := range ctx.Index.Types {
		if typ.IsTest && !includeTests {
			continue
		}
		if typ.File != "" && !seen[typ.File] {
			paths = append(paths, typ.File)
			seen[typ.File] = true
		}
	}
	for _, trigger := range ctx.Index.Triggers {
		if trigger.File != "" && !seen[trigger.File] {
			paths = append(paths, trigger.File)
			seen[trigger.File] = true
		}
	}
	if includeMetadata {
		paths = append(paths, filepath.Join(ctx.Project.Root, "sfdx-project.json"))
		for _, path := range apexMetadataFiles(ctx.Project.Root) {
			if !seen[path] {
				paths = append(paths, path)
				seen[path] = true
			}
		}
	}
	var files []SourceFile
	for _, path := range paths {
		text := readText(path)
		if text != "" {
			files = append(files, SourceFile{Path: path, Text: text})
		}
	}
	return files
}

func diagnosticFindings(ctx enterprise.Context) []enterprise.Finding {
	var findings []enterprise.Finding
	findings = append(findings, diagnosticFindingsFor("parse", ctx.Index.Diagnostics)...)
	findings = append(findings, diagnosticFindingsFor("semantic", ctx.Sema.Diagnostics)...)
	return findings
}

func diagnosticFindingsFor(prefix string, diagnostics []diagnostic.Diagnostic) []enterprise.Finding {
	var findings []enterprise.Finding
	for i, diag := range diagnostics {
		if diag.Severity != diagnostic.Error {
			continue
		}
		line := 0
		column := 0
		if diag.Range != nil {
			line = diag.Range.Start.Line
			column = diag.Range.Start.Column
		}
		findings = append(findings, enterprise.Finding{
			ID:         fmt.Sprintf("enterprise.assess.%s_diagnostic.%03d", prefix, i+1),
			Category:   enterprise.CategoryArchitecture,
			Severity:   enterprise.SeverityCritical,
			Confidence: enterprise.ConfidenceHigh,
			Title:      prefix + " diagnostic",
			Summary:    diag.Message,
			Location: enterprise.Location{
				File:        diag.File,
				LineStart:   line,
				ColumnStart: column,
			},
			Evidence:       []enterprise.Evidence{{Type: enterprise.EvidenceSema, Message: diag.Message}},
			Recommendation: "Fix diagnostics before using this assessment as release evidence.",
			Tags:           []string{"strict", prefix},
		})
	}
	return findings
}

func readText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func testNameSet(ctx enterprise.Context) map[string]bool {
	out := map[string]bool{}
	for _, typ := range ctx.Index.Types {
		if typ.IsTest {
			out[strings.ToLower(typ.Name)] = true
		}
	}
	return out
}

func triggerTargetNames(ctx enterprise.Context) map[string]bool {
	out := map[string]bool{}
	for _, typ := range ctx.Index.Types {
		if strings.Contains(strings.ToLower(typ.Name), "triggerhandler") {
			out[typ.Name] = true
		}
	}
	return out
}

func hasPublicOrGlobal(modifiers []string) bool {
	for _, modifier := range modifiers {
		switch strings.ToLower(modifier) {
		case "public", "global":
			return true
		}
	}
	return false
}

func countSOQL(text string) int {
	return regexpCount(`(?is)\[\s*SELECT\b`, text)
}

func countDML(text string) int {
	return regexpCount(`(?i)(?:\bDatabase\s*\.\s*)?\b(insert|update|upsert|delete|undelete|merge)\b`, text)
}

func regexpCount(expr, text string) int {
	re := regexpCache(expr)
	return len(re.FindAllStringIndex(text, -1))
}

func regexpCache(expr string) *regexp.Regexp {
	return regexp.MustCompile(expr)
}

func DetectDynamicReferenceText(text string) bool {
	low := strings.ToLower(text)
	return strings.Contains(low, "type.forname") || strings.Contains(low, "__mdt") || strings.Contains(low, "getinstance(")
}

type graphStats struct {
	fanOut map[string]int
	fanIn  map[string]int
}

func apexMetadataFiles(root string) []string {
	var paths []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".cls-meta.xml") {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths
}

func inspectGraph(graph any) graphStats {
	stats := graphStats{fanOut: map[string]int{}, fanIn: map[string]int{}}
	value := reflect.ValueOf(graph)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return stats
	}
	edges := value.FieldByName("Edges")
	if !edges.IsValid() || edges.Kind() != reflect.Slice {
		return stats
	}
	for i := 0; i < edges.Len(); i++ {
		edge := edges.Index(i)
		if edge.Kind() == reflect.Pointer {
			edge = edge.Elem()
		}
		from := stringField(edge, "From", "Source", "SourceID", "FromID")
		to := stringField(edge, "To", "Target", "TargetID", "ToID")
		if from != "" {
			stats.fanOut[from]++
		}
		if to != "" {
			stats.fanIn[to]++
		}
	}
	return stats
}

func stringField(value reflect.Value, names ...string) string {
	if value.Kind() != reflect.Struct {
		return ""
	}
	for _, name := range names {
		field := value.FieldByName(name)
		if field.IsValid() && field.Kind() == reflect.String {
			return field.String()
		}
	}
	return ""
}
