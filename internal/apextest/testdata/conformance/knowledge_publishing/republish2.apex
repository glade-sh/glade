String tag=String.valueOf(Datetime.now().getTime()); Knowledge__kav k=new Knowledge__kav(Title='Cycle '+tag,UrlName='cycle-'+tag,Language='en_US');insert k;k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId;
KbManagement.PublishingService.publishArticle(parent,false); Id draft=KbManagement.PublishingService.editOnlineArticle(parent,false); KbManagement.PublishingService.publishArticle(parent,false);
System.debug('B4:old_count:'+ [SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id]);
Knowledge__kav latest=[SELECT PublishStatus,VersionNumber FROM Knowledge__kav WHERE Id=:draft];System.debug('B4:republish:'+latest.PublishStatus+':'+latest.VersionNumber);
