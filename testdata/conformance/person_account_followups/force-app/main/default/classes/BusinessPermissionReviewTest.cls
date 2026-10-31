@IsTest private class BusinessPermissionReviewTest {
 @IsTest static void readonly() { capture('Route Readonly'); }
 @IsTest static void editonly() { capture('Route Editonly'); }
 private static void capture(String profileName) {
  Profile p=[SELECT Id FROM Profile WHERE Name=:profileName];
  User u=new User(ProfileId=p.Id,Username='perms.'+profileName.replace(' ','')+System.now().getTime()+'@example.invalid',Email='perms@example.invalid',Alias='perms',LastName='Probe',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  insert u;
  List<String> rows=new List<String>();
  System.runAs(u) {
   for(String route:new List<String>{'new_scalar','new_list','new_selector'}) {
    Account a=new Account(Name='Probe Business'); Account b=new Account(Name='Second Business');
    Integer statements=Limits.getDmlStatements(), count=Limits.getDmlRows(); String out='success';
    try { if(route=='new_scalar') { upsert as user a; } else if(route=='new_selector') { upsert as user a Account.Id; } else { List<Account> aa=new List<Account>{a,b}; upsert as user aa; } }
    catch(Exception e) { out=e.getTypeName()+':'+e.getMessage(); }
    rows.add(route+'|'+out+'|'+(Limits.getDmlStatements()-statements)+'|'+(Limits.getDmlRows()-count));
   }
  }
  System.assertEquals('CAPTURE',String.join(rows,'~'));
 }
}
