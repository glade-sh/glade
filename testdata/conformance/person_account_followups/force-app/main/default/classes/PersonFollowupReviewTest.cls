@IsTest private class PersonFollowupReviewTest {
 @IsTest static void capture() {
  Profile p=[SELECT Id FROM Profile WHERE Name='Route False'];
  User u=new User(ProfileId=p.Id,Username='review.'+System.now().getTime()+'@example.invalid',Email='review@example.invalid',Alias='review',LastName='Probe',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  insert u;
  System.runAs(u) { System.assertEquals('CAPTURE',String.join(PersonFollowupReview.observe(),'~')); }
 }
}
