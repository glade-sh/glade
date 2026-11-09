String tag=String.valueOf(Datetime.now().getTime());
Knowledge__kav k=new Knowledge__kav(Title='Boundary '+tag,UrlName='boundary-'+tag,Language='en_US'); insert k;
k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId;
try { KbManagement.PublishingService.editOnlineArticle(parent,true); System.debug('B4:edit_unpublished:returned'); } catch(Exception e) { System.debug('B4:edit_unpublished:'+e.getTypeName()); }
KbManagement.PublishingService.publishArticle(parent,true);
Id draft=KbManagement.PublishingService.editOnlineArticle(parent,true);
List<Knowledge__kav> a=[SELECT Id,PublishStatus,VersionNumber FROM Knowledge__kav WHERE PublishStatus='Draft' AND KnowledgeArticleId=:parent];
List<Knowledge__kav> b=[SELECT Id,PublishStatus,VersionNumber FROM Knowledge__kav WHERE PublishStatus='Online' AND KnowledgeArticleId=:parent];
System.debug('B4:edit_true:'+a.size()+':'+b.size()+':'+(a[0].Id==draft)+':'+a[0].VersionNumber);
List<Knowledge__kav> c=[SELECT Id,PublishStatus,VersionNumber FROM Knowledge__kav WHERE PublishStatus='Archived' AND KnowledgeArticleId=:parent];
System.debug('B4:edit_true_archived:'+c.size());
System.debug('B4:edit_true_original_count:'+ [SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id]);
Knowledge__kav prior=[SELECT Id,PublishStatus,VersionNumber FROM Knowledge__kav WHERE Id=:k.Id]; System.debug('B4:edit_true_original:'+prior.PublishStatus+':'+prior.VersionNumber+':'+(draft==prior.Id));
