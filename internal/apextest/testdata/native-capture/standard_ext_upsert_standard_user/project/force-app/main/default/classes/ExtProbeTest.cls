@IsTest private class ExtProbeTest {
 @IsTest static void extProbe() {
  Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  String salt=String.valueOf(Math.abs(Crypto.getRandomInteger()));
  User u=new User(Alias='stdprobe',Email='probe@example.invalid',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',Username='synthetic'+salt+'@example.invalid',ProfileId=p.Id);
  insert u;
  System.runAs(u) { System.assertEquals('', 'OBSERVED_EXT\n' + String.join(ExtProbe.run(), '\n')); }
 }
}