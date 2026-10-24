public class FlowValueProbe {
 public static Object observe(String family, String route, String input) {
  FlowIntegerProbe.seen = null;
  Account parent = new Account(Name='value-parent'); insert parent;
  Account twin = new Account(Name='value-parent'); insert twin;
  FlowValueRecord__c record = new FlowValueRecord__c(Name='value-row', Search__c='value-parent', When__c=Date.newInstance(2026,1,2), Parent__c=parent.Id, Result__c='seed', True__c=false, False__c=true, Whole__c=7, Fraction__c=7.75);
  if (input=='missing') { record.Search__c='absent-parent'; record.Parent__c=null; }
  if (input=='null') { record.Search__c=null; record.When__c=null; record.Whole__c=null; record.Fraction__c=null; }
  if (input=='maximum') { record.Whole__c=2147483647; record.Fraction__c=2147483647.49; }
  if (input=='overflow') { record.Whole__c=2147483648L; record.Fraction__c=2147483648.75; }
  if (input=='integral') { record.Fraction__c=7.00; }
  if (input=='negative') { record.Whole__c=-7; record.Fraction__c=-7.75; }
  if (route=='trigger' || route=='subflow') record.Mode__c=family+route;
  try {
   insert record;
   if (route=='interview') {
    Flow.Interview interview=Flow.Interview.createInterview('Value'+family,new Map<String,Object>{'InputRecord'=>record}); interview.start();
   }
   if (route=='action') {
    Invocable.Action action=Invocable.Action.createCustomAction('flow','Value'+family);
    action.setInvocations(new List<Map<String,Object>>{new Map<String,Object>{'InputRecord'=>record}});
    Invocable.Action.Result outcome=action.invoke()[0];
    if (!outcome.isSuccess()) return new Map<String,Object>{'error'=>true};
   }
   if (family.startsWith('Integer')) return FlowIntegerProbe.seen;
   if (family=='ParentCriteria' || family=='ParentRecordUpdate') { Integer changed=0; for (Account item : [SELECT Description FROM Account]) if (item.Description=='changed') changed++; return changed; }
   record=[SELECT Result__c,True__c,False__c FROM FlowValueRecord__c WHERE Id=:record.Id];
   if (family.startsWith('Constants')) return new Map<String,Object>{'text'=>record.Result__c,'yes'=>record.True__c,'no'=>record.False__c};
   return record.Result__c;
  } catch (Exception e) { return new Map<String,Object>{'error'=>true}; }
 }
}
