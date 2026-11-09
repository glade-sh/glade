@IsTest private class B4AccessBoundary62 { @IsTest static void observe() {
 User current=[SELECT ProfileId,TimeZoneSidKey,LocaleSidKey,EmailEncodingKey,LanguageLocaleKey,UserPermissionsKnowledgeUser FROM User WHERE Id=:UserInfo.getUserId()];
 System.assertEquals(false,current.UserPermissionsKnowledgeUser);
 User licensed=new User(Username='synthetic-access-'+String.valueOf(Datetime.now().getTime())+'@example.invalid',Email='synthetic@example.invalid',LastName='Synthetic',Alias='b4lic',ProfileId=current.ProfileId,TimeZoneSidKey=current.TimeZoneSidKey,LocaleSidKey=current.LocaleSidKey,EmailEncodingKey=current.EmailEncodingKey,LanguageLocaleKey=current.LanguageLocaleKey,UserPermissionsKnowledgeUser=true); insert licensed;
 Id parent;
 System.runAs(licensed) { Knowledge__kav article=new Knowledge__kav(Title='Synthetic Access Test',UrlName='synthetic-access-test',Language='en_US'); insert article; parent=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:article.Id].KnowledgeArticleId; }
 try { KbManagement.PublishingService.publishArticle(parent,false); System.assert(false,'expected no access'); } catch(System.NoAccessException expected) { System.debug('B4:unlicensed_valid:System.NoAccessException'); }
} }
