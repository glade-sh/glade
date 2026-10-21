package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// API 62/67 named observations return a fixed version ID, but not its parent.
func TestKnowledgeFixedSearchVersionNative(t *testing.T) {
	runKnowledgeFixedSearchVersion(t, false)
}

func TestKnowledgeFixedSearchGeneratedRuntime(t *testing.T) {
	runKnowledgeFixedSearchVersion(t, true)
}

func runKnowledgeFixedSearchVersion(t *testing.T, generated bool) {
	t.Helper()
	p, err := CompileAnonymousWithOptions(`
Knowledge__kav article=new Knowledge__kav(Title='Owned Search',UrlName='owned-search',Language='en_US');
Database.insert(article,AccessLevel.SYSTEM_MODE);
article=[SELECT Id,KnowledgeArticleId FROM Knowledge__kav WHERE Id=:article.Id WITH SYSTEM_MODE];
KbManagement.PublishingService.publishArticle(article.KnowledgeArticleId,true);
String status='Online';String language='en_US';
String query='FIND {Owned*} IN ALL FIELDS RETURNING Knowledge__kav(Id,Title WHERE PublishStatus=:status AND Language=:language)';
Test.setFixedSearchResults(new List<Id>{article.Id});
List<Search.SearchResult> found=Search.find(query).get('Knowledge__kav');
System.assertEquals(1,found.size());
System.assertEquals(article.Id,found[0].getSObject().Id);
Test.setFixedSearchResults(new List<Id>{article.KnowledgeArticleId});
System.assertEquals(0,Search.find(query).get('Knowledge__kav').size());
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Knowledge__kav")
	storage.EnsureStandardObject(&org, "Knowledge__ka")
	m := New(nil)
	m.SetOrg(&org)
	m.EnableTestContext()
	if generated {
		if err = m.RegisterClass(Class{Name: "Search", PlatformType: true, Access: "global", Methods: map[string]Method{
			"find": {Name: "Search.find", ClassName: "Search", IsStatic: true, Access: "global", ReturnType: "Search.SearchResults", Modifiers: []string{"passive-generated"}, Params: []Param{{Name: "searchQuery", Type: "String"}}},
		}}); err != nil {
			t.Fatal(err)
		}
		if err = m.RegisterClass(Class{Name: "Search.SearchResults", PlatformType: true, Access: "global", Methods: map[string]Method{
			"get": {Name: "Search.SearchResults.get", ClassName: "Search.SearchResults", Access: "global", ReturnType: "List<Search.SearchResult>", Modifiers: []string{"passive-generated"}, Params: []Param{{Name: "sobjectType", Type: "String"}}},
		}}); err != nil {
			t.Fatal(err)
		}
		if err = m.RegisterClass(Class{Name: "Search.SearchResult", PlatformType: true, Access: "global", Methods: map[string]Method{
			"getSObject": {Name: "Search.SearchResult.getSObject", ClassName: "Search.SearchResult", Access: "global", ReturnType: "SObject", Modifiers: []string{"passive-generated"}},
		}}); err != nil {
			t.Fatal(err)
		}
	}

	if _, err = m.Execute(p); err != nil {
		t.Fatal(err)
	}
}
