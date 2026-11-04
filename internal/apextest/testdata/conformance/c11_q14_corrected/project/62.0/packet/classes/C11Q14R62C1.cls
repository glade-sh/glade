@IsTest(SeeAllData=false)
private without sharing class C11Q14R62C1 {
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
  System.assert(false,'CAPTURE|c11.Q14.restricted.api62|'+id+'|'+j);
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

 private static User reader(){List<User> us=[SELECT Id FROM User WHERE LastName='DootSecQ14ReaderB1031' WITH SYSTEM_MODE];System.assertEquals(1,us.size(),'fixture unique reader');return us[0];}
 private static void accessGate(){
  System.assertEquals(true,DootSecQ14B1031__c.SObjectType.getDescribe().isAccessible(),'fixture object readable');
  System.assertEquals(true,DootSecQ14B1031__c.Allowed__c.getDescribe().isAccessible(),'fixture allowed readable');
  System.assertEquals(false,DootSecQ14B1031__c.Hidden__c.getDescribe().isAccessible(),'fixture hidden must be denied');System.assertEquals(true,DootSecQ14B1031__c.Hidden__c.getDescribe().isGroupable(),'fixture hidden text groupable');
  System.assertEquals(false,DootSecQ14B1031__c.Value__c.getDescribe().isAccessible(),'fixture value must be denied');
 }
 @TestSetup static void setupReader(){
  List<Profile> ps=[SELECT Id,PermissionsModifyAllData,PermissionsViewAllData FROM Profile WHERE Name='Minimum Access - Salesforce' WITH SYSTEM_MODE];
  System.assertEquals(1,ps.size(),'fixture requires exact Minimum Access - Salesforce profile');
  System.assertEquals(false,ps[0].PermissionsModifyAllData,'fixture no ModifyAllData');System.assertEquals(false,ps[0].PermissionsViewAllData,'fixture no ViewAllData');
  User u=new User(Alias='dsec',LastName='DootSecQ14ReaderB1031',Email='capture@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='GMT',ProfileId=ps[0].Id,Username='DootSecQ14ReaderB1031.'+UserInfo.getOrganizationId()+'.'+System.now().getTime()+'@example.invalid');insert as system u;
  List<PermissionSet> grants=[SELECT Id FROM PermissionSet WHERE Name='DootSecQ14ReaderB1031' WITH SYSTEM_MODE];System.assertEquals(1,grants.size(),'fixture unique owned permission set');insert as system new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=grants[0].Id);
  
 }
 @IsTest static void allowedGroup(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ14B1031__c> seed=new List<DootSecQ14B1031__c>{new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=50,Allowed__c='ok',OwnerId=r.Id),new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=100,Allowed__c='ok',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ14B1031__c>(seed).keySet();String oname=DootSecQ14B1031__c.SObjectType.getDescribe().getName(),value=DootSecQ14B1031__c.Value__c.getDescribe().getName();System.runAs(r){identityGate(r.Id,o);accessGate();System.assertEquals(2,[SELECT COUNT() FROM DootSecQ14B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture identity');stage='query';o.put('rows',compact(Database.query('SELECT COUNT(Id) n FROM '+oname+' WHERE Id IN :ids WITH USER_MODE GROUP BY Name')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q14.allowedGroup',o);
 }
 @IsTest static void directDenied(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ14B1031__c> seed=new List<DootSecQ14B1031__c>{new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=50,Allowed__c='ok',OwnerId=r.Id),new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=100,Allowed__c='ok',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ14B1031__c>(seed).keySet();String oname=DootSecQ14B1031__c.SObjectType.getDescribe().getName(),value=DootSecQ14B1031__c.Value__c.getDescribe().getName();System.runAs(r){identityGate(r.Id,o);accessGate();System.assertEquals(2,[SELECT COUNT() FROM DootSecQ14B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture identity');stage='query';o.put('rows',compact(Database.query('SELECT '+value+' FROM '+oname+' WHERE Id IN :ids WITH USER_MODE')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q14.directDenied',o);
 }
 @IsTest static void sum(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ14B1031__c> seed=new List<DootSecQ14B1031__c>{new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=50,Allowed__c='ok',OwnerId=r.Id),new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=100,Allowed__c='ok',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ14B1031__c>(seed).keySet();String oname=DootSecQ14B1031__c.SObjectType.getDescribe().getName(),value=DootSecQ14B1031__c.Value__c.getDescribe().getName();System.runAs(r){identityGate(r.Id,o);accessGate();System.assertEquals(2,[SELECT COUNT() FROM DootSecQ14B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture identity');stage='query';o.put('rows',compact(Database.query('SELECT SUM('+value+') total FROM '+oname+' WHERE Id IN :ids WITH USER_MODE')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q14.sum',o);
 }
 @IsTest static void hiddenGroup(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ14B1031__c> seed=new List<DootSecQ14B1031__c>{new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=50,Allowed__c='ok',OwnerId=r.Id),new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=100,Allowed__c='ok',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ14B1031__c>(seed).keySet();String oname=DootSecQ14B1031__c.SObjectType.getDescribe().getName(),value=DootSecQ14B1031__c.Hidden__c.getDescribe().getName();System.runAs(r){identityGate(r.Id,o);accessGate();System.assertEquals(2,[SELECT COUNT() FROM DootSecQ14B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture identity');stage='query';o.put('rows',compact(Database.query('SELECT COUNT(Id) n FROM '+oname+' WHERE Id IN :ids WITH USER_MODE GROUP BY '+value)));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q14.hiddenGroup',o);
 }
 @IsTest static void hiddenHaving(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ14B1031__c> seed=new List<DootSecQ14B1031__c>{new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=50,Allowed__c='ok',OwnerId=r.Id),new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=100,Allowed__c='ok',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ14B1031__c>(seed).keySet();String oname=DootSecQ14B1031__c.SObjectType.getDescribe().getName(),value=DootSecQ14B1031__c.Value__c.getDescribe().getName();System.runAs(r){identityGate(r.Id,o);accessGate();System.assertEquals(2,[SELECT COUNT() FROM DootSecQ14B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture identity');stage='query';o.put('rows',compact(Database.query('SELECT Name,COUNT(Id) n FROM '+oname+' WHERE Id IN :ids WITH USER_MODE GROUP BY Name HAVING SUM('+value+')>0')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q14.hiddenHaving',o);
 }
 @IsTest static void systemGroup(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ14B1031__c> seed=new List<DootSecQ14B1031__c>{new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=50,Allowed__c='ok',OwnerId=r.Id),new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=100,Allowed__c='ok',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ14B1031__c>(seed).keySet();String oname=DootSecQ14B1031__c.SObjectType.getDescribe().getName(),value=DootSecQ14B1031__c.Hidden__c.getDescribe().getName();System.runAs(r){identityGate(r.Id,o);accessGate();System.assertEquals(2,[SELECT COUNT() FROM DootSecQ14B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture identity');stage='query';List<SObject> admitted=Database.query('SELECT COUNT(Id) n FROM '+oname+' WHERE Id IN :ids WITH SYSTEM_MODE GROUP BY '+value);System.assertEquals(1,admitted.size(),'fixture system group count');System.assertEquals(2,admitted[0].get('n'),'fixture system group n');o.put('rows',compact(admitted));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q14.systemGroup',o);
 }
 @IsTest static void systemSum(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ14B1031__c> seed=new List<DootSecQ14B1031__c>{new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=50,Allowed__c='ok',OwnerId=r.Id),new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=100,Allowed__c='ok',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ14B1031__c>(seed).keySet();String oname=DootSecQ14B1031__c.SObjectType.getDescribe().getName(),value=DootSecQ14B1031__c.Value__c.getDescribe().getName();System.runAs(r){identityGate(r.Id,o);accessGate();System.assertEquals(2,[SELECT COUNT() FROM DootSecQ14B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture identity');stage='query';List<SObject> admitted=Database.query('SELECT SUM('+value+') total FROM '+oname+' WHERE Id IN :ids WITH SYSTEM_MODE');System.assertEquals(1,admitted.size(),'fixture system sum size');System.assertEquals(Decimal.valueOf('150'),admitted[0].get('total'),'fixture system sum total');o.put('rows',compact(admitted));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q14.systemSum',o);
 }
 @IsTest static void systemHaving(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ14B1031__c> seed=new List<DootSecQ14B1031__c>{new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=50,Allowed__c='ok',OwnerId=r.Id),new DootSecQ14B1031__c(Name='same',Hidden__c='group-key',Value__c=100,Allowed__c='ok',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ14B1031__c>(seed).keySet();String oname=DootSecQ14B1031__c.SObjectType.getDescribe().getName(),value=DootSecQ14B1031__c.Value__c.getDescribe().getName();System.runAs(r){identityGate(r.Id,o);accessGate();System.assertEquals(2,[SELECT COUNT() FROM DootSecQ14B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture identity');stage='query';List<SObject> admitted=Database.query('SELECT Name,COUNT(Id) n FROM '+oname+' WHERE Id IN :ids WITH SYSTEM_MODE GROUP BY Name HAVING SUM('+value+')>0');System.assertEquals(1,admitted.size(),'fixture system having size');System.assertEquals('same',admitted[0].get('Name'),'fixture system having key');System.assertEquals(2,admitted[0].get('n'),'fixture system having n');o.put('rows',compact(admitted));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q14.systemHaving',o);
 }
 private static void identityGate(Id actorId,Map<String,Object> o){
  System.assertEquals(actorId,UserInfo.getUserId(),'gate exact runAs actor');
  User actor=[SELECT Id,IsActive,ProfileId,Profile.Name,Profile.PermissionsModifyAllData,Profile.PermissionsViewAllData FROM User WHERE Id=:actorId WITH SYSTEM_MODE];
  System.assertEquals(true,actor.IsActive,'gate active test-local actor');
  List<Profile> expectedProfiles=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' WITH SYSTEM_MODE];
  System.assertEquals(1,expectedProfiles.size(),'gate unique expected profile');
  System.assertEquals(expectedProfiles[0].Id,actor.ProfileId,'gate exact actor ProfileId');
  System.assertEquals('Minimum Access - Salesforce',actor.Profile.Name,'gate exact actor profile');
  System.assertEquals(false,actor.Profile.PermissionsModifyAllData,'gate actor ModifyAllData');
  System.assertEquals(false,actor.Profile.PermissionsViewAllData,'gate actor ViewAllData');
  List<PermissionSetAssignment> assignments=[SELECT PermissionSetId,PermissionSetGroupId,PermissionSet.Name,PermissionSet.IsOwnedByProfile,PermissionSet.ProfileId,PermissionSet.PermissionSetGroupId,PermissionSet.PermissionsModifyAllData,PermissionSet.PermissionsViewAllData FROM PermissionSetAssignment WHERE AssigneeId=:actorId WITH SYSTEM_MODE];
  List<PermissionSet> expectedGrants=[SELECT Id FROM PermissionSet WHERE Name='DootSecQ14ReaderB1031' WITH SYSTEM_MODE];
  System.assertEquals(1,expectedGrants.size(),'gate unique expected synthetic permission set');
  Integer explicitCount=0;Integer profileCount=0;
  for(PermissionSetAssignment a:assignments){
   System.assertEquals(null,a.PermissionSetGroupId,'gate no permission set group assignment');
   System.assertEquals(null,a.PermissionSet.PermissionSetGroupId,'gate no group-derived permission set');
   if(a.PermissionSetId==expectedGrants[0].Id){
    System.assertEquals('DootSecQ14ReaderB1031',a.PermissionSet.Name,'gate exact synthetic permission set name');
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
  System.assertEquals('DootSecQ14B1031__c',DootSecQ14B1031__c.SObjectType.getDescribe().getName(),'gate exact unnamespaced synthetic type');
  o.put('arm','restricted');o.put('api','62.0');o.put('orgId',org.Id);o.put('userId',actor.Id);o.put('profileId',actor.ProfileId);
  o.put('locale',UserInfo.getLocale());o.put('tz',UserInfo.getTimeZone().getID());o.put('namespace','');
 }
}
