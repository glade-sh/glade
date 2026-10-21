package vm

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These API 62/67 observations came from named tests in a Knowledge-enabled
// scratch org. Their terminal assertions carry observations, not passing tests.
func TestKnowledgePublishingNative(t *testing.T) {
	var data struct {
		Observations []struct {
			API, Terminal               string
			ComponentErrors, TestErrors int
			Observed                    struct {
				InputParentNull, ParentPresent, Latest                                             bool
				DraftStatus, ParentType, IDArgument, StringArgument, PublishedStatus, NullArgument string
				Version                                                                            int
			}
			EdgeObservation struct {
				Terminal                    string
				ComponentErrors, TestErrors int
				Observed                    struct {
					ParentRows, ParentsAfterRollback int
					MissingParent, InvalidID         string
				}
			}
		}
	}
	raw, err := os.ReadFile("testdata/knowledge_publishing_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Observations) != 2 {
		t.Fatal("missing native observations")
	}
	for _, row := range data.Observations {
		t.Run(row.API, func(t *testing.T) {
			o := row.Observed
			if row.Terminal != "observation_assertion" || row.ComponentErrors != 0 || row.TestErrors != 1 || !o.InputParentNull || !o.ParentPresent || !o.Latest || o.DraftStatus != "Draft" || o.Version != 0 || o.ParentType != "Knowledge__ka" || o.IDArgument != "returned" || o.StringArgument != "returned" || o.PublishedStatus != "Online" || o.NullArgument != "System.InvalidParameterValueException" {
				t.Fatal("native publishing observation differs")
			}
			e := row.EdgeObservation
			if e.Terminal != "observation_assertion" || e.ComponentErrors != 0 || e.TestErrors != 1 || e.Observed.ParentRows != 1 || e.Observed.ParentsAfterRollback != 0 || e.Observed.MissingParent != "System.HandledException" || e.Observed.InvalidID != "System.InvalidParameterValueException" {
				t.Fatal("native parent/rollback observation differs")
			}
			for name, source := range map[string]string{
				"stored_defaults": `
Knowledge__kav article=new Knowledge__kav(Title='Owned Defaults',UrlName='owned-defaults',Language='en_US');
Database.insert(article,AccessLevel.SYSTEM_MODE);
System.assertEquals(null,article.KnowledgeArticleId);
Knowledge__kav loaded=[SELECT KnowledgeArticleId,PublishStatus,VersionNumber,IsLatestVersion FROM Knowledge__kav WHERE Id=:article.Id WITH SYSTEM_MODE];
System.assertNotEquals(null,loaded.KnowledgeArticleId);
System.assertEquals('Knowledge__ka',String.valueOf(loaded.KnowledgeArticleId.getSObjectType()));
System.assertEquals('Draft',loaded.PublishStatus);
System.assertEquals(0,loaded.VersionNumber);
System.assertEquals(true,loaded.IsLatestVersion);`,
				"publish_id_and_string": `
List<Knowledge__kav> articles=new List<Knowledge__kav>{new Knowledge__kav(Title='Owned ID',UrlName='owned-id',Language='en_US'),new Knowledge__kav(Title='Owned String',UrlName='owned-string',Language='en_US')};
Database.insert(articles,AccessLevel.SYSTEM_MODE);
Knowledge__kav first=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:articles[0].Id WITH SYSTEM_MODE];
Knowledge__kav second=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:articles[1].Id WITH SYSTEM_MODE];
KbManagement.PublishingService.publishArticle(first.KnowledgeArticleId,true);
KbManagement.PublishingService.publishArticle(String.valueOf(second.KnowledgeArticleId),false);
System.assertEquals(2,[SELECT COUNT() FROM Knowledge__kav WHERE PublishStatus='Online' WITH SYSTEM_MODE]);`,
				"null_argument": `
try { KbManagement.PublishingService.publishArticle(null,false); System.assert(false,'expected invalid parameter'); }
catch (System.InvalidParameterValueException expected) { }`,
				"parent_rollback_and_errors": `
Savepoint point=Database.setSavepoint();
Knowledge__kav article=new Knowledge__kav(Title='Owned Rollback',UrlName='owned-rollback',Language='en_US');
Database.insert(article,AccessLevel.SYSTEM_MODE);
article=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:article.Id WITH SYSTEM_MODE];
Id parentId=article.KnowledgeArticleId;
System.assertEquals(1,[SELECT COUNT() FROM Knowledge__ka WHERE Id=:parentId WITH SYSTEM_MODE]);
Database.rollback(point);
System.assertEquals(0,[SELECT COUNT() FROM Knowledge__ka WHERE Id=:parentId WITH SYSTEM_MODE]);
try { KbManagement.PublishingService.publishArticle(parentId,false); System.assert(false,'expected missing article'); }
catch (System.HandledException expected) { }
try { KbManagement.PublishingService.publishArticle('bad-id',false); System.assert(false,'expected invalid parameter'); }
catch (System.InvalidParameterValueException expected) { }`,
			} {
				t.Run(name, func(t *testing.T) {
					p, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row.API})
					if err != nil {
						t.Fatal(err)
					}
					org := testDataOrg()
					storage.EnsureStandardObject(&org, "Knowledge__kav")
					// Both schemas exist before the savepoint in the native org.
					storage.EnsureStandardObject(&org, "Knowledge__ka")
					m := New(nil)
					m.SetOrg(&org)
					m.EnableTestContext()
					if _, err = m.Execute(p); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
