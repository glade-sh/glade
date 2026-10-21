package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestKnowledgeBaseVersionViewNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"count", "find"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				body := `
Knowledge__kav article=new Knowledge__kav(Title='Owned Base Search',UrlName='owned-base-search',Language='en_US');
Database.insert(article,AccessLevel.SYSTEM_MODE);
article=[SELECT Id,KnowledgeArticleId FROM Knowledge__kav WHERE Id=:article.Id WITH SYSTEM_MODE];
KbManagement.PublishingService.publishArticle(article.KnowledgeArticleId,true);
`
				if route == "count" {
					body += `System.assertEquals(1,[SELECT COUNT() FROM KnowledgeArticleVersion WHERE Id=:article.Id]);`
				} else {
					body += `
String query='FIND {Owned*} IN ALL FIELDS RETURNING KnowledgeArticleVersion(Id,Title WHERE PublishStatus=\'Online\' AND Language=\'en_US\')';
Test.setFixedSearchResults(new List<Id>{article.Id});
List<Search.SearchResult> found=Search.find(query).get('KnowledgeArticleVersion');
System.assertEquals(1,found.size());
System.assertEquals(article.Id,found[0].getSObject().Id);
System.assertEquals('KnowledgeArticleVersion',found[0].getSObject().getSObjectType().getDescribe().getName());
Test.setFixedSearchResults(new List<Id>{article.KnowledgeArticleId});
System.assertEquals(0,Search.find(query).get('KnowledgeArticleVersion').size());
`
				}
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				for _, name := range []string{"Knowledge__kav", "Knowledge__ka", "KnowledgeArticleVersion"} {
					storage.EnsureStandardObject(&org, name)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				machine.EnableTestContext()
				if _, err = machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				if got := len(org.Objects["KnowledgeArticleVersion"].Records); got != 0 {
					t.Fatalf("query persisted %d synthetic version rows", got)
				}
			})
		}
	}
}
