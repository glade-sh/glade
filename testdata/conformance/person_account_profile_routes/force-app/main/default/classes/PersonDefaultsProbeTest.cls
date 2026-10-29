@IsTest
private class PersonDefaultsProbeTest {
 @IsTest static void genericDefault(){
  Profile generic=[SELECT Id FROM Profile WHERE Name='Default Generic'];
  User u=new User(ProfileId=generic.Id,Username='generic.'+System.now().getTime()+'@example.invalid',Email='generic@example.invalid',Alias='rtgen',LastName='Generic Probe',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  insert u;
  System.runAs(u){System.assertEquals('CAPTURE',String.join(PersonDefaultsProbe.observe('generic'),';'));}
 }
 @IsTest static void profileDefaults(){
  Profile alternate=[SELECT Id FROM Profile WHERE Name='Default Alternate'];
  User u=new User(ProfileId=alternate.Id,Username='defaults.'+System.now().getTime()+'@example.invalid',Email='defaults@example.invalid',Alias='rtdef',LastName='Default Probe',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  insert u;
  List<String> rows=PersonDefaultsProbe.observe('admin');
  System.runAs(u){rows.addAll(PersonDefaultsProbe.observe('alternate'));}
  rows.addAll(PersonDefaultsProbe.observe('restored'));
  System.assertEquals('CAPTURE',String.join(rows,';'));
 }
}
