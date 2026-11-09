@IsTest private class B4Lifecycle { @IsTest static void observe() {
String tag=String.valueOf(Datetime.now().getTime());
Knowledge__kav k=new Knowledge__kav(Title='Synthetic '+tag,UrlName='synthetic-'+tag,Language='en_US');
insert k;
k=[SELECT KnowledgeArticleId,PublishStatus,VersionNumber,Language FROM Knowledge__kav WHERE Id=:k.Id];
Id parent=k.KnowledgeArticleId;
System.assertEquals('Draft:0:en_US',k.PublishStatus+':'+k.VersionNumber+':'+k.Language);
KbManagement.PublishingService.publishArticle(parent,false);
k=[SELECT PublishStatus,VersionNumber FROM Knowledge__kav WHERE Id=:k.Id];
System.assertEquals('Online:1',k.PublishStatus+':'+k.VersionNumber);
Id edited=KbManagement.PublishingService.editOnlineArticle(parent,false);
System.assertNotEquals(parent,edited);
System.assertEquals(1,[SELECT COUNT() FROM KnowledgeArticleVersion WHERE PublishStatus='Draft' AND KnowledgeArticleId=:parent]);
for(KnowledgeArticleVersion v:[SELECT PublishStatus,VersionNumber,Language FROM KnowledgeArticleVersion WHERE PublishStatus='Draft' AND KnowledgeArticleId=:parent]) System.assertEquals('Draft:0:en_US',v.PublishStatus+':'+v.VersionNumber+':'+v.Language);
try { KbManagement.PublishingService.editOnlineArticle(parent,false); System.assert(false,'edit should reject'); } catch(Exception e) { System.assertEquals('System.HandledException',e.getTypeName()); }
try { KbManagement.PublishingService.assignDraftArticleTask(parent,UserInfo.getUserId(),'Synthetic task',Datetime.now().addDays(1),true); System.assert(true); } catch(Exception e) { System.assert(false,e.getTypeName()); }
try { KbManagement.PublishingService.archiveOnlineArticle(parent,null); System.assert(false,'archive should reject'); } catch(Exception e) { System.assertEquals('System.HandledException',e.getTypeName()); }
for(KnowledgeArticleVersion v:[SELECT PublishStatus,VersionNumber,Language FROM KnowledgeArticleVersion WHERE PublishStatus='Archived' AND KnowledgeArticleId=:parent]) System.debug('B4:archived:'+v.PublishStatus+':'+v.VersionNumber+':'+v.Language);
Knowledge__kav other=new Knowledge__kav(Title='Archive '+tag,UrlName='archive-'+tag,Language='en_US');
insert other;
other=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:other.Id];
Id parent2=other.KnowledgeArticleId;
KbManagement.PublishingService.publishArticle(parent2,true);
KbManagement.PublishingService.archiveOnlineArticle(parent2,null);
System.assertEquals(1,[SELECT COUNT() FROM KnowledgeArticleVersion WHERE PublishStatus='Archived' AND KnowledgeArticleId=:parent2]);
for(KnowledgeArticleVersion v:[SELECT PublishStatus,VersionNumber,Language FROM KnowledgeArticleVersion WHERE PublishStatus='Archived' AND KnowledgeArticleId=:parent2]) System.assertEquals('Archived:1:en_US',v.PublishStatus+':'+v.VersionNumber+':'+v.Language);
try { KbManagement.PublishingService.archiveOnlineArticle(parent2,null); System.assert(false,'archive should reject'); } catch(Exception e) { System.assertEquals('System.HandledException',e.getTypeName()); }

} }
