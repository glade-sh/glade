@IsTest private class MetadataChildrenTest {
 private static Map<String,RoutePlan__mdt> plans {
  get {
   if(plans==null) {
    plans=new Map<String,RoutePlan__mdt>();
    for(RoutePlan__mdt plan:queryPlans()) plans.put(plan.DeveloperName,plan);
   }
   return plans;
  }
  set;
 }
 private static List<RoutePlan__mdt> queryPlans() {
  return Database.query('SELECT DeveloperName,(SELECT DeveloperName,Plan__c FROM Steps__r WHERE Enabled__c=true) FROM RoutePlan__mdt WHERE Enabled__c=true AND DeveloperName IN (\'RoutePlanSample\',\'RoutePlanSampleOne\',\'Short\')',AccessLevel.SYSTEM_MODE);
 }
 @IsTest static void directChildren() {
  Map<String,Object> result=new Map<String,Object>();
  try {
   List<RoutePlan__mdt> found=queryPlans();
   result.put('parents',found.size());
   for(RoutePlan__mdt plan:found) {
    if(plan.DeveloperName=='RoutePlanSample') {
     result.put('children',plan.Steps__r.size());
     result.put('first',plan.Steps__r.iterator().next().DeveloperName);
    }
   }
  } catch(Exception issue) { result.put('error',issue.getTypeName()+'|'+issue.getMessage()); }
  System.assert(false,'OBSERVED|directChildren|'+JSON.serialize(result));
 }
 @IsTest static void cachedChildren() {
  Map<String,Object> result=new Map<String,Object>();
  try {
   RoutePlan__mdt plan=plans.get('RoutePlanSample');
   result.put('children',plan.Steps__r.size());
   result.put('first',plans.get('RoutePlanSample').Steps__r.iterator().next().DeveloperName);
   result.put('again',plans.get('RoutePlanSample').Steps__r.iterator().next().DeveloperName);
  } catch(Exception issue) { result.put('error',issue.getTypeName()+'|'+issue.getMessage()); }
  System.assert(false,'OBSERVED|cachedChildren|'+JSON.serialize(result));
 }
 @IsTest static void nameControls() {
  Map<String,Object> result=new Map<String,Object>();
  for(String name:new List<String>{'Short','RoutePlanSampleOne'}) {
   try {
    RoutePlan__mdt plan=plans.get(name);
    result.put(name+'Children',plan.Steps__r.size());
    result.put(name+'First',plan.Steps__r.iterator().next().DeveloperName);
   } catch(Exception issue) { result.put(name+'Error',issue.getTypeName()+'|'+issue.getMessage()); }
  }
  System.assert(false,'OBSERVED|nameControls|'+JSON.serialize(result));
 }

}
