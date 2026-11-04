@IsTest(SeeAllData=false)
private without sharing class DootSoqlU5TestB1031 {
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
 @TestSetup static void setupUsers(){List<User> users=new List<User>();users.add(new User(Alias='dsq0',LastName='DootU5B1031_0',Email='capture@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='GMT',ProfileId=UserInfo.getProfileId(),Username='DootU5B1031_0.'+UserInfo.getOrganizationId()+'.'+System.now().getTime()+'@example.invalid')); users.add(new User(Alias='dsq1',LastName='DootU5B1031_1',Email='capture@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='tr_TR',TimeZoneSidKey='GMT',ProfileId=UserInfo.getProfileId(),Username='DootU5B1031_1.'+UserInfo.getOrganizationId()+'.'+System.now().getTime()+'@example.invalid')); insert as system users;}
 @IsTest static void locale0(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{List<User> us=[SELECT Id FROM User WHERE LastName='DootU5B1031_0' WITH SYSTEM_MODE];System.assertEquals(1,us.size(),'fixture unique user');System.runAs(us[0]){System.assertEquals('en_US',UserInfo.getLocale(),'fixture locale');System.assertEquals('GMT',UserInfo.getTimeZone().getID(),'fixture tz');o.put('runLocale',UserInfo.getLocale());o.put('runTz',UserInfo.getTimeZone().getID());List<Account> seed=new List<Account>{new Account(Name='Service'),new Account(Name='SERVICE'),new Account(Name='SERVİCE')};insert as system seed;Set<Id> ids=new Map<Id,Account>(seed).keySet();System.assertEquals(3,[SELECT COUNT() FROM Account WHERE Id IN :ids WITH SYSTEM_MODE],'fixture rows');for(Account a:[SELECT Id,Name FROM Account WHERE Id IN :ids WITH SYSTEM_MODE])for(Account e:seed)if(a.Id==e.Id)System.assertEquals(e.Name,a.Name,'fixture text');stage='query';o.put('rows',compact(Database.query('SELECT Id,Name FROM Account WHERE Id IN :ids AND Name=\'Service\' WITH SYSTEM_MODE ORDER BY Name,Id')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('U5.en_US',o);
 }
 @IsTest static void locale1(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{List<User> us=[SELECT Id FROM User WHERE LastName='DootU5B1031_1' WITH SYSTEM_MODE];System.assertEquals(1,us.size(),'fixture unique user');System.runAs(us[0]){System.assertEquals('tr_TR',UserInfo.getLocale(),'fixture locale');System.assertEquals('GMT',UserInfo.getTimeZone().getID(),'fixture tz');o.put('runLocale',UserInfo.getLocale());o.put('runTz',UserInfo.getTimeZone().getID());List<Account> seed=new List<Account>{new Account(Name='Service'),new Account(Name='SERVICE'),new Account(Name='SERVİCE')};insert as system seed;Set<Id> ids=new Map<Id,Account>(seed).keySet();System.assertEquals(3,[SELECT COUNT() FROM Account WHERE Id IN :ids WITH SYSTEM_MODE],'fixture rows');for(Account a:[SELECT Id,Name FROM Account WHERE Id IN :ids WITH SYSTEM_MODE])for(Account e:seed)if(a.Id==e.Id)System.assertEquals(e.Name,a.Name,'fixture text');stage='query';o.put('rows',compact(Database.query('SELECT Id,Name FROM Account WHERE Id IN :ids AND Name=\'Service\' WITH SYSTEM_MODE ORDER BY Name,Id')));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('U5.tr_TR',o);
 }
}
