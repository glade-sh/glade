public class ElementProbe {
 public static void seed(String key,String operation) {
  Id parent;
  if(operation=='update'||operation=='delete'||operation=='loop'||operation=='chatter') {
   Account seed=new Account(Name=key,Phone='old');insert seed;parent=seed.Id;
   if(operation=='loop') insert new Account(Name=key,Phone='old');
  }
 }
 public static Boolean publish(String key,String operation) {seed(key,operation);return send(key,operation);}
 public static Boolean send(String key,String operation) {
  List<Account> parents=[SELECT Id FROM Account WHERE Name=:key LIMIT 1];Id parent=parents.isEmpty()?null:parents[0].Id;
  List<ElementSignal__e> signals=new List<ElementSignal__e>();
  Integer count=operation=='batch'?3:1;
  for(Integer i=0;i<count;i++) signals.add(new ElementSignal__e(Key__c=key,Operation__c=operation,Parent__c=parent));
  return EventBus.publish(signals)[0].isSuccess();
 }
 public static Map<String,Object> read(String key) {
  List<ElementLog__c> logs=[SELECT Phase__c,Actor__c,CreatedById,CreatedBy.UserType FROM ElementLog__c WHERE Name=:key ORDER BY CreatedDate,Id];
  Integer flowCount=0,triggerCount=0,actionCount=0;Boolean automated=true,actorMatches=true;List<String> phases=new List<String>();Boolean flowCaller=true,triggerCaller=true,actionCaller=true;Boolean flowAutomated=true,triggerAutomated=true,actionAutomated=true;
  for(ElementLog__c row:logs) {
   phases.add(row.Phase__c);Boolean same=String.valueOf(row.Actor__c).substring(0,15)==String.valueOf(UserInfo.getUserId()).substring(0,15);
   if(row.Phase__c=='flow'){flowCaller=flowCaller&&same;flowAutomated=flowAutomated&&row.CreatedBy.UserType=='AutomatedProcess';}
   if(row.Phase__c=='trigger'){triggerCaller=triggerCaller&&same;triggerAutomated=triggerAutomated&&row.CreatedBy.UserType=='AutomatedProcess';}
   if(row.Phase__c=='action'){actionCaller=actionCaller&&same;actionAutomated=actionAutomated&&row.CreatedBy.UserType=='AutomatedProcess';}if(row.Phase__c=='flow')flowCount++;if(row.Phase__c=='trigger')triggerCount++;if(row.Phase__c=='action')actionCount++;
   automated=automated&&row.CreatedBy.UserType=='AutomatedProcess';actorMatches=actorMatches&&String.valueOf(row.Actor__c).substring(0,15)==String.valueOf(row.CreatedById).substring(0,15);
  }
  List<Account> parents=[SELECT Id FROM Account WHERE Name=:key];Integer posts=0;
  for(FeedItem post:[SELECT Body FROM FeedItem WHERE ParentId IN :parents]) {if(post.Body==key)posts++;}
  Integer updated=[SELECT count() FROM Account WHERE Name=:key AND Phone='updated'];
  return new Map<String,Object>{'flow'=>flowCount,'trigger'=>triggerCount,'action'=>actionCount,'automated'=>automated,'actorMatches'=>actorMatches,'accounts'=>[SELECT count() FROM Account WHERE Name=:key],'updated'=>updated,'posts'=>posts,'phases'=>phases,'flowCaller'=>flowCaller,'triggerCaller'=>triggerCaller,'actionCaller'=>actionCaller,'flowAutomated'=>flowAutomated,'triggerAutomated'=>triggerAutomated,'actionAutomated'=>actionAutomated};
 }
 public static void observe(String id,String operation,String route) {
  String key='Element-'+id+'-'+route;
  Id caller=UserInfo.getUserId();Boolean published;
  if(route=='stop') Test.startTest();
  published=publish(key,operation);
  if(route=='stop')Test.stopTest();else Test.getEventBus().deliver();
  Map<String,Object> out=read(key);out.put('published',published);out.put('restored',caller==UserInfo.getUserId());
  System.assert(false,'ELEMENT_OBS|'+id+'|'+route+'|'+JSON.serialize(out));
 }
}
