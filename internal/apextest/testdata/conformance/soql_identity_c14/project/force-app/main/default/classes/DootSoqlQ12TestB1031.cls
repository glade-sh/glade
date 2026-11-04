@IsTest(SeeAllData=false)
private without sharing class DootSoqlQ12TestB1031 {
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
 @IsTest static void calendar_year(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Contact c=new Contact(LastName='Q12',Birthdate=Date.newInstance(2024,2,29));insert as system c;Id target=c.Id;System.assertEquals(c.Birthdate,[SELECT Birthdate FROM Contact WHERE Id=:target WITH SYSTEM_MODE].Birthdate,'fixture date');stage='query';o.put('rows',compact(Database.query('SELECT CALENDAR_YEAR(Birthdate) part,COUNT(Id) n FROM Contact WHERE Id=:target WITH SYSTEM_MODE GROUP BY CALENDAR_YEAR(Birthdate)'))); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('Q12.CALENDAR_YEAR',o);
 }
 @IsTest static void day_only(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Contact c=new Contact(LastName='Q12',Birthdate=Date.newInstance(2024,2,29));insert as system c;Id target=c.Id;System.assertEquals(c.Birthdate,[SELECT Birthdate FROM Contact WHERE Id=:target WITH SYSTEM_MODE].Birthdate,'fixture date');stage='query';o.put('rows',compact(Database.query('SELECT DAY_ONLY(Birthdate) part,COUNT(Id) n FROM Contact WHERE Id=:target WITH SYSTEM_MODE GROUP BY DAY_ONLY(Birthdate)'))); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('Q12.DAY_ONLY',o);
 }
 @IsTest static void hour_in_day(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Contact c=new Contact(LastName='Q12',Birthdate=Date.newInstance(2024,2,29));insert as system c;Id target=c.Id;System.assertEquals(c.Birthdate,[SELECT Birthdate FROM Contact WHERE Id=:target WITH SYSTEM_MODE].Birthdate,'fixture date');stage='query';o.put('rows',compact(Database.query('SELECT HOUR_IN_DAY(Birthdate) part,COUNT(Id) n FROM Contact WHERE Id=:target WITH SYSTEM_MODE GROUP BY HOUR_IN_DAY(Birthdate)'))); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('Q12.HOUR_IN_DAY',o);
 }
}
