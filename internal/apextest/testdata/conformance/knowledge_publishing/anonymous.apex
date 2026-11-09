String tag=String.valueOf(Datetime.now().getTime());
Knowledge__kav k=new Knowledge__kav(Title='Synthetic '+tag,UrlName='synthetic-'+tag,Language='en_US');
insert k;
k=[SELECT KnowledgeArticleId,PublishStatus,VersionNumber,Language FROM Knowledge__kav WHERE Id=:k.Id];
Id parent=k.KnowledgeArticleId;
System.debug('B4:draft:'+k.PublishStatus+':'+k.VersionNumber+':'+k.Language);
KbManagement.PublishingService.publishArticle(parent,false);
k=[SELECT PublishStatus,VersionNumber FROM Knowledge__kav WHERE Id=:k.Id];
System.debug('B4:published:'+k.PublishStatus+':'+k.VersionNumber);
Id edited=KbManagement.PublishingService.editOnlineArticle(parent,false);
System.debug('B4:edit_parent:'+(edited==parent));
for(KnowledgeArticleVersion v:[SELECT PublishStatus,VersionNumber,Language FROM KnowledgeArticleVersion WHERE PublishStatus='Draft' AND KnowledgeArticleId=:parent]) System.debug('B4:edit_draft:'+v.PublishStatus+':'+v.VersionNumber+':'+v.Language);
try { KbManagement.PublishingService.editOnlineArticle(parent,false); System.debug('B4:edit_again:returned'); } catch(Exception e) { System.debug('B4:edit_again:'+e.getTypeName()); }
try { KbManagement.PublishingService.assignDraftArticleTask(parent,UserInfo.getUserId(),'Synthetic task',Datetime.now().addDays(1),true); System.debug('B4:assign:returned'); } catch(Exception e) { System.debug('B4:assign:'+e.getTypeName()); }
try { KbManagement.PublishingService.archiveOnlineArticle(parent,null); System.debug('B4:archive:returned'); } catch(Exception e) { System.debug('B4:archive:'+e.getTypeName()); }
for(KnowledgeArticleVersion v:[SELECT PublishStatus,VersionNumber,Language FROM KnowledgeArticleVersion WHERE PublishStatus='Archived' AND KnowledgeArticleId=:parent]) System.debug('B4:archived:'+v.PublishStatus+':'+v.VersionNumber+':'+v.Language);
Knowledge__kav other=new Knowledge__kav(Title='Archive '+tag,UrlName='archive-'+tag,Language='en_US');
insert other;
other=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:other.Id];
Id parent2=other.KnowledgeArticleId;
KbManagement.PublishingService.publishArticle(parent2,true);
KbManagement.PublishingService.archiveOnlineArticle(parent2,null);
for(KnowledgeArticleVersion v:[SELECT PublishStatus,VersionNumber,Language FROM KnowledgeArticleVersion WHERE PublishStatus='Archived' AND KnowledgeArticleId=:parent2]) System.debug('B4:archive_clean:'+v.PublishStatus+':'+v.VersionNumber+':'+v.Language);
try { KbManagement.PublishingService.archiveOnlineArticle(parent2,null); System.debug('B4:archive_again:returned'); } catch(Exception e) { System.debug('B4:archive_again:'+e.getTypeName()); }
