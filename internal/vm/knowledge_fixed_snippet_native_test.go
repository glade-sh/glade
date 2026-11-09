package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestKnowledgeFixedSearchSnippetsNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, clause := range []string{"WITH SNIPPET", "WITH SNIPPET(TARGET_LENGTH=120)"} {
			t.Run(api+"/"+clause, func(t *testing.T) {
				body := `
Knowledge__kav article=new Knowledge__kav(Title='Owned snippet target fixture amber copper silver cedar meadow river cloud stone dawn sunset',UrlName='owned-snippet',Language='en_US');
Database.insert(article,AccessLevel.SYSTEM_MODE);
article=[SELECT Id,KnowledgeArticleId FROM Knowledge__kav WHERE Id=:article.Id WITH SYSTEM_MODE];
KbManagement.PublishingService.publishArticle(article.KnowledgeArticleId,true);
Test.setFixedSearchResults(new List<Id>{article.Id});
String query='FIND {Owned} IN ALL FIELDS RETURNING KnowledgeArticleVersion(Id,Title WHERE PublishStatus=\'Online\' AND Language=\'en_US\') ` + clause + `';
List<Search.SearchResult> found=Search.find(query).get('KnowledgeArticleVersion');
System.assertEquals(1,found.size());
System.assertEquals(null,found[0].getSnippet('Title'));
`
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				// Knowledge__ka and Knowledge__kav exist only with the captured Knowledge setting (org_knowledge_availability/org.tsv).
				storage.ApplyOrgShape(&org, []string{"Knowledge", "KnowledgeEnabled"})
				for _, name := range []string{"Knowledge__kav", "Knowledge__ka", "KnowledgeArticleVersion"} {
					storage.EnsureStandardObject(&org, name)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				machine.SetCurrentUser(storage.Record{ID: storage.DefaultUserID, Object: "User", Fields: map[string]storage.Value{"UserPermissionsKnowledgeUser": storage.BooleanValue(true)}})
				machine.EnableTestContext()
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
