String tag=String.valueOf(Datetime.now().getTime());
Knowledge__kav k=new Knowledge__kav(Title='Future '+tag,UrlName='future-'+tag,Language='en_US'); insert k;k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id];KbManagement.PublishingService.publishArticle(k.KnowledgeArticleId,false);
try { KbManagement.PublishingService.archiveOnlineArticle(k.KnowledgeArticleId,Datetime.now().addDays(1)); System.debug('B4:archive_future:returned'); } catch(Exception e) { System.debug('B4:archive_future:'+e.getTypeName()); }
System.debug('B4:archive_future_status:'+ [SELECT PublishStatus FROM Knowledge__kav WHERE Id=:k.Id].PublishStatus);
Knowledge__kav x=new Knowledge__kav(Title='Past '+tag,UrlName='past-'+tag,Language='en_US');insert x;x=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:x.Id];KbManagement.PublishingService.publishArticle(x.KnowledgeArticleId,false);
try { KbManagement.PublishingService.archiveOnlineArticle(x.KnowledgeArticleId,Datetime.now().addDays(-1)); System.debug('B4:archive_past:returned'); } catch(Exception e) { System.debug('B4:archive_past:'+e.getTypeName()); }
System.debug('B4:archive_past_status:'+ [SELECT PublishStatus FROM Knowledge__kav WHERE Id=:x.Id].PublishStatus);
