package sema

import (
	"github.com/glade-sh/glade/internal/soql"
	"testing"
)

// Native knowledge_publishing/org.tsv: latest_without_archived and
// old_by_status_latest, API 62 and 67.
func TestKnowledgeLatestRestrictionB4(t *testing.T) {
	for _, tc := range []struct {
		source string
		reject bool
	}{
		{"SELECT Id FROM Knowledge__kav WHERE IsLatestVersion=false", true},
		{"SELECT Id FROM Knowledge__kav WHERE PublishStatus='Archived' AND IsLatestVersion=false", false},
	} {
		query, err := soql.Parse(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		diagnostics := queryShapeDiagnostics(query, queryTextContext{queryText: tc.source, locator: newSemaSourceLocator(tc.source)})
		if (len(diagnostics) > 0) != tc.reject {
			t.Errorf("%s: diagnostics=%v", tc.source, diagnostics)
		}
	}
}
