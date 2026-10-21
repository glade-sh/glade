package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local integration checks for generated state in the shared DML and rollback
// paths. The native observations are in TestKnowledgePublishingNative.
func TestKnowledgeGeneratedStateTransactions(t *testing.T) {
	for _, journal := range []bool{false, true} {
		mode := "snapshot"
		if journal {
			mode = "journal"
		}
		for name, source := range map[string]string{
			"publish_rollback": `
Knowledge__kav article=new Knowledge__kav(Title='Owned Transaction',UrlName='owned-transaction',Language='en_US');
Database.insert(article,AccessLevel.SYSTEM_MODE);
article=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:article.Id WITH SYSTEM_MODE];
Savepoint point=Database.setSavepoint();
KbManagement.PublishingService.publishArticle(article.KnowledgeArticleId,false);
System.assertEquals('Online',[SELECT PublishStatus FROM Knowledge__kav WHERE Id=:article.Id WITH SYSTEM_MODE].PublishStatus);
Database.rollback(point);
System.assertEquals('Draft',[SELECT PublishStatus FROM Knowledge__kav WHERE Id=:article.Id WITH SYSTEM_MODE].PublishStatus);`,
			"upsert_update_preserves_parent": `
Knowledge__kav article=new Knowledge__kav(Title='Owned Upsert',UrlName='owned-upsert',Language='en_US');
Database.UpsertResult created=Database.upsert(article,AccessLevel.SYSTEM_MODE);
System.assertEquals(true,created.isSuccess());
article=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:article.Id WITH SYSTEM_MODE];
Id parentId=article.KnowledgeArticleId;
System.assertNotEquals(null,parentId);
KbManagement.PublishingService.publishArticle(parentId,true);
Database.upsert(new Knowledge__kav(Id=article.Id,Title='Owned Changed'),AccessLevel.SYSTEM_MODE);
article=[SELECT KnowledgeArticleId,PublishStatus,Title FROM Knowledge__kav WHERE Id=:article.Id WITH SYSTEM_MODE];
System.assertEquals(parentId,article.KnowledgeArticleId);
System.assertEquals('Online',article.PublishStatus);
System.assertEquals('Owned Changed',article.Title);
System.assertEquals(1,[SELECT COUNT() FROM Knowledge__ka WITH SYSTEM_MODE]);`,
			"partial_failure_creates_one_parent": `
Knowledge__kav existing=new Knowledge__kav(Title='Owned Existing',UrlName='owned-existing',Language='en_US');
Database.insert(existing,AccessLevel.SYSTEM_MODE);
List<Knowledge__kav> articles=new List<Knowledge__kav>{new Knowledge__kav(Title='Owned Partial',UrlName='owned-partial',Language='en_US'),new Knowledge__kav(Id=existing.Id,Title='Reject')};
List<Database.SaveResult> rows=Database.insert(articles,false,AccessLevel.SYSTEM_MODE);
System.assertEquals(true,rows[0].isSuccess());
System.assertEquals(false,rows[1].isSuccess());
System.assertEquals(2,[SELECT COUNT() FROM Knowledge__kav WITH SYSTEM_MODE]);
System.assertEquals(2,[SELECT COUNT() FROM Knowledge__ka WITH SYSTEM_MODE]);`,
		} {
			t.Run(mode+"/"+name, func(t *testing.T) {
				program, err := CompileAnonymous(source)
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				storage.EnsureStandardObject(&org, "Knowledge__kav")
				storage.EnsureStandardObject(&org, "Knowledge__ka")
				machine := New(nil)
				machine.SetOrg(&org)
				machine.EnableTestContext()
				if journal {
					machine.SetIsolationJournal(storage.NewIsolationJournal(&org))
				}
				if _, err = machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
