@IsTest(SeeAllData=false)
private without sharing class C11Q2A67C1 {
 @IsTest static void baseline(){System.assertEquals(2,1+1,'BASELINE|arithmetic');}

 private static Map<String,Object> failure(String stage,Exception e){
  Map<String,Object> x=new Map<String,Object>{'stage'=>stage,'type'=>e.getTypeName(),'message'=>e.getMessage()};
  if(e instanceof QueryException)x.put('inaccessible',((QueryException)e).getInaccessibleFields());
  if(e instanceof DmlException){DmlException d=(DmlException)e;List<Object> rows=new List<Object>();for(Integer i=0;i<d.getNumDml();i++)rows.add(new Map<String,Object>{'i'=>d.getDmlIndex(i),'code'=>String.valueOf(d.getDmlStatusCode(i)),'message'=>d.getDmlMessage(i),'fields'=>d.getDmlFieldNames(i)});x.put('dml',rows);}return x;
 }
 private static Object compactValue(Object v){if(v instanceof SObject){Map<String,Object> m=new Map<String,Object>(),raw=((SObject)v).getPopulatedFieldsAsMap();for(String k:raw.keySet())m.put(k,compactValue(raw.get(k)));return m;}if(v instanceof List<SObject>)return compact((List<SObject>)v);return v;}
 private static List<Object> compact(List<SObject> rows){List<Object> xs=new List<Object>();for(SObject r:rows)xs.add(compactValue(r));return xs;}
 private static void emit(String id,Map<String,Object> o){
  String j=JSON.serialize(o);if(j.length()>2048)j=JSON.serialize(new Map<String,Object>{'status'=>'inconclusive_overlength','originalLength'=>j.length()});
  System.assert(false,'CAPTURE|c11.Q2.admin.api67|'+id+'|'+j);
 }


 private static void verifySeed(SObject seed){
  Id fixtureId=(Id)seed.get('Id');Map<String,Object> expected=new Map<String,Object>(seed.getPopulatedFieldsAsMap());
  Schema.DescribeSObjectResult ds=seed.getSObjectType().getDescribe();
  if(ds.fields.getMap().containsKey('OwnerId')&&!expected.containsKey('OwnerId'))expected.put('OwnerId',UserInfo.getUserId());
  List<String> fields=new List<String>(expected.keySet());
  List<SObject> actual=Database.query('SELECT '+String.join(fields,',')+' FROM '+ds.getName()+' WHERE Id=:fixtureId WITH SYSTEM_MODE');
  System.assertEquals(1,actual.size(),'fixture exact system readback');
  for(String f:fields)System.assertEquals(expected.get(f),actual[0].get(f),'fixture persisted '+f);
 }
 private static void verifySeeds(List<SObject> seeds){for(SObject seed:seeds)verifySeed(seed);}

 private static User reader(){List<User> us=[SELECT Id FROM User WHERE LastName='DootSecQ2ReaderB1031' WITH SYSTEM_MODE];System.assertEquals(1,us.size(),'fixture unique reader');return us[0];}
 private static void accessGate(){
  System.assertEquals(true,DootSecQ2B1031__c.SObjectType.getDescribe().isAccessible(),'fixture object readable');
  System.assertEquals(true,DootSecQ2B1031__c.Allowed__c.getDescribe().isAccessible(),'fixture allowed readable');
  System.assertEquals(true,DootSecQ2B1031__c.Hidden__c.getDescribe().isAccessible(),'fixture admin hidden readable');
  System.assertEquals(true,DootSecQ2B1031__c.Value__c.getDescribe().isAccessible(),'fixture admin value readable');
 }
 @TestSetup static void setupReader(){
  List<Profile> ps=[SELECT Id,PermissionsModifyAllData,PermissionsViewAllData FROM Profile WHERE Name='Minimum Access - Salesforce' WITH SYSTEM_MODE];
  System.assertEquals(1,ps.size(),'fixture requires exact Minimum Access - Salesforce profile');
  System.assertEquals(false,ps[0].PermissionsModifyAllData,'fixture no ModifyAllData');System.assertEquals(false,ps[0].PermissionsViewAllData,'fixture no ViewAllData');
  User u=new User(Alias='dsec',LastName='DootSecQ2ReaderB1031',Email='capture@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='GMT',ProfileId=ps[0].Id,Username='DootSecQ2ReaderB1031.'+UserInfo.getOrganizationId()+'.'+System.now().getTime()+'@example.invalid');insert as system u;
  List<PermissionSet> grants=[SELECT Id FROM PermissionSet WHERE Name='DootSecQ2ReaderB1031' WITH SYSTEM_MODE];System.assertEquals(1,grants.size(),'fixture unique owned permission set');insert as system new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=grants[0].Id);
  
 
  List<Profile> adminProfiles=[SELECT Id,PermissionsModifyAllData,PermissionsViewAllData FROM Profile WHERE Name='System Administrator' WITH SYSTEM_MODE];
  System.assertEquals(1,adminProfiles.size(),'gate unique existing System Administrator profile');
  System.assertEquals(true,adminProfiles[0].PermissionsModifyAllData,'gate admin profile ModifyAllData');
  System.assertEquals(true,adminProfiles[0].PermissionsViewAllData,'gate admin profile ViewAllData');
  User adminUser=new User(Alias='c11adm',LastName='C11Q2AdminC1',Email='capture@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='GMT',ProfileId=adminProfiles[0].Id,Username='C11Q2AdminC1.'+UserInfo.getOrganizationId()+'.'+System.now().getTime()+'@example.invalid');
  insert as system adminUser;
  System.assertNotEquals(u.Id,adminUser.Id,'gate separate test-local admin and restricted seed users');
  System.assertNotEquals(UserInfo.getUserId(),adminUser.Id,'gate admin actor is not the harness user');
  List<PermissionSet> adminGrants=[SELECT Id FROM PermissionSet WHERE Name='C11Q2AdminC1' WITH SYSTEM_MODE];
  System.assertEquals(1,adminGrants.size(),'gate exact synthetic admin-field permission set');
  insert as system new PermissionSetAssignment(AssigneeId=adminUser.Id,PermissionSetId=adminGrants[0].Id);
 }
 @IsTest static void limitRows(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ2B1031__c> seed=new List<DootSecQ2B1031__c>{new DootSecQ2B1031__c(Name='A-hidden'),new DootSecQ2B1031__c(Name='B-visible',OwnerId=r.Id),new DootSecQ2B1031__c(Name='C-visible',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ2B1031__c>(seed).keySet();String objectName=DootSecQ2B1031__c.SObjectType.getDescribe().getName();System.runAs(admin()){identityGate(admin().Id,o);accessGate();System.assertEquals(3,[SELECT COUNT() FROM DootSecQ2B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture direct visible count');stage='query';o.put('rows',compact(Database.query('SELECT Id,Name FROM '+objectName+' WHERE Id IN :ids WITH USER_MODE ORDER BY Name,Id LIMIT 1')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q2.limit',o);
 }
 @IsTest static void offsetRows(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ2B1031__c> seed=new List<DootSecQ2B1031__c>{new DootSecQ2B1031__c(Name='A-hidden'),new DootSecQ2B1031__c(Name='B-visible',OwnerId=r.Id),new DootSecQ2B1031__c(Name='C-visible',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ2B1031__c>(seed).keySet();String objectName=DootSecQ2B1031__c.SObjectType.getDescribe().getName();System.runAs(admin()){identityGate(admin().Id,o);accessGate();System.assertEquals(3,[SELECT COUNT() FROM DootSecQ2B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture direct visible count');stage='query';o.put('rows',compact(Database.query('SELECT Id,Name FROM '+objectName+' WHERE Id IN :ids WITH USER_MODE ORDER BY Name,Id LIMIT 1 OFFSET 1')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q2.offset',o);
 }
 @IsTest static void unwindowed(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ2B1031__c> seed=new List<DootSecQ2B1031__c>{new DootSecQ2B1031__c(Name='A-hidden'),new DootSecQ2B1031__c(Name='B-visible',OwnerId=r.Id),new DootSecQ2B1031__c(Name='C-visible',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ2B1031__c>(seed).keySet();String objectName=DootSecQ2B1031__c.SObjectType.getDescribe().getName();System.runAs(admin()){identityGate(admin().Id,o);accessGate();System.assertEquals(3,[SELECT COUNT() FROM DootSecQ2B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture direct visible count');stage='query';o.put('rows',compact(Database.query('SELECT Id,Name FROM '+objectName+' WHERE Id IN :ids WITH USER_MODE ORDER BY Name,Id ')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q2.unwindowed',o);
 }
 private static User admin(){List<User> users=[SELECT Id FROM User WHERE LastName='C11Q2AdminC1' WITH SYSTEM_MODE];System.assertEquals(1,users.size(),'gate unique test-local admin');return users[0];}
 private static void identityGate(Id actorId,Map<String,Object> o){
  System.assertEquals(actorId,UserInfo.getUserId(),'gate exact runAs actor');
  User actor=[SELECT Id,IsActive,ProfileId,Profile.Name,Profile.PermissionsModifyAllData,Profile.PermissionsViewAllData FROM User WHERE Id=:actorId WITH SYSTEM_MODE];
  System.assertEquals(true,actor.IsActive,'gate active test-local actor');
  List<Profile> expectedProfiles=[SELECT Id FROM Profile WHERE Name='System Administrator' WITH SYSTEM_MODE];
  System.assertEquals(1,expectedProfiles.size(),'gate unique expected profile');
  System.assertEquals(expectedProfiles[0].Id,actor.ProfileId,'gate exact actor ProfileId');
  System.assertEquals('System Administrator',actor.Profile.Name,'gate exact actor profile');
  System.assertEquals(true,actor.Profile.PermissionsModifyAllData,'gate actor ModifyAllData');
  System.assertEquals(true,actor.Profile.PermissionsViewAllData,'gate actor ViewAllData');
  List<PermissionSetAssignment> assignments=[SELECT PermissionSetId,PermissionSetGroupId,PermissionSet.Name,PermissionSet.IsOwnedByProfile,PermissionSet.ProfileId,PermissionSet.PermissionSetGroupId,PermissionSet.PermissionsModifyAllData,PermissionSet.PermissionsViewAllData FROM PermissionSetAssignment WHERE AssigneeId=:actorId WITH SYSTEM_MODE];
  List<PermissionSet> expectedGrants=[SELECT Id FROM PermissionSet WHERE Name='C11Q2AdminC1' WITH SYSTEM_MODE];
  System.assertEquals(1,expectedGrants.size(),'gate unique expected synthetic permission set');
  Integer explicitCount=0;Integer profileCount=0;
  for(PermissionSetAssignment a:assignments){
   System.assertEquals(null,a.PermissionSetGroupId,'gate no permission set group assignment');
   System.assertEquals(null,a.PermissionSet.PermissionSetGroupId,'gate no group-derived permission set');
   if(a.PermissionSetId==expectedGrants[0].Id){
    System.assertEquals('C11Q2AdminC1',a.PermissionSet.Name,'gate exact synthetic permission set name');
    System.assertEquals(false,a.PermissionSet.IsOwnedByProfile,'gate explicit grant is not profile-owned');
    explicitCount++;System.assertEquals(null,a.PermissionSet.ProfileId,'gate explicit grant has no profile');
    System.assertEquals(false,a.PermissionSet.PermissionsModifyAllData,'gate no supplemental ModifyAllData');
    System.assertEquals(false,a.PermissionSet.PermissionsViewAllData,'gate no supplemental ViewAllData');
   }else{
    System.assertEquals(true,a.PermissionSet.IsOwnedByProfile,'gate no unexpected explicit grant');
    System.assertEquals(actor.ProfileId,a.PermissionSet.ProfileId,'gate profile-owned assignment matches actor profile');
    profileCount++;
   }
  }
  System.assertEquals(1,explicitCount,'gate exact assigned synthetic permission set');
  System.assert(profileCount<=1,'gate at most one actor profile-owned assignment');
  Organization org=[SELECT Id,NamespacePrefix FROM Organization WITH SYSTEM_MODE LIMIT 1];
  System.assertEquals(UserInfo.getOrganizationId(),org.Id,'gate exact Organization identity');
  System.assert(String.isBlank(org.NamespacePrefix),'gate unnamespaced org required');
  System.assertNotEquals(reader().Id,actorId,'gate distinct restricted seed user and admin actor');
  System.assertEquals('DootSecQ2B1031__c',DootSecQ2B1031__c.SObjectType.getDescribe().getName(),'gate exact unnamespaced synthetic type');
  o.put('arm','admin');o.put('api','67.0');o.put('orgId',org.Id);o.put('userId',actor.Id);o.put('profileId',actor.ProfileId);
  o.put('locale',UserInfo.getLocale());o.put('tz',UserInfo.getTimeZone().getID());o.put('namespace','');
 }
}
