@IsTest private class PersonBoundaryReviewTest {
 @IsTest static void capture() {
  Profile p=[SELECT Id FROM Profile WHERE Name='Route False'];
  User u=new User(ProfileId=p.Id,Username='boundary.'+System.now().getTime()+'@example.invalid',Email='boundary@example.invalid',Alias='bound',LastName='Probe',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  insert u;
  System.runAs(u) { System.assertEquals('CAPTURE',String.join(PersonBoundaryReview.observe(),'~')); }
 }
}
