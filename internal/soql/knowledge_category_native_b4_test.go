package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Supplemental direct replay of knowledge_publishing/org.tsv cat_* rows.
func TestKnowledgeCategoryHierarchyB4(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyOrgShape(&org, []string{"Knowledge", "KnowledgeEnabled"})
	storage.EnsureStandardObject(&org, "Knowledge__kav")
	storage.EnsureStandardObject(&org, "Knowledge__DataCategorySelection")
	id := storage.ID("ka0000000000001")
	articles := org.Objects["Knowledge__kav"]
	articles.Records[id] = storage.Record{ID: id, Object: "Knowledge__kav", Fields: map[string]storage.Value{"PublishStatus": storage.StringValue("Draft")}}
	org.Objects["Knowledge__kav"] = articles
	selections := org.Objects["Knowledge__DataCategorySelection"]
	selections.Records["a00000000000001"] = storage.Record{ID: "a00000000000001", Object: "Knowledge__DataCategorySelection", Fields: map[string]storage.Value{"ParentId": storage.IDValue(id), "DataCategoryGroupName": storage.StringValue("Guides"), "DataCategoryName": storage.StringValue("General")}}
	org.Objects["Knowledge__DataCategorySelection"] = selections
	org.Metadata.DataCategoryGroups = []storage.DataCategoryGroup{{Name: "Guides", Categories: []storage.DataCategory{{Name: "All", Children: []storage.DataCategory{{Name: "General"}}}}}}
	for _, tc := range []struct {
		op, name string
		want     int
	}{{"AT", "General", 1}, {"ABOVE", "General", 1}, {"BELOW", "General", 1}, {"ABOVE_OR_BELOW", "General", 1}, {"AT", "All", 0}, {"ABOVE", "All", 0}, {"BELOW", "All", 1}, {"ABOVE_OR_BELOW", "All", 1}} {
		query := "SELECT Id FROM Knowledge__kav WITH DATA CATEGORY Guides__c " + tc.op + " " + tc.name + "__c"
		result, err := ParseAndExecute(org, query)
		if err != nil {
			t.Errorf("%s: %v", query, err)
			continue
		}
		if len(result.Records) != tc.want {
			t.Errorf("%s: got %d want %d", query, len(result.Records), tc.want)
		}
	}
}
