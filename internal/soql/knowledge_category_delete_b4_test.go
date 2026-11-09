package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// knowledge_publishing/org.tsv category_delete_* rows, API 62 and 67.
func TestKnowledgeCategoryDeletedSelectionB4(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyOrgShape(&org, []string{"Knowledge", "KnowledgeEnabled"})
	storage.EnsureStandardObject(&org, "Knowledge__kav")
	storage.EnsureStandardObject(&org, "Knowledge__DataCategorySelection")
	id := storage.ID("ka0000000000001")
	articles := org.Objects["Knowledge__kav"]
	articles.Records[id] = storage.Record{ID: id, Object: "Knowledge__kav", Fields: map[string]storage.Value{"PublishStatus": storage.StringValue("Draft")}}
	org.Objects["Knowledge__kav"] = articles
	selectionID := storage.ID("a00000000000001")
	selections := org.Objects["Knowledge__DataCategorySelection"]
	selection := storage.Record{ID: selectionID, Object: "Knowledge__DataCategorySelection", Fields: map[string]storage.Value{"ParentId": storage.IDValue(id), "DataCategoryGroupName": storage.StringValue("Guides"), "DataCategoryName": storage.StringValue("General")}}
	selection.System.IsDeleted = true
	selections.Records[selectionID] = selection
	org.Objects["Knowledge__DataCategorySelection"] = selections
	org.Metadata.DataCategoryGroups = []storage.DataCategoryGroup{{Name: "Guides", Categories: []storage.DataCategory{{Name: "All", Children: []storage.DataCategory{{Name: "General"}}}}}}
	for _, query := range []string{"SELECT Id FROM Knowledge__DataCategorySelection ALL ROWS", "SELECT Id FROM Knowledge__kav WHERE PublishStatus='Draft' WITH DATA CATEGORY Guides__c AT General__c"} {
		result, err := ParseAndExecute(org, query)
		if err != nil {
			t.Errorf("%s: %v", query, err)
			continue
		}
		if len(result.Records) != 0 {
			t.Errorf("%s: got %d rows, want 0", query, len(result.Records))
		}
	}
}
