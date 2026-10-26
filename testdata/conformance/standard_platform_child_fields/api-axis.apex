Schema.SObjectType t = Schema.getGlobalDescribe().get('ExtlRecShrStagedRecord');
if (t == null) { System.debug(LoggingLevel.ERROR, 'EXTL|unavailable'); }
else {
 for (String name : new List<String>{'AccountId','ParentRecordId'}) {
  Schema.SObjectField f = t.getDescribe().fields.getMap().get(name);
  if (f == null) { System.debug(LoggingLevel.ERROR, 'EXTL|'+name+'|absent'); continue; }
  Schema.DescribeFieldResult d = f.getDescribe();
  List<String> refs = new List<String>();
  for (Schema.SObjectType r : d.getReferenceTo()) { refs.add(String.valueOf(r)); }
  refs.sort();
  System.debug(LoggingLevel.ERROR, 'EXTL|'+name+'|'+d.getType()+'|'+String.join(refs,',')+'|'+d.isNillable()+'|'+d.isCreateable()+'|'+d.isUpdateable()+'|'+d.isDefaultedOnCreate()+'|'+d.getRelationshipName()+'|'+d.isAccessible());
 }
}
