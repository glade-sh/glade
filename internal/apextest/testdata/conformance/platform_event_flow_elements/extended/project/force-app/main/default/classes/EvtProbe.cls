public class EvtProbe {
 public static SObject make(String event,String key,String op) {
  if(event=='imm') return new EvtImm__e(Key__c=key,Operation__c=op);
  return new EvtAfter__e(Key__c=key,Operation__c=op);
 }
 public static List<Boolean> send(String key,String event,String op,String mode) {
  List<Boolean> results=new List<Boolean>();
  Savepoint sp=Database.setSavepoint();
  if(mode=='partial') {
   results.add(EventBus.publish(make(event,key+'-A',op)).isSuccess());
   Savepoint nested=Database.setSavepoint();
   results.add(EventBus.publish(make(event,key+'-B',op)).isSuccess());
   Database.rollback(nested);
  } else if(mode=='mixed') {
   results.add(EventBus.publish(make('after',key+'-A',op)).isSuccess());
   results.add(EventBus.publish(make('imm',key+'-B',op)).isSuccess());
   Database.rollback(sp);
  } else if(mode=='rollback') {
   results.add(EventBus.publish(make(event,key,op)).isSuccess());
   Database.rollback(sp);
  } else {
   results.add(EventBus.publish(make(event,key,op)).isSuccess());
  }
  return results;
 }
 public static Map<String,Object> read(String key) {
  List<String> entries=new List<String>();String pattern=key+'%';
  List<String> faults=new List<String>();
  for(EvtLog__c row:[SELECT Name,Phase__c,Note__c,Detail__c FROM EvtLog__c WHERE Name LIKE :pattern]) {
   entries.add(row.Name.substring(key.length())+'|'+row.Phase__c);
   if(row.Phase__c=='fault') faults.add(row.Detail__c!=null?row.Detail__c:row.Note__c);
  }
  entries.sort();faults.sort();
  return new Map<String,Object>{'entries'=>entries,'faults'=>faults};
 }
 public static void observe(String id,String event,String op,String mode,String route) {
  String key='Evt-'+id+'-'+route;
  if(route=='stop') Test.startTest();
  List<Boolean> published=send(key,event,op,mode);
  if(route=='stop')Test.stopTest();else {Test.getEventBus().deliver();if(route=='deliver2')Test.getEventBus().deliver();}
  Map<String,Object> out=read(key);out.put('published',published);
  System.assert(false,'EVT_OBS|'+id+'|'+route+'|'+JSON.serialize(out));
 }
}
