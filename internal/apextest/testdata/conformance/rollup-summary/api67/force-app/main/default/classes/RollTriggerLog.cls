public class RollTriggerLog {
 public static List<String> events=new List<String>();
 public static void capture(System.TriggerOperation operation,List<RollChild__c> fresh,List<RollChild__c> prior) {
  RollChild__c row=(fresh!=null && !fresh.isEmpty())?fresh[0]:prior[0];
  Id parentId=row.Parent__c;
  if (parentId==null && prior!=null && !prior.isEmpty()) parentId=prior[0].Parent__c;
  if (parentId==null) return;
  RollParent__c parent=[SELECT CountAll__c,CountActive__c,SumAll__c,SumActive__c,MinAll__c,MaxAll__c,MinActive__c,MaxActive__c FROM RollParent__c WHERE Id=:parentId];
  events.add(String.valueOf(operation)+'='+RollProbe.shap(parent));
 }
}
