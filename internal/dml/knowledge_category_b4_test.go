package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native knowledge_publishing/org.tsv category_insert and category_invalid.
func TestKnowledgeCategoryGroupB4(t *testing.T) {
	org := testOrg()
	storage.ApplyOrgShape(&org, []string{"Knowledge", "KnowledgeEnabled"})
	storage.EnsureStandardObject(&org, "Knowledge__kav")
	storage.EnsureStandardObject(&org, "Knowledge__DataCategorySelection")
	parent := storage.ID("ka0000000000001")
	articles := org.Objects["Knowledge__kav"]
	articles.Records[parent] = storage.Record{ID: parent, Object: "Knowledge__kav", Fields: map[string]storage.Value{}}
	org.Objects["Knowledge__kav"] = articles
	org.Metadata.DataCategoryGroups = []storage.DataCategoryGroup{{Name: "Guides", Categories: []storage.DataCategory{{Name: "General"}}}}
	engine := NewEngine(&org)
	for _, tc := range []struct {
		group string
		valid bool
	}{{"Guides", true}, {"Guides__c", false}} {
		result := engine.Insert([]storage.Record{{Object: "Knowledge__DataCategorySelection", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent), "DataCategoryGroupName": storage.StringValue(tc.group), "DataCategoryName": storage.StringValue("General")}}})[0]
		if tc.valid && !result.Success {
			t.Errorf("valid: %+v", result)
		}
		if !tc.valid && (result.Success || len(result.Errors) == 0 || result.Errors[0].StatusCode != "INVALID_DATA_CATEGORY_GROUP_REFERENCE") {
			t.Errorf("invalid: %+v", result)
		}
	}
}
