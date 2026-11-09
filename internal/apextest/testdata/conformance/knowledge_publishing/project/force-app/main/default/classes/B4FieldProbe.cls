@IsTest private class B4FieldProbe { @IsTest static void observe() {
 Knowledge__kav k=new Knowledge__kav(Title='Field Test',UrlName='field-test',Language='en_US',Detail__c='owned');insert k;
 k=[SELECT Detail__c,KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; System.assertEquals('owned',k.Detail__c);
 KbManagement.PublishingService.publishArticle(k.KnowledgeArticleId,false);
 System.assertEquals('owned',[SELECT Detail__c FROM Knowledge__kav WHERE Id=:k.Id].Detail__c);
} }