package perfscan

import (
	"os"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

var (
	loopStartRe          = regexp.MustCompile(`(?i)\b(for|while)\s*\(`)
	soqlInlineRe         = regexp.MustCompile(`(?is)\[[\s\n]*SELECT\b.*?\]`)
	dmlStatementRe       = regexp.MustCompile(`(?i)\b(insert|update|delete|upsert|undelete|merge)\s+[^;]+;`)
	databaseDMLRe        = regexp.MustCompile(`(?i)\bDatabase\.(insert|update|delete|upsert|undelete|merge)\s*\(`)
	describeCallRe       = regexp.MustCompile(`(?i)\bSchema\.(getGlobalDescribe|describeSObjects)\s*\(|\.getDescribe\s*\(`)
	enqueueJobRe         = regexp.MustCompile(`(?i)\bSystem\.enqueueJob\s*\(|\bDatabase\.executeBatch\s*\(|\bSystem\.schedule\s*\(`)
	auraEnabledRe        = regexp.MustCompile(`(?i)@AuraEnabled(\s*\([^)]*cacheable\s*=\s*true[^)]*\))?`)
	invocableRe          = regexp.MustCompile(`(?i)@InvocableMethod\b`)
	batchableRe          = regexp.MustCompile(`(?i)\bimplements\b[^{;]*Database\.Batchable\b`)
	queryLocatorStringRe = regexp.MustCompile(`(?is)Database\.getQueryLocator\s*\(\s*'([^']*)'\s*\)`)
)

func scanApex(report *Report, p project.Project, parsed apexast.Result, index typesys.Index) {
	_ = index
	for _, file := range parsed.Files {
		for _, decl := range file.Declarations {
			if decl.Kind == apexast.DeclarationTrigger {
				report.AddEntryPoint(EntryPoint{Kind: EntryTrigger, Name: decl.Name, File: file.Path, Line: decl.Range.Start.Line})
				report.AddFinding(Finding{
					ID:         "perf.entry.trigger",
					Category:   CategoryApex,
					Severity:   SeverityLow,
					Confidence: ConfidenceStatic,
					Score:      20,
					EntryPoint: EntryPoint{Kind: EntryTrigger, Name: decl.Name, File: file.Path, Line: decl.Range.Start.Line},
					Message:    "Trigger work runs in bulk transactions and shares limits with Apex, DML, Workflow, and Flow side effects.",
					Location:   Location{File: file.Path, Line: decl.Range.Start.Line, Column: decl.Range.Start.Column},
					Fix:        "Keep trigger logic bulk-safe, handler-based, and free of per-record SOQL, DML, describe, callout, or async enqueue work.",
				})
			}
		}
	}
	for _, path := range p.ApexFiles {
		sourceBytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		source := string(sourceBytes)
		scanApexSource(report, path, source)
	}
}

func scanApexSource(report *Report, path, source string) {
	loops := loopBlocks(source)
	for _, loop := range loops {
		body := source[loop.start:loop.end]
		line := lineAt(source, loop.start)
		if soqlInlineRe.MatchString(body) {
			report.AddFinding(staticFinding("perf.soql.loop", CategorySOQL, SeverityHigh, 95, path, line, "SOQL inside a loop can exceed query limits and repeats database work per record.", "Move the query outside the loop, query all needed rows once, and use a map keyed by Id or business key."))
		}
		if dmlStatementRe.MatchString(body) || databaseDMLRe.MatchString(body) {
			report.AddFinding(staticFinding("perf.dml.loop", CategoryDML, SeverityHigh, 92, path, line, "DML inside a loop can exceed statement limits and repeats save-order automation per record.", "Build a collection inside the loop and run one DML operation after the loop."))
		}
		if describeCallRe.MatchString(body) {
			report.AddFinding(staticFinding("perf.describe.loop", CategoryDescribe, SeverityMedium, 70, path, line, "Describe calls inside loops repeat metadata work and can add heap pressure.", "Cache describe maps outside the loop or use one shared immutable describe lookup."))
		}
		if enqueueJobRe.MatchString(body) {
			report.AddFinding(staticFinding("perf.async.loop", CategoryAsync, SeverityHigh, 88, path, line, "Async enqueue inside a loop can exceed queueable, future, scheduled, or batch enqueue limits.", "Enqueue one job with the full work set or batch the records through a bounded async entry point."))
		}
	}

	describeMatches := describeCallRe.FindAllStringIndex(source, -1)
	if len(describeMatches) > 1 {
		line := lineAt(source, describeMatches[1][0])
		report.AddFinding(staticFinding("perf.describe.repeated", CategoryDescribe, SeverityMedium, 55, path, line, "Repeated describe calls in the same class can waste CPU and heap.", "Store describe results in a local variable or immutable per-transaction cache."))
	}

	for _, match := range auraEnabledRe.FindAllStringSubmatchIndex(source, -1) {
		if len(match) < 4 || match[2] == -1 {
			line := lineAt(source, match[0])
			report.AddFinding(staticFinding("perf.ui.auraenabled.uncached", CategoryUI, SeverityMedium, 64, path, line, "@AuraEnabled read methods without cacheable=true can make Lightning clients repeat server work.", "Mark read-only Aura/LWC Apex methods as cacheable=true when they do not mutate state and can use client-side caching."))
			continue
		}
	}

	for _, match := range invocableRe.FindAllStringIndex(source, -1) {
		line := lineAt(source, match[0])
		report.AddEntryPoint(EntryPoint{Kind: EntryInvocable, Name: "InvocableMethod", File: path, Line: line})
	}

	if batchableRe.MatchString(source) {
		className := classNameFromSource(path, source)
		report.AddEntryPoint(EntryPoint{Kind: EntryBatch, Name: className, File: path, Line: 1})
		for _, match := range queryLocatorStringRe.FindAllStringSubmatchIndex(source, -1) {
			query := source[match[2]:match[3]]
			if !strings.Contains(strings.ToLower(query), " where ") {
				line := lineAt(source, match[0])
				report.AddFinding(Finding{
					ID:         "perf.async.batch.unfiltered-start",
					Category:   CategoryAsync,
					Severity:   SeverityMedium,
					Confidence: ConfidenceStatic,
					Score:      68,
					EntryPoint: EntryPoint{Kind: EntryBatch, Name: className, File: path, Line: line},
					Message:    "Batch start query has no WHERE clause and may scan a large object before execute chunks begin.",
					Location:   Location{File: path, Line: line},
					Evidence:   []Evidence{{Kind: "soql", Message: "batch query locator", Value: query}},
					Fix:        "Add selective filters or split large-object work by indexed fields and date windows.",
				})
			}
		}
	}
}

type sourceRange struct {
	start int
	end   int
}

func loopBlocks(source string) []sourceRange {
	var ranges []sourceRange
	for _, match := range loopStartRe.FindAllStringIndex(source, -1) {
		open := strings.Index(source[match[1]:], "{")
		if open < 0 {
			continue
		}
		start := match[1] + open
		end := matchingBrace(source, start)
		if end > start {
			ranges = append(ranges, sourceRange{start: start, end: end})
		}
	}
	return ranges
}

func matchingBrace(source string, open int) int {
	depth := 0
	for i := open; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(source)
}

func staticFinding(id string, category Category, severity Severity, score int, file string, line int, message, fix string) Finding {
	return Finding{
		ID:         id,
		Category:   category,
		Severity:   severity,
		Confidence: ConfidenceStatic,
		Score:      score,
		Message:    message,
		Location:   Location{File: file, Line: line},
		Evidence:   []Evidence{{Kind: "static", Message: message}},
		Fix:        fix,
	}
}

func lineAt(source string, offset int) int {
	line := 1
	for i := 0; i < offset && i < len(source); i++ {
		if source[i] == '\n' {
			line++
		}
	}
	return line
}

func classNameFromSource(path, source string) string {
	classRe := regexp.MustCompile(`(?i)\bclass\s+([A-Za-z_][A-Za-z0-9_]*)`)
	match := classRe.FindStringSubmatch(source)
	if len(match) > 1 {
		return match[1]
	}
	return path
}
