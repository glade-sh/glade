package profile

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/trace"
)

type Report struct {
	Format      string           `json:"format"`
	Events      int              `json:"events"`
	WallClockUS int64            `json:"wallClockUs,omitempty"`
	WallClockMS int64            `json:"wallClockMs,omitempty"`
	Hot         []Entry          `json:"hot"`
	Spans       []Entry          `json:"spans,omitempty"`
	Categories  map[string]int   `json:"categories,omitempty"`
	Limits      LimitAttribution `json:"limits,omitempty"`
	Statements  []Entry          `json:"statements,omitempty"`
	Methods     []Entry          `json:"methods,omitempty"`
	SOQL        []Entry          `json:"soql,omitempty"`
	DML         []Entry          `json:"dml,omitempty"`
	Triggers    []Entry          `json:"triggers,omitempty"`
	Describe    []Entry          `json:"describe,omitempty"`
	Callouts    []Entry          `json:"callouts,omitempty"`
	Async       []Entry          `json:"async,omitempty"`
	Platform    []Entry          `json:"platform,omitempty"`
	Automation  []Entry          `json:"automation,omitempty"`
	Visualforce []Entry          `json:"visualforce,omitempty"`
	Metadata    []Entry          `json:"metadata,omitempty"`
}

type Entry struct {
	Name          string  `json:"name"`
	Category      string  `json:"category,omitempty"`
	Count         int     `json:"count"`
	Rows          int     `json:"rows,omitempty"`
	DurationCount int     `json:"durationCount,omitempty"`
	FirstTS       int64   `json:"firstTs"`
	LastTS        int64   `json:"lastTs"`
	File          string  `json:"file,omitempty"`
	DurationUS    int64   `json:"durationUs,omitempty"`
	DurationMS    int64   `json:"durationMs,omitempty"`
	SourceOffsets []int   `json:"sourceOffsets,omitempty"`
	SourceRanges  []Range `json:"sourceRanges,omitempty"`
}

type Range struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

type LimitAttribution struct {
	SOQLQueries      int `json:"soqlQueries,omitempty"`
	SOQLRows         int `json:"soqlRows,omitempty"`
	DML              int `json:"dml,omitempty"`
	DMLRows          int `json:"dmlRows,omitempty"`
	Callouts         int `json:"callouts,omitempty"`
	AsyncJobs        int `json:"asyncJobs,omitempty"`
	FutureCalls      int `json:"futureCalls,omitempty"`
	QueueableJobs    int `json:"queueableJobs,omitempty"`
	BatchJobs        int `json:"batchJobs,omitempty"`
	ScheduledJobs    int `json:"scheduledJobs,omitempty"`
	EmailInvocations int `json:"emailInvocations,omitempty"`
	CPUTimeMS        int `json:"cpuTimeMs,omitempty"`
	HeapSize         int `json:"heapSize,omitempty"`
}

func Analyze(doc trace.Document) Report {
	byKey := make(map[string]*Entry)
	var firstTS, lastTS int64
	hasTimestamps := false
	for _, event := range doc.TraceEvents {
		endTS := event.Timestamp
		if event.Duration > 0 {
			endTS += event.Duration
		}
		if !hasTimestamps || event.Timestamp < firstTS {
			firstTS = event.Timestamp
		}
		if endTS > lastTS {
			lastTS = endTS
		}
		hasTimestamps = true
		key := profileEntryKey(event)
		entry := byKey[key]
		if entry == nil {
			entry = &Entry{
				Name:     profileEntryName(event),
				Category: event.Category,
				FirstTS:  event.Timestamp,
				LastTS:   event.Timestamp,
			}
			byKey[key] = entry
		}
		entry.Count++
		if event.Timestamp < entry.FirstTS {
			entry.FirstTS = event.Timestamp
		}
		if event.Timestamp > entry.LastTS {
			entry.LastTS = event.Timestamp
		}
		if entry.File == "" {
			if file, ok := event.Args["file"].(string); ok && strings.TrimSpace(file) != "" {
				entry.File = file
			}
		}
		if event.Duration > 0 {
			entry.DurationCount++
			entry.DurationUS += event.Duration
			entry.DurationMS = entry.DurationUS / 1000
		}
		if event.Phase == trace.PhaseInstant {
			if rows, ok := intArg(event.Args["rows"]); ok {
				entry.Rows += rows
			}
		}
		if offset, ok := intArg(event.Args["sourceOffset"]); ok && !containsInt(entry.SourceOffsets, offset) {
			entry.SourceOffsets = append(entry.SourceOffsets, offset)
		}
		if offset, ok := intArg(event.Args["sourceOffset"]); ok {
			line, hasLine := intArg(event.Args["line"])
			column, hasColumn := intArg(event.Args["column"])
			if hasLine && hasColumn && !containsRange(entry.SourceRanges, offset, line, column) {
				entry.SourceRanges = append(entry.SourceRanges, Range{Offset: offset, Line: line, Column: column})
			}
			continue
		}
		line, hasLine := intArg(event.Args["line"])
		column, hasColumn := intArg(event.Args["column"])
		if hasLine {
			if !hasColumn {
				column = 0
			}
			if !containsRange(entry.SourceRanges, -1, line, column) {
				entry.SourceRanges = append(entry.SourceRanges, Range{Offset: -1, Line: line, Column: column})
			}
		}
	}

	report := Report{Format: doc.Format, Events: len(doc.TraceEvents), Hot: make([]Entry, 0, len(byKey)), Categories: make(map[string]int)}
	if lastTS > firstTS {
		report.WallClockUS = lastTS - firstTS
		report.WallClockMS = report.WallClockUS / 1000
	}
	for _, entry := range byKey {
		sort.Ints(entry.SourceOffsets)
		sort.Slice(entry.SourceRanges, func(i, j int) bool {
			return entry.SourceRanges[i].Offset < entry.SourceRanges[j].Offset
		})
		report.Hot = append(report.Hot, *entry)
	}
	for _, event := range doc.TraceEvents {
		report.Categories[event.Category]++
		switch event.Category {
		case "apex.soql":
			if event.Phase == trace.PhaseInstant {
				report.Limits.SOQLQueries++
				if rows, ok := intArg(event.Args["rows"]); ok {
					report.Limits.SOQLRows += rows
				}
			}
		case "apex.dml":
			if event.Phase == trace.PhaseInstant {
				report.Limits.DML++
				if rows, ok := intArg(event.Args["rows"]); ok {
					report.Limits.DMLRows += rows
				}
			}
		case "apex.callout":
			report.Limits.Callouts++
		case "apex.async":
			if event.Name == "apex.async.enqueue" {
				report.Limits.AsyncJobs++
				if kind, _ := event.Args["kind"].(string); kind != "" {
					switch kind {
					case "Future":
						report.Limits.FutureCalls++
					case "Queueable":
						report.Limits.QueueableJobs++
					case "BatchApex":
						report.Limits.BatchJobs++
					case "ScheduledApex":
						report.Limits.ScheduledJobs++
					}
				}
			}
		case "apex.email":
			report.Limits.EmailInvocations++
		case "apex.limits":
			if value, ok := intArg(event.Args["callouts"]); ok {
				report.Limits.Callouts = maxInt(report.Limits.Callouts, value)
			}
			if value, ok := intArg(event.Args["asyncJobs"]); ok {
				report.Limits.AsyncJobs = maxInt(report.Limits.AsyncJobs, value)
			}
			if value, ok := intArg(event.Args["futureCalls"]); ok {
				report.Limits.FutureCalls = maxInt(report.Limits.FutureCalls, value)
			}
			if value, ok := intArg(event.Args["queueableJobs"]); ok {
				report.Limits.QueueableJobs = maxInt(report.Limits.QueueableJobs, value)
			}
			if value, ok := intArg(event.Args["batchJobs"]); ok {
				report.Limits.BatchJobs = maxInt(report.Limits.BatchJobs, value)
			}
			if value, ok := intArg(event.Args["scheduledJobs"]); ok {
				report.Limits.ScheduledJobs = maxInt(report.Limits.ScheduledJobs, value)
			}
			if value, ok := intArg(event.Args["emailInvocations"]); ok {
				report.Limits.EmailInvocations = maxInt(report.Limits.EmailInvocations, value)
			}
			if value, ok := intArg(event.Args["cpuTimeMs"]); ok {
				report.Limits.CPUTimeMS = maxInt(report.Limits.CPUTimeMS, value)
			}
			if value, ok := intArg(event.Args["heapSize"]); ok {
				report.Limits.HeapSize = maxInt(report.Limits.HeapSize, value)
			}
		}
	}
	sort.Slice(report.Hot, func(i, j int) bool {
		if report.Hot[i].Count != report.Hot[j].Count {
			return report.Hot[i].Count > report.Hot[j].Count
		}
		return report.Hot[i].Name < report.Hot[j].Name
	})
	for _, entry := range report.Hot {
		if entry.DurationUS > 0 {
			report.Spans = append(report.Spans, entry)
		}
	}
	sort.Slice(report.Spans, func(i, j int) bool {
		if report.Spans[i].DurationUS != report.Spans[j].DurationUS {
			return report.Spans[i].DurationUS > report.Spans[j].DurationUS
		}
		if report.Spans[i].Count != report.Spans[j].Count {
			return report.Spans[i].Count > report.Spans[j].Count
		}
		return report.Spans[i].Name < report.Spans[j].Name
	})
	report.Statements = entriesForCategory(report.Hot, "apex.statement")
	report.Methods = entriesForCategory(report.Hot, "apex.method")
	report.SOQL = entriesForCategory(report.Hot, "apex.soql")
	report.DML = entriesForCategory(report.Hot, "apex.dml")
	report.Triggers = entriesForCategory(report.Hot, "apex.trigger")
	report.Describe = entriesForCategory(report.Hot, "apex.describe")
	report.Callouts = entriesForCategory(report.Hot, "apex.callout")
	report.Async = entriesForCategory(report.Hot, "apex.async")
	report.Platform = append(entriesForCategory(report.Hot, "apex.email"), entriesForCategory(report.Hot, "apex.limits")...)
	report.Automation = entriesForCategories(report.Hot, "apex.flow", "apex.workflow")
	report.Visualforce = entriesForCategories(report.Hot, "apex.visualforce", "apex.visualforce.standard_controller")
	report.Metadata = entriesForCategory(report.Hot, "apex.metadata")
	return report
}

func ReadTrace(r io.Reader) (trace.Document, error) {
	var doc trace.Document
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return trace.Document{}, err
	}
	if doc.Format == "" && len(doc.TraceEvents) > 0 {
		doc.Format = trace.FormatChromeTraceEvent
	}
	return doc, nil
}

func WriteJSON(w io.Writer, report Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteMarkdown(w io.Writer, report Report) error {
	if _, err := fmt.Fprintf(w, "# glade profile\n\nEvents: %d\n\n", report.Events); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "## Runtime summary\n\nSOQL: %d queries / %d rows\n\nDML: %d statements / %d rows\n\nCallouts: %d\n\nAsync: %d jobs\n\nEmail: %d invocations\n\nCPU: %d ms\n\nWall clock: %d ms\n\nHeap: %d bytes\n\n", report.Limits.SOQLQueries, report.Limits.SOQLRows, report.Limits.DML, report.Limits.DMLRows, report.Limits.Callouts, report.Limits.AsyncJobs, report.Limits.EmailInvocations, report.Limits.CPUTimeMS, report.WallClockMS, report.Limits.HeapSize); err != nil {
		return err
	}
	if err := writeCategorySummary(w, report.Categories); err != nil {
		return err
	}
	if err := writeEntriesSection(w, "Measured spans", report.Spans); err != nil {
		return err
	}
	if err := writeEntriesSection(w, "Hot events", report.Hot); err != nil {
		return err
	}
	sections := []struct {
		title   string
		entries []Entry
	}{
		{"Statements", report.Statements},
		{"Methods", report.Methods},
		{"SOQL", report.SOQL},
		{"DML", report.DML},
		{"Triggers", report.Triggers},
		{"Describe", report.Describe},
		{"Callouts", report.Callouts},
		{"Async", report.Async},
		{"Platform", report.Platform},
		{"Automation", report.Automation},
		{"Visualforce", report.Visualforce},
		{"Metadata", report.Metadata},
	}
	for _, section := range sections {
		if err := writeEntriesSection(w, section.title, section.entries); err != nil {
			return err
		}
	}
	return nil
}

func WriteText(w io.Writer, report Report, logPath string) error {
	if _, err := fmt.Fprintln(w, "Glade debug profile"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Events: %d\n\n", report.Events); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Runtime:"); err != nil {
		return err
	}
	rows := []struct {
		label string
		value string
	}{
		{"SOQL queries", fmt.Sprintf("%d query / %d rows", report.Limits.SOQLQueries, report.Limits.SOQLRows)},
		{"DML statements", fmt.Sprintf("%d statement / %d rows", report.Limits.DML, report.Limits.DMLRows)},
		{"Callouts", fmt.Sprint(report.Limits.Callouts)},
		{"CPU", fmt.Sprintf("%dms", report.Limits.CPUTimeMS)},
		{"Wall clock", fmt.Sprintf("%dms", report.WallClockMS)},
		{"Heap", fmt.Sprintf("%d bytes", report.Limits.HeapSize)},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "  %-15s %s\n", row.label, row.value); err != nil {
			return err
		}
	}
	if len(report.Hot) > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Hot events:"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "  Rank  Event                  Count  Rows  Duration"); err != nil {
			return err
		}
		limit := len(report.Hot)
		if limit > 8 {
			limit = 8
		}
		for i := 0; i < limit; i++ {
			entry := report.Hot[i]
			name := entry.Name
			if len(name) > 22 {
				name = name[:19] + "..."
			}
			if _, err := fmt.Fprintf(w, "  %-4d  %-22s %-5d  %-4d  %dms\n", i+1, name, entry.Count, entry.Rows, entry.DurationMS); err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(logPath) != "" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Next:"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  glade debug explain --log %s --project .\n", logPath); err != nil {
			return err
		}
	}
	return nil
}

func writeCategorySummary(w io.Writer, categories map[string]int) error {
	if len(categories) == 0 {
		return nil
	}
	keys := make([]string, 0, len(categories))
	for key := range categories {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if _, err := fmt.Fprintln(w, "## Categories"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Category | Count |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | ---: |"); err != nil {
		return err
	}
	for _, key := range keys {
		if _, err := fmt.Fprintf(w, "| `%s` | %d |\n", key, categories[key]); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func writeEntriesSection(w io.Writer, title string, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "## %s\n\n", title); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Rank | Event | Category | Count | Rows | Duration ms | Source offsets |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- | ---: | ---: | ---: | --- |"); err != nil {
		return err
	}
	for i, entry := range entries {
		if _, err := fmt.Fprintf(w, "| %d | `%s` | `%s` | %d | %d | %d | %v |\n", i+1, entry.Name, entry.Category, entry.Count, entry.Rows, entry.DurationMS, entry.SourceOffsets); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func profileEntryKey(event trace.Event) string {
	key := event.Category + "\x00" + event.Name
	if event.Category == "apex.soql" {
		if query, ok := event.Args["query"].(string); ok && strings.TrimSpace(query) != "" {
			key += "\x00" + strings.TrimSpace(query)
		}
	}
	return key
}

func profileEntryName(event trace.Event) string {
	if event.Category == "apex.soql" {
		if query, ok := event.Args["query"].(string); ok && strings.TrimSpace(query) != "" {
			return strings.TrimSpace(query)
		}
	}
	return event.Name
}

func entriesForCategory(entries []Entry, category string) []Entry {
	var out []Entry
	for _, entry := range entries {
		if entry.Category == category {
			out = append(out, entry)
		}
	}
	return out
}

func entriesForCategories(entries []Entry, categories ...string) []Entry {
	wanted := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		wanted[category] = struct{}{}
	}
	var out []Entry
	for _, entry := range entries {
		if _, ok := wanted[entry.Category]; ok {
			out = append(out, entry)
		}
	}
	return out
}

func maxInt(a, b int) int {
	if b > a {
		return b
	}
	return a
}

func intArg(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

func containsInt(values []int, needle int) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func containsRange(values []Range, offset, line, column int) bool {
	for _, value := range values {
		if value.Offset == offset && value.Line == line && value.Column == column {
			return true
		}
	}
	return false
}
