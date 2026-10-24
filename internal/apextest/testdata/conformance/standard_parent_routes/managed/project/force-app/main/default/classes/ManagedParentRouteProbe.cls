public without sharing class ManagedParentRouteProbe {
 public static Map<String,Object> observe() {
  Contact parent = new Contact(LastName='Parent', gladens001__ParentRouteValue__c='managed');
  insert as system parent;
  gladens003__ParentRouteTwin__c link = new gladens003__ParentRouteTwin__c(Name='Link', gladens003__Contact__c=parent.Id);
  insert as system link;
  gladens003__ParentRouteTwin__c constructed = new gladens003__ParentRouteTwin__c(gladens003__Contact__r=parent);
  gladens003__ParentRouteTwin__c queried = [SELECT Id, gladens003__Contact__r.LastName, gladens003__Contact__r.gladens001__ParentRouteValue__c FROM gladens003__ParentRouteTwin__c WHERE Id=:link.Id WITH SYSTEM_MODE];
  Map<String,Object> result = new Map<String,Object>{'constructed'=>constructed.gladens003__Contact__r.gladens001__ParentRouteValue__c, 'queried'=>queried.gladens003__Contact__r.gladens001__ParentRouteValue__c, 'standard'=>queried.gladens003__Contact__r.LastName};
  try { List<SObject> rows = Database.query('SELECT gladens003__Contact__r.AbsentProbeField FROM gladens003__ParentRouteTwin__c'); result.put('rejection','accepted'); }
  catch(QueryException failure) { result.put('rejection',failure.getTypeName()); }
  gladens003__ParentRouteTwin__c empty = new gladens003__ParentRouteTwin__c(Name='Empty'); insert as system empty;
  gladens003__ParentRouteTwin__c boundary = [SELECT Id, gladens003__Contact__r.gladens001__ParentRouteValue__c FROM gladens003__ParentRouteTwin__c WHERE Id=:empty.Id WITH SYSTEM_MODE];
  result.put('nullParent',boundary.gladens003__Contact__r==null);
  return result;
 }
}
