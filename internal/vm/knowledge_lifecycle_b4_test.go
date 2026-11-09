package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Captured at both 62 and 67 in knowledge_publishing/org.tsv.
func TestKnowledgeLifecycleB4(t *testing.T) {
	cases := map[string]string{
		"published_version":           `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US'); insert k; k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; KbManagement.PublishingService.publishArticle(k.KnowledgeArticleId,false); k=[SELECT PublishStatus,VersionNumber FROM Knowledge__kav WHERE Id=:k.Id]; System.assertEquals('Online',k.PublishStatus); System.assertEquals(1,k.VersionNumber);`,
		"edit_creates_draft":          `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US'); insert k; k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId; KbManagement.PublishingService.publishArticle(parent,false); Id draft=KbManagement.PublishingService.editOnlineArticle(parent,false); System.assertNotEquals(parent,draft); System.assertEquals(1,[SELECT COUNT() FROM Knowledge__kav WHERE KnowledgeArticleId=:parent AND PublishStatus='Draft']);`,
		"edit_unpublished":            `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US'); insert k; k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; try { KbManagement.PublishingService.editOnlineArticle(k.KnowledgeArticleId,true); System.assert(false); } catch(System.HandledException expected) { }`,
		"edit_unpublish":              `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US'); insert k; k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId; KbManagement.PublishingService.publishArticle(parent,true); Id draft=KbManagement.PublishingService.editOnlineArticle(parent,true); System.assertEquals(0,[SELECT COUNT() FROM Knowledge__kav WHERE KnowledgeArticleId=:parent AND PublishStatus='Online']); System.assertEquals(1,[SELECT COUNT() FROM Knowledge__kav WHERE Id=:draft AND PublishStatus='Draft']);`,
		"assign_no_draft":             `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US'); insert k; k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId; KbManagement.PublishingService.publishArticle(parent,true); try { KbManagement.PublishingService.assignDraftArticleTask(parent,UserInfo.getUserId(),'Synthetic task',Datetime.now().addDays(1),true); System.assert(false); } catch(System.HandledException expected) { }`,
		"missing_archive":             `try { KbManagement.PublishingService.archiveOnlineArticle('ka0000000000001',null); System.assert(false); } catch(System.InvalidParameterValueException expected) { }`,
		"archive_rolled_back_parent":  `Savepoint p=Database.setSavepoint(); Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US'); insert k; k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId; Database.rollback(p); try { KbManagement.PublishingService.archiveOnlineArticle(String.valueOf(parent),null); System.assert(false); } catch(System.HandledException expected) { }`,
		"republish_replaces_old":      `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US'); insert k; k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId; KbManagement.PublishingService.publishArticle(parent,false); Id draft=KbManagement.PublishingService.editOnlineArticle(parent,false); KbManagement.PublishingService.publishArticle(parent,false); System.assertEquals(0,[SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id]); Knowledge__kav current=[SELECT PublishStatus,VersionNumber FROM Knowledge__kav WHERE Id=:draft]; System.assertEquals('Online',current.PublishStatus); System.assertEquals(1,current.VersionNumber);`,
		"assign_state":                `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US'); insert k; k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Datetime due=Datetime.now().addDays(1); KbManagement.PublishingService.assignDraftArticleTask(k.KnowledgeArticleId,UserInfo.getUserId(),'Synthetic task',due,true); k=[SELECT AssignedToId,AssignedById,AssignmentDate,AssignmentDueDate,AssignmentNote FROM Knowledge__kav WHERE Id=:k.Id]; System.assertEquals(UserInfo.getUserId(),k.AssignedToId); System.assertEquals(UserInfo.getUserId(),k.AssignedById); System.assertNotEquals(null,k.AssignmentDate); System.assertEquals(due.getTime()/1000,k.AssignmentDueDate.getTime()/1000); System.assertEquals('Synthetic task',k.AssignmentNote);`,
		"unpublished_original_latest": `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US');insert k;k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id];Id parent=k.KnowledgeArticleId;KbManagement.PublishingService.publishArticle(parent,false);Id draft=KbManagement.PublishingService.editOnlineArticle(parent,true);Knowledge__kav old=[SELECT PublishStatus,VersionNumber,IsLatestVersion FROM Knowledge__kav WHERE Id=:k.Id];System.assertEquals('Archived',old.PublishStatus);System.assertEquals(1,old.VersionNumber);System.assertEquals(false,old.IsLatestVersion);System.assertEquals(0,[SELECT COUNT() FROM Knowledge__kav WHERE PublishStatus='Archived' AND KnowledgeArticleId=:parent]);`,
		"replace_retains_deleted":     `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US');insert k;k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id];Id parent=k.KnowledgeArticleId;KbManagement.PublishingService.publishArticle(parent,false);Id draft=KbManagement.PublishingService.editOnlineArticle(parent,false);KbManagement.PublishingService.publishArticle(parent,false);List<Knowledge__kav> deleted=[SELECT PublishStatus,IsDeleted FROM Knowledge__kav WHERE Id=:k.Id ALL ROWS];System.assertEquals(1,deleted.size());System.assertEquals('Online',deleted[0].PublishStatus);System.assertEquals(true,deleted[0].IsDeleted);`,
		"major_version":               `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US');insert k;k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id];Id parent=k.KnowledgeArticleId;KbManagement.PublishingService.publishArticle(parent,true);Id draft=KbManagement.PublishingService.editOnlineArticle(parent,false);KbManagement.PublishingService.publishArticle(parent,true);Knowledge__kav old=[SELECT PublishStatus,VersionNumber,IsLatestVersion FROM Knowledge__kav WHERE Id=:k.Id];Knowledge__kav now=[SELECT PublishStatus,VersionNumber FROM Knowledge__kav WHERE Id=:draft];System.assertEquals('Archived',old.PublishStatus);System.assertEquals(1,old.VersionNumber);System.assertEquals(false,old.IsLatestVersion);System.assertEquals('Online',now.PublishStatus);System.assertEquals(2,now.VersionNumber);`,
		"archive_online":              `Knowledge__kav k=new Knowledge__kav(Title='Synthetic',UrlName='synthetic',Language='en_US'); insert k; k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId; KbManagement.PublishingService.publishArticle(parent,true); KbManagement.PublishingService.archiveOnlineArticle(parent,null); k=[SELECT PublishStatus,VersionNumber FROM Knowledge__kav WHERE Id=:k.Id]; System.assertEquals('Archived',k.PublishStatus); System.assertEquals(1,k.VersionNumber);`,
	}
	for _, api := range []string{"62.0", "67.0"} {
		for name, source := range cases {
			t.Run(api+"/"+name, func(t *testing.T) {
				p, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				storage.ApplyOrgShape(&org, []string{"Knowledge", "KnowledgeEnabled"})
				storage.EnsureStandardObject(&org, "Knowledge__kav")
				storage.EnsureStandardObject(&org, "Knowledge__ka")
				m := New(nil)
				m.SetOrg(&org)
				m.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"UserPermissionsKnowledgeUser": storage.BooleanValue(true)}})
				m.EnableTestContext()
				if _, err = m.Execute(p); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Issued-parent history belongs to one installed org, not a reusable VM.
func TestKnowledgeIssuedParentsResetOnTemplateOrg(t *testing.T) {
	template := storage.NewRuntimeTemplate(testDataOrg())
	PrimeRuntimeTemplateSchema(&template)
	org := template.CloneRuntimeOrg()
	base := New(nil)
	base.PrimeMetadataSchema(&org)
	machine := base.CloneRuntime(nil)
	machine.knowledgeIssuedParents = map[storage.ID]bool{"ka0000000000001": true}
	machine.SetRuntimeTemplateOrg(&org)
	if len(machine.knowledgeIssuedParents) != 0 {
		t.Fatal("issued parent history leaked into a fresh template org")
	}
	machine.knowledgeIssuedParents = map[storage.ID]bool{"ka0000000000001": true}
	other := testDataOrg()
	machine.SetOrg(&other)
	if len(machine.knowledgeIssuedParents) != 0 {
		t.Fatal("issued parent history leaked into a replacement org")
	}
}
