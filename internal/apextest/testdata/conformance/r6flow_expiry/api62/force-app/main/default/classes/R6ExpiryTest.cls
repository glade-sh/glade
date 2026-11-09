@IsTest private class R6ExpiryTest {
 static User makeUser() { String x='r6.'+String.valueOf(Crypto.getRandomInteger()).replace('-','n')+'@example.invalid'; User u=new User(Username=x,Email=x,Alias='r6test',LastName='Probe',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',ProfileId=UserInfo.getProfileId());insert u;return u; }
 static void observe(String mode) {
  User u=makeUser();PermissionSet ps=[SELECT Id FROM PermissionSet WHERE Name='R6_Bypass' LIMIT 1];PermissionSetAssignment a=new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps.Id);
  if(mode=='future') a.ExpirationDate=Datetime.now().addDays(2);
  if(mode=='past') a.ExpirationDate=Datetime.now().addDays(-2);
  try {insert a;if(mode=='revoked'){a.IsRevoked=true;update a;}}
  catch(Exception e){System.assert(false,'R6|SETUP|'+e.getMessage());}
  System.runAs(u){System.assert(false,'R6|'+String.valueOf(FeatureManagement.checkPermission('R6_Bypass'))+'|'+R6Probe.observe('R6Perm'));}
 }
 @IsTest static void future(){observe('future');}
 @IsTest static void past(){observe('past');}
 @IsTest static void revoked(){observe('revoked');}
}
