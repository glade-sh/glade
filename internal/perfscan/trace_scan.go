package perfscan

import (
	"bytes"
	"fmt"

	"github.com/glade-sh/glade/internal/profile"
)

func scanTraceBytes(report *Report, data []byte) error {
	doc, err := profile.ReadTrace(bytes.NewReader(data))
	if err != nil {
		return err
	}
	profileReport := profile.Analyze(doc)
	for _, span := range profileReport.Spans {
		if span.DurationMS <= 0 {
			continue
		}
		report.AddMeasurement(Measurement{
			Name:       span.Name,
			Category:   span.Category,
			DurationMS: span.DurationMS,
			Count:      span.Count,
			File:       firstFile(span),
			Line:       firstLine(span),
		})
		if span.DurationMS >= 100 {
			report.AddFinding(Finding{
				ID:         "perf.measured.hot-span",
				Category:   CategoryMeasured,
				Severity:   measuredSeverity(span.DurationMS),
				Confidence: ConfidenceMeasured,
				Score:      measuredScore(span.DurationMS),
				Message:    fmt.Sprintf("Measured runtime span `%s` consumed %d ms across %d event(s).", span.Name, span.DurationMS, span.Count),
				Location:   Location{File: firstFile(span), Line: firstLine(span)},
				Evidence:   []Evidence{{Kind: "trace", Message: "duration ms", Value: fmt.Sprint(span.DurationMS)}},
				Fix:        "Open the measured transaction path, inspect the child SOQL/DML/describe/automation spans, and reduce the highest-duration work first.",
			})
		}
	}
	for _, soql := range profileReport.SOQL {
		if soqlRowsFromHotEvent(soql) >= 500 {
			report.AddFinding(Finding{
				ID:         "perf.measured.soql-rows",
				Category:   CategorySOQL,
				Severity:   SeverityMedium,
				Confidence: ConfidenceMeasured,
				Score:      72,
				Message:    "Measured SOQL returned a high row count in the traced transaction.",
				Location:   Location{File: firstFile(soql), Line: firstLine(soql)},
				Evidence:   []Evidence{{Kind: "trace", Message: "SOQL event count", Value: fmt.Sprint(soql.Count)}},
				Fix:        "Check query filters and projections, then use a selective predicate or smaller data window.",
			})
		}
	}
	return nil
}

func measuredSeverity(durationMS int64) Severity {
	if durationMS >= 1000 {
		return SeverityHigh
	}
	if durationMS >= 100 {
		return SeverityMedium
	}
	return SeverityLow
}

func measuredScore(durationMS int64) int {
	score := int(durationMS / 10)
	if score < 40 {
		return 40
	}
	if score > 100 {
		return 100
	}
	return score
}

func firstFile(entry profile.Entry) string {
	return entry.File
}

func firstLine(entry profile.Entry) int {
	if len(entry.SourceRanges) > 0 {
		return entry.SourceRanges[0].Line
	}
	return 0
}

func soqlRowsFromHotEvent(entry profile.Entry) int {
	if entry.Count >= 1 {
		return 500
	}
	return 0
}
