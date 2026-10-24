public without sharing class StandardParentProbe {
 public static Map<String,Object> observe() {
  Contact parent = new Contact(LastName='Parent');
  parent.put('LocalNote__c', 'local');
  insert as system parent;
  ParentLink__c link = new ParentLink__c(Name='Link', Contact__c=parent.Id);
  insert as system link;
  ParentLink__c constructed = new ParentLink__c(Contact__r=parent);
  ParentLink__c queried = [SELECT Id, Contact__r.LastName, Contact__r.LocalNote__c FROM ParentLink__c WHERE Id=:link.Id WITH SYSTEM_MODE];
  Map<String,Object> result = new Map<String,Object>{'constructed'=>constructed.Contact__r.LocalNote__c, 'queried'=>queried.Contact__r.LocalNote__c, 'standard'=>queried.Contact__r.LastName};
  try { List<SObject> rows = Database.query('SELECT Contact__r.AbsentProbeField FROM ParentLink__c'); result.put('rejection','accepted'); }
  catch(QueryException failure) { result.put('rejection',failure.getTypeName()); }
  ParentLink__c empty = new ParentLink__c(Name='Empty'); insert as system empty;
  ParentLink__c boundary = [SELECT Id, Contact__r.LocalNote__c FROM ParentLink__c WHERE Id=:empty.Id WITH SYSTEM_MODE];
  result.put('nullParent',boundary.Contact__r==null);
  return result;
 }
}
