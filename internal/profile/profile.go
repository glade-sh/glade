package profile

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/open-aer/oaer/internal/trace"
)

type Report struct {
	Format     string           `json:"format"`
	Events     int              `json:"events"`
	Hot        []Entry          `json:"hot"`
	Categories map[string]int   `json:"categories,omitempty"`
	Limits     LimitAttribution `json:"limits,omitempty"`
}

type Entry struct {
	Name          string `json:"name"`
	Category      string `json:"category,omitempty"`
	Count         int    `json:"count"`
	FirstTS       int64  `json:"firstTs"`
	LastTS        int64  `json:"lastTs"`
	SourceOffsets []int  `json:"sourceOffsets,omitempty"`
}

type LimitAttribution struct {
	SOQLQueries int `json:"soqlQueries,omitempty"`
	SOQLRows    int `json:"soqlRows,omitempty"`
	DML         int `json:"dml,omitempty"`
	DMLRows     int `json:"dmlRows,omitempty"`
}

func Analyze(doc trace.Document) Report {
	byKey := make(map[string]*Entry)
	for _, event := range doc.TraceEvents {
		key := event.Category + "\x00" + event.Name
		entry := byKey[key]
		if entry == nil {
			entry = &Entry{
				Name:     event.Name,
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
		if offset, ok := intArg(event.Args["sourceOffset"]); ok && !containsInt(entry.SourceOffsets, offset) {
			entry.SourceOffsets = append(entry.SourceOffsets, offset)
		}
	}

	report := Report{Format: doc.Format, Events: len(doc.TraceEvents), Hot: make([]Entry, 0, len(byKey)), Categories: make(map[string]int)}
	for _, entry := range byKey {
		sort.Ints(entry.SourceOffsets)
		report.Hot = append(report.Hot, *entry)
	}
	for _, event := range doc.TraceEvents {
		report.Categories[event.Category]++
		switch event.Category {
		case "apex.soql":
			report.Limits.SOQLQueries++
			if rows, ok := intArg(event.Args["rows"]); ok {
				report.Limits.SOQLRows += rows
			}
		case "apex.dml":
			report.Limits.DML++
			if rows, ok := intArg(event.Args["rows"]); ok {
				report.Limits.DMLRows += rows
			}
		}
	}
	sort.Slice(report.Hot, func(i, j int) bool {
		if report.Hot[i].Count != report.Hot[j].Count {
			return report.Hot[i].Count > report.Hot[j].Count
		}
		return report.Hot[i].Name < report.Hot[j].Name
	})
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
	if _, err := fmt.Fprintf(w, "# oaer profile\n\nEvents: %d\n\n", report.Events); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "SOQL: %d queries / %d rows\n\nDML: %d statements / %d rows\n\n", report.Limits.SOQLQueries, report.Limits.SOQLRows, report.Limits.DML, report.Limits.DMLRows); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Rank | Event | Category | Count | Source offsets |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- | ---: | --- |"); err != nil {
		return err
	}
	for i, entry := range report.Hot {
		if _, err := fmt.Fprintf(w, "| %d | `%s` | `%s` | %d | %v |\n", i+1, entry.Name, entry.Category, entry.Count, entry.SourceOffsets); err != nil {
			return err
		}
	}
	return nil
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
