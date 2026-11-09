String tag=String.valueOf(Datetime.now().getTime());
Knowledge__kav k=new Knowledge__kav(Title='Details '+tag,UrlName='details-'+tag,Language='en_US'); insert k;
k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId;
KbManagement.PublishingService.publishArticle(parent,false);
Id draft=KbManagement.PublishingService.editOnlineArticle(parent,false);
List<Knowledge__kav> found=[SELECT Id,PublishStatus,VersionNumber,IsLatestVersion FROM Knowledge__kav WHERE PublishStatus='Draft' AND KnowledgeArticleId=:parent]; found.addAll([SELECT Id,PublishStatus,VersionNumber,IsLatestVersion FROM Knowledge__kav WHERE PublishStatus='Online' AND KnowledgeArticleId=:parent]);
System.debug('B4:count:'+found.size());
for(Knowledge__kav x:found) System.debug('B4:version:'+x.PublishStatus+':'+x.VersionNumber+':'+x.IsLatestVersion+':'+(x.Id==draft)+':'+(x.Id==k.Id));
