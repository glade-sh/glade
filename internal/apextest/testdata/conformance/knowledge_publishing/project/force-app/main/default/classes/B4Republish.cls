@IsTest private class B4Republish { @IsTest static void observe() {
Knowledge__kav k=new Knowledge__kav(Title='Cycle Test',UrlName='cycle-test',Language='en_US');insert k;k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id];Id parent=k.KnowledgeArticleId;
KbManagement.PublishingService.publishArticle(parent,false);Id draft=KbManagement.PublishingService.editOnlineArticle(parent,false);KbManagement.PublishingService.publishArticle(parent,false);
System.assertEquals(0,[SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id]);Knowledge__kav latest=[SELECT PublishStatus,VersionNumber FROM Knowledge__kav WHERE Id=:draft];System.assertEquals('Online',latest.PublishStatus);System.assertEquals(1,latest.VersionNumber);
} }