String tag=String.valueOf(Datetime.now().getTime()); Knowledge__kav k=new Knowledge__kav(Title='Field '+tag,UrlName='field-'+tag,Language='en_US',Detail__c='owned');insert k;
Knowledge__kav loaded=[SELECT Detail__c,KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; System.debug('B4:field_draft:'+loaded.Detail__c);
KbManagement.PublishingService.publishArticle(loaded.KnowledgeArticleId,false);
System.debug('B4:field_online:'+ [SELECT Detail__c FROM Knowledge__kav WHERE Id=:k.Id].Detail__c);
