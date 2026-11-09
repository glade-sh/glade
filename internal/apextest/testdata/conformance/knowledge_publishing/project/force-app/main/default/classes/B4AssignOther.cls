@IsTest private class B4AssignOther { @IsTest static void observe() {
 User current=[SELECT ProfileId,TimeZoneSidKey,LocaleSidKey,EmailEncodingKey,LanguageLocaleKey FROM User WHERE Id=:UserInfo.getUserId()];
 User target=new User(Username='synthetic-B4AssignOther@example.invalid',Email='synthetic@example.invalid',LastName='Synthetic',Alias='b4test',ProfileId=current.ProfileId,TimeZoneSidKey=current.TimeZoneSidKey,LocaleSidKey=current.LocaleSidKey,EmailEncodingKey=current.EmailEncodingKey,LanguageLocaleKey=current.LanguageLocaleKey,UserPermissionsKnowledgeUser=true);insert target;
 System.runAs(current) {
 Knowledge__kav k=new Knowledge__kav(Title='Other Test',UrlName='other-test',Language='en_US');insert k;k=[SELECT KnowledgeArticleId,AssignedToId FROM Knowledge__kav WHERE Id=:k.Id];System.assertEquals(UserInfo.getUserId(),k.AssignedToId);
 KbManagement.PublishingService.assignDraftArticleTask(k.KnowledgeArticleId,target.Id,'Review',null,false);
 k=[SELECT AssignedToId,AssignedById,AssignmentDate,AssignmentDueDate,AssignmentNote FROM Knowledge__kav WHERE Id=:k.Id];System.assertEquals(target.Id,k.AssignedToId);System.assertEquals(UserInfo.getUserId(),k.AssignedById);System.assertNotEquals(null,k.AssignmentDate);System.assertEquals(null,k.AssignmentDueDate);System.assertEquals('Review',k.AssignmentNote);
 }
} }