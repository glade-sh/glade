package enterprise

import (
	"strings"

	"github.com/glade-sh/glade/internal/trace"
)

func SummarizeTrace(events []trace.Event) TraceSummary {
	out := TraceSummary{
		Events:     len(events),
		ByCategory: make(map[string]int),
		ByName:     make(map[string]int),
	}
	for _, event := range events {
		out.ByCategory[event.Category]++
		out.ByName[event.Name]++
		switch {
		case strings.HasPrefix(event.Category, "apex.soql"):
			out.SOQLStatements++
		case strings.HasPrefix(event.Category, "apex.dml"):
			out.DMLOperations++
		case strings.HasPrefix(event.Category, "apex.async"):
			out.AsyncEvents++
		case strings.HasPrefix(event.Category, "apex.callout"):
			out.Callouts++
		}
	}
	return out
}
