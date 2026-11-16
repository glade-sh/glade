public class RollProbe {
 public static String shap(RollParent__c p) {
  return p.CountAll__c+'|'+p.CountActive__c+'|'+String.valueOf(p.SumAll__c)+'|'+String.valueOf(p.SumActive__c)
   +'|'+String.valueOf(p.MinAll__c)+'|'+String.valueOf(p.MaxAll__c)+'|'+String.valueOf(p.MinActive__c)+'|'+String.valueOf(p.MaxActive__c);
 }
 public static RollParent__c fresh(Id id) {
  return [SELECT CountAll__c,CountActive__c,SumAll__c,SumActive__c,MinAll__c,MaxAll__c,MinActive__c,MaxActive__c FROM RollParent__c WHERE Id=:id];
 }
}
