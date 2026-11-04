@IsTest(SeeAllData=false)
private without sharing class DootSoqlQ9TestB1031 {
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
 @TestSetup static void setupUsers(){List<User> users=new List<User>();users.add(new User(Alias='dsq0',LastName='DootQ9B1031_0',Email='capture@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=UserInfo.getProfileId(),Username='DootQ9B1031_0.'+UserInfo.getOrganizationId()+'.'+System.now().getTime()+'@example.invalid')); insert as system users;}
 @IsTest static void raw(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{List<User> us=[SELECT Id FROM User WHERE LastName='DootQ9B1031_0' WITH SYSTEM_MODE];System.assertEquals(1,us.size(),'fixture unique user');System.runAs(us[0]){System.assertEquals('en_US',UserInfo.getLocale(),'fixture locale');System.assertEquals('America/Los_Angeles',UserInfo.getTimeZone().getID(),'fixture tz');o.put('runLocale',UserInfo.getLocale());o.put('runTz',UserInfo.getTimeZone().getID());Account a=new Account(Name='Q9');insert as system a;Datetime instant=Datetime.newInstanceGmt(2026,1,1,0,30,0);Test.setCreatedDate(a.Id,instant);Id target=a.Id;System.assertEquals(instant,[SELECT CreatedDate FROM Account WHERE Id=:target WITH SYSTEM_MODE].CreatedDate,'fixture created');String arg='CreatedDate';String fields='CALENDAR_MONTH('+arg+'),HOUR_IN_DAY('+arg+'),DAY_ONLY('+arg+')';stage='query';o.put('instant',instant);o.put('rows',compact(Database.query('SELECT CALENDAR_MONTH('+arg+') m,HOUR_IN_DAY('+arg+') h,DAY_ONLY('+arg+') d,COUNT(Id) n FROM Account WHERE Id=:target WITH SYSTEM_MODE GROUP BY '+fields)));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('Q9.raw',o);
 }
 @IsTest static void converted(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{List<User> us=[SELECT Id FROM User WHERE LastName='DootQ9B1031_0' WITH SYSTEM_MODE];System.assertEquals(1,us.size(),'fixture unique user');System.runAs(us[0]){System.assertEquals('en_US',UserInfo.getLocale(),'fixture locale');System.assertEquals('America/Los_Angeles',UserInfo.getTimeZone().getID(),'fixture tz');o.put('runLocale',UserInfo.getLocale());o.put('runTz',UserInfo.getTimeZone().getID());Account a=new Account(Name='Q9');insert as system a;Datetime instant=Datetime.newInstanceGmt(2026,1,1,0,30,0);Test.setCreatedDate(a.Id,instant);Id target=a.Id;System.assertEquals(instant,[SELECT CreatedDate FROM Account WHERE Id=:target WITH SYSTEM_MODE].CreatedDate,'fixture created');String arg='convertTimezone(CreatedDate)';String fields='CALENDAR_MONTH('+arg+'),HOUR_IN_DAY('+arg+'),DAY_ONLY('+arg+')';stage='query';o.put('instant',instant);o.put('rows',compact(Database.query('SELECT CALENDAR_MONTH('+arg+') m,HOUR_IN_DAY('+arg+') h,DAY_ONLY('+arg+') d,COUNT(Id) n FROM Account WHERE Id=:target WITH SYSTEM_MODE GROUP BY '+fields)));} o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('Q9.converted',o);
 }
}
