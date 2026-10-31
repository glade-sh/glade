@IsTest private class PersonFollowupTest {
 @IsTest static void capture() {
  Profile p=[SELECT Id FROM Profile WHERE Name='Route False'];
  User u=new User(ProfileId=p.Id,Username='followup.'+System.now().getTime()+'@example.invalid',Email='followup@example.invalid',Alias='follow',LastName='Probe',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  insert u;
  System.runAs(u) { System.assertEquals('CAPTURE',String.join(PersonFollowup.observe(),';')); }
 }
}
