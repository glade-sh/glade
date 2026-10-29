@IsTest private class CedarExplicitAccessTest {
 private static User actor() {
  User person=new User(Alias='cedar',Email='cedar@example.invalid',EmailEncodingKey='UTF-8',LastName='Cedar Observer',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',Username='cedar-'+UserInfo.getOrganizationId()+'@example.invalid',ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User'].Id);
  insert person; return person;
 }
 private static String failure(Exception error) { return error.getTypeName(); }
 private static SObject sample(String name,Boolean lower) {
  SObject r=Schema.getGlobalDescribe().get(name).newSObject();
  r.put(lower?'name':'Name','Cedar');r.put(lower?'detail__c':'Detail__c','detail');return r;
 }
 private static void describe(Boolean lower,String method) {
  User person=actor(); Map<String,Object> row=new Map<String,Object>();
  System.runAs(person) {
   for(String name:new List<String>{'CedarNote__c','CedarCard__c'}) {
    Schema.DescribeSObjectResult o=Schema.getGlobalDescribe().get(lower?name.toLowerCase():name).getDescribe();
    Schema.DescribeFieldResult f=o.fields.getMap().get(lower?'detail__c':'Detail__c').getDescribe();
    row.put(name,new List<Boolean>{o.isAccessible(),o.isCreateable(),o.isUpdateable(),o.isDeletable(),f.isAccessible(),f.isCreateable(),f.isUpdateable()});
   }
  }
  System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(row));
 }
 private static void queries(Boolean lower,String method) {
  User person=actor();Map<String,Object> row=new Map<String,Object>();
  System.runAs(new User(Id=UserInfo.getUserId())) { insert new CedarNote__c(Name='Seed');insert new CedarCard__c(Name='Seed'); }
  System.runAs(person) {
   for(String canonical:new List<String>{'CedarNote__c','CedarCard__c'}) {
    String name=lower?canonical.toLowerCase():canonical; Map<String,String> r=new Map<String,String>();
    try {r.put('plainCount',String.valueOf(Database.countQuery('SELECT COUNT() FROM '+name)));}catch(Exception e){r.put('plainCount',failure(e));}
    try {r.put('systemCount',String.valueOf(Database.countQuery('SELECT COUNT() FROM '+name+' WITH SYSTEM_MODE')));}catch(Exception e){r.put('systemCount',failure(e));}
    try {Database.query('SELECT Id,Detail__c FROM '+name+' WITH USER_MODE');r.put('userQuery','ok');}catch(Exception e){r.put('userQuery',failure(e));}
    try {Database.query('SELECT Id,Detail__c FROM '+name+' WITH SECURITY_ENFORCED');r.put('enforcedQuery','ok');}catch(Exception e){r.put('enforcedQuery',failure(e));}
    try {Database.insert(sample(name,lower));r.put('plainInsert','ok');}catch(Exception e){r.put('plainInsert',failure(e));}
    try {Database.insert(sample(name,lower),AccessLevel.USER_MODE);r.put('userInsert','ok');}catch(Exception e){r.put('userInsert',failure(e));}
    try {Database.insert(sample(name,lower),AccessLevel.SYSTEM_MODE);r.put('systemInsert','ok');}catch(Exception e){r.put('systemInsert',failure(e));}
    row.put(canonical,r);
   }
  }
  System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(row));
 }
 private static void strips(Boolean lower,String method) {
  User person=actor();Map<String,Object> row=new Map<String,Object>();
  System.runAs(person) {
   for(String canonical:new List<String>{'CedarNote__c','CedarCard__c'}) {
    String name=lower?canonical.toLowerCase():canonical;Map<String,String> r=new Map<String,String>();
    for(System.AccessType mode:new List<System.AccessType>{System.AccessType.READABLE,System.AccessType.CREATABLE,System.AccessType.UPDATABLE}) {
     try {SObjectAccessDecision d=Security.stripInaccessible(mode,new List<SObject>{sample(name,lower)},false);Set<String> f=d.getRemovedFields().get(canonical);r.put(String.valueOf(mode),String.valueOf(f!=null&&f.contains('Name'))+':'+String.valueOf(f!=null&&f.contains('Detail__c')));}catch(Exception e){r.put(String.valueOf(mode),failure(e));}
    }
    try {Security.stripInaccessible(System.AccessType.READABLE,new List<SObject>{sample(name,lower)});r.put('enforced','ok');}catch(Exception e){r.put('enforced',failure(e));}
    row.put(canonical,r);
   }
   row.put('empty',Security.stripInaccessible(System.AccessType.CREATABLE,new List<SObject>(),false).getRecords().size());
  }
  System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(row));
 }
 @IsTest private static void explicitDescribeAccess(){describe(false,'explicitDescribeAccess');}
 @IsTest private static void explicitDescribeCase(){describe(true,'explicitDescribeCase');}
 @IsTest private static void explicitQueryAndDml(){queries(false,'explicitQueryAndDml');}
 @IsTest private static void explicitQueryCase(){queries(true,'explicitQueryCase');}
 @IsTest private static void explicitStripAccess(){strips(false,'explicitStripAccess');}
 @IsTest private static void explicitStripCase(){strips(true,'explicitStripCase');}
}
