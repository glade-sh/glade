@IsTest(SeeAllData=false)
private without sharing class DootSoqlQ10TestB1031 {
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
 @IsTest static void futureToday(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Datetime started=System.now();Datetime later=Datetime.newInstance(Date.today(),Time.newInstance(23,55,0,0));System.assert(later>started.addMinutes(2),'fixture: before 23:53 local');Event e=new Event(Subject='Q10',StartDateTime=later,EndDateTime=later.addMinutes(1));insert as system e;Id target=e.Id;System.assertEquals(later,[SELECT StartDateTime FROM Event WHERE Id=:target WITH SYSTEM_MODE].StartDateTime,'fixture start changed');Datetime beforeQuery=System.now();stage='query';List<Event> found=[SELECT Id,StartDateTime FROM Event WHERE Id=:target AND StartDateTime=LAST_N_DAYS:1 WITH SYSTEM_MODE];Datetime afterQuery=System.now();stage='clock-gate';System.assert(afterQuery<later,'fixture expired');o.put('before',beforeQuery);o.put('after',afterQuery);o.put('future',later);o.put('rows',compact(found)); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('Q10.futureToday',o);
 }
}
