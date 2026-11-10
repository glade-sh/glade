public with sharing class DefaultHostV62 {
 static String sorted(Set<String> values) { List<String> names=new List<String>(values); names.sort(); return String.join(names,','); }
 @AuraEnabled public static String observe() {
  SObject loaded=new Account().getSObjectType().newSObject(null,true);
  SObject blank=new Account().getSObjectType().newSObject(null,false);
  SObjectAccessDecision d=Security.stripInaccessible(AccessType.CREATABLE,new List<SObject>{loaded},true);
  Set<String> removed=d.getRemovedFields().get('Account');
  return 'pop='+sorted(loaded.getPopulatedFieldsAsMap().keySet())+'|blank='+sorted(blank.getPopulatedFieldsAsMap().keySet())+'|removed='+(removed==null?'':sorted(removed));
 }
 public String getOutcome() { return observe(); }
}