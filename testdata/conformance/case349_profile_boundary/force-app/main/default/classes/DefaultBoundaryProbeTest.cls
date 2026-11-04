@IsTest private class DefaultBoundaryProbeTest {
 @IsTest static void person() { Profile p=[SELECT Id FROM Profile WHERE Name='Person Probe']; User u=new User(ProfileId=p.Id,Username='probe.'+System.now().getTime()+'@example.invalid',Email='probe@example.invalid',Alias='probe',LastName='Synthetic',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US'); insert u; System.runAs(u) { System.assertEquals('CAPTURE',DefaultBoundaryProbe.run()); } }
 @IsTest static void admin() { System.assertEquals('CAPTURE',DefaultBoundaryProbe.run()); }
}
