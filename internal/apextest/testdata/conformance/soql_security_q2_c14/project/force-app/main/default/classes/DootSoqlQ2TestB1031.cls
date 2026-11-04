@IsTest(SeeAllData=false)
private without sharing class DootSoqlQ2TestB1031 {
 @IsTest static void baseline(){System.assertEquals(2,1+1,'BASELINE|arithmetic');}

 private static Map<String,Object> failure(String stage,Exception e){
  Map<String,Object> x=new Map<String,Object>{'stage'=>stage,'type'=>e.getTypeName(),'message'=>e.getMessage()};
  if(e instanceof QueryException)x.put('inaccessible',((QueryException)e).getInaccessibleFields());
  if(e instanceof DmlException){DmlException d=(DmlException)e;List<Object> rows=new List<Object>();for(Integer i=0;i<d.getNumDml();i++)rows.add(new Map<String,Object>{'i'=>d.getDmlIndex(i),'code'=>String.valueOf(d.getDmlStatusCode(i)),'message'=>d.getDmlMessage(i),'fields'=>d.getDmlFieldNames(i)});x.put('dml',rows);}return x;
 }
 private static Object compactValue(Object v){if(v instanceof SObject){Map<String,Object> m=new Map<String,Object>(),raw=((SObject)v).getPopulatedFieldsAsMap();for(String k:raw.keySet())m.put(k,compactValue(raw.get(k)));return m;}if(v instanceof List<SObject>)return compact((List<SObject>)v);return v;}
 private static List<Object> compact(List<SObject> rows){List<Object> xs=new List<Object>();for(SObject r:rows)xs.add(compactValue(r));return xs;}
 private static void emit(String id,Map<String,Object> o){
  o.put('locale',UserInfo.getLocale());o.put('tz',UserInfo.getTimeZone().getID());
  String j=JSON.serialize(o);if(j.length()>620)j=JSON.serialize(new Map<String,Object>{'status'=>'inconclusive_overlength','originalLength'=>j.length()});
  System.assert(false,'CAPTURE|'+id+'|'+j);
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
  System.assertEquals(false,DootSecQ2B1031__c.Hidden__c.getDescribe().isAccessible(),'fixture hidden must be denied');
  System.assertEquals(false,DootSecQ2B1031__c.Value__c.getDescribe().isAccessible(),'fixture value must be denied');
 }
 @TestSetup static void setupReader(){
  List<Profile> ps=[SELECT Id,PermissionsModifyAllData,PermissionsViewAllData FROM Profile WHERE Name='Minimum Access - Salesforce' WITH SYSTEM_MODE];
  System.assertEquals(1,ps.size(),'fixture requires exact Minimum Access - Salesforce profile');
  System.assertEquals(false,ps[0].PermissionsModifyAllData,'fixture no ModifyAllData');System.assertEquals(false,ps[0].PermissionsViewAllData,'fixture no ViewAllData');
  User u=new User(Alias='dsec',LastName='DootSecQ2ReaderB1031',Email='capture@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='GMT',ProfileId=ps[0].Id,Username='DootSecQ2ReaderB1031.'+UserInfo.getOrganizationId()+'.'+System.now().getTime()+'@example.invalid');insert as system u;
  List<PermissionSet> grants=[SELECT Id FROM PermissionSet WHERE Name='DootSecQ2ReaderB1031' WITH SYSTEM_MODE];System.assertEquals(1,grants.size(),'fixture unique owned permission set');insert as system new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=grants[0].Id);
  
 }
 @IsTest static void limitRows(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ2B1031__c> seed=new List<DootSecQ2B1031__c>{new DootSecQ2B1031__c(Name='A-hidden'),new DootSecQ2B1031__c(Name='B-visible',OwnerId=r.Id),new DootSecQ2B1031__c(Name='C-visible',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ2B1031__c>(seed).keySet();String objectName=DootSecQ2B1031__c.SObjectType.getDescribe().getName();System.runAs(r){accessGate();System.assertEquals(2,[SELECT COUNT() FROM DootSecQ2B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture direct visible count');stage='query';o.put('rows',compact(Database.query('SELECT Id,Name FROM '+objectName+' WHERE Id IN :ids WITH USER_MODE ORDER BY Name,Id LIMIT 1')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q2.limit',o);
 }
 @IsTest static void offsetRows(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ2B1031__c> seed=new List<DootSecQ2B1031__c>{new DootSecQ2B1031__c(Name='A-hidden'),new DootSecQ2B1031__c(Name='B-visible',OwnerId=r.Id),new DootSecQ2B1031__c(Name='C-visible',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ2B1031__c>(seed).keySet();String objectName=DootSecQ2B1031__c.SObjectType.getDescribe().getName();System.runAs(r){accessGate();System.assertEquals(2,[SELECT COUNT() FROM DootSecQ2B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture direct visible count');stage='query';o.put('rows',compact(Database.query('SELECT Id,Name FROM '+objectName+' WHERE Id IN :ids WITH USER_MODE ORDER BY Name,Id LIMIT 1 OFFSET 1')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q2.offset',o);
 }
 @IsTest static void unwindowed(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{User r=reader();List<DootSecQ2B1031__c> seed=new List<DootSecQ2B1031__c>{new DootSecQ2B1031__c(Name='A-hidden'),new DootSecQ2B1031__c(Name='B-visible',OwnerId=r.Id),new DootSecQ2B1031__c(Name='C-visible',OwnerId=r.Id)};insert as system seed;verifySeeds(seed);Set<Id> ids=new Map<Id,DootSecQ2B1031__c>(seed).keySet();String objectName=DootSecQ2B1031__c.SObjectType.getDescribe().getName();System.runAs(r){accessGate();System.assertEquals(2,[SELECT COUNT() FROM DootSecQ2B1031__c WHERE Id IN :ids WITH USER_MODE],'fixture direct visible count');stage='query';o.put('rows',compact(Database.query('SELECT Id,Name FROM '+objectName+' WHERE Id IN :ids WITH USER_MODE ORDER BY Name,Id ')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('status',stage=='fixture'?'inconclusive_fixture':'operation_exception');o.put('stage',stage);o.put('exception',failure(stage,e));}
  emit('Q2.unwindowed',o);
 }
}
