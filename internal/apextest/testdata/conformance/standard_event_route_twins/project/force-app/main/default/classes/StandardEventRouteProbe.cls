public class StandardEventRouteProbe {
 public static String one(Database.SaveResult result) {
  List<String> codes=new List<String>();
  for(Database.Error problem:result.getErrors()) codes.add(String.valueOf(problem.getStatusCode()));
  return result.isSuccess()+'|'+(result.getId()==null)+'|'+result.getErrors().size()+'|'+String.join(codes,',');
 }
 public static String many(List<Database.SaveResult> results) {
  List<String> rows=new List<String>();
  for(Database.SaveResult result:results) rows.add(one(result));
  return String.join(rows,';');
 }
 public static Map<String,String> observe() {
Map<String,String> rows = new Map<String,String>();
try { SObject eventValue=new BatchApexErrorEvent(); rows.put('publish_scalar', StandardEventRouteProbe.one(EventBus.publish(eventValue))); } catch(Exception e) { rows.put('publish_scalar',e.getTypeName()+'|'+e.getMessage()); }
try { rows.put('publish_erased_scalar', StandardEventRouteProbe.one(EventBus.publish((SObject)new BatchApexErrorEvent()))); } catch(Exception e) { rows.put('publish_erased_scalar',e.getTypeName()+'|'+e.getMessage()); }
try { rows.put('publish_erased_single', StandardEventRouteProbe.many(EventBus.publish(new List<SObject>{new BatchApexErrorEvent()}))); } catch(Exception e) { rows.put('publish_erased_single',e.getTypeName()+'|'+e.getMessage()); }
try { List<SObject> eventValues=new List<BatchApexErrorEvent>{new BatchApexErrorEvent()}; rows.put('publish_concrete_single', StandardEventRouteProbe.many(EventBus.publish(eventValues))); } catch(Exception e) { rows.put('publish_concrete_single',e.getTypeName()+'|'+e.getMessage()); }
try { rows.put('publish_erased_concrete_single', StandardEventRouteProbe.many(EventBus.publish((List<SObject>)new List<BatchApexErrorEvent>{new BatchApexErrorEvent()}))); } catch(Exception e) { rows.put('publish_erased_concrete_single',e.getTypeName()+'|'+e.getMessage()); }
try { rows.put('publish_erased_multi', StandardEventRouteProbe.many(EventBus.publish(new List<SObject>{new BatchApexErrorEvent(),new BatchApexErrorEvent()}))); } catch(Exception e) { rows.put('publish_erased_multi',e.getTypeName()+'|'+e.getMessage()); }
try { List<SObject> eventValues=new List<BatchApexErrorEvent>{new BatchApexErrorEvent(),new BatchApexErrorEvent()}; rows.put('publish_concrete_multi', StandardEventRouteProbe.many(EventBus.publish(eventValues))); } catch(Exception e) { rows.put('publish_concrete_multi',e.getTypeName()+'|'+e.getMessage()); }
try { SObject value=new BatchApexErrorEvent(); insert value; rows.put('dml_scalar','inserted'); } catch(Exception e) { rows.put('dml_scalar',e.getTypeName()+'|'+e.getMessage()); }
try { List<SObject> values=new List<SObject>{new BatchApexErrorEvent()}; insert values; rows.put('dml_erased_single','inserted'); } catch(Exception e) { rows.put('dml_erased_single',e.getTypeName()+'|'+e.getMessage()); }
try { List<SObject> values=new List<BatchApexErrorEvent>{new BatchApexErrorEvent()}; insert values; rows.put('dml_concrete_single','inserted'); } catch(Exception e) { rows.put('dml_concrete_single',e.getTypeName()+'|'+e.getMessage()); }
try { SObject value=new BatchApexErrorEvent(); rows.put('database_erased_scalar',StandardEventRouteProbe.one(Database.insert(value,false))); } catch(Exception e) { rows.put('database_erased_scalar',e.getTypeName()+'|'+e.getMessage()); }
return rows;
 }
}
