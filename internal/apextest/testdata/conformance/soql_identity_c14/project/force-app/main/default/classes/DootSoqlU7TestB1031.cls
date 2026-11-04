@IsTest(SeeAllData=false)
private without sharing class DootSoqlU7TestB1031 {
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
 @IsTest static void asciiPunctuation(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{List<Account> seed=new List<Account>{new Account(Name='['),new Account(Name='a')};insert as system seed;Set<Id> ids=new Map<Id,Account>(seed).keySet();System.assertEquals(2,[SELECT COUNT() FROM Account WHERE Id IN :ids WITH SYSTEM_MODE],'fixture row count');for(Account a:[SELECT Id,Name FROM Account WHERE Id IN :ids WITH SYSTEM_MODE])for(Account expected:seed)if(a.Id==expected.Id)System.assertEquals(expected.Name,a.Name,'fixture name');stage='query';o.put('rows',compact([SELECT Id,Name FROM Account WHERE Id IN :ids WITH SYSTEM_MODE ORDER BY Name,Id])); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('U7.asciiPunctuation',o);
 }
}
