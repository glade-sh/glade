package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The API 62/67 category_insert and category_invalid native rows use the
// same synthetic group and a draft article.
func TestKnowledgeCategorySelectionB4(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for name, body := range map[string]string{
			"valid":         `insert new Knowledge__DataCategorySelection(ParentId=k.Id,DataCategoryGroupName='Guides',DataCategoryName='General'); System.assertEquals(1,[SELECT COUNT() FROM Knowledge__DataCategorySelection WHERE ParentId=:k.Id]);`,
			"invalid_group": `try { insert new Knowledge__DataCategorySelection(ParentId=k.Id,DataCategoryGroupName='Guides__c',DataCategoryName='General__c'); System.assert(false); } catch(DmlException e) { System.assertEquals('INVALID_DATA_CATEGORY_GROUP_REFERENCE',String.valueOf(e.getDmlStatusCode(0))); }`,
		} {
			t.Run(api+"/"+name, func(t *testing.T) {
				src := `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US');insert k;` + body
				p, err := CompileAnonymousWithOptions(src, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				storage.ApplyOrgShape(&org, []string{"Knowledge", "KnowledgeEnabled"})
				storage.EnsureStandardObject(&org, "Knowledge__kav")
				storage.EnsureStandardObject(&org, "Knowledge__ka")
				storage.EnsureStandardObject(&org, "Knowledge__DataCategorySelection")
				org.Metadata.DataCategoryGroups = []storage.DataCategoryGroup{{Name: "Guides", Categories: []storage.DataCategory{{Name: "All", Children: []storage.DataCategory{{Name: "General"}}}}}}
				m := New(nil)
				m.SetOrg(&org)
				m.EnableTestContext()
				if _, err = m.Execute(p); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
