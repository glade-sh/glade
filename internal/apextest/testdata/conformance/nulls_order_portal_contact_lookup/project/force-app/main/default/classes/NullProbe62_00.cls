@IsTest private class NullProbe62_00 {
 static User user(String alias, Id profile, Id role, Id contact) {return new User(Alias=alias,Email=alias+'@example.invalid',EmailEncodingKey='UTF-8',LastName=alias,LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='GMT',ProfileId=profile,UserRoleId=role,ContactId=contact,Username=alias+Math.abs(Crypto.getRandomInteger())+'@example.invalid');}
 @IsTest static void m00(){
  UserRole role=new UserRole(Name='Synthetic Owner');insert role;
  Id admin=[SELECT Id FROM Profile WHERE Name='System Administrator' LIMIT 1].Id;
  Id external=[SELECT Id FROM Profile WHERE Name='Customer Community User' LIMIT 1].Id;
  User owner=user('lookupow',admin,role.Id,null);insert owner;List<Contact> contacts=new List<Contact>();
  System.runAs(owner){ Account a=new Account(Name='Synthetic Portal');insert a;contacts.add(new Contact(LastName='Alpha',AccountId=a.Id));contacts.add(new Contact(LastName='Beta',AccountId=a.Id));insert contacts; }
  List<Id> contactIds=new List<Id>{contacts[0].Id,contacts[1].Id};contactIds.sort();
  User low=user('lookuplo',external,null,contactIds[0]);User high=user('lookuphi',external,null,contactIds[1]);insert new List<User>{low,high};
  Set<Id> ids=new Set<Id>{UserInfo.getUserId(),low.Id,high.Id};Map<Id,User> saved=new Map<Id,User>([SELECT Id,ContactId FROM User WHERE Id IN :ids]);
  if(saved.size()!=3 || saved.get(UserInfo.getUserId()).ContactId!=null || saved.get(low.Id).ContactId!=contactIds[0] || saved.get(high.Id).ContactId!=contactIds[1])System.assert(false,'invalid saved shape');
  Map<Id,String> labels=new Map<Id,String>{UserInfo.getUserId()=>'N',low.Id=>'L',high.Id=>'H'};Map<String,String> obs=new Map<String,String>();
  for(String tail:new List<String>{'NULLS FIRST','NULLS LAST','ASC','DESC','DESC NULLS LAST'}){List<String> seen=new List<String>();for(SObject x:Database.query('SELECT Id FROM User WHERE Id IN :ids ORDER BY ContactId '+tail))seen.add(labels.get((Id)x.get('Id')));obs.put(tail,String.join(seen,''));}
  System.assert(false,'CAPTURE|User|'+JSON.serialize(new Map<String,Object>{'ContactId'=>obs}));
 }
}
