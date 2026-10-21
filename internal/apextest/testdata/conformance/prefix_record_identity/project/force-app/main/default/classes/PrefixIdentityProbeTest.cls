@IsTest private class PrefixIdentityProbeTest {
 @IsTest static void observe() {
  IdentityNode__c parent=new IdentityNode__c(Name='parent');insert parent;
  IdentityNode__c child=new IdentityNode__c(Name='child',Parent__c=parent.Id);insert child;
  Id originalParent=parent.Id,originalChild=child.Id;
  String prefix=IdentityNode__c.SObjectType.getDescribe().getKeyPrefix();
  String commentPrefix=CaseComment.SObjectType.getDescribe().getKeyPrefix();
  String accountPrefix=Account.SObjectType.getDescribe().getKeyPrefix();
  Map<String,Schema.SObjectType> types=Schema.getGlobalDescribe();
  child.Name='after';update child;
  IdentityNode__c readBack=[SELECT Id,Parent__c,Parent__r.Name FROM IdentityNode__c WHERE Id=:originalChild];
  Map<String,Object> result=new Map<String,Object>{
   'parentStable'=>parent.Id==originalParent,'childStable'=>readBack.Id==originalChild,
   'referenceStable'=>readBack.Parent__c==originalParent,'relatedName'=>readBack.Parent__r.Name,
   'prefixMatches'=>String.valueOf(originalChild).startsWith(prefix),
   'prefixDistinct'=>prefix!=commentPrefix && prefix!=accountPrefix,
   'typeStable'=>originalChild.getSObjectType()==IdentityNode__c.SObjectType,
   'accountPrefix'=>accountPrefix,'commentPrefix'=>commentPrefix};
  System.assert(false,'OBS|'+JSON.serialize(result));
 }
}
