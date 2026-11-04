@IsTest(SeeAllData=false)
private without sharing class DootSoqlU6TestB1031 {
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
 @IsTest static void exact(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Schema.DescribeFieldResult d=DootSoqlU6B1031__c.CaseKey__c.getDescribe();System.assertEquals(true,d.isUnique(),'fixture unique');System.assertEquals(true,d.isCaseSensitive(),'fixture case');o.put('unique',d.isUnique());o.put('caseSensitive',d.isCaseSensitive());DootSoqlU6B1031__c a=new DootSoqlU6B1031__c(Name='U6',CaseKey__c='North');insert as system a;Id target=a.Id;System.assertEquals('North',[SELECT CaseKey__c FROM DootSoqlU6B1031__c WHERE Id=:target WITH SYSTEM_MODE].CaseKey__c,'fixture value');String objectName=DootSoqlU6B1031__c.SObjectType.getDescribe().getName();String fieldName=d.getName();stage='query';o.put('rows',compact(Database.query('SELECT Id,'+fieldName+' FROM '+objectName+' WHERE Id=:target AND '+fieldName+' = \'North\' WITH SYSTEM_MODE'))); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('U6.exact',o);
 }
 @IsTest static void mismatchedEq(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Schema.DescribeFieldResult d=DootSoqlU6B1031__c.CaseKey__c.getDescribe();System.assertEquals(true,d.isUnique(),'fixture unique');System.assertEquals(true,d.isCaseSensitive(),'fixture case');o.put('unique',d.isUnique());o.put('caseSensitive',d.isCaseSensitive());DootSoqlU6B1031__c a=new DootSoqlU6B1031__c(Name='U6',CaseKey__c='North');insert as system a;Id target=a.Id;System.assertEquals('North',[SELECT CaseKey__c FROM DootSoqlU6B1031__c WHERE Id=:target WITH SYSTEM_MODE].CaseKey__c,'fixture value');String objectName=DootSoqlU6B1031__c.SObjectType.getDescribe().getName();String fieldName=d.getName();stage='query';o.put('rows',compact(Database.query('SELECT Id,'+fieldName+' FROM '+objectName+' WHERE Id=:target AND '+fieldName+' = \'north\' WITH SYSTEM_MODE'))); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('U6.mismatchedEq',o);
 }
 @IsTest static void mismatchedIn(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Schema.DescribeFieldResult d=DootSoqlU6B1031__c.CaseKey__c.getDescribe();System.assertEquals(true,d.isUnique(),'fixture unique');System.assertEquals(true,d.isCaseSensitive(),'fixture case');o.put('unique',d.isUnique());o.put('caseSensitive',d.isCaseSensitive());DootSoqlU6B1031__c a=new DootSoqlU6B1031__c(Name='U6',CaseKey__c='North');insert as system a;Id target=a.Id;System.assertEquals('North',[SELECT CaseKey__c FROM DootSoqlU6B1031__c WHERE Id=:target WITH SYSTEM_MODE].CaseKey__c,'fixture value');String objectName=DootSoqlU6B1031__c.SObjectType.getDescribe().getName();String fieldName=d.getName();stage='query';o.put('rows',compact(Database.query('SELECT Id,'+fieldName+' FROM '+objectName+' WHERE Id=:target AND '+fieldName+' IN (\'north\') WITH SYSTEM_MODE'))); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('U6.mismatchedIn',o);
 }
}
