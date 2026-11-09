package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native observations: knowledge_publishing/org.tsv, archived_status_query,
// old_by_id_status, and old_by_status_latest at API 62 and 67.
func TestKnowledgeArchivedHistoryB4(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyOrgShape(&org, []string{"Knowledge", "KnowledgeEnabled"})
	storage.EnsureStandardObject(&org, "Knowledge__kav")
	state := org.Objects["Knowledge__kav"]
	state.Records["ka0000000000001"] = storage.Record{ID: "ka0000000000001", Object: "Knowledge__kav", Fields: map[string]storage.Value{"PublishStatus": storage.StringValue("Archived"), "IsLatestVersion": storage.BooleanValue(false)}}
	org.Objects["Knowledge__kav"] = state
	for _, tc := range []struct {
		query string
		want  int
	}{
		{"SELECT Id FROM Knowledge__kav WHERE PublishStatus='Archived'", 0},
		{"SELECT Id FROM Knowledge__kav WHERE Id='ka0000000000001' AND PublishStatus='Archived'", 1},
		{"SELECT Id FROM Knowledge__kav WHERE PublishStatus='Archived' AND IsLatestVersion=false", 1},
	} {
		result, err := ParseAndExecute(org, tc.query)
		if err != nil {
			t.Fatalf("%s: %v", tc.query, err)
		}
		if len(result.Records) != tc.want {
			t.Errorf("%s: got %d want %d", tc.query, len(result.Records), tc.want)
		}
	}
}
